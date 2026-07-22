package resource

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

//go:generate go run ../../tools/codegen resource --api-type "Microsoft.Resources/resourceGroups@2025-04-01" --tf-type "azapi_resource_group"

func Resources() []func() resource.Resource {
	return []func() resource.Resource{
		framework.WrapResource(NewAzApiResourceGroupResource(), framework.ResourceOption{}),
	}
}
