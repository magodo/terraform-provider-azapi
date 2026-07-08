package tfconv

import (
	"math/big"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// -----------------------------------------------------------------------------
// Expand
// -----------------------------------------------------------------------------

func TestExpand_ObjectSnakeToCamel(t *testing.T) {
	t.Parallel()

	attrTypes := map[string]attr.Type{
		"user_name":  basetypes.StringType{},
		"item_count": basetypes.Int64Type{},
		"is_enabled": basetypes.BoolType{},
	}
	ov, _ := basetypes.NewObjectValue(attrTypes, map[string]attr.Value{
		"user_name":  basetypes.NewStringValue("alice"),
		"item_count": basetypes.NewInt64Value(3),
		"is_enabled": basetypes.NewBoolValue(true),
	})

	got, diags := Expand(t.Context(), ov, nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := map[string]any{
		"userName":  "alice",
		"itemCount": int64(3),
		"isEnabled": true,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestExpand_MapKeysNotTranslated(t *testing.T) {
	t.Parallel()

	attrTypes := map[string]attr.Type{
		"resource_tags": basetypes.MapType{ElemType: basetypes.StringType{}},
	}
	ov := basetypes.NewObjectValueMust(attrTypes, map[string]attr.Value{
		"resource_tags": basetypes.NewMapValueMust(basetypes.StringType{}, map[string]attr.Value{
			"env_stage":  basetypes.NewStringValue("prod"),
			"cost_owner": basetypes.NewStringValue("core"),
		})})

	got, diags := Expand(t.Context(), ov, nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := map[string]any{
		"resourceTags": map[string]any{
			"env_stage":  "prod", // NOT translated
			"cost_owner": "core", // NOT translated
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestExpand_NestedObjectTranslated(t *testing.T) {
	t.Parallel()

	inner := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"port_number": basetypes.Int64Type{},
		"proto_name":  basetypes.StringType{},
	}}
	outerAttrs := map[string]attr.Type{
		"listen_rules": basetypes.ListType{ElemType: inner},
	}
	ov := basetypes.NewObjectValueMust(outerAttrs, map[string]attr.Value{
		"listen_rules": basetypes.NewListValueMust(inner, []attr.Value{
			basetypes.NewObjectValueMust(inner.AttrTypes, map[string]attr.Value{
				"port_number": basetypes.NewInt64Value(22),
				"proto_name":  basetypes.NewStringValue("tcp"),
			}),
		}),
	})

	got, diags := Expand(t.Context(), ov, nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := map[string]any{
		"listenRules": []any{
			map[string]any{
				"portNumber": int64(22),
				"protoName":  "tcp",
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestExpand_WithOverrides(t *testing.T) {
	t.Parallel()

	attrTypes := map[string]attr.Type{
		"id":         basetypes.StringType{},
		"user_email": basetypes.StringType{},
	}
	ov := basetypes.NewObjectValueMust(attrTypes, map[string]attr.Value{
		"id":         basetypes.NewStringValue("r-1"),
		"user_email": basetypes.NewStringValue("a@b"),
	})

	got, diags := Expand(t.Context(), ov, &Option{
		NameMapper: NewSnakeCamelNameMapper(
			map[string]string{
				"id": "ID",
			},
		),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := map[string]any{
		"ID":        "r-1",
		"userEmail": "a@b",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

// -----------------------------------------------------------------------------
// Flatten
// -----------------------------------------------------------------------------

func TestFlatten_ObjectCamelToSnake(t *testing.T) {
	t.Parallel()

	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"user_name":  basetypes.StringType{},
		"item_count": basetypes.Int64Type{},
		"is_enabled": basetypes.BoolType{},
	}}
	apiResp := map[string]any{
		"userName":  "alice",
		"itemCount": float64(3),
		"isEnabled": true,
	}
	got, diags := Flatten(t.Context(), schemaType, apiResp, nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := basetypes.NewObjectValueMust(schemaType.AttrTypes, map[string]attr.Value{
		"user_name":  basetypes.NewStringValue("alice"),
		"item_count": basetypes.NewInt64Value(3),
		"is_enabled": basetypes.NewBoolValue(true),
	})
	if !got.Equal(want) {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestFlatten_MapKeysNotTranslated(t *testing.T) {
	t.Parallel()

	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"resource_tags": basetypes.MapType{ElemType: basetypes.StringType{}},
	}}
	apiResp := map[string]any{
		"resourceTags": map[string]any{
			"env_stage":  "prod",
			"cost_owner": "core",
		},
	}
	got, diags := Flatten(t.Context(), schemaType, apiResp, nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := basetypes.NewObjectValueMust(schemaType.AttrTypes, map[string]attr.Value{
		"resource_tags": basetypes.NewMapValueMust(basetypes.StringType{}, map[string]attr.Value{
			"env_stage":  basetypes.NewStringValue("prod"),
			"cost_owner": basetypes.NewStringValue("core"),
		}),
	})
	if !got.Equal(want) {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestFlatten_MissingCamelKeyBecomesNull(t *testing.T) {
	t.Parallel()

	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"present_field": basetypes.StringType{},
		"missing_field": basetypes.Int64Type{},
	}}
	// Only "presentField" in the payload; "missingField" absent.
	apiResp := map[string]any{"presentField": "hi"}
	got, diags := Flatten(t.Context(), schemaType, apiResp, nil)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := basetypes.NewObjectValueMust(schemaType.AttrTypes, map[string]attr.Value{
		"present_field": basetypes.NewStringValue("hi"),
		"missing_field": basetypes.NewInt64Null(),
	})
	if !got.Equal(want) {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestFlatten_WithOverrides(t *testing.T) {
	t.Parallel()

	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"id":         basetypes.StringType{},
		"user_email": basetypes.StringType{},
	}}
	apiResp := map[string]any{
		"ID":        "r-1",
		"userEmail": "a@b",
	}
	got, diags := Flatten(t.Context(), schemaType, apiResp, &Option{
		NameMapper: NewSnakeCamelNameMapper(
			map[string]string{
				"id": "ID",
			},
		),
	})
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := basetypes.NewObjectValueMust(schemaType.AttrTypes, map[string]attr.Value{
		"id":         basetypes.NewStringValue("r-1"),
		"user_email": basetypes.NewStringValue("a@b"),
	})
	if !got.Equal(want) {
		t.Fatalf("want %s, got %s", want, got)
	}
}

// -----------------------------------------------------------------------------
// Round-trip
// -----------------------------------------------------------------------------

func TestRoundTrip_WithNaming(t *testing.T) {
	t.Parallel()

	ruleType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"port_number": basetypes.Int64Type{},
		"proto_name":  basetypes.StringType{},
	}}
	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"id":          basetypes.StringType{},
		"is_enabled":  basetypes.BoolType{},
		"item_count":  basetypes.Int64Type{},
		"user_tags":   basetypes.MapType{ElemType: basetypes.StringType{}},
		"cidr_blocks": basetypes.SetType{ElemType: basetypes.StringType{}},
		"rule_list":   basetypes.ListType{ElemType: ruleType},
		"nested_object": basetypes.ObjectType{AttrTypes: map[string]attr.Type{
			"inner_key": basetypes.StringType{},
		}},
	}}
	opt := Option{NameMapper: NewSnakeCamelNameMapper(map[string]string{"id": "ID"})}

	apiResp := map[string]any{
		"ID":         "res-42",
		"isEnabled":  true,
		"itemCount":  float64(7),
		"userTags":   map[string]any{"env_stage": "prod"},
		"cidrBlocks": []any{"10.0.0.0/8"},
		"ruleList": []any{
			map[string]any{"portNumber": float64(22), "protoName": "tcp"},
		},
		"nestedObject": map[string]any{"innerKey": "hello"},
	}

	v, diags := Flatten(t.Context(), schemaType, apiResp, &opt)
	if diags.HasError() {
		t.Fatal(diags)
	}
	rt, diags := Expand(t.Context(), v, &opt)
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := map[string]any{
		"ID":         "res-42",
		"isEnabled":  true,
		"itemCount":  int64(7),
		"userTags":   map[string]any{"env_stage": "prod"}, // map key preserved
		"cidrBlocks": []any{"10.0.0.0/8"},
		"ruleList": []any{
			map[string]any{"portNumber": int64(22), "protoName": "tcp"},
		},
		"nestedObject": map[string]any{"innerKey": "hello"},
	}
	if diff := cmp.Diff(want, rt); diff != "" {
		t.Fatalf("round-trip mismatch (-want +got):\n%s", diff)
	}
}

func TestExpand_Primitives(t *testing.T) {
	t.Parallel()

	bigFloatEq := func(a, b *big.Float) bool {
		if a == nil || b == nil {
			return a == b
		}
		return a.Cmp(b) == 0
	}

	tests := map[string]struct {
		in   attr.Value
		want any
	}{
		"bool true":      {basetypes.NewBoolValue(true), true},
		"bool false":     {basetypes.NewBoolValue(false), false},
		"bool null":      {basetypes.NewBoolNull(), nil},
		"bool unknown":   {basetypes.NewBoolUnknown(), nil},
		"string":         {basetypes.NewStringValue("hello"), "hello"},
		"string empty":   {basetypes.NewStringValue(""), ""},
		"string null":    {basetypes.NewStringNull(), nil},
		"string unknown": {basetypes.NewStringUnknown(), nil},
		"int64":          {basetypes.NewInt64Value(42), int64(42)},
		"int64 null":     {basetypes.NewInt64Null(), nil},
		"int64 uknown":   {basetypes.NewInt64Unknown(), nil},
		"int32":          {basetypes.NewInt32Value(42), int32(42)},
		"float32":        {basetypes.NewFloat32Value(2.5), float32(2.5)},
		"float64":        {basetypes.NewFloat64Value(3.14), 3.14},
		"number":         {basetypes.NewNumberValue(big.NewFloat(1.5)), big.NewFloat(1.5)},
		"number null":    {basetypes.NewNumberNull(), nil},
		"nil attr.Value": {nil, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, diags := Expand(t.Context(), tc.in, nil)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if diff := cmp.Diff(tc.want, got, cmp.Comparer(bigFloatEq)); diff != "" {
				t.Fatalf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFlatten_Primitives(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		targetType attr.Type
		data       any
		want       attr.Value
	}{
		"bool":             {basetypes.BoolType{}, true, basetypes.NewBoolValue(true)},
		"bool null":        {basetypes.BoolType{}, nil, basetypes.NewBoolNull()},
		"string":           {basetypes.StringType{}, "hi", basetypes.NewStringValue("hi")},
		"int64 from float": {basetypes.Int64Type{}, float64(5), basetypes.NewInt64Value(5)},
		"int32":            {basetypes.Int32Type{}, float64(7), basetypes.NewInt32Value(7)},
		"float64":          {basetypes.Float64Type{}, 3.5, basetypes.NewFloat64Value(3.5)},
		"float32":          {basetypes.Float32Type{}, 2.5, basetypes.NewFloat32Value(2.5)},
		"number":           {basetypes.NumberType{}, 1.25, basetypes.NewNumberValue(big.NewFloat(1.25))},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, diags := Flatten(t.Context(), tc.targetType, tc.data, nil)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("want %s, got %s", tc.want, got)
			}
		})
	}
}
