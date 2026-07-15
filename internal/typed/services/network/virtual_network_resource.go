package network

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/clients"
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func NewAzApiVirtualNetworkResource() AzApiVirtualNetworkResource {
	return AzApiVirtualNetworkResource{
		hooks: servicehooks.ResourceHooks{
			ModelConvOptionHook: func(o modelconv.Option) modelconv.Option {
				o.NameOverrides["properties.privateEndpointVNetPolicies"] = "private_endpoint_vnet_policies"
				return o
			},
			SchemaHook: func(s schema.Schema) schema.Schema {
				servicehooks.UpdateSchemaAttribute(s, "properties.address_space.address_prefixes", func(a schema.ListAttribute) schema.ListAttribute {
					a.Validators = []validator.List{
						listvalidator.SizeAtLeast(1),
					}
					return a
				})

				servicehooks.RenameSchemaAttribute(s, "properties.private_endpoint_v_net_policies", "private_endpoint_vnet_policies")
				servicehooks.UpdateSchemaAttribute(s, "properties.private_endpoint_vnet_policies", func(a schema.StringAttribute) schema.StringAttribute {
					a.Computed = true
					a.Validators = []validator.String{
						stringvalidator.OneOf("Basic", "Disabled"),
					}
					return a
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.enable_ddos_protection", func(a schema.BoolAttribute) schema.BoolAttribute {
					a.Computed = true
					return a
				})
				return s
			},
		},
	}
}

// A dummy read post create to showcase.
func (r AzApiVirtualNetworkResource) PostCreate(ctx context.Context, req resource.CreateRequest, meta framework.Meta) diag.Diagnostics {
	id, diags := framework.ResourceIdFromPlan(ctx, req.Plan, r.AzureResourceType())
	if diags.HasError() {
		return diags
	}

	if _, err := meta.ResourceClient.Get(ctx, id.ID(), id.ApiVersion, clients.DefaultRequestOptions()); err != nil {
		diags.AddError("failed to read resource", err.Error())
		return diags
	}

	return diags
}
