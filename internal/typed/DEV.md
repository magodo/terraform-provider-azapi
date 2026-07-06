This is a devlog to put down some of the thoughts.

# Overall Process

Codegen reads from bicep types, which contains the basic API information (inc. API path, operation, version, data model of structure, type, and minimal attributes), generates the vanilla while injectable resource code (e.g. foo_resource_gen.go). This resource type should work out of the box in an ideal API.

The developer's main jobs are:

- Improve user experience in terms of validation, description, etc, which can be overriden programmably in a file besides `foo_resource_gen.go`), by enhancing the vanilla resource schema.
- For API special behavior, the developer should be also available to inject additional code to mitigate

In this process, the codegen shall take minimal input and solely just generate a vanilla resource implementation. Keep the customization programmable along side the generated source code.

# Expand/Flatten

The expand and flatten shall be a generic implementation that covers all the scenarios.

The first question is should this be a static or a dynamic implementation. We chose dynamic to cost minor performance for smaller size and cleaner codebase.

With this, we shall cover different nuances during expand/flatten to adopt to the target API behavior in terms of null/absent/zero value.

## Expand

API might differentiate a value for being null or absent, e.g. the request for an attribute `foo`:

- Null: `{"foo": null, ...}`
- Absent: `{...}`

The input to expand in this case we shall consider is null or unknown (for O+C without default value) TF values. Accordingly, we shall have an expand option to allow the users to specify a specific attribute path to opt-in one of the above behavior, with the "absent" case be the default behavior.

People might also argue that in some cases, there are APIs that expect a zero value in this case:

- TF `null` -> API "Zero" e.g. `{"foo": false/0/""/[]/{}, ...}`

Instead of making this an expand config, we shall simply declare the TF attribute O+C with a default value.


## Flatten

Due to the same reason, when we meet the API response, we need to do the reverse, but with a slight difference: Both `null` and absent in API response ends up to be TF value of `null` (as TF regards `null` and absense the same).

Again for the "zero" value case, we regard zero value has no difference than other known values. If there causes a diff due to the config/plan has a null value, it is reasonable enough to mark that attribute O+C, which reflects the actual API behavior (takes null returns zero).

## Naming Conversion

By convention, ARM API uses camelCase for API properties while TF uses snake_case. When expand/flatten, we need to convert the casing along the process. In many cases, this can be automatically done. Whilst for the abbreviations, the intent might be ambiguous. E.g. in API there is an attribute called: `fooID` and `fooId`, they shall be converted to TF as `foo_id`.

The plan is to keep the customization at the post-codegen phase:

- Override the schema to rename the TF attribute
- Specify the expand/flatten override to include this special mapping, instead of following the default mapping logic which takes each capital case letter as a new separation. This means `fooID` -> `foo_i_d`
