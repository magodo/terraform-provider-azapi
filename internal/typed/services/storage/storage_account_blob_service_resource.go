package storage

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func NewAzApiStorageAccountBlobServiceResource() AzApiStorageAccountBlobServiceResource {
	return AzApiStorageAccountBlobServiceResource{
		hooks: servicehooks.ResourceHooks{
			ModelConvOptionHook: func(o modelconv.Option) modelconv.Option {
				return o
			},
			SchemaHook: func(s schema.Schema) schema.Schema {
				// Remove this since it is deprecated
				servicehooks.RemoveSchemaAttribute(s, "properties.restore_policy.last_enabled_time")
				return s
			},
		},
	}
}
