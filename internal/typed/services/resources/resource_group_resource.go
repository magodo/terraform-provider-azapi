package resources

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func NewAzApiResourceGroupResource() AzApiResourceGroupResource {
	return AzApiResourceGroupResource{
		hooks: servicehooks.ResourceHooks{
			SchemaHook: func(ctx context.Context, s schema.Schema) schema.Schema {
				servicehooks.RemoveSchemaAttribute(s, "properties")
				servicehooks.RemoveSchemaAttribute(s, "managed_by")
				return s
			},
		},
	}
}
