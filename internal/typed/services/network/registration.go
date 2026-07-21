package network

import (
	"github.com/Azure/terraform-provider-azapi/internal/typed/framework"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

//go:generate go run ../../tools/codegen resource --api-type "Microsoft.Network/virtualNetworks@2025-01-01" --tf-type "azapi_virtual_network" --remove-attr properties.subnets --remove-attr properties.virtualNetworkPeerings --remove-attr properties.virtualNetworkPeerings --remove-attr properties.flowLogs --add-attr properties.flowLogs.*.id

func Resources() []func() resource.Resource {
	return []func() resource.Resource{
		framework.WrapResource(NewAzApiVirtualNetworkResource(), framework.ResourceOption{}),
	}
}
