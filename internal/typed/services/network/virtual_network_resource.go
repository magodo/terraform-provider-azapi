package network

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/azure/location"
	"github.com/Azure/terraform-provider-azapi/internal/services/myplanmodifier"
	"github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
)

type AzApiVirtualNetworkResource struct{}

var _ framework.Resource = AzApiVirtualNetworkResource{}

func NewAzApiVirtualNetworkResource() AzApiVirtualNetworkResource {
	return AzApiVirtualNetworkResource{}
}

func (r AzApiVirtualNetworkResource) AzureResourceType() string {
	return "Microsoft.Network/virtualNetworks@2022-07-01"
}

func (r AzApiVirtualNetworkResource) TFResourceType() string {
	return "azapi_virtual_network"
}

func (r AzApiVirtualNetworkResource) GetSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
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
			"location": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					myplanmodifier.UseStateWhen(func(a, b types.String) bool {
						return location.Normalize(a.ValueString()) == location.Normalize(b.ValueString())
					}),
				},
			},
			"properties": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"address_space": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"address_prefixes": schema.SetAttribute{
								Required:    true,
								ElementType: types.StringType,
							},
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
}

func (r AzApiVirtualNetworkResource) RenderOption() tffwdocs.ResourceRenderOption {
	return tffwdocs.ResourceRenderOption{
		// TODO
	}
}
