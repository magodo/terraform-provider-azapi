This package implements static typed azapi resources.

# Goal

Support static typed azapi resources to improve the quality and user experience comparing to the current dynamic azapi resource. 

# Prerequisites

A good understand of the [`terraform-plugin-framework`](https://developer.hashicorp.com/terraform/plugin/framework) concepts and APIs is needed as most of the APIs and types used are all derived natively from `terraform-plugin-framework`.

# Princiles

- **API and Terraform resource 1:1 mapping**: Note that currently we only consider to support control plane APIs, whilst supporting data plane APIs shall follow the similar process.
- **Terraform Schema == API model**: Note the naming convention is different though (sname_case in TF while camelCase in API).
- **Whitebox lifecycle operations**: Each resource shall follow the same and simple lifecycle operation patterns, i.e. `PUT` for Create, `GET` for Read, `GET`-then-`PUT` for Update, `DELETE` for delete. Only for some edge cases (e.g. a post-write polling is needed for mitigating ARM eventual consistency issue), we shall not introduce additional lifecycle API calls. This also means we shall avoid introducing *artificial* properties at the resource type level to control a resource's runtime behavior.

# Project Structure

```
internal/typed
├── framework                                         # A thin framework layer on top of the terraform-plugin-framework
│   ├── resource.go                                   # Interface for Managed Resource (mandatory + opt-in)
│   ├── resource_wrapper.go                           # A resource wrapper that implements the `terraform-plugin-framework` resource
│   └── ...                                           
├── modelconv                                         # API<->TF data model converter, implements the expand/flatten feature
│   └── ...
├── servicehooks                                      # Service hooks
│   ├── resource.go                                   # Hook type for Managed Resource
│   └── ...
├── services
│   ├── network                                       # Service package which groups all the resources under this service.
│   │   ├── registration.go                           # Registration for resources under this service. This file also contains the `go:generate` directives to generate resources.
│   │   ├── virtual_network_resource_gen.go           # The generated resource from bicep. Shall not edit.
│   │   └── virtual_network_resource.go               # The customizations applied on top of the generated resource above.
│   └── storage
│       └── ...
└── tools
    └── codegen                                       # The code generator for generating a single resource from bicep type.
        └── ...
```

# Overall Process

## 1. Generate Resource From API

The tool under `internal/typed/tools/codegen` is a code generator that generates a static typed AzAPI resources from the Azure bicep types.

Given a single API type at a single API version, it emits a vanilla `<name>_resource_gen.go` whose schema *mirrors* the API model. The generated code is meant to be committed as-is and never hand-edited. This is a genuin translation from the corresponding API model to the Terraform schema, with all the bicep information reserved (e.g. description, validation rules, require/read-only, sensitivity, etc.), except that the namings are converted form camelCase to snake_case. 

The developer is supposed to finalize the complete CLI invocation and put it to the corresponding service's `registration.go` file as a *go generate* directive, e.g. in file `internal/typed/services/storage/registration.go`:

```
//go:generate go run ../../tools/codegen resource --api-type "Microsoft.Storage/storageAccounts@2025-06-01" --tf-type "azapi_storage_account" --remove-attr properties.privateEndpointConnections --add-attr properties.privateEndpointConnections.*.id --remove-attr properties.encryption.services.blob.lastEnabledTime --remove-attr properties.encryption.services.table.lastEnabledTime --remove-attr properties.encryption.services.queue.lastEnabledTime --remove-attr properties.encryption.services.file.lastEnabledTime
//go:generate go run ../../tools/codegen resource --api-type "Microsoft.Storage/storageAccounts/blobServices@2025-06-01" --tf-type "azapi_storage_account_blob_service"
```

Then you can simply run `go generate ./internal/typed/...` to (re)generate all the registered resources (Note: running `go generate` under the project root will involve other time-consuming generate tasks).

## 2. Customize the Resource 

Based on the actual Azure bicep types quality/correctness, as well as the corresponding API behavior, we might have to adjust facts of the generated resource above. These changes shall be reside in a sibiling file named `<name>_resource.go` besides the `<name>_resource_gen.go`.

The customizations, based on their purpose, can be categorized into three classes:

1. Lifecycle pattern: A regular resource-like API shall support CRUD. Whilst there are also APIs that might only support some of them. One can specify these high level lifecycle behavior via `framework.ResourceOption`.

2. Resource behavior: In `internal/typed/framework/resource.go`, there defines a list of opt-in interfaces that one resource can implement. These includes all the `terraform-plugin-framework` native interfaces, plus the following interfaces:

    - `PostCreate()`
    - `PostUpdate()`
    - `PostDelete()`

3. Resource data : Each resource's Go structure contains a `ResourceHooks` member, which provides a seam to modify the resource bound data. Currently, the data contains:

    - Schema: Can be modified by the `SchemaHook()`
    - Expand/Flatten Option (aka. ModelConv Option): Can be modified by `ModelConvOptionHook()`
    - Document Option: Can be modified by `RenderOptionHook()`

**NOTE** Even if there is no customization is needed, the `<name>_resource.go` is still needed to have the minimal content like below, e.g.:

```go
func NewAzApiVirtualNetworkResource() AzApiVirtualNetworkResource {
	return AzApiVirtualNetworkResource{}
```

## 3. Test

TBD

## 4. Document

The document shall be generated by using the `internal/tools/gendoc`, though the work is not yet done.
