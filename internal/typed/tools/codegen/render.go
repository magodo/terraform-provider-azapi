package main

import (
	"fmt"
	"go/format"
	"sort"
	"strings"

	"github.com/Azure/bicep-types/src/bicep-types-go/types"
)

// render.go implements the RENDER phase of the codegen pipeline: it walks the
// schema IR (see ir.go) and emits formatted Go source. It contains no bicep
// logic; that is the job of the BUILD phase (see resource_generator.go).

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
	importCustomTypes        = goImport{path: "github.com/Azure/terraform-provider-azapi/internal/typed/customtypes"}
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

// The special attributes are the fixed, hand-authored parts of every generated
// resource schema. They are modelled as ordinary IR so that the render phase
// handles them uniformly with the bicep-derived attributes.

func specialParentID() *Attribute {
	return &Attribute{Name: "parent_id", Type: StringAttr{
		Mode:          Required,
		PlanModifiers: []StringPlanModifier{RequiresReplace{}},
		Validators:    []StringValidator{StringIsResourceID{}},
	}}
}

func specialName() *Attribute {
	return &Attribute{Name: "name", Type: StringAttr{
		Mode:          Required,
		PlanModifiers: []StringPlanModifier{RequiresReplace{}},
	}}
}

func specialLocation() *Attribute {
	// The "location" attribute uses a dedicated CustomType so that its behavior
	// (e.g. semantic equality across casing/spacing) can be changed globally in
	// one place (see internal/typed/customtypes) without regenerating resources.
	return &Attribute{Name: "location", Type: StringAttr{
		Mode:          Required,
		CustomType:    "customtypes.LocationType{}",
		PlanModifiers: []StringPlanModifier{RequiresReplace{}},
	}}
}

func specialID() *Attribute {
	return &Attribute{Name: "id", Type: StringAttr{
		Mode:          Computed,
		PlanModifiers: []StringPlanModifier{UseStateForUnknown{}},
	}}
}

func specialTimeouts() *Attribute {
	return &Attribute{Name: "timeouts", Type: RawAttr{Code: `timeouts.Attributes(ctx, timeouts.Opts{
Create: true,
Read:   true,
Update: true,
Delete: true,
})`}}
}

// -----------------------------------------------------------------------------
// IR rendering
// -----------------------------------------------------------------------------

// renderAttributes renders an ordered list of attributes as the body of an
// attributes map: an optional leading comment followed by `"name": <value>,`.
func (g *resourceGenerator) renderAttributes(attrs []*Attribute) string {
	var b strings.Builder
	for _, a := range attrs {
		if a.Comment != "" {
			fmt.Fprintf(&b, "// %s\n", a.Comment)
		}
		fmt.Fprintf(&b, "%q: %s,\n", a.Name, g.renderAttributeType(a.Type))
	}
	return b.String()
}

