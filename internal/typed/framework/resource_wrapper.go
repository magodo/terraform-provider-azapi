package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/terraform-provider-azapi/internal/clients"
	"github.com/Azure/terraform-provider-azapi/internal/services/parse"
	"github.com/Azure/terraform-provider-azapi/internal/tf"
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/Azure/terraform-provider-azapi/utils"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
)

var _ resource.Resource = &resourceWrapper{}
var _ resource.ResourceWithConfigure = &resourceWrapper{}
var _ resource.ResourceWithImportState = &resourceWrapper{}
var _ resource.ResourceWithConfigValidators = &resourceWrapper{}
var _ resource.ResourceWithModifyPlan = &resourceWrapper{}
var _ resource.ResourceWithMoveState = &resourceWrapper{}
var _ resource.ResourceWithUpgradeState = &resourceWrapper{}
var _ resource.ResourceWithValidateConfig = &resourceWrapper{}
var _ resource.ResourceWithUpgradeIdentity = &resourceWrapper{}
var _ ResourceWithTimeout = &resourceWrapper{}
var _ tffwdocs.ResourceWithRenderOption = &resourceWrapper{}

// TODO: support identity
// var _ resource.ResourceWithIdentity = &resourceWrapper{}

type resourceWrapper struct {
	Resource
	meta Meta
}

func WrapResource(in Resource) func() resource.Resource {
	return func() resource.Resource {
		return &resourceWrapper{Resource: in}
	}
}

func (r resourceWrapper) Debug(ctx context.Context, msg string, additionalFields ...map[string]any) {
	tflog.SubsystemDebug(ctx, r.TFResourceType(), msg, additionalFields...)
}

func (r resourceWrapper) Info(ctx context.Context, msg string, additionalFields ...map[string]any) {
	tflog.SubsystemInfo(ctx, r.TFResourceType(), msg, additionalFields...)
}

func (r resourceWrapper) Warn(ctx context.Context, msg string, additionalFields ...map[string]any) {
	tflog.SubsystemWarn(ctx, r.TFResourceType(), msg, additionalFields...)
}

func (r resourceWrapper) Error(ctx context.Context, msg string, additionalFields ...map[string]any) {
	tflog.SubsystemError(ctx, r.TFResourceType(), msg, additionalFields...)
}

func (r resourceWrapper) Timeout() ResourceTimeout {
	if r, ok := r.Resource.(ResourceWithTimeout); ok {
		return r.Timeout()
	}
	return ResourceTimeout{
		Create: 30 * time.Minute,
		Read:   5 * time.Minute,
		Update: 30 * time.Minute,
		Delete: 30 * time.Minute,
	}
}

func (r *resourceWrapper) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = r.Resource.TFResourceType()
}

func (r resourceWrapper) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.Resource.GetSchema(ctx)

	timeout := r.Timeout()
	resp.Schema.Attributes["timeouts"] = timeouts.Attributes(ctx, timeouts.Opts{
		Create:            true,
		Read:              true,
		Update:            true,
		Delete:            true,
		CreateDescription: fmt.Sprintf("(Defaults to %s) Used when creating this resource.", timeout.Create),
		ReadDescription:   fmt.Sprintf("(Defaults to %s) Used when reading this resource.", timeout.Read),
		UpdateDescription: fmt.Sprintf("(Defaults to %s) Used when updating this resource.", timeout.Update),
		DeleteDescription: fmt.Sprintf("(Defaults to %s) Used when deleting this resource.", timeout.Delete),
	})
}

func (r resourceWrapper) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	defer func() {
		r.logDiags(ctx, resp.Diagnostics)
	}()

	ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "Import")

	if req.ID != "" {
		// Import via ID
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	} else {
		// TODO: Import via Identity. New ID from identity fields.
		panic("Import via Identity not supported yet")
	}
}

