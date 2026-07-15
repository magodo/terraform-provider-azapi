package storage

import (
	"context"
	"github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	return "Microsoft.Storage/storageAccounts/blobServices@2025-06-01"
}

func (r AzApiStorageAccountBlobServiceResource) TFResourceType() string {
	return "azapi_storage_account_blob_service"
}

func (r AzApiStorageAccountBlobServiceResource) GetSchema(ctx context.Context) schema.Schema {
	schema := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"parent_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					myvalidator.StringIsResourceID(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "The properties of a storage account’s Blob service.",
				Attributes: map[string]schema.Attribute{
					"automatic_snapshot_policy_enabled": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Deprecated in favor of isVersioningEnabled property.",
					},
					"change_feed": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The blob service properties for change feed events.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Indicates whether change feed event logging is enabled for the Blob service.",
							},
							"retention_in_days": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "Indicates the duration of changeFeed retention in days. Minimum value is 1 day and maximum value is 146000 days (400 years). A null value indicates an infinite retention of the change feed.",
								Validators: []validator.Int64{
									int64validator.Between(1, 146000),
								},
							},
						},
					},
					"container_delete_retention_policy": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The blob service properties for container soft delete.",
						Attributes: map[string]schema.Attribute{
							"allow_permanent_delete": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "This property when set to true allows deletion of the soft deleted blob versions and snapshots. This property cannot be used blob restore policy. This property only applies to blob service and does not apply to containers or file share.",
							},
							"days": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "Indicates the number of days that the deleted item should be retained. The minimum specified value can be 1 and the maximum value can be 365.",
								Validators: []validator.Int64{
									int64validator.Between(1, 365),
								},
							},
							"enabled": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Indicates whether DeleteRetentionPolicy is enabled.",
							},
						},
					},
					"cors": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Specifies CORS rules for the Blob service. You can include up to five CorsRule elements in the request. If no CorsRule elements are included in the request body, all CORS rules will be deleted, and CORS will be disabled for the Blob service.",
						Attributes: map[string]schema.Attribute{
							"cors_rules": schema.ListNestedAttribute{
								Optional:            true,
								MarkdownDescription: "The List of CORS rules. You can include up to five CorsRule elements in the request.",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"allowed_headers": schema.ListAttribute{
											Required:            true,
											ElementType:         types.StringType,
											MarkdownDescription: "Required if CorsRule element is present. A list of headers allowed to be part of the cross-origin request.",
										},
										"allowed_methods": schema.ListAttribute{
											Required:            true,
											ElementType:         types.StringType,
											MarkdownDescription: "Required if CorsRule element is present. A list of HTTP methods that are allowed to be executed by the origin.",
										},
										"allowed_origins": schema.ListAttribute{
											Required:            true,
											ElementType:         types.StringType,
											MarkdownDescription: "Required if CorsRule element is present. A list of origin domains that will be allowed via CORS, or \"*\" to allow all domains",
										},
										"exposed_headers": schema.ListAttribute{
											Required:            true,
											ElementType:         types.StringType,
											MarkdownDescription: "Required if CorsRule element is present. A list of response headers to expose to CORS clients.",
										},
										"max_age_in_seconds": schema.Int64Attribute{
											Required:            true,
											MarkdownDescription: "Required if CorsRule element is present. The number of seconds that the client/browser should cache a preflight response.",
										},
									},
								},
							},
						},
					},
					"default_service_version": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "DefaultServiceVersion indicates the default version to use for requests to the Blob service if an incoming request’s version is not specified. Possible values include version 2008-10-27 and all more recent versions.",
					},
					"delete_retention_policy": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The blob service properties for blob soft delete.",
						Attributes: map[string]schema.Attribute{
							"allow_permanent_delete": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "This property when set to true allows deletion of the soft deleted blob versions and snapshots. This property cannot be used blob restore policy. This property only applies to blob service and does not apply to containers or file share.",
							},
							"days": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "Indicates the number of days that the deleted item should be retained. The minimum specified value can be 1 and the maximum value can be 365.",
								Validators: []validator.Int64{
									int64validator.Between(1, 365),
								},
							},
							"enabled": schema.BoolAttribute{
								Optional:            true,
								MarkdownDescription: "Indicates whether DeleteRetentionPolicy is enabled.",
							},
						},
					},
					"is_versioning_enabled": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Versioning is enabled if set to true.",
					},
					"last_access_time_tracking_policy": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The blob service property to configure last access time based tracking policy.",
						Attributes: map[string]schema.Attribute{
							"enable": schema.BoolAttribute{
								Required:            true,
								MarkdownDescription: "When set to true last access time based tracking is enabled.",
							},
							"blob_type": schema.ListAttribute{
								Optional:            true,
								ElementType:         types.StringType,
								MarkdownDescription: "An array of predefined supported blob types. Only blockBlob is the supported value. This field is currently read only",
							},
							"name": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Name of the policy. The valid value is AccessTimeTracking. This field is currently read only",
							},
							"tracking_granularity_in_days": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "The field specifies blob object tracking granularity in days, typically how often the blob object should be tracked.This field is currently read only with value as 1",
							},
						},
					},
					"restore_policy": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The blob service properties for blob restore policy.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:            true,
								MarkdownDescription: "Blob restore is enabled if set to true.",
							},
							"days": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: "how long this blob can be restored. It should be great than zero and less than DeleteRetentionPolicy.days.",
								Validators: []validator.Int64{
									int64validator.Between(1, 365),
								},
							},
							"last_enabled_time": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Deprecated in favor of minRestoreTime property.",
							},
							"min_restore_time": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Returns the minimum date and time that the restore can be started.",
							},
						},
					},
				},
			},
			"sku": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Sku name and tier.",
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The SKU name. Required for account creation; optional for update. Note that in older versions, SKU name was called accountType.",
					},
					"tier": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The SKU tier. This is based on the SKU name.",
						Validators: []validator.String{
							stringvalidator.OneOf("Standard", "Premium"),
						},
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
		schema = r.hooks.SchemaHook(ctx, schema)
	}
	return schema
}

func (r AzApiStorageAccountBlobServiceResource) GetModelConvOption(ctx context.Context) *modelconv.Option {
	opt := modelconv.NewDefaultOption()
	if r.hooks.ModelConvOptionHook != nil {
		opt = r.hooks.ModelConvOptionHook(ctx, opt)
	}
	return &opt
}

func (r AzApiStorageAccountBlobServiceResource) RenderOption() tffwdocs.ResourceRenderOption {
	opt := tffwdocs.ResourceRenderOption{
		Subcategory: "storage",
		ImportId: &tffwdocs.ImportId{
			Format:    "<resource_id>",
			ExampleId: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Storage/storageAccounts/myStorageAccount/blobServices/myBlobService",
		},
	}
	if r.hooks.RenderOptionHook != nil {
		opt = r.hooks.RenderOptionHook(opt)
	}
	return opt
}
