package tfconv

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// -----------------------------------------------------------------------------
// Naming convention helpers
// -----------------------------------------------------------------------------

func TestSnakeToCamel(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"":                 "",
		"id":               "id",
		"some_field":       "someField",
		"some_field_name":  "someFieldName",
		"a_b_c":            "aBC",
		"http_url":         "httpUrl",
		"trailing_":        "trailing_",
		"_leading":         "_leading",
		"already_snake_ok": "alreadySnakeOk",
	}
	for in, want := range tests {
		if got := SnakeToCamel(in); got != want {
			t.Errorf("SnakeToCamel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamelToSnake(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"":            "",
		"id":          "id",
		"someField":   "some_field",
		"SomeField":   "some_field",
		"HTTPServer":  "http_server",
		"URL":         "url",
		"userID":      "user_id",
		"parseXMLDoc": "parse_xml_doc",
		"a2b":         "a2b",
	}
	for in, want := range tests {
		if got := CamelToSnake(in); got != want {
			t.Errorf("CamelToSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnakeCamelMapper_Overrides(t *testing.T) {
	t.Parallel()
	m := &SnakeCamelMapper{Overrides: map[string]string{
		"id":         "ID",           // acronym exception
		"custom_key": "totally_diff", // arbitrary mapping
	}}
	if got := m.ToAPI("id"); got != "ID" {
		t.Errorf("ToAPI id = %q", got)
	}
	if got := m.ToAPI("some_field"); got != "someField" {
		t.Errorf("ToAPI some_field = %q", got)
	}
	if got := m.ToTF("ID"); got != "id" {
		t.Errorf("ToTF ID = %q", got)
	}
	if got := m.ToTF("someField"); got != "some_field" {
		t.Errorf("ToTF someField = %q", got)
	}
	if got := m.ToTF("totally_diff"); got != "custom_key" {
		t.Errorf("ToTF totally_diff = %q", got)
	}
}

// -----------------------------------------------------------------------------
// Expand with naming
// -----------------------------------------------------------------------------

func TestExpand_ObjectSnakeToCamel(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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

	got, diags := Expand(ctx, ov, WithSnakeCamel())
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
	ctx := context.Background()

	// Object containing a map whose keys are user data (tags with
	// snake_case_style keys that must NOT be camel-cased).
	attrTypes := map[string]attr.Type{
		"resource_tags": basetypes.MapType{ElemType: basetypes.StringType{}},
	}
	mv, _ := basetypes.NewMapValue(basetypes.StringType{}, map[string]attr.Value{
		"env_stage":  basetypes.NewStringValue("prod"),
		"cost_owner": basetypes.NewStringValue("core"),
	})
	ov, _ := basetypes.NewObjectValue(attrTypes, map[string]attr.Value{"resource_tags": mv})

	got, diags := Expand(ctx, ov, WithSnakeCamel())
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := map[string]any{
		"resourceTags": map[string]any{
			"env_stage":  "prod",  // NOT translated
			"cost_owner": "core",  // NOT translated
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func TestExpand_NestedObjectTranslated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	inner := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"port_number": basetypes.Int64Type{},
		"proto_name":  basetypes.StringType{},
	}}
	outerAttrs := map[string]attr.Type{
		"listen_rules": basetypes.ListType{ElemType: inner},
	}
	r1, _ := basetypes.NewObjectValue(inner.AttrTypes, map[string]attr.Value{
		"port_number": basetypes.NewInt64Value(22),
		"proto_name":  basetypes.NewStringValue("tcp"),
	})
	lv, _ := basetypes.NewListValue(inner, []attr.Value{r1})
	ov, _ := basetypes.NewObjectValue(outerAttrs, map[string]attr.Value{
		"listen_rules": lv,
	})

	got, diags := Expand(ctx, ov, WithSnakeCamel())
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
	ctx := context.Background()

	attrTypes := map[string]attr.Type{
		"id":         basetypes.StringType{},
		"user_email": basetypes.StringType{},
	}
	ov, _ := basetypes.NewObjectValue(attrTypes, map[string]attr.Value{
		"id":         basetypes.NewStringValue("r-1"),
		"user_email": basetypes.NewStringValue("a@b"),
	})

	got, diags := Expand(ctx, ov, WithSnakeCamel(map[string]string{
		"id": "ID", // API uses uppercase ID
	}))
	if diags.HasError() {
		t.Fatal(diags)
	}
	want := map[string]any{"ID": "r-1", "userEmail": "a@b"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

// -----------------------------------------------------------------------------
// Flatten with naming
// -----------------------------------------------------------------------------

func TestFlatten_ObjectCamelToSnake(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
	got, diags := Flatten(ctx, schemaType, apiResp, WithSnakeCamel())
	if diags.HasError() {
		t.Fatal(diags)
	}
	want, _ := basetypes.NewObjectValue(schemaType.AttrTypes, map[string]attr.Value{
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
	ctx := context.Background()

	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"resource_tags": basetypes.MapType{ElemType: basetypes.StringType{}},
	}}
	apiResp := map[string]any{
		"resourceTags": map[string]any{
			"env_stage":  "prod",
			"cost_owner": "core",
		},
	}
	got, diags := Flatten(ctx, schemaType, apiResp, WithSnakeCamel())
	if diags.HasError() {
		t.Fatal(diags)
	}
	tagsVal, _ := basetypes.NewMapValue(basetypes.StringType{}, map[string]attr.Value{
		"env_stage":  basetypes.NewStringValue("prod"),
		"cost_owner": basetypes.NewStringValue("core"),
	})
	want, _ := basetypes.NewObjectValue(schemaType.AttrTypes, map[string]attr.Value{
		"resource_tags": tagsVal,
	})
	if !got.Equal(want) {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestFlatten_MissingCamelKeyBecomesNull(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"present_field": basetypes.StringType{},
		"missing_field": basetypes.Int64Type{},
	}}
	// Only "presentField" in the payload; "missingField" absent.
	apiResp := map[string]any{"presentField": "hi"}
	got, diags := Flatten(ctx, schemaType, apiResp, WithSnakeCamel())
	if diags.HasError() {
		t.Fatal(diags)
	}
	want, _ := basetypes.NewObjectValue(schemaType.AttrTypes, map[string]attr.Value{
		"present_field": basetypes.NewStringValue("hi"),
		"missing_field": basetypes.NewInt64Null(),
	})
	if !got.Equal(want) {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestFlatten_WithOverrides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	schemaType := basetypes.ObjectType{AttrTypes: map[string]attr.Type{
		"id":         basetypes.StringType{},
		"user_email": basetypes.StringType{},
	}}
	apiResp := map[string]any{"ID": "r-1", "userEmail": "a@b"}
	got, diags := Flatten(ctx, schemaType, apiResp, WithSnakeCamel(map[string]string{
		"id": "ID",
	}))
	if diags.HasError() {
		t.Fatal(diags)
	}
	want, _ := basetypes.NewObjectValue(schemaType.AttrTypes, map[string]attr.Value{
		"id":         basetypes.NewStringValue("r-1"),
		"user_email": basetypes.NewStringValue("a@b"),
	})
	if !got.Equal(want) {
		t.Fatalf("want %s, got %s", want, got)
	}
}

// -----------------------------------------------------------------------------
// Round-trip with naming
// -----------------------------------------------------------------------------

func TestRoundTrip_WithNaming(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
	opt := WithSnakeCamel(map[string]string{"id": "ID"})

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

	v, diags := Flatten(ctx, schemaType, apiResp, opt)
	if diags.HasError() {
		t.Fatal(diags)
	}
	rt, diags := Expand(ctx, v, opt)
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

// -----------------------------------------------------------------------------
// The original (identity) tests — everything below should keep working
// unchanged because WithNameMapper defaults to identity.
// -----------------------------------------------------------------------------

func TestExpand_Primitives(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

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
		"int64":          {basetypes.NewInt64Value(42), int64(42)},
		"int64 negative": {basetypes.NewInt64Value(-7), int64(-7)},
		"int64 null":     {basetypes.NewInt64Null(), nil},
		"int32":          {basetypes.NewInt32Value(42), int32(42)},
		"float64":        {basetypes.NewFloat64Value(3.14), 3.14},
		"float32":        {basetypes.NewFloat32Value(2.5), float32(2.5)},
		"number":         {basetypes.NewNumberValue(big.NewFloat(1.5)), big.NewFloat(1.5)},
		"number null":    {basetypes.NewNumberNull(), nil},
		"nil attr.Value": {nil, nil},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, diags := Expand(ctx, tc.in)
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
	ctx := context.Background()

	tests := map[string]struct {
		targetType attr.Type
		data       any
		want       attr.Value
	}{
		"bool":              {basetypes.BoolType{}, true, basetypes.NewBoolValue(true)},
		"bool from string":  {basetypes.BoolType{}, "true", basetypes.NewBoolValue(true)},
		"bool null":         {basetypes.BoolType{}, nil, basetypes.NewBoolNull()},
		"string":            {basetypes.StringType{}, "hi", basetypes.NewStringValue("hi")},
		"int64 from float":  {basetypes.Int64Type{}, float64(5), basetypes.NewInt64Value(5)},
		"int64 from jsonN":  {basetypes.Int64Type{}, json.Number("5"), basetypes.NewInt64Value(5)},
		"int32":             {basetypes.Int32Type{}, float64(7), basetypes.NewInt32Value(7)},
		"float64":           {basetypes.Float64Type{}, 3.5, basetypes.NewFloat64Value(3.5)},
		"float32":           {basetypes.Float32Type{}, 2.5, basetypes.NewFloat32Value(2.5)},
		"number":            {basetypes.NumberType{}, 1.25, basetypes.NewNumberValue(big.NewFloat(1.25))},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, diags := Flatten(ctx, tc.targetType, tc.data)
			if diags.HasError() {
				t.Fatalf("unexpected diags: %v", diags)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("want %s, got %s", tc.want, got)
			}
		})
	}
}

func TestFlatten_TypeMismatchError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if _, d := Flatten(ctx, basetypes.BoolType{}, 42); !d.HasError() {
		t.Fatal("expected bool error")
	}
	target := basetypes.ObjectType{AttrTypes: map[string]attr.Type{"x": basetypes.StringType{}}}
	if _, d := Flatten(ctx, target, []any{"nope"}); !d.HasError() {
		t.Fatal("expected object error")
	}
}

func TestFlatten_Int32Overflow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if _, d := Flatten(ctx, basetypes.Int32Type{}, int64(1)<<40); !d.HasError() {
		t.Fatal("expected int32 overflow error")
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func bigFloatEq(a, b *big.Float) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Cmp(b) == 0
}