func (r resourceWrapper) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() {
		r.logDiags(ctx, resp.Diagnostics)
	}()

	ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "Create")

	var timeout timeouts.Value
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("timeouts"), &timeout)...)
	if resp.Diagnostics.HasError() {
		return
	}

	duration, diags := timeout.Create(ctx, r.Timeout().Create)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	mc := modelconv.NewModelConv(r.GetModelConvOption())

	// Build the resource id
	id, diags := ResourceIdFromPlan(ctx, req.Plan, r.AzureResourceType())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "id", id.ID())

	plan, diags := modelconv.ObjectFromRaw(ctx, r.GetSchema(ctx).Type(), req.Plan.Raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Existence Check
	{
		r.Info(ctx, "Start to check the existence of the resource")
		if _, err := r.meta.ResourceClient.Get(ctx, id.AzureResourceId, id.ApiVersion, clients.DefaultRequestOptions()); err != nil {
			if !utils.ResponseErrorWasNotFound(err) {
				resp.Diagnostics.AddError("failed to check the existence of the resource", err.Error())
				return
			}
		} else {
			resp.Diagnostics.AddError("Resource already exists", tf.ImportAsExistsError(r.TFResourceType(), id.ID()).Error())
			return
		}
		r.Info(ctx, "Finish to check the existence of the resource")
	}

	// Create the resource
	{
		r.Info(ctx, "Start to create the resource")
		apiReqAny, diags := mc.Expand(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq := apiReqAny.(map[string]any)
		delete(apiReq, "id")
		delete(apiReq, "name")
		delete(apiReq, "parentId")
		delete(apiReq, "timeouts")

		if _, err := r.meta.ResourceClient.CreateOrUpdate(ctx, id.AzureResourceId, id.ApiVersion, apiReq, clients.DefaultRequestOptions()); err != nil {
			resp.Diagnostics.AddError("failed to create", err.Error())
			return
		}
		r.Info(ctx, "Finish to create the resource")
	}

	// (optional) PostCreate
	if rr, ok := r.Resource.(ResourceWithPostCreate); ok {
		r.Info(ctx, "Start to post-create the resource")
		resp.Diagnostics.Append(rr.PostCreate(ctx, req, r.meta)...)
		if resp.Diagnostics.HasError() {
			return
		}
		r.Info(ctx, "Finish to post-create the resource")
	}

	// Read the resource
	resp.Diagnostics.Append(r.read(ctx, *id, &resp.State)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add up the special attributes not belong to the API response.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), plan.Attributes()["name"])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parent_id"), plan.Attributes()["parent_id"])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("timeouts"), plan.Attributes()["timeouts"])...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceWrapper) read(ctx context.Context, id parse.ResourceId, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	r.Info(ctx, "Start to read the resource")
	defer r.Info(ctx, "Finish to read the resource")

	mc := modelconv.NewModelConv(r.GetModelConvOption())

	apiRespAny, err := r.meta.ResourceClient.Get(ctx, id.AzureResourceId, id.ApiVersion, clients.DefaultRequestOptions())
	if err != nil {
		if utils.ResponseErrorWasNotFound(err) {
			diags.Append(DiagResourceNotFound)
			return diags
		}
		diags.AddError("failed to read", err.Error())
		return diags
	}

	respBody := apiRespAny.(map[string]any)

	stateObj, diags := mc.Flatten(ctx, r.GetSchema(ctx).Type(), respBody)
	if diags.HasError() {
		return diags
	}

	diags.Append(state.Set(ctx, stateObj)...)
	if diags.HasError() {
		return diags
	}

	return diags
}

