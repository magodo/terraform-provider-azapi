package customtypes

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/azure/location"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/attr/xattr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// LocationValue is the value type of LocationType. It behaves like an ordinary
// string value, but implements semantic equality so that two locations that
// only differ in casing or spacing are considered equal (e.g. "West US" and
// "westus").
type LocationValue struct {
	basetypes.StringValue
}

var (
	_ basetypes.StringValuableWithSemanticEquals = LocationValue{}
	_ xattr.ValidateableAttribute                = LocationValue{}
)

func (v LocationValue) Type(ctx context.Context) attr.Type {
	return LocationType{}
}

// ValidateAttribute ensures the location is not an empty string. It is called
// implicitly by the framework when the Terraform value is converted into this
// custom value type.
func (v LocationValue) ValidateAttribute(ctx context.Context, req xattr.ValidateAttributeRequest, resp *xattr.ValidateAttributeResponse) {
	if v.IsNull() || v.IsUnknown() {
		return
	}

	if v.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Location Value",
			"The location must not be empty.",
		)
	}
}

func (v LocationValue) Equal(o attr.Value) bool {
	other, ok := o.(LocationValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// StringSemanticEquals returns true when the two location values are equal after
// normalization (lower-cased, spaces removed).
func (v LocationValue) StringSemanticEquals(ctx context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(LocationValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+v.String()+"\n"+
				"Got Value Type: "+newValuable.String(),
		)
		return false, diags
	}

	return location.Normalize(v.ValueString()) == location.Normalize(newValue.ValueString()), diags
}
