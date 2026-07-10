package network

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func NewAzApiVirtualNetworkResource() AzApiVirtualNetworkResource {
	return AzApiVirtualNetworkResource{
		hooks: servicehooks.ResourceHooks{
			SchemaHook: func(s schema.Schema) schema.Schema {
				return s
			},
		},
	}
}
