package network

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

func NewAzApiVirtualNetworkResource() AzApiVirtualNetworkResource {
	return AzApiVirtualNetworkResource{
		hooks: servicehooks.ResourceHooks{
			SchemaHook: func(s schema.Schema) schema.Schema {
				addressSpace := s.Attributes["properties"].(schema.SingleNestedAttribute).Attributes["address_space"].(schema.SingleNestedAttribute)

				addressPrefixes := addressSpace.Attributes["address_prefixes"].(schema.SetAttribute)
				addressPrefixes.Validators = []validator.Set{
					setvalidator.SizeAtLeast(1),
				}
				addressSpace.Attributes["address_prefixes"] = addressPrefixes

				return s
			},
		},
	}
}
