package storage

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

//go:generate go run ../../tools/codegen resource --api-type "Microsoft.Storage/storageAccounts@2025-06-01" --tf-type "azapi_storage_account" --remove-attr properties.privateEndpointConnections --add-attr properties.privateEndpointConnections.*.id
//go:generate go run ../../tools/codegen resource --api-type "Microsoft.Storage/storageAccounts/blobServices@2025-06-01" --tf-type "azapi_storage_account_blob_service"

func Resources() []func() resource.Resource {
	return []func() resource.Resource{
		framework.WrapResource(NewAzApiStorageAccountResource(), framework.ResourceOption{}),
		framework.WrapResource(NewAzApiStorageAccountBlobServiceResource(),
			framework.ResourceOption{
				SkipExistenceCheck: true,
				DeleteNoop:         true,
			},
		),
	}
}
