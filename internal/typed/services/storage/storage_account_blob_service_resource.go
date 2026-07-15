package storage

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func NewAzApiStorageAccountBlobServiceResource() AzApiStorageAccountBlobServiceResource {
	return AzApiStorageAccountBlobServiceResource{
		hooks: servicehooks.ResourceHooks{
			SchemaHook: func(ctx context.Context, s schema.Schema) schema.Schema {
				// Remove this since it is deprecated
				servicehooks.RemoveSchemaAttribute(s, "properties.restore_policy.last_enabled_time")
				return s
			},
		},
	}
}
