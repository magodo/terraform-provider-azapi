package main

import (
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/Azure/bicep-types/src/bicep-types-go/types"
)

// -----------------------------------------------------------------------------
// import set
// -----------------------------------------------------------------------------

type GoImportSet struct {
	m map[string]string
}

type goImport struct {
	path  string
	alias string
}

var (
	importContext            = goImport{path: "context"}
	importRegexp             = goImport{path: "regexp"}
	importMyValidator        = goImport{path: "github.com/Azure/terraform-provider-azapi/internal/services/myvalidator"}
	importFramework          = goImport{path: "github.com/Azure/terraform-provider-azapi/internal/typed/framework"}
	importModelConv          = goImport{path: "github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"}
	importServiceHooks       = goImport{path: "github.com/Azure/terraform-provider-azapi/internal/typed/servicehooks"}
	importTimeouts           = goImport{path: "github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"}
	importInt64Validator     = goImport{path: "github.com/hashicorp/terraform-plugin-framework-validators/int64validator"}
	importListValidator      = goImport{path: "github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"}
	importStringValidator    = goImport{path: "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"}
	importAttr               = goImport{path: "github.com/hashicorp/terraform-plugin-framework/attr"}
	importResourceSchema     = goImport{path: "github.com/hashicorp/terraform-plugin-framework/resource/schema"}
	importPlanModifier       = goImport{path: "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"}
	importStringPlanModifier = goImport{path: "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"}
	importValidator          = goImport{path: "github.com/hashicorp/terraform-plugin-framework/schema/validator"}
	importFrameworkTypes     = goImport{path: "github.com/hashicorp/terraform-plugin-framework/types"}
	importTFFrameworkDocs    = goImport{path: "github.com/magodo/terraform-plugin-framework-docs", alias: "tffwdocs"}
)

func newImportSet() *GoImportSet {
	return &GoImportSet{m: make(map[string]string)}
}

func (s *GoImportSet) add(value goImport) {
	if _, ok := s.m[value.path]; ok {
		return
	}
	s.m[value.path] = value.alias
}