func (g *resourceGenerator) renderAttributeType(t AttributeType) string {
	switch a := t.(type) {
	case StringAttr:
		return g.renderStringAttr(a)
	case BoolAttr:
		return "schema.BoolAttribute{\n" + behaviorLine(a.Mode) + descLine(a.Description) + sensitiveLine(a.Sensitive) + "}"
	case Int64Attr:
		return "schema.Int64Attribute{\n" + behaviorLine(a.Mode) + descLine(a.Description) + g.renderInt64Validators(a.Validators) + "}"
	case DynamicAttr:
		return "schema.DynamicAttribute{\n" + behaviorLine(a.Mode) + descLine(a.Description) + sensitiveLine(a.Sensitive) + "}"
	case SingleNestedAttr:
		return "schema.SingleNestedAttribute{\n" + behaviorLine(a.Mode) + descLine(a.Description) + sensitiveLine(a.Sensitive) +
			"Attributes: map[string]schema.Attribute{\n" + g.renderAttributes(a.Attributes) + "},\n}"
	case ListNestedAttr:
		return "schema.ListNestedAttribute{\n" + behaviorLine(a.Mode) + descLine(a.Description) + g.renderListValidators(a.Validators) +
			"NestedObject: schema.NestedAttributeObject{\nAttributes: map[string]schema.Attribute{\n" + g.renderAttributes(a.Attributes) + "},\n},\n}"
	case MapNestedAttr:
		return "schema.MapNestedAttribute{\n" + behaviorLine(a.Mode) + descLine(a.Description) + sensitiveLine(a.Sensitive) +
			"NestedObject: schema.NestedAttributeObject{\nAttributes: map[string]schema.Attribute{\n" + g.renderAttributes(a.Attributes) + "},\n},\n}"
	case ListAttr:
		return "schema.ListAttribute{\n" + behaviorLine(a.Mode) +
			fmt.Sprintf("ElementType: %s,\n", g.renderElemType(a.ElementType)) +
			descLine(a.Description) + g.renderListValidators(a.Validators) + "}"
	case MapAttr:
		return "schema.MapAttribute{\n" + behaviorLine(a.Mode) +
			fmt.Sprintf("ElementType: %s,\n", g.renderElemType(a.ElementType)) +
			descLine(a.Description) + sensitiveLine(a.Sensitive) + "}"
	case RawAttr:
		return a.Code
	default:
		panic(fmt.Sprintf("unknown attribute type %T", t))
	}
}

func (g *resourceGenerator) renderStringAttr(a StringAttr) string {
	var b strings.Builder
	b.WriteString("schema.StringAttribute{\n")
	if a.CustomType != "" {
		g.goImports.add(importCustomTypes)
		fmt.Fprintf(&b, "CustomType: %s,\n", a.CustomType)
	}
	b.WriteString(behaviorLine(a.Mode))
	b.WriteString(descLine(a.Description))
	b.WriteString(sensitiveLine(a.Sensitive))
	if len(a.PlanModifiers) > 0 {
		b.WriteString("PlanModifiers: []planmodifier.String{\n")
		for _, pm := range a.PlanModifiers {
			b.WriteString(g.renderStringPlanModifier(pm) + "\n")
		}
		b.WriteString("},\n")
	}
	if len(a.Validators) > 0 {
		b.WriteString("Validators: []validator.String{\n")
		for _, v := range a.Validators {
			b.WriteString(g.renderStringValidator(v) + "\n")
		}
		b.WriteString("},\n")
	}
	b.WriteString("}")
	return b.String()
}

func (g *resourceGenerator) renderStringValidator(v StringValidator) string {
	switch vv := v.(type) {
	case OneOf:
		g.goImports.add(importStringValidator)
		quoted := make([]string, len(vv.Values))
		for i, s := range vv.Values {
			quoted[i] = fmt.Sprintf("%q", s)
		}
		return fmt.Sprintf("stringvalidator.OneOf(%s),", strings.Join(quoted, ", "))
	case RegexMatches:
		g.goImports.add(importStringValidator)
		g.goImports.add(importRegexp)
		return fmt.Sprintf("stringvalidator.RegexMatches(regexp.MustCompile(%q), \"\"),", vv.Pattern)
	case LengthBetween:
		g.goImports.add(importStringValidator)
		return fmt.Sprintf("stringvalidator.LengthBetween(%d, %d),", vv.Min, vv.Max)
	case LengthAtLeast:
		g.goImports.add(importStringValidator)
		return fmt.Sprintf("stringvalidator.LengthAtLeast(%d),", vv.Min)
	case LengthAtMost:
		g.goImports.add(importStringValidator)
		return fmt.Sprintf("stringvalidator.LengthAtMost(%d),", vv.Max)
	case StringIsResourceID:
		g.goImports.add(importMyValidator)
		return "myvalidator.StringIsResourceID(),"
	default:
		panic(fmt.Sprintf("unknown string validator %T", v))
	}
}

