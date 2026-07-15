package storage

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

//go:generate go run ../../tools/codegen resource --api-type "Microsoft.Storage/storageAccounts/blobServices@2025-06-01" --tf-type "azapi_storage_account_blob_service"

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
