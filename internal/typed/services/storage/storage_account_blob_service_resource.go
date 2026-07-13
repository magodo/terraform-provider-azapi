package storage

import "github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"

func NewAzApiStorageAccountBlobServiceResource() AzApiStorageAccountBlobServiceResource {
	return AzApiStorageAccountBlobServiceResource{
		hooks: servicehooks.ResourceHooks{},
	}
}
