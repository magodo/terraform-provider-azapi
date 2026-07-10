package servicehooks

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
)

type ResourceHooks struct {
	// SchemaHook patches the schema
	SchemaHook func(schema.Schema) schema.Schema

	// ModelConvOptionHook patches the modelconv.Option
	ModelConvOptionHook func(*modelconv.Option) *modelconv.Option

	// RenderOptionHook patches the tffwdocs.ResourceRenderOption
	RenderOptionHook func(tffwdocs.ResourceRenderOption) tffwdocs.ResourceRenderOption
}
