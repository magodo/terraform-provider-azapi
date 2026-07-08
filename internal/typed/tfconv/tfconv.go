// Package tfconv provides schema-driven expand/flatten helpers that bridge
// terraform-plugin-framework attr.Value trees with untyped Go values
// (map[string]any / []any) — the shape most REST/JSON API clients speak.
package tfconv

import (
	"context"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// -----------------------------------------------------------------------------
// Expand: attr.Value  ->  Go native
// -----------------------------------------------------------------------------

// Expand converts a framework attr.Value tree into an untyped Go value
// suitable for JSON marshalling / SDK bodies.
//
// Note: Null and unknown values become nil atm.
// Note: Object attribute keys are translated through the supplied NameMapper, while Map
// values and Dynamic-inferred object keys are NEVER translated.
func Expand(ctx context.Context, v attr.Value, opt *Option) (any, diag.Diagnostics) {
	if opt == nil {
		opt = new(NewDefaultOption())
	}
	return expand(ctx, v, *opt, nil)
}

func expand(ctx context.Context, v attr.Value, opt Option, paths []string) (any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v == nil || v.IsNull() || v.IsUnknown() {
		return nil, nil
	}

	switch tv := v.(type) {
	case basetypes.DynamicValuable:
		dv, d := tv.ToDynamicValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		// Underlying is user data — bypass name translation.
		newOpt := Option{NameMapper: NoopNameMapper{}}
		return expand(ctx, dv.UnderlyingValue(), newOpt, paths)
	case basetypes.BoolValuable:
		bv, d := tv.ToBoolValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return bv.ValueBool(), diags
	case basetypes.StringValuable:
		sv, d := tv.ToStringValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return sv.ValueString(), diags
	case basetypes.Int64Valuable:
		iv, d := tv.ToInt64Value(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return iv.ValueInt64(), diags
	case basetypes.Int32Valuable:
		iv, d := tv.ToInt32Value(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return iv.ValueInt32(), diags
	case basetypes.Float64Valuable:
		fv, d := tv.ToFloat64Value(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return fv.ValueFloat64(), diags
	case basetypes.Float32Valuable:
		fv, d := tv.ToFloat32Value(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return fv.ValueFloat32(), diags
	case basetypes.NumberValuable:
		nv, d := tv.ToNumberValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return nv.ValueBigFloat(), diags
	case basetypes.ListValuable:
		lv, d := tv.ToListValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return expandSlice(ctx, lv.Elements(), opt, paths)
	case basetypes.SetValuable:
		sv, d := tv.ToSetValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return expandSlice(ctx, sv.Elements(), opt, paths)
	case basetypes.MapValuable:
		mv, d := tv.ToMapValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		// Map keys are user data — never translate.
		return expandStringMap(ctx, mv.Elements(), opt, paths, false)
	case basetypes.ObjectValuable:
		ov, d := tv.ToObjectValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return expandStringMap(ctx, ov.Attributes(), opt, paths, true)
	case basetypes.TupleValue:
		return expandSlice(ctx, tv.Elements(), opt, paths)
	}

	diags.AddError("Unsupported attr.Value in tfconv.Expand",
		fmt.Sprintf("value of Go type %T is not a recognised framework type", v))
	return nil, diags
}

func expandSlice(ctx context.Context, elems []attr.Value, opt Option, paths []string) ([]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]any, 0, len(elems))
	for _, e := range elems {
		newPaths := append(slices.Clone(paths), "*")
		raw, d := expand(ctx, e, opt, newPaths)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		if raw == nil && opt.ExpandSkipNull[strings.Join(newPaths, ".")] {
			continue
		}
		out = append(out, raw)
	}
	return out, diags
}

// expandStringMap writes children into a map. When isObj is true, keys
// are passed through NameMapper; otherwise they're used verbatim.
func expandStringMap(ctx context.Context, elems map[string]attr.Value, opt Option, paths []string, isObj bool) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make(map[string]any, len(elems))
	for k, e := range elems {
		var newPaths []string
		if isObj {
			newPaths = append(slices.Clone(paths), k)
		} else {
			newPaths = append(slices.Clone(paths), "*")
		}
		raw, d := expand(ctx, e, opt, newPaths)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}

		if raw == nil && opt.ExpandSkipNull[strings.Join(newPaths, ".")] {
			continue
		}

		outKey := k
		if isObj {
			outKey = opt.NameMapper.ToCamelCase(strings.Join(newPaths, "."))
		}
		out[outKey] = raw
	}
	return out, diags
}

// -----------------------------------------------------------------------------
// Flatten: (target attr.Type, Go native)  ->  attr.Value
// -----------------------------------------------------------------------------

// Flatten converts an untyped Go value into a framework attr.Value matching
// targetType. Object attribute keys in `data` are looked up by translating
// the schema (TF) name through the NameMapper. Map values and DynamicType-inferred
// object keys are used verbatim.
func Flatten(ctx context.Context, targetType attr.Type, data any, opt *Option) (attr.Value, diag.Diagnostics) {
	if opt == nil {
		opt = new(NewDefaultOption())
	}
	return flatten(ctx, targetType, data, *opt, nil)
}

func flatten(ctx context.Context, targetType attr.Type, data any, opt Option, paths []string) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	if data == nil {
		return nullValue(ctx, targetType)
	}

	switch t := targetType.(type) {
	case basetypes.DynamicTypable:
		underlying, d := inferDynamic(ctx, data)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		v, d := t.ValueFromDynamic(ctx, basetypes.NewDynamicValue(underlying))
		diags.Append(d...)
		return v, diags
	case basetypes.BoolTypable:
		v, ok := data.(bool)
		if !ok {
			return nil, typeCastError("bool", data)
		}
		tv, d := t.ValueFromBool(ctx, basetypes.NewBoolValue(v))
		diags.Append(d...)
		return tv, diags
	case basetypes.StringTypable:
		v, ok := data.(string)
		if !ok {
			return nil, typeCastError("string", data)
		}
		tv, d := t.ValueFromString(ctx, basetypes.NewStringValue(v))
		diags.Append(d...)
		return tv, diags
	case basetypes.Int64Typable:
		v, ok := data.(float64)
		if !ok {
			return nil, typeCastError("float64", data)
		}
		tv, d := t.ValueFromInt64(ctx, basetypes.NewInt64Value(int64(v)))
		diags.Append(d...)
		return tv, diags
	case basetypes.Int32Typable:
		v, ok := data.(float64)
		if !ok {
			return nil, typeCastError("float64", data)
		}
		tv, d := t.ValueFromInt32(ctx, basetypes.NewInt32Value(int32(v)))
		diags.Append(d...)
		return tv, diags
	case basetypes.Float64Typable:
		v, ok := data.(float64)
		if !ok {
			return nil, typeCastError("float64", data)
		}
		tv, d := t.ValueFromFloat64(ctx, basetypes.NewFloat64Value(v))
		diags.Append(d...)
		return tv, diags
	case basetypes.Float32Typable:
		v, ok := data.(float64)
		if !ok {
			return nil, typeCastError("float64", data)
		}
		tv, d := t.ValueFromFloat32(ctx, basetypes.NewFloat32Value(float32(v)))
		diags.Append(d...)
		return tv, diags
	case basetypes.NumberTypable:
		v, ok := data.(float64)
		if !ok {
			return nil, typeCastError("float64", data)
		}
		tv, d := t.ValueFromNumber(ctx, basetypes.NewNumberValue(big.NewFloat(v)))
		diags.Append(d...)
		return tv, diags
	case basetypes.ObjectTypable:
		return flattenObject(ctx, t, data, opt, paths)
	case basetypes.MapTypable:
		return flattenMap(ctx, t, data, opt, paths)
	case basetypes.SetTypable:
		return flattenSet(ctx, t, data, opt, paths)
	case basetypes.ListTypable:
		return flattenList(ctx, t, data, opt, paths)
	case basetypes.TupleType:
		return flattenTuple(ctx, t, data, opt, paths)
	}

	diags.AddError("Unsupported attr.Type in tfconv.Flatten",
		fmt.Sprintf("target type of Go type %T is not a recognised framework type", targetType))
	return nil, diags
}

