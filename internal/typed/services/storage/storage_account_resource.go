package storage

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewAzApiStorageAccountResource() AzApiStorageAccountResource {
	return AzApiStorageAccountResource{
		hooks: servicehooks.ResourceHooks{
			SchemaHook: func(ctx context.Context, s schema.Schema) schema.Schema {
				servicehooks.UpdateSchemaAttribute(s, "properties.access_tier", func(s schema.StringAttribute) schema.StringAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.allow_cross_tenant_replication", func(s schema.BoolAttribute) schema.BoolAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.encryption", func(s schema.SingleNestedAttribute) schema.SingleNestedAttribute {
					var encryptionServiceAttributeTypes = map[string]attr.Type{
						"enabled":  types.BoolType,
						"key_type": types.StringType,
					}

					encryptionServiceDefault := types.ObjectValueMust(encryptionServiceAttributeTypes, map[string]attr.Value{
						"enabled":  types.BoolValue(true),
						"key_type": types.StringValue("Account"),
					})

					s.Default = objectdefault.StaticValue(types.ObjectValueMust(
						map[string]attr.Type{
							"identity": types.ObjectType{AttrTypes: map[string]attr.Type{
								"federated_identity_client_id": types.StringType,
								"user_assigned_identity":       types.StringType,
							}},
							"key_source": types.StringType,
							"keyvaultproperties": types.ObjectType{AttrTypes: map[string]attr.Type{
								"keyname":     types.StringType,
								"keyvaulturi": types.StringType,
								"keyversion":  types.StringType,
								"current_versioned_key_expiration_timestamp": types.StringType,
								"current_versioned_key_identifier":           types.StringType,
								"last_key_rotation_timestamp":                types.StringType,
							}},
							"require_infrastructure_encryption": types.BoolType,
							"services": types.ObjectType{AttrTypes: map[string]attr.Type{
								"blob":  types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
								"file":  types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
								"queue": types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
								"table": types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
							}},
						},
						map[string]attr.Value{
							"identity": types.ObjectNull(map[string]attr.Type{
								"federated_identity_client_id": types.StringType,
								"user_assigned_identity":       types.StringType,
							}),
							"key_source": types.StringValue("Microsoft.Storage"),
							"keyvaultproperties": types.ObjectNull(map[string]attr.Type{
								"keyname":     types.StringType,
								"keyvaulturi": types.StringType,
								"keyversion":  types.StringType,
								"current_versioned_key_expiration_timestamp": types.StringType,
								"current_versioned_key_identifier":           types.StringType,
								"last_key_rotation_timestamp":                types.StringType,
							}),
							"require_infrastructure_encryption": types.BoolNull(),
							"services": types.ObjectValueMust(map[string]attr.Type{
								"blob":  types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
								"file":  types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
								"queue": types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
								"table": types.ObjectType{AttrTypes: encryptionServiceAttributeTypes},
							}, map[string]attr.Value{
								"blob":  encryptionServiceDefault,
								"file":  encryptionServiceDefault,
								"queue": types.ObjectNull(encryptionServiceAttributeTypes),
								"table": types.ObjectNull(encryptionServiceAttributeTypes),
							}),
						}))
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.minimum_tls_version", func(s schema.StringAttribute) schema.StringAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.network_acls", func(s schema.SingleNestedAttribute) schema.SingleNestedAttribute {
					s.Default = objectdefault.StaticValue(types.ObjectValueMust(
						map[string]attr.Type{
							"default_action":        types.StringType,
							"bypass":                types.StringType,
							"ip_rules":              types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"action": types.StringType, "value": types.StringType}}},
							"ipv6_rules":            types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"action": types.StringType, "value": types.StringType}}},
							"resource_access_rules": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"resource_id": types.StringType, "tenant_id": types.StringType}}},
							"virtual_network_rules": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"action": types.StringType, "id": types.StringType, "state": types.StringType}}},
						},
						map[string]attr.Value{
							"default_action": types.StringValue("Allow"),
							"bypass":         types.StringValue("None"),
							"ip_rules": types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
								"action": types.StringType,
								"value":  types.StringType,
							}}, []attr.Value{}),
							"ipv6_rules": types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
								"action": types.StringType,
								"value":  types.StringType,
							}}, []attr.Value{}),
							"resource_access_rules": types.ListNull(types.ObjectType{AttrTypes: map[string]attr.Type{
								"resource_id": types.StringType,
								"tenant_id":   types.StringType,
							}}),
							"virtual_network_rules": types.ListValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
								"action": types.StringType,
								"id":     types.StringType,
								"state":  types.StringType,
							}}, []attr.Value{}),
						}))
					s.Computed = true

					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.allow_blob_public_access", func(s schema.BoolAttribute) schema.BoolAttribute {
					s.Computed = true
					return s
				})
				servicehooks.UpdateSchemaAttribute(s, "properties.supports_https_traffic_only", func(s schema.BoolAttribute) schema.BoolAttribute {
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
