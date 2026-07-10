package framework

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
)

// Resource interface defines the mandatory methods that a resource requires to implement.
type Resource interface {
	// TFResourceType returns the terraform type of a resource/data source. E.g. azapi_virtual_network.
	TFResourceType() string

	// AzureResourceType returns the Azure resource type in the format of "<resourceType>@<api-version>".
	// E.g. Microsoft.Network/virtualNetworks@2022-07-01
	AzureResourceType() string

	// Get the resource schema.
	GetSchema(context.Context) schema.Schema

	// ResourceWithRenderOption implements the interface to generate document.
	RenderOption() tffwdocs.ResourceRenderOption
}

// ResourceTimeout specifies the default timeout value for each operation.
type ResourceTimeout struct {
	Create time.Duration
	Read   time.Duration
	Update time.Duration
	Delete time.Duration
}

// ResourceWithTimeout is an opt-in interface that can implement customized timeout.
type ResourceWithTimeout interface {
	Resource

	// Timeout returns the timeout for each operation.
	Timeout() ResourceTimeout
}

// ResourceWithConfigValidators is an opt-in interface that can implement customized ConfigValidators.
type ResourceWithConfigValidators interface {
	Resource

	ConfigValidators(context.Context) []resource.ConfigValidator
}

// ResourceWithModifyPlan is an opt-in interface that can implement customized ModifyPlan.
type ResourceWithModifyPlan interface {
	Resource

	ModifyPlan(context.Context, resource.ModifyPlanRequest, *resource.ModifyPlanResponse)
}

// ResourceWithMoveState is an opt-in interface that can implement customized MoveState.
type ResourceWithMoveState interface {
	Resource

	MoveState(context.Context) []resource.StateMover
}

// ResourceWithUpgradeState is an opt-in interface that can implement customized UpgradeState.
type ResourceWithUpgradeState interface {
	Resource

	UpgradeState(context.Context) map[int64]resource.StateUpgrader
}

// ResourceWithValidateConfig is an opt-in interface that can implement customized ValidateConfig.
type ResourceWithValidateConfig interface {
	Resource

	ValidateConfig(context.Context, resource.ValidateConfigRequest, *resource.ValidateConfigResponse)
}

// ResourceWithUpgradeIdentity is an opt-in interface that can implement customized UpgradeIdentity.
type ResourceWithUpgradeIdentity interface {
	Resource

	UpgradeIdentity(context.Context) map[int64]resource.IdentityUpgrader
}