func flattenObject(ctx context.Context, t basetypes.ObjectTypable, data any, opt Option, paths []string) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	m, ok := data.(map[string]any)
	if !ok {
		return nil, typeCastError("map[string]any", data)
	}
	withAttrs, ok := t.(attr.TypeWithAttributeTypes)
	if !ok {
		diags.AddError("Unsupported ObjectTypable",
			fmt.Sprintf("%T does not implement attr.TypeWithAttributeTypes", t))
		return nil, diags
	}
	attrTypes := withAttrs.AttributeTypes()
	attrs := make(map[string]attr.Value, len(attrTypes))
	for attrName, attrType := range attrTypes {
		newPaths := append(slices.Clone(paths), attrName)
		apiName := opt.NameMapper.ToCamelCase(strings.Join(newPaths, "."))
		child, d := flatten(ctx, attrType, m[apiName], opt, newPaths)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		attrs[attrName] = child
	}
	obj, d := basetypes.NewObjectValue(attrTypes, attrs)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	v, d := t.ValueFromObject(ctx, obj)
	diags.Append(d...)
	return v, diags
}

func flattenMap(ctx context.Context, t basetypes.MapTypable, data any, opt Option, paths []string) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	m, ok := data.(map[string]any)
	if !ok {
		return nil, typeCastError("map[string]any", data)
	}

	withElements, ok := t.(attr.TypeWithElementType)
	if !ok {
		diags.AddError("Unsupported MapTypable",
			fmt.Sprintf("%T does not implement attr.TypeWithElementType", t))
		return nil, diags
	}
	et := withElements.ElementType()

	elems := make(map[string]attr.Value, len(m))
	for k, raw := range m {
		newPaths := append(slices.Clone(paths), "*")
		child, d := flatten(ctx, et, raw, opt, newPaths)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		elems[k] = child
	}
	mv, d := basetypes.NewMapValue(et, elems)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	v, d := t.ValueFromMap(ctx, mv)
	diags.Append(d...)
	return v, diags
}

