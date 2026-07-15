package storage

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func NewAzApiStorageAccountResource() AzApiStorageAccountResource {
	return AzApiStorageAccountResource{
		hooks: servicehooks.ResourceHooks{
			SchemaHook: func(s schema.Schema) schema.Schema {
				servicehooks.UpdateSchemaAttribute(s, "properties.access_tier", func(s schema.StringAttribute) schema.StringAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.allow_cross_tenant_replication", func(s schema.BoolAttribute) schema.BoolAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.encryption", func(s schema.SingleNestedAttribute) schema.SingleNestedAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.minimum_tls_version", func(s schema.StringAttribute) schema.StringAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.network_acls", func(s schema.SingleNestedAttribute) schema.SingleNestedAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.supports_https_traffic_only", func(s schema.BoolAttribute) schema.BoolAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "sku.tier", func(s schema.StringAttribute) schema.StringAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "tags", func(s schema.MapAttribute) schema.MapAttribute {
					s.Computed = true
					return s
				})
				return s
			},
		},
	}
}