func (r resourceWrapper) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer func() {
		r.logDiags(ctx, resp.Diagnostics)
	}()

	ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "Read")

	var timeout timeouts.Value
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("timeouts"), &timeout)...)
	if resp.Diagnostics.HasError() {
		return
	}

	duration, diags := timeout.Read(ctx, r.Timeout().Read)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Build the resource id
	id, diags := ResourceIdFromState(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "id", id.ID())

	// Read the resource
	if diags := r.read(ctx, *id, &resp.State); diags.HasError() {
		if errs := diags.Errors(); len(errs) == 1 && errs[0] == DiagResourceNotFound {
			r.Warn(ctx, "resource is not found, remove the resource from the state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(diags...)
		return
	}

	// Add up the special attributes not belong to the API response.
	state, diags := modelconv.ObjectFromRaw(ctx, r.GetSchema(ctx).Type(), req.State.Raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), state.Attributes()["name"])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parent_id"), state.Attributes()["parent_id"])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("timeouts"), state.Attributes()["timeouts"])...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceWrapper) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() {
		r.logDiags(ctx, resp.Diagnostics)
	}()

	ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "Update")

	var timeout timeouts.Value
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("timeouts"), &timeout)...)
	if resp.Diagnostics.HasError() {
		return
	}

	duration, diags := timeout.Update(ctx, r.Timeout().Update)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	mc := modelconv.NewModelConv(r.GetModelConvOption())

	// Build the resource id
	id, diags := ResourceIdFromState(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "id", id.ID())

	plan, diags := modelconv.ObjectFromRaw(ctx, r.GetSchema(ctx).Type(), req.Plan.Raw)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the resource
	{
		tflog.SubsystemInfo(ctx, r.TFResourceType(), "Start to update the resource")
		apiReqAny, diags := mc.Expand(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		apiReq := apiReqAny.(map[string]any)
		delete(apiReq, "id")
		delete(apiReq, "name")
		delete(apiReq, "parentId")
		delete(apiReq, "timeouts")

		if _, err := r.meta.ResourceClient.CreateOrUpdate(ctx, id.AzureResourceId, id.ApiVersion, apiReq, clients.DefaultRequestOptions()); err != nil {
			resp.Diagnostics.AddError("failed to create", err.Error())
			return
		}
		tflog.SubsystemInfo(ctx, r.TFResourceType(), "Finish to update the resource")
	}

	// (optional) PostUpdate
	if rr, ok := r.Resource.(ResourceWithPostUpdate); ok {
		r.Info(ctx, "Start to post-update the resource")
		resp.Diagnostics.Append(rr.PostUpdate(ctx, req, r.meta)...)
		if resp.Diagnostics.HasError() {
			return
		}
		r.Info(ctx, "Finish to post-update the resource")
	}

	// Read the resource
	resp.Diagnostics.Append(r.read(ctx, *id, &resp.State)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add up the special attributes not belong to the API response.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), plan.Attributes()["name"])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parent_id"), plan.Attributes()["parent_id"])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("timeouts"), plan.Attributes()["timeouts"])...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r resourceWrapper) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer func() {
		r.logDiags(ctx, resp.Diagnostics)
	}()

	ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "Delete")

	var timeout timeouts.Value
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("timeouts"), &timeout)...)
	if resp.Diagnostics.HasError() {
		return
	}

	duration, diags := timeout.Delete(ctx, r.Timeout().Delete)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Build the resource id
	id, diags := ResourceIdFromState(ctx, req.State)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "id", id.ID())

	// Delete the resource
	{
		tflog.SubsystemInfo(ctx, r.TFResourceType(), "Start to delete the resource")
		if _, err := r.meta.ResourceClient.Delete(ctx, id.AzureResourceId, id.ApiVersion, clients.DefaultRequestOptions()); err != nil {
			if utils.ResponseErrorWasNotFound(err) {
				tflog.Info(ctx, fmt.Sprintf("%s is not found, return successfully", id.ID()))
				return
			}
			resp.Diagnostics.AddError("failed to delete", err.Error())
			return
		}
		tflog.SubsystemInfo(ctx, r.TFResourceType(), "Finish to delete the resource")
	}

	// (optional) PostDelete
	if rr, ok := r.Resource.(ResourceWithPostDelete); ok {
		r.Info(ctx, "Start to post-delete the resource")
		resp.Diagnostics.Append(rr.PostDelete(ctx, req, r.meta)...)
		if resp.Diagnostics.HasError() {
			return
		}
		r.Info(ctx, "Finish to post-delete the resource")
	}
}

