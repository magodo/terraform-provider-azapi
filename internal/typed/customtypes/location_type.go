package customtypes

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// LocationType is a custom string type that represents the location (region) of
// an Azure resource, e.g. "westus" or "West US".
type LocationType struct {
	basetypes.StringType
}

var _ basetypes.StringTypable = LocationType{}

func (t LocationType) String() string {
	return "customtypes.LocationType"
}

func (t LocationType) ValueType(ctx context.Context) attr.Value {
	return LocationValue{}
}

func (t LocationType) Equal(o attr.Type) bool {
	other, ok := o.(LocationType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t LocationType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	return LocationValue{StringValue: in}, nil
}

func (t LocationType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type %T", attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable, nil
}
