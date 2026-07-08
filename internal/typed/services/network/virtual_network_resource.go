package network

import (
	"context"
	"fmt"

	"github.com/Azure/terraform-provider-azapi/internal/azure/location"
	"github.com/Azure/terraform-provider-azapi/internal/clients"
	"github.com/Azure/terraform-provider-azapi/internal/services/myplanmodifier"
	"github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"
	"github.com/Azure/terraform-provider-azapi/internal/services/parse"
	"github.com/Azure/terraform-provider-azapi/internal/typed/tfconv"
	"github.com/Azure/terraform-provider-azapi/utils"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type AzApiVirtualNetworkResource struct {
	ProviderData *clients.Client
	apiType      string
	schema       schema.Schema
}

var _ resource.Resource = &AzApiVirtualNetworkResource{}
var _ resource.ResourceWithConfigure = &AzApiVirtualNetworkResource{}

func NewAzApiVirtualNetworkResource() *AzApiVirtualNetworkResource {
	return &AzApiVirtualNetworkResource{
		apiType: "Microsoft.Network/virtualNetworks@2022-07-01",
		schema: schema.Schema{
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
		},
	}
}

func (r *AzApiVirtualNetworkResource) Configure(ctx context.Context, request resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if v, ok := request.ProviderData.(*clients.Client); ok {
		r.ProviderData = v
	}
}

func (r *AzApiVirtualNetworkResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_virtual_network"
}

func (r *AzApiVirtualNetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.schema
}

func (r *AzApiVirtualNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	plan, diags := tfconv.ObjectFromRaw(ctx, r.schema.Type(), req.Plan.Raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Attributes()["name"].(types.String).ValueString()
	parentId := plan.Attributes()["parent_id"].(types.String).ValueString()

	id, err := parse.NewResourceID(name, parentId, r.apiType)
	if err != nil {
		resp.Diagnostics.AddError("failed to new resource id", err.Error())
		return
	}

	apiReqAny, diags := tfconv.Expand(ctx, plan, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := apiReqAny.(map[string]any)
	delete(apiReq, "id")
	delete(apiReq, "name")
	delete(apiReq, "parentId")

	apiRespAny, err := r.ProviderData.ResourceClient.CreateOrUpdate(ctx, id.AzureResourceId, id.ApiVersion, apiReq, clients.DefaultRequestOptions())
	if err != nil {
		resp.Diagnostics.AddError("failed to create", err.Error())
		return
	}

	respBody := apiRespAny.(map[string]any)
	respBody["parentId"] = parentId
	respBody["name"] = name

	stateObj, diags := tfconv.Flatten(ctx, r.schema.Type(), respBody, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, stateObj)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *AzApiVirtualNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	state, diags := tfconv.ObjectFromRaw(ctx, r.schema.Type(), req.State.Raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Attributes()["name"].(types.String).ValueString()
	parentId := state.Attributes()["parent_id"].(types.String).ValueString()

	id, err := parse.NewResourceID(name, parentId, r.apiType)
	if err != nil {
		resp.Diagnostics.AddError("failed to new resource id", err.Error())
		return
	}

	apiRespAny, err := r.ProviderData.ResourceClient.Get(ctx, id.AzureResourceId, id.ApiVersion, clients.DefaultRequestOptions())
	if err != nil {
		if utils.ResponseErrorWasNotFound(err) {
			tflog.Info(ctx, fmt.Sprintf("%s is not found", id.ID()))
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to read", err.Error())
		return
	}

	respBody := apiRespAny.(map[string]any)
	respBody["parentId"] = parentId
	respBody["name"] = name

	stateObj, diags := tfconv.Flatten(ctx, r.schema.Type(), respBody, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, stateObj)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *AzApiVirtualNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	plan, diags := tfconv.ObjectFromRaw(ctx, r.schema.Type(), req.Plan.Raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Attributes()["name"].(types.String).ValueString()
	parentId := plan.Attributes()["parent_id"].(types.String).ValueString()

	id, err := parse.NewResourceID(name, parentId, r.apiType)
	if err != nil {
		resp.Diagnostics.AddError("failed to new resource id", err.Error())
		return
	}

	apiReqAny, diags := tfconv.Expand(ctx, plan, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiReq := apiReqAny.(map[string]any)
	delete(apiReq, "id")
	delete(apiReq, "name")
	delete(apiReq, "parentId")

	apiRespAny, err := r.ProviderData.ResourceClient.CreateOrUpdate(ctx, id.AzureResourceId, id.ApiVersion, apiReq, clients.DefaultRequestOptions())
	if err != nil {
		resp.Diagnostics.AddError("failed to update", err.Error())
		return
	}

	respBody := apiRespAny.(map[string]any)
	respBody["parentId"] = parentId
	respBody["name"] = name

	stateObj, diags := tfconv.Flatten(ctx, r.schema.Type(), respBody, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, stateObj)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *AzApiVirtualNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	state, diags := tfconv.ObjectFromRaw(ctx, r.schema.Type(), req.State.Raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Attributes()["name"].(types.String).ValueString()
	parentId := state.Attributes()["parent_id"].(types.String).ValueString()

	id, err := parse.NewResourceID(name, parentId, r.apiType)
	if err != nil {
		resp.Diagnostics.AddError("failed to new resource id", err.Error())
		return
	}

	if _, err := r.ProviderData.ResourceClient.Delete(ctx, id.AzureResourceId, id.ApiVersion, clients.DefaultRequestOptions()); err != nil {
		if utils.ResponseErrorWasNotFound(err) {
			tflog.Info(ctx, fmt.Sprintf("%s is not found, just return without error", id.ID()))
			return
		}
		resp.Diagnostics.AddError("failed to delete", err.Error())
		return
	}
}