func flattenSet(ctx context.Context, t basetypes.SetTypable, data any, opt Option, paths []string) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	s, ok := data.([]any)
	if !ok {
		return nil, typeCastError("[]any", data)
	}

	withElements, ok := t.(attr.TypeWithElementType)
	if !ok {
		diags.AddError("Unsupported SetTypable",
			fmt.Sprintf("%T does not implement attr.TypeWithElementType", t))
		return nil, diags
	}
	et := withElements.ElementType()

	elems := make([]attr.Value, 0, len(s))
	for _, raw := range s {
		newPaths := append(slices.Clone(paths), "*")
		child, d := flatten(ctx, et, raw, opt, newPaths)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		elems = append(elems, child)
	}
	sv, d := basetypes.NewSetValue(et, elems)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	v, d := t.ValueFromSet(ctx, sv)
	diags.Append(d...)
	return v, diags
}

func flattenList(ctx context.Context, t basetypes.ListTypable, data any, opt Option, paths []string) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	s, ok := data.([]any)
	if !ok {
		return nil, typeCastError("[]any", data)
	}
	withElements, ok := t.(attr.TypeWithElementType)
	if !ok {
		diags.AddError("Unsupported ListTypable",
			fmt.Sprintf("%T does not implement attr.TypeWithElementType", t))
		return nil, diags
	}
	et := withElements.ElementType()
	elems := make([]attr.Value, 0, len(s))
	for _, raw := range s {
		newPaths := append(slices.Clone(paths), "*")
		child, d := flatten(ctx, et, raw, opt, newPaths)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		elems = append(elems, child)
	}
	lv, d := basetypes.NewListValue(et, elems)
	diags.Append(d...)
	if diags.HasError() {
		return nil, diags
	}
	v, d := t.ValueFromList(ctx, lv)
	diags.Append(d...)
	return v, diags
}

