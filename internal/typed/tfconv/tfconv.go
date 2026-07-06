// Package tfconv provides schema-driven expand/flatten helpers that bridge
// terraform-plugin-framework attr.Value trees with untyped Go values
// (map[string]any / []any) — the shape most REST/JSON API clients speak.
//
// Terraform schema attributes are snake_case by convention; JSON APIs
// typically use camelCase. tfconv translates Object attribute keys through
// a pluggable NameMapper (default: SnakeCamelMapper). Map keys and
// DynamicType-inferred object keys are NEVER translated — those are user
// data, not schema names.
package tfconv

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// -----------------------------------------------------------------------------
// Expand: attr.Value  ->  Go native
// -----------------------------------------------------------------------------

// Expand converts a framework attr.Value tree into an untyped Go value
// suitable for JSON marshalling / SDK bodies. Null and unknown values become
// nil. Object attribute keys are translated through the supplied NameMapper
// (default: identity). Map values and Dynamic-inferred object keys are
// NEVER translated.
func Expand(ctx context.Context, v attr.Value, opt *Option) (any, diag.Diagnostics) {
	if opt == nil {
		opt = new(NewDefaultOption())
	}
	return expand(ctx, v, *opt)
}

func expand(ctx context.Context, v attr.Value, opt Option) (any, diag.Diagnostics) {
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
		return expandUntranslated(ctx, dv.UnderlyingValue(), opt)
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
		return expandSlice(ctx, lv.Elements(), opt)
	case basetypes.SetValuable:
		sv, d := tv.ToSetValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return expandSlice(ctx, sv.Elements(), opt)
	case basetypes.MapValuable:
		mv, d := tv.ToMapValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		// Map keys are user data — never translate.
		return expandStringMap(ctx, mv.Elements(), opt, false)
	case basetypes.ObjectValuable:
		ov, d := tv.ToObjectValue(ctx)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		return expandStringMap(ctx, ov.Attributes(), opt, true)
	case basetypes.TupleValue:
		return expandSlice(ctx, tv.Elements(), opt)
	}

	diags.AddError("Unsupported attr.Value in tfconv.Expand",
		fmt.Sprintf("value of Go type %T is not a recognised framework type", v))
	return nil, diags
}

// expandUntranslated forces the sub-tree to skip name translation. Used for
// Dynamic underlying values (which have no schema).
func expandUntranslated(ctx context.Context, v attr.Value, opt Option) (any, diag.Diagnostics) {
	sub := &config{mapper: identityMapper{}}
	_ = opt // preserved for future use if we need path-scoped mapping
	return expand(ctx, v, sub)
}

func expandSlice(ctx context.Context, elems []attr.Value, opt Option) ([]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]any, 0, len(elems))
	for _, e := range elems {
		raw, d := expand(ctx, e, opt)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		out = append(out, raw)
	}
	return out, diags
}

