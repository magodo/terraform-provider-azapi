package framework

import (
	"context"
	"fmt"

	"github.com/Azure/terraform-provider-azapi/internal/services/parse"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

// ResourceIdFromPlan constructs the resource id by combining the rt and the "name", "parent_id" read from plan.
func ResourceIdFromPlan(ctx context.Context, plan tfsdk.Plan, rt string) (*parse.ResourceId, diag.Diagnostics) {
	var diags diag.Diagnostics

	var name, parentId string
	diags.Append(plan.GetAttribute(ctx, path.Root("name"), &name)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("parent_id"), &parentId)...)
	if diags.HasError() {
		return nil, diags
	}
	id, err := parse.NewResourceID(name, parentId, rt)
	if err != nil {
		diags.AddError("failed to new resource id", err.Error())
		return nil, diags
	}

	return &id, diags
}

// ResourceIdFromState constructs the resource id by reading and parsing the "id" from state.
func ResourceIdFromState(ctx context.Context, state tfsdk.State) (*parse.ResourceId, diag.Diagnostics) {
	var diags diag.Diagnostics

	var idstr string
	diags.Append(state.GetAttribute(ctx, path.Root("id"), &idstr)...)
	if diags.HasError() {
		return nil, diags
	}
	id, err := parse.ResourceID(idstr)
	if err != nil {
		diags.AddError(fmt.Sprintf("failed to parse resource id %q", idstr), err.Error())
		return nil, diags
	}

	return &id, nil
}
