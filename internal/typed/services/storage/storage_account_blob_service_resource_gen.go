package storage

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
)

type AzApiStorageAccountBlobServiceResource struct {
	hooks servicehooks.ResourceHooks
}

var _ framework.Resource = AzApiStorageAccountBlobServiceResource{}

func (r AzApiStorageAccountBlobServiceResource) AzureResourceType() string {
	return "Microsoft.Storage/storageAccounts/blobServices@2025-08-01"
}

func (r AzApiStorageAccountBlobServiceResource) TFResourceType() string {
	return "azapi_storage_account_blob_service"
}

func (r AzApiStorageAccountBlobServiceResource) GetSchema(ctx context.Context) schema.Schema {
	schema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parent_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					myvalidator.StringIsResourceID(),
				},
			},
			"properties": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"cors": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"cors_rules": schema.ListNestedAttribute{
								Required: true,
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"allowed_origins": schema.ListAttribute{
											Required:    true,
											ElementType: types.StringType,
										},
										"allowed_methods": schema.ListAttribute{
											Required:    true,
											ElementType: types.StringType,
										},
										"allowed_headers": schema.ListAttribute{
											Required:    true,
											ElementType: types.StringType,
										},
										"max_age_in_seconds": schema.Int32Attribute{
											Required: true,
										},
										"exposed_headers": schema.ListAttribute{
											Required:    true,
											ElementType: types.StringType,
										},
									},
								},
							},
							"default_service_version": schema.StringAttribute{
								Optional: true,
								Computed: true,
							},
							"delete_retention_policy": schema.ListNestedAttribute{
								Optional: true,
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"enabled": schema.BoolAttribute{
											Optional: true,
										},
										"days": schema.Int32Attribute{
											Optional: true,
										},
										"allow_permanent_delete": schema.BoolAttribute{
											Optional: true,
										},
									},
								},
							},
						},
					},
				},
			},
			"sku": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Computed: true,
					},
					"tier": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
				Read:   true,
				Update: true,
				Delete: true,
			}),
		},
	}
	if r.hooks.SchemaHook != nil {
		schema = r.hooks.SchemaHook(schema)
	}
	return schema
}

func (r AzApiStorageAccountBlobServiceResource) GetModelConvOption() *modelconv.Option {
	opt := &modelconv.Option{}
	if r.hooks.ModelConvOptionHook != nil {
		opt = r.hooks.ModelConvOptionHook(opt)
	}
	return opt
}

func (r AzApiStorageAccountBlobServiceResource) RenderOption() tffwdocs.ResourceRenderOption {
	opt := tffwdocs.ResourceRenderOption{
		Subcategory: "network",
		ImportId: &tffwdocs.ImportId{
			Format:    "<resource_id>",
			ExampleId: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/mygroup1/providers/Microsoft.Storage/storageAccounts/account1/blobServices/default",
		},
	}

	if r.hooks.RenderOptionHook != nil {
		opt = r.hooks.RenderOptionHook(opt)
	}

	return opt
}
