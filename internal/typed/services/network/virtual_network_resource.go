package network

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func NewAzApiVirtualNetworkResource() AzApiVirtualNetworkResource {
	return AzApiVirtualNetworkResource{
		hooks: servicehooks.ResourceHooks{
			ModelConvOptionHook: func(o *modelconv.Option) *modelconv.Option {
				o.NameOverrides["properties.privateEndpointVNetPolicies"] = "private_endpoint_vnet_policy"
				return o
			},
			SchemaHook: func(s schema.Schema) schema.Schema {
				servicehooks.UpdateSchemaAttribute(s, "properties.address_space.address_prefixes", func(a schema.SetAttribute) schema.SetAttribute {
					a.Validators = []validator.Set{
						setvalidator.SizeAtLeast(1),
					}
					return a
				})

				servicehooks.RenameSchemaAttribute(s, "properties.private_endpoint_vnet_policies", "private_endpoint_vnet_policy")
				servicehooks.UpdateSchemaAttribute(s, "properties.private_endpoint_vnet_policy", func(a schema.StringAttribute) schema.StringAttribute {
					a.Validators = []validator.String{
						stringvalidator.OneOf("Basic", "Disabled"),
					}
					return a
				})
				return s
			},
		},
	}
}
