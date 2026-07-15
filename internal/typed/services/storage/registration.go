package storage

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func Resources() []func() resource.Resource {
	return []func() resource.Resource{
		framework.WrapResource(NewAzApiStorageAccountBlobServiceResource(),
			framework.ResourceOption{
				SkipExistenceCheck: true,
				DeleteNoop:         true,
			},
		),
	}
}