// expandStringMap writes children into a map. When translate is true, keys
// are passed through cfg.mapper.ToAPI; otherwise they're used verbatim.
func expandStringMap(ctx context.Context, elems map[string]attr.Value, opt Option, translate bool) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make(map[string]any, len(elems))
	for k, e := range elems {
		raw, d := expand(ctx, e, opt)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		outKey := k
		if translate {
			outKey = opt.NameMapper.ToCamelCase(k)
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
// the schema (TF) name through the NameMapper's ToAPI. Map values and
// DynamicType-inferred object keys are used verbatim.
func Flatten(ctx context.Context, targetType attr.Type, data any, opt *Option) (attr.Value, diag.Diagnostics) {
	if opt == nil {
		opt = new(NewDefaultOption())
	}
	return flatten(ctx, targetType, data, *opt)
}

func flatten(ctx context.Context, targetType attr.Type, data any, opt Option) (attr.Value, diag.Diagnostics) {
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
		b, err := coerceBool(data)
		if err != nil {
			return nil, coerceErr("bool", data, err)
		}
		v, d := t.ValueFromBool(ctx, basetypes.NewBoolValue(b))
		diags.Append(d...)
		return v, diags
	case basetypes.StringTypable:
		s, err := coerceString(data)
		if err != nil {
			return nil, coerceErr("string", data, err)
		}
		v, d := t.ValueFromString(ctx, basetypes.NewStringValue(s))
		diags.Append(d...)
		return v, diags
	case basetypes.Int64Typable:
		i, err := coerceInt64(data)
		if err != nil {
			return nil, coerceErr("int64", data, err)
		}
		v, d := t.ValueFromInt64(ctx, basetypes.NewInt64Value(i))
		diags.Append(d...)
		return v, diags
	case basetypes.Int32Typable:
		i, err := coerceInt64(data)
		if err != nil {
			return nil, coerceErr("int32", data, err)
		}
		if i > (1<<31)-1 || i < -(1<<31) {
			return nil, coerceErr("int32", data, fmt.Errorf("value %d overflows int32", i))
		}
		v, d := t.ValueFromInt32(ctx, basetypes.NewInt32Value(int32(i)))
		diags.Append(d...)
		return v, diags
	case basetypes.Float64Typable:
		f, err := coerceFloat64(data)
		if err != nil {
			return nil, coerceErr("float64", data, err)
		}
		v, d := t.ValueFromFloat64(ctx, basetypes.NewFloat64Value(f))
		diags.Append(d...)
		return v, diags
	case basetypes.Float32Typable:
		f, err := coerceFloat64(data)
		if err != nil {
			return nil, coerceErr("float32", data, err)
		}
		v, d := t.ValueFromFloat32(ctx, basetypes.NewFloat32Value(float32(f)))
		diags.Append(d...)
		return v, diags
	case basetypes.NumberTypable:
		bf, err := coerceBigFloat(data)
		if err != nil {
			return nil, coerceErr("number", data, err)
		}
		v, d := t.ValueFromNumber(ctx, basetypes.NewNumberValue(bf))
		diags.Append(d...)
		return v, diags
	case basetypes.ObjectTypable:
		return flattenObject(ctx, t, data, opt)
	case basetypes.MapTypable:
		return flattenMap(ctx, t, data, opt)
	case basetypes.SetTypable:
		return flattenSet(ctx, t, data, opt)
	case basetypes.ListTypable:
		return flattenList(ctx, t, data, opt)
	case basetypes.TupleType:
		return flattenTuple(ctx, t, data, opt)
	}

	diags.AddError("Unsupported attr.Type in tfconv.Flatten",
		fmt.Sprintf("target type of Go type %T is not a recognised framework type", targetType))
	return nil, diags
}

func flattenObject(ctx context.Context, t basetypes.ObjectTypable, data any, opt Option) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	m, ok := data.(map[string]any)
	if !ok {
		return nil, coerceErr("object", data, fmt.Errorf("expected map[string]any"))
	}
	withAttrs, ok := any(t).(attr.TypeWithAttributeTypes)
	if !ok {
		diags.AddError("Unsupported ObjectTypable",
			fmt.Sprintf("%T does not implement attr.TypeWithAttributeTypes", t))
		return nil, diags
	}
	attrTypes := withAttrs.AttributeTypes()
	attrs := make(map[string]attr.Value, len(attrTypes))
	for tfName, at := range attrTypes {
		apiName := opt.NameMapper.ToCamelCase(tfName)
		child, d := flatten(ctx, at, m[apiName], opt)
		diags.Append(d...)
		if diags.HasError() {
			return nil, diags
		}
		attrs[tfName] = child
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

func flattenMap(ctx context.Context, t basetypes.MapTypable, data any, opt Option) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	m, ok := data.(map[string]any)
	if !ok {
		return nil, coerceErr("map", data, fmt.Errorf("expected map[string]any"))
	}
	et := any(t).(attr.TypeWithElementType).ElementType()
	elems := make(map[string]attr.Value, len(m))
	for k, raw := range m {
		// Map keys are user data — not translated.
		child, d := flatten(ctx, et, raw, opt)
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

func flattenSet(ctx context.Context, t basetypes.SetTypable, data any, opt Option) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	s, ok := data.([]any)
	if !ok {
		return nil, coerceErr("set", data, fmt.Errorf("expected []any"))
	}
	et := any(t).(attr.TypeWithElementType).ElementType()
	elems := make([]attr.Value, 0, len(s))
	for _, raw := range s {
		child, d := flatten(ctx, et, raw, opt)
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

func flattenList(ctx context.Context, t basetypes.ListTypable, data any, opt Option) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	s, ok := data.([]any)
	if !ok {
		return nil, coerceErr("list", data, fmt.Errorf("expected []any"))
	}
	et := any(t).(attr.TypeWithElementType).ElementType()
	elems := make([]attr.Value, 0, len(s))
	for _, raw := range s {
		child, d := flatten(ctx, et, raw, opt)
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

func flattenTuple(ctx context.Context, t basetypes.TupleType, data any, opt Option) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics
	s, ok := data.([]any)
	if !ok {
		return nil, coerceErr("tuple", data, fmt.Errorf("expected []any"))
	}
	if len(s) != len(t.ElemTypes) {
		diags.AddError("Tuple arity mismatch",
			fmt.Sprintf("tuple expects %d elements, got %d", len(t.ElemTypes), len(s)))
		return nil, diags
	}
	elems := make([]attr.Value, len(s))
	for i, raw := range s {
		child, d := flatten(ctx, t.ElemTypes[i], raw, opt)
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
	case int:
		return basetypes.NewInt64Value(int64(v)), diags
	case int32:
		return basetypes.NewInt32Value(v), diags
	case int64:
		return basetypes.NewInt64Value(v), diags
	case float32:
		return basetypes.NewFloat32Value(v), diags
	case float64:
		return basetypes.NewFloat64Value(v), diags
	case *big.Float:
		return basetypes.NewNumberValue(v), diags
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return basetypes.NewInt64Value(i), diags
		}
		if f, err := v.Float64(); err == nil {
			return basetypes.NewFloat64Value(f), diags
		}
		bf, _, err := big.ParseFloat(string(v), 10, 512, big.ToNearestEven)
		if err != nil {
			diags.AddError("Invalid number", err.Error())
			return nil, diags
		}
		return basetypes.NewNumberValue(bf), diags
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

// -----------------------------------------------------------------------------
// Scalar coercion helpers
// -----------------------------------------------------------------------------

func coerceBool(v any) (bool, error) {
	switch x := v.(type) {
	case bool:
		return x, nil
	case string:
		return strconv.ParseBool(x)
	}
	return false, fmt.Errorf("cannot convert %T to bool", v)
}
func coerceString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case fmt.Stringer:
		return x.String(), nil
	case []byte:
		return string(x), nil
	}
	return "", fmt.Errorf("cannot convert %T to string", v)
}
func coerceInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int8:
		return int64(x), nil
	case int16:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case uint:
		return int64(x), nil
	case uint8:
		return int64(x), nil
	case uint16:
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case uint64:
		if x > (1<<63)-1 {
			return 0, fmt.Errorf("uint64 %d overflows int64", x)
		}
		return int64(x), nil
	case float32:
		return int64(x), nil
	case float64:
		return int64(x), nil
	case json.Number:
		return x.Int64()
	case string:
		return strconv.ParseInt(x, 10, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to int64", v)
}
func coerceFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float32:
		return float64(x), nil
	case float64:
		return x, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		i, _ := coerceInt64(x)
		return float64(i), nil
	case json.Number:
		return x.Float64()
	case string:
		return strconv.ParseFloat(x, 64)
	}
	return 0, fmt.Errorf("cannot convert %T to float64", v)
}
func coerceBigFloat(v any) (*big.Float, error) {
	switch x := v.(type) {
	case *big.Float:
		return x, nil
	case big.Float:
		return &x, nil
	case json.Number:
		bf, _, err := big.ParseFloat(string(x), 10, 512, big.ToNearestEven)
		return bf, err
	case string:
		bf, _, err := big.ParseFloat(x, 10, 512, big.ToNearestEven)
		return bf, err
	}
	if f, err := coerceFloat64(v); err == nil {
		return big.NewFloat(f), nil
	}
	if i, err := coerceInt64(v); err == nil {
		return new(big.Float).SetInt64(i), nil
	}
	return nil, fmt.Errorf("cannot convert %T to *big.Float", v)
}
func coerceErr(kind string, data any, err error) diag.Diagnostics {
	var d diag.Diagnostics
	d.AddError("tfconv.Flatten type mismatch",
		fmt.Sprintf("cannot coerce Go %T into framework %s: %s", data, kind, err))
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

// SetObjectIntoState writes an ObjectValue back into a State's Raw.
func SetObjectIntoState(ctx context.Context, state *tfsdk.State, obj basetypes.ObjectValue) diag.Diagnostics {
	var diags diag.Diagnostics
	raw, err := obj.ToTerraformValue(ctx)
	if err != nil {
		diags.AddError("tfconv.SetObjectIntoState", err.Error())
		return diags
	}
	state.Raw = raw
	return diags
}
