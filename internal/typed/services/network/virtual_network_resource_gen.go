package network

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

type AzApiVirtualNetworkResource struct {
	hooks servicehooks.ResourceHooks
}

var _ framework.Resource = AzApiVirtualNetworkResource{}

func (r AzApiVirtualNetworkResource) AzureResourceType() string {
	return "Microsoft.Network/virtualNetworks@2025-01-01"
}

func (r AzApiVirtualNetworkResource) TFResourceType() string {
	return "azapi_virtual_network"
}

func (r AzApiVirtualNetworkResource) GetSchema(ctx context.Context) schema.Schema {
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
			"location": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Properties of the virtual network.",
				Attributes: map[string]schema.Attribute{
					"address_space": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The AddressSpace that contains an array of IP address ranges that can be used by subnets.",
						Attributes: map[string]schema.Attribute{
							"address_prefixes": schema.ListAttribute{
								Optional:            true,
								ElementType:         types.StringType,
								MarkdownDescription: "A list of address blocks reserved for this virtual network in CIDR notation.",
							},
							"ipam_pool_prefix_allocations": schema.ListNestedAttribute{
								Optional:            true,
								MarkdownDescription: "A list of IPAM Pools allocating IP address prefixes.",
								NestedObject: schema.NestedAttributeObject{
									Attributes: map[string]schema.Attribute{
										"number_of_ip_addresses": schema.StringAttribute{
											Optional:            true,
											MarkdownDescription: "Number of IP addresses to allocate.",
										},
										"pool": schema.SingleNestedAttribute{
											Optional: true,
											Attributes: map[string]schema.Attribute{
												"id": schema.StringAttribute{
													Optional:            true,
													MarkdownDescription: "Resource id of the associated Azure IpamPool resource.",
												},
											},
										},
										"allocated_address_prefixes": schema.ListAttribute{
											Computed:            true,
											ElementType:         types.StringType,
											MarkdownDescription: "List of assigned IP address prefixes in the IpamPool of the associated resource.",
										},
									},
								},
							},
						},
					},
					"bgp_communities": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Bgp Communities sent over ExpressRoute with each route corresponding to a prefix in this VNET.",
						Attributes: map[string]schema.Attribute{
							"virtual_network_community": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: "The BGP community associated with the virtual network.",
							},
							"regional_community": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "The BGP community associated with the region of the virtual network.",
							},
						},
					},
					"ddos_protection_plan": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The DDoS protection plan associated with the virtual network.",
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "Resource ID.",
							},
						},
					},
					"dhcp_options": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The dhcpOptions that contains an array of DNS servers available to VMs deployed in the virtual network.",
						Attributes: map[string]schema.Attribute{
							"dns_servers": schema.ListAttribute{
								Optional:            true,
								ElementType:         types.StringType,
								MarkdownDescription: "The list of DNS servers IP addresses.",
							},
						},
					},
					"enable_ddos_protection": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Indicates if DDoS protection is enabled for all the protected resources in the virtual network. It requires a DDoS protection plan associated with the resource.",
					},
					"enable_vm_protection": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Indicates if VM protection is enabled for all the subnets in the virtual network.",
					},
					"encryption": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Indicates if encryption is enabled on virtual network and if VM without encryption is allowed in encrypted VNet.",
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:            true,
								MarkdownDescription: "Indicates if encryption is enabled on the virtual network.",
							},
							"enforcement": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "If the encrypted VNet allows VM that does not support encryption. This field is for future support, AllowUnencrypted is the only supported value at general availability.",
							},
						},
					},
					"flow_timeout_in_minutes": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "The FlowTimeout value (in minutes) for the Virtual Network",
					},
					"ip_allocations": schema.ListNestedAttribute{
						Optional:            true,
						MarkdownDescription: "Array of IpAllocation which reference this VNET.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Optional:            true,
									MarkdownDescription: "Resource ID.",
								},
							},
						},
					},
					"private_endpoint_v_net_policies": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Private Endpoint VNet Policies.",
					},
					"default_public_nat_gateway": schema.SingleNestedAttribute{
						Computed:            true,
						MarkdownDescription: "A reference to the default public nat gateway being used by this virtual network resource.",
						Attributes: map[string]schema.Attribute{
							"id": schema.StringAttribute{
								Computed:            true,
								MarkdownDescription: "Resource ID.",
							},
						},
					},
					"resource_guid": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The resourceGuid property of the Virtual Network resource.",
					},
				},
			},
			"tags": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Resource tags.",
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

func (r AzApiVirtualNetworkResource) GetModelConvOption() *modelconv.Option {
	opt := modelconv.NewDefaultOption()
	if r.hooks.ModelConvOptionHook != nil {
		opt = r.hooks.ModelConvOptionHook(opt)
	}
	return &opt
}

func (r AzApiVirtualNetworkResource) RenderOption() tffwdocs.ResourceRenderOption {
	opt := tffwdocs.ResourceRenderOption{
		Subcategory: "network",
		ImportId: &tffwdocs.ImportId{
			Format:    "<resource_id>",
			ExampleId: "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup/providers/Microsoft.Network/virtualNetworks/myVirtualNetwork",
		},
	}
	if r.hooks.RenderOptionHook != nil {
		opt = r.hooks.RenderOptionHook(opt)
	}
	return opt
}