func flattenTuple(ctx context.Context, t basetypes.TupleType, data any, opt Option, paths []string) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	s, ok := data.([]any)
	if !ok {
		return nil, typeCastError("[]any", data)
	}
	if len(s) != len(t.ElemTypes) {
		diags.AddError("Tuple arity mismatch",
			fmt.Sprintf("tuple expects %d elements, got %d", len(t.ElemTypes), len(s)))
		return nil, diags
	}
	elems := make([]attr.Value, len(s))
	for i, raw := range s {
		newPaths := append(slices.Clone(paths), "*")
		child, d := flatten(ctx, t.ElemTypes[i], raw, opt, newPaths)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		elems[i] = child
	}
	tv, d := basetypes.NewTupleValue(t.ElemTypes, elems)
	diags.Append(d...)
	return tv, diags
}

// -----------------------------------------------------------------------------
// DynamicType inference
// -----------------------------------------------------------------------------

func inferDynamic(ctx context.Context, data any) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	switch v := data.(type) {
	case nil:
		return basetypes.NewDynamicNull(), diags
	case bool:
		return basetypes.NewBoolValue(v), diags
	case string:
		return basetypes.NewStringValue(v), diags
	case float64:
		return basetypes.NewFloat64Value(v), diags
	case []any:
		elems := make([]attr.Value, len(v))
		types := make([]attr.Type, len(v))
		for i, e := range v {
			c, d := inferDynamic(ctx, e)
			diags.Append(d...)
			if diags.HasError() {
				return nil, diags
			}
			elems[i] = c
			types[i] = c.Type(ctx)
		}
		tv, d := basetypes.NewTupleValue(types, elems)
		diags.Append(d...)
		return tv, diags
	case map[string]any:
		attrs := make(map[string]attr.Value, len(v))
		types := make(map[string]attr.Type, len(v))
		for k, e := range v {
			c, d := inferDynamic(ctx, e)
			diags.Append(d...)
			if diags.HasError() {
				return nil, diags
			}
			attrs[k] = c
			types[k] = c.Type(ctx)
		}
		ov, d := basetypes.NewObjectValue(types, attrs)
		diags.Append(d...)
		return ov, diags
	}
	diags.AddError("Cannot infer dynamic type",
		fmt.Sprintf("Go type %T not supported for DynamicType inference", data))
	return nil, diags
}

func typeCastError(expect string, actual any) diag.Diagnostics {
	var d diag.Diagnostics
	d.AddError("tfconv.Flatten type mismatch",
		fmt.Sprintf("expect=%s get=%T", expect, actual))
	return d
}

func nullValue(ctx context.Context, t attr.Type) (attr.Value, diag.Diagnostics) {
	var d diag.Diagnostics
	v, err := t.ValueFromTerraform(ctx, tftypes.NewValue(t.TerraformType(ctx), nil))
	if err != nil {
		d.AddError("tfconv.Flatten null value", err.Error())
		return nil, d
	}
	return v, d
}

// -----------------------------------------------------------------------------
// State / Plan / Config bridging
// -----------------------------------------------------------------------------

// ObjectFromRaw converts the raw tftypes.Value of a state/plan/config into
// an ObjectValue whose AttrTypes match schemaType.
func ObjectFromRaw(ctx context.Context, schemaType attr.Type, raw tftypes.Value) (basetypes.ObjectValue, diag.Diagnostics) {
	var diags diag.Diagnostics
	v, err := schemaType.ValueFromTerraform(ctx, raw)
	if err != nil {
		diags.AddError("tfconv.ObjectFromRaw", err.Error())
		return basetypes.ObjectValue{}, diags
	}
	valuable, ok := v.(basetypes.ObjectValuable)
	if !ok {
		diags.AddError("tfconv.ObjectFromRaw",
			fmt.Sprintf("expected ObjectValuable, got %T", v))
		return basetypes.ObjectValue{}, diags
	}
	ov, d := valuable.ToObjectValue(ctx)
	diags.Append(d...)
	return ov, diags
}