func (s *GoImportSet) render() string {
	paths := make([]string, 0, len(s.m))
	for p := range s.m {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var b strings.Builder
	b.WriteString("import (\n")
	for _, p := range paths {
		if alias := s.m[p]; alias != "" {
			fmt.Fprintf(&b, "\t%s %q\n", alias, p)
		} else {
			fmt.Fprintf(&b, "\t%q\n", p)
		}
	}
	b.WriteString(")\n")
	return b.String()
}

// -----------------------------------------------------------------------------
// naming helpers
// -----------------------------------------------------------------------------

// serviceName derives the service/package name from the resource type, e.g.
// "Microsoft.Network/virtualNetworks" -> "network".
func (g *resourceGenerator) serviceName() string {
	namespace, _, _ := strings.Cut(g.apiResourceType, "/")
	segs := strings.Split(namespace, ".")
	return strings.ToLower(segs[len(segs)-1])
}

// fileBaseName strips the "azapi_" prefix from the TF type.
func (g *resourceGenerator) fileBaseName() string {
	return strings.TrimPrefix(g.tfType, "azapi_")
}

// structName produces e.g. "AzApiVirtualNetworkResource".
func (g *resourceGenerator) structName() string {
	return "AzApi" + pascalCase(g.fileBaseName()) + "Resource"
}

func pascalCase(snake string) string {
	var b strings.Builder
	for _, part := range strings.Split(snake, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}

// -----------------------------------------------------------------------------
// special (fixed) attributes
// -----------------------------------------------------------------------------

func specialParentIDAttribute() string {
	return `"parent_id": schema.StringAttribute{
Required: true,
PlanModifiers: []planmodifier.String{
stringplanmodifier.RequiresReplace(),
},
Validators: []validator.String{
myvalidator.StringIsResourceID(),
},
},
`
}

func specialNameAttribute() string {
	return `"name": schema.StringAttribute{
Required: true,
PlanModifiers: []planmodifier.String{
stringplanmodifier.RequiresReplace(),
},
},
`
}

func specialLocationAttribute() string {
	return `"location": schema.StringAttribute{
Required: true,
PlanModifiers: []planmodifier.String{
stringplanmodifier.RequiresReplace(),
},
},
`
}

func specialIDAttribute() string {
	return `"id": schema.StringAttribute{
Computed: true,
PlanModifiers: []planmodifier.String{
stringplanmodifier.UseStateForUnknown(),
},
},
`
}

func specialTimeoutsAttribute() string {
	return `"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
Create: true,
Read:   true,
Update: true,
Delete: true,
}),
`
}

// -----------------------------------------------------------------------------
// file assembly
// -----------------------------------------------------------------------------

func (g *resourceGenerator) renderFile(attrs string, res *types.ResourceType) ([]byte, error) {
	for _, value := range []goImport{
		importContext,
		importMyValidator,
		importFramework,
		importModelConv,
		importServiceHooks,
		importTimeouts,
		importResourceSchema,
		importPlanModifier,
		importStringPlanModifier,
		importValidator,
		importTFFrameworkDocs,
	} {
		g.goImports.add(value)
	}

	structName := g.structName()

	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\n", g.serviceName())
	b.WriteString(g.goImports.render())
	b.WriteString("\n")

	fmt.Fprintf(&b, "type %s struct {\n\thooks servicehooks.ResourceHooks\n}\n\n", structName)
	fmt.Fprintf(&b, "var _ framework.Resource = %s{}\n\n", structName)

	fmt.Fprintf(&b, "func (r %s) AzureResourceType() string {\n\treturn \"%s@%s\"\n}\n\n", structName, g.apiResourceType, g.apiVersion)
	fmt.Fprintf(&b, "func (r %s) TFResourceType() string {\n\treturn %q\n}\n\n", structName, g.tfType)

	// GetSchema
	fmt.Fprintf(&b, "func (r %s) GetSchema(ctx context.Context) schema.Schema {\n", structName)
	b.WriteString("schema := schema.Schema{\nAttributes: map[string]schema.Attribute{\n")
	b.WriteString(attrs)
	b.WriteString("},\n}\n")
	b.WriteString("if r.hooks.SchemaHook != nil {\nschema = r.hooks.SchemaHook(ctx, schema)\n}\nreturn schema\n}\n\n")

	// GetModelConvOption
	fmt.Fprintf(&b, "func (r %s) GetModelConvOption(ctx context.Context) *modelconv.Option {\n", structName)
	b.WriteString(g.renderModelConvOption())
	b.WriteString("if r.hooks.ModelConvOptionHook != nil {\nopt = r.hooks.ModelConvOptionHook(ctx, opt)\n}\nreturn &opt\n}\n\n")

	// RenderOption
	fmt.Fprintf(&b, "func (r %s) RenderOption() tffwdocs.ResourceRenderOption {\n", structName)
	fmt.Fprintf(&b, "opt := tffwdocs.ResourceRenderOption{\nSubcategory: %q,\nImportId: &tffwdocs.ImportId{\nFormat: \"<resource_id>\",\nExampleId: %q,\n},\n}\n",
		g.serviceName(), g.exampleResourceID(res))
	b.WriteString("if r.hooks.RenderOptionHook != nil {\nopt = r.hooks.RenderOptionHook(opt)\n}\nreturn opt\n}\n")

	src, err := format.Source([]byte(b.String()))
	if err != nil {
		return nil, fmt.Errorf("failed to format generated source: %w\n----\n%s", err, b.String())
	}
	return src, nil
}

func (g *resourceGenerator) renderModelConvOption() string {
	if len(g.nameOverrides) == 0 {
		return "opt := modelconv.NewDefaultOption()\n"
	}
	keys := make([]string, 0, len(g.nameOverrides))
	for k := range g.nameOverrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("opt := &modelconv.Option{\nNameOverrides: map[string]string{\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%q: %q,\n", k, g.nameOverrides[k])
	}
	b.WriteString("},\n}\n")
	return b.String()
}

// -----------------------------------------------------------------------------
// example resource id
// -----------------------------------------------------------------------------

func (g *resourceGenerator) exampleResourceID(res *types.ResourceType) string {
	namespace, rest, _ := strings.Cut(g.apiResourceType, "/")
	typeSegs := strings.Split(rest, "/")

	var b strings.Builder
	b.WriteString(scopePrefix(res.WritableScopes))
	fmt.Fprintf(&b, "/providers/%s", namespace)
	for _, seg := range typeSegs {
		fmt.Fprintf(&b, "/%s/%s", seg, examplePlaceholder(seg))
	}
	return b.String()
}

func scopePrefix(scopes types.ScopeType) string {
	switch {
	case scopes&types.ScopeTypeResourceGroup != 0 || scopes == types.ScopeTypeNone:
		return "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup"
	case scopes&types.ScopeTypeSubscription != 0:
		return "/subscriptions/00000000-0000-0000-0000-000000000000"
	case scopes&types.ScopeTypeManagementGroup != 0:
		return "/providers/Microsoft.Management/managementGroups/myManagementGroup"
	case scopes&types.ScopeTypeTenant != 0:
		return ""
	default:
		return "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myResourceGroup"
	}
}

func examplePlaceholder(typeSegment string) string {
	singular := strings.TrimSuffix(typeSegment, "s")
	if singular == "" {
		singular = typeSegment
	}
	return "my" + strings.ToUpper(singular[:1]) + singular[1:]
}