func (r *resourceWrapper) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	defer func() {
		r.logDiags(ctx, resp.Diagnostics)
	}()

	ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
	ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "Configure")

	if req.ProviderData == nil {
		return
	}

	client := req.ProviderData.(*clients.Client)

	r.meta = Meta{
		ResourceClient: client.ResourceClient,
		Option:         client.Option,
	}
}

// ConfigValidators implements resource.ResourceWithConfigValidators.
func (r resourceWrapper) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	if rr, ok := r.Resource.(ResourceWithConfigValidators); ok {
		ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
		ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "ConfigValidators")

		return rr.ConfigValidators(ctx)
	}
	return nil
}

// ModifyPlan implements resource.ResourceWithModifyPlan.
func (r resourceWrapper) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if rr, ok := r.Resource.(ResourceWithModifyPlan); ok {
		defer func() {
			r.logDiags(ctx, resp.Diagnostics)
		}()
		ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
		ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "ModifyPlan")

		rr.ModifyPlan(ctx, req, resp)
		return
	}
}

// MoveState implements resource.ResourceWithMoveState.
func (r resourceWrapper) MoveState(ctx context.Context) []resource.StateMover {
	if rr, ok := r.Resource.(ResourceWithMoveState); ok {
		ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
		ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "MoveState")

		return rr.MoveState(ctx)
	}
	return nil
}

// UpgradeState implements resource.ResourceWithUpgradeState.
func (r resourceWrapper) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {

	if rr, ok := r.Resource.(ResourceWithUpgradeState); ok {
		ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
		ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "UpgradeState")

		return rr.UpgradeState(ctx)
	}
	return nil
}

// ValidateConfig implements resource.ResourceWithValidateConfig.
func (r resourceWrapper) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if rr, ok := r.Resource.(ResourceWithValidateConfig); ok {
		defer func() {
			r.logDiags(ctx, resp.Diagnostics)
		}()

		ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
		ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "ValidateConfig")

		rr.ValidateConfig(ctx, req, resp)
		return
	}
}

// UpgradeIdentity implements resource.ResourceWithUpgradeIdentity.
func (r resourceWrapper) UpgradeIdentity(ctx context.Context) map[int64]resource.IdentityUpgrader {
	if rr, ok := r.Resource.(ResourceWithUpgradeIdentity); ok {
		ctx = tflog.NewSubsystem(ctx, r.TFResourceType())
		ctx = tflog.SubsystemSetField(ctx, r.TFResourceType(), "stage", "UpgradeIdentity")

		return rr.UpgradeIdentity(ctx)
	}
	return nil
}

// TODO: This doesn't support opt-in, either support or not support.
// IdentitySchema implements resource.ResourceWithIdentity.
// func (r resourceWrapper) IdentitySchema(ctx context.Context, req resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
// 	defer func() {
// 		r.logDiags(ctx, resp.Diagnostics)
// 	}()
//
// 	ctx = tflog.NewSubsystem(ctx, r.Resource.TFResourceType())
// 	ctx = tflog.SubsystemSetField(ctx, r.Resource.TFResourceType(), "stage", "IdentitySchema")
//
//  //TODO
//
// }

func (r resourceWrapper) RenderOption() tffwdocs.ResourceRenderOption {
	return r.Resource.RenderOption()
}

func (r resourceWrapper) logDiags(ctx context.Context, diags diag.Diagnostics) {
	for _, warning := range diags.Warnings() {
		r.Warn(ctx, fmt.Sprintf("%s: %s", warning.Summary(), warning.Detail()))
	}
	for _, err := range diags.Errors() {
		r.Warn(ctx, fmt.Sprintf("%s: %s", err.Summary(), err.Detail()))
	}
}
