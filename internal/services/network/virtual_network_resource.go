package network

import (
	"context"

	"github.com/Azure/terraform-provider-azapi/internal/azure/location"
	"github.com/Azure/terraform-provider-azapi/internal/clients"
	"github.com/Azure/terraform-provider-azapi/internal/services/myplanmodifier"
	"github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"
	"github.com/Azure/terraform-provider-azapi/internal/services/parse"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type AzApiVirtualNetworkResource struct {
	ProviderData *clients.Client
}

type AzApiVirtualNetworkModel struct {
	Name       types.String `tfsdk:"name"`
	ParentId   types.String `tfsdk:"parent_id"`
	Location   types.String `tfsdk:"location"`
	Properties types.Object `tfsdk:"properties"`
	Id         types.String `tfsdk:"id"`
}

type AzApiVirtualNetworkPropertiesModel struct {
	AddressSpace types.Object `tfsdk:"address_space"`
}

type AzApiVirtualNetworkPropertiesAddressSpaceModel struct {
	AddressPrefixes types.Set `tfsdk:"address_prefixes"`
}

func (m AzApiVirtualNetworkModel) ExpandObject(ctx context.Context) (map[string]any, diag.Diagnostics) {
	out := map[string]any{}

	// Name and ParentId are not needed to be expanded.

	// The pattern below shall be:
	// if !m.Foo.IsUnknown() {
	//  // For null value, there are two behavior (needs to be differentiate by additional knowledge (e.g. an option input param):
	//  // - Set the key but keep the value null
	//  // - Not set the key at all (default)
	// }

	if !m.Location.IsUnknown() {
		if !m.Location.IsNull() {
			out["location"] = m.Location.ValueString()
		}
	}

	if !m.Properties.IsUnknown() {
		if !m.Properties.IsNull() {
			var innerM AzApiVirtualNetworkPropertiesModel
			diags := m.Properties.As(ctx, &innerM, basetypes.ObjectAsOptions{})
			if diags.HasError() {
				return nil, diags
			}

			innerOut, diags := innerM.ExpandObject(ctx)
			if diags.HasError() {
				return nil, diags
			}
			out["properties"] = innerOut
		}
	}

	return out, nil
}

func (m AzApiVirtualNetworkPropertiesModel) ExpandObject(ctx context.Context) (map[string]any, diag.Diagnostics) {
	out := map[string]any{}

	if !m.AddressSpace.IsUnknown() {
		if !m.AddressSpace.IsNull() {
			var innerM AzApiVirtualNetworkPropertiesAddressSpaceModel
			diags := m.AddressSpace.As(ctx, &innerM, basetypes.ObjectAsOptions{})
			if diags.HasError() {
				return nil, diags
			}

			innerOut, diags := innerM.ExpandObject(ctx)
			if diags.HasError() {
				return nil, diags
			}
			out["addressSpace"] = innerOut
		}
	}

	return out, nil
}

func (m AzApiVirtualNetworkPropertiesAddressSpaceModel) ExpandObject(ctx context.Context) (map[string]any, diag.Diagnostics) {
	out := map[string]any{}

	if !m.AddressPrefixes.IsUnknown() {
		if !m.AddressPrefixes.IsNull() {
			l := make([]string, 0, len(m.AddressPrefixes.Elements()))
			for _, e := range m.AddressPrefixes.Elements() {
				l = append(l, e.(types.String).ValueString())
			}
			out["addressPrefixes"] = l
		}
	}

	return out, nil
}

var _ resource.Resource = &AzApiVirtualNetworkResource{}
var _ resource.ResourceWithConfigure = &AzApiVirtualNetworkResource{}

func (r *AzApiVirtualNetworkResource) Configure(ctx context.Context, request resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if v, ok := request.ProviderData.(*clients.Client); ok {
		r.ProviderData = v
	}
}

func (r *AzApiVirtualNetworkResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_virtual_network"
}

func (r *AzApiVirtualNetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
		},
	}
}

func (r *AzApiVirtualNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AzApiVirtualNetworkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := parse.NewResourceID(plan.Name.ValueString(), plan.ParentId.ValueString(), "Microsoft.Network/virtualNetworks@2022-07-01")
	if err != nil {
		resp.Diagnostics.AddError("failed to new resource id", err.Error())
		return
	}

	body, diags := plan.ExpandObject(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.ProviderData.ResourceClient.CreateOrUpdate(ctx, id.AzureResourceId, id.ApiVersion, body, clients.DefaultRequestOptions()); err != nil {
		resp.Diagnostics.AddError("failed to create", err.Error())
		return
	}

	plan.Id = types.StringValue(id.ID())
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *AzApiVirtualNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	panic("unimplemented")
}

func (r *AzApiVirtualNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	panic("unimplemented")
}

func (r *AzApiVirtualNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	panic("unimplemented")
}
