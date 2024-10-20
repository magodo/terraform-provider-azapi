terraform {
  required_providers {
    azapi = {
      source = "Azure/azapi"
    }
  }
}

provider "azurerm" {
  features {}
}

provider "azapi" {
  schema_validation_enabled = false
}

resource "azurerm_resource_group" "test" {
  name     = "acctest-rg"
  location = "West Europe"
}

resource "azurerm_container_registry" "acr" {
  name                = "acctest"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location
  sku                 = "Premium"
  admin_enabled       = false
}

resource "azapi_patch_resource" "test" {
  resource_id = azurerm_container_registry.acr.id
  type = "Microsoft.ContainerRegistry/registries@2020-11-01-preview"
  body = <<BODY
{
  "properties": {
    "anonymousPullEnabled": true
  }
}
  BODY

}