func (g *resourceGenerator) renderInt64Validators(vs []Int64Validator) string {
	if len(vs) == 0 {
		return ""
	}
	g.goImports.add(importInt64Validator)
	var b strings.Builder
	b.WriteString("Validators: []validator.Int64{\n")
	for _, v := range vs {
		switch vv := v.(type) {
		case IntBetween:
			fmt.Fprintf(&b, "int64validator.Between(%d, %d),\n", vv.Min, vv.Max)
		case IntAtLeast:
			fmt.Fprintf(&b, "int64validator.AtLeast(%d),\n", vv.Min)
		case IntAtMost:
			fmt.Fprintf(&b, "int64validator.AtMost(%d),\n", vv.Max)
		default:
			panic(fmt.Sprintf("unknown int64 validator %T", v))
		}
	}
	b.WriteString("},\n")
	return b.String()
}

func (g *resourceGenerator) renderListValidators(vs []ListValidator) string {
	if len(vs) == 0 {
		return ""
	}
	g.goImports.add(importListValidator)
	var b strings.Builder
	b.WriteString("Validators: []validator.List{\n")
	for _, v := range vs {
		switch vv := v.(type) {
		case SizeBetween:
			fmt.Fprintf(&b, "listvalidator.SizeBetween(%d, %d),\n", vv.Min, vv.Max)
		case SizeAtLeast:
			fmt.Fprintf(&b, "listvalidator.SizeAtLeast(%d),\n", vv.Min)
		case SizeAtMost:
			fmt.Fprintf(&b, "listvalidator.SizeAtMost(%d),\n", vv.Max)
		default:
			panic(fmt.Sprintf("unknown list validator %T", v))
		}
	}
	b.WriteString("},\n")
	return b.String()
}

func (g *resourceGenerator) renderStringPlanModifier(pm StringPlanModifier) string {
	switch pm.(type) {
	case RequiresReplace:
		return "stringplanmodifier.RequiresReplace(),"
	case UseStateForUnknown:
		return "stringplanmodifier.UseStateForUnknown(),"
	default:
		panic(fmt.Sprintf("unknown string plan modifier %T", pm))
	}
}

func (g *resourceGenerator) renderElemType(e ElemType) string {
	g.goImports.add(importFrameworkTypes)
	switch et := e.(type) {
	case StringElem:
		return "types.StringType"
	case Int64Elem:
		return "types.Int64Type"
	case BoolElem:
		return "types.BoolType"
	case DynamicElem:
		return "types.DynamicType"
	case ListElem:
		return fmt.Sprintf("types.ListType{ElemType: %s}", g.renderElemType(et.Elem))
	case MapElem:
		return fmt.Sprintf("types.MapType{ElemType: %s}", g.renderElemType(et.Elem))
	case ObjectElem:
		g.goImports.add(importAttr)
		var b strings.Builder
		b.WriteString("types.ObjectType{AttrTypes: map[string]attr.Type{\n")
		for _, a := range et.Attributes {
			fmt.Fprintf(&b, "%q: %s,\n", a.Name, g.renderElemType(a.Type))
		}
		b.WriteString("}}")
		return b.String()
	default:
		panic(fmt.Sprintf("unknown elem type %T", e))
	}
}

func behaviorLine(b Mode) string {
	switch b {
	case Required:
		return "Required: true,\n"
	case Computed:
		return "Computed: true,\n"
	case Optional:
		return "Optional: true,\n"
	default:
		panic("unreachable behaviorLine")
	}
}

func descLine(desc string) string {
	if desc == "" {
		return ""
	}
	return fmt.Sprintf("MarkdownDescription: %q,\n", desc)
}

func sensitiveLine(sensitive bool) string {
	if !sensitive {
		return ""
	}
	return "Sensitive: true,\n"
}

// -----------------------------------------------------------------------------
// file assembly
// -----------------------------------------------------------------------------

func (g *resourceGenerator) renderFile(schema *Schema, res *types.ResourceType) ([]byte, error) {
	// Render the attributes first: this populates the dynamic imports (via the
	// g.goImports.add calls in the render helpers) before the import block is
	// emitted below.
	attrs := g.renderAttributes(schema.Attributes)

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
