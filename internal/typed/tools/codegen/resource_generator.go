package main

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	"github.com/Azure/bicep-types/src/bicep-types-go/index"
	"github.com/Azure/bicep-types/src/bicep-types-go/types"
	"github.com/Azure/terraform-provider-azapi/internal/azure"
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
)

type resourceGenerator struct {
	loader bicepTypeLoader

	apiResourceType string // "Microsoft.Network/virtualNetworks"
	apiVersion      string // "2025-01-01"
	tfType          string // "azapi_virtual_network"
	rules           []attrRule

	// nameOverrides records API(camel) path -> TF(snake) name, only when the
	// naive snake conversion differs from the smart one.
	nameOverrides map[string]string

	goImports *GoImportSet

	visiting map[bicepTypeKey]bool

	// computedDepth is > 0 while we are rendering the subtree of an attribute
	// that is Computed-only (ReadOnly, not Required).
	// Every descendant in such a subtree is forced to Computed-only as well,
	// regardless of its own bicep flags.
	computedDepth int
}

type attrRule struct {
	path    []string
	include bool
}

type resourceGeneratorOptions struct {
	apiType string
	tfType  string
	rules   []attrRule
}

func newResourceGenerator(options resourceGeneratorOptions) (*resourceGenerator, error) {
	apiResourceType, apiVersion, ok := strings.Cut(options.apiType, "@")
	if !ok {
		return nil, fmt.Errorf("invalid api type %q, expected format <ApiResourceType>@<ApiVersion>", options.apiType)
	}

	return &resourceGenerator{
		loader:          newBicepTypeLoader(),
		apiResourceType: apiResourceType,
		apiVersion:      apiVersion,
		tfType:          options.tfType,
		rules:           options.rules,
		nameOverrides:   map[string]string{},
		goImports:       newImportSet(),
		visiting:        map[bicepTypeKey]bool{},
	}, nil
}

// alwaysIgnoreAttributes lists API (camelCase) property names that are
// unconditionally excluded from the generated schema at any nesting level.
var alwaysIgnoreAttributes = map[string]bool{
	"etag":              true,
	"provisioningState": true,
	"systemData":        true,
}

// includePath reports whether the attribute at the given API path should be
// rendered, given the ordered rules.
func (g *resourceGenerator) includePath(apiPath []string) bool {
	// Always ignore certain attributes at any nesting level.
	if len(apiPath) > 0 {
		if alwaysIgnoreAttributes[apiPath[len(apiPath)-1]] {
			return false
		}
	}
	include := true
	for _, r := range g.rules {
		if len(r.path) <= len(apiPath) {
			if slices.Equal(r.path, apiPath[:len(r.path)]) {
				include = r.include
			}
		} else if r.include && slices.Equal(r.path[:len(apiPath)], apiPath) {
			include = true
		}
	}
	return include
}

// attrMode categorises an attribute for ordering purposes.
type attrMode int

const (
	modeRequired attrMode = iota
	modeOptional          // Optional, with or without Computed
	modeComputed
)

func modeOf(flags types.TypePropertyFlags) attrMode {
	switch {
	case flags&types.TypePropertyFlagsRequired != 0:
		return modeRequired
	case flags&types.TypePropertyFlagsReadOnly != 0:
		return modeComputed
	default:
		return modeOptional
	}
}

type attrEntry struct {
	tfName  string
	code    string
	comment string
	mode    attrMode
}

type attributeContext struct {
	file     string
	property types.ObjectTypeProperty
	apiPath  []string
}

type resolvedAttribute struct {
	typeInfo bicepType
	mode     string
	desc     string
	apiPath  []string
}

type renderedAttribute struct {
	code    string
	comment string
}

func (g *resourceGenerator) Generate() ([]byte, error) {
	indexFile := "generated/index.json"

	indexContent, err := azure.StaticFiles.ReadFile(indexFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema index: %w", err)
	}
	var idx index.TypeIndex
	if err := json.Unmarshal(indexContent, &idx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema index: %w", err)
	}

	ref, ok := idx.GetResource(g.apiResourceType, g.apiVersion)
	if !ok {
		return nil, fmt.Errorf("resource type %s@%s not found in the bicep index", g.apiResourceType, g.apiVersion)
	}

	resourceType, err := g.loader.resolve(indexFile, ref)
	if err != nil {
		return nil, err
	}
	rt, ok := resourceType.t.(*types.ResourceType)
	if !ok {
		return nil, fmt.Errorf("index entry for %s@%s is not a ResourceType (got %T)", g.apiResourceType, g.apiVersion, resourceType.t)
	}

	bodyType, err := g.loader.resolve(resourceType.key.file, rt.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource body: %w", err)
	}
	body, ok := bodyType.t.(*types.ObjectType)
	if !ok {
		return nil, fmt.Errorf("resource body is not an object type (got %T)", bodyType.t)
	}
	g.visiting = map[bicepTypeKey]bool{bodyType.key: true}

	_, hasLocation := body.Properties["location"]

	// Top-level bicep resource properties that we skip to convert to TF schema,
	// either because we don't export them or they have special rendering.
	var skipTopLevel = map[string]bool{
		"type":       true,
		"apiVersion": true,
		"name":       true, // rendered as the fixed "name" attribute
		"id":         true, // rendered as the fixed "id" attribute
		"location":   true, // rendered as the fixed "location" attribute (when present)
	}

	// Render the body-derived attributes (everything except the special ones),
	// bucketed by mode so that we can emit them in the desired order.
	var required, optional, computed []attrEntry
	for name, prop := range body.Properties {
		if skipTopLevel[name] {
			continue
		}
		apiPath := []string{name}
		if !g.includePath(apiPath) {
			continue
		}
		// Force the top-level "properties" attribute to always be Required.
		// The ARM body's "properties" wrapper is where the meaningful,
		// user-configurable resource fields live, so it should never be
		// optional/computed in the generated schema.
		if name == "properties" {
			prop.Flags = (prop.Flags &^ types.TypePropertyFlagsReadOnly) | types.TypePropertyFlagsRequired
		}
		tfName := g.tfName(apiPath)
		rendered, err := g.renderAttribute(attributeContext{file: bodyType.key.file, property: prop, apiPath: apiPath})
		if err != nil {
			return nil, fmt.Errorf("failed to render attribute %q: %w", name, err)
		}
		entry := attrEntry{tfName: tfName, code: rendered.code, comment: rendered.comment, mode: modeOf(prop.Flags)}
		switch entry.mode {
		case modeRequired:
			required = append(required, entry)
		case modeOptional:
			optional = append(optional, entry)
		case modeComputed:
			computed = append(computed, entry)
		}
	}
	byTFName := func(s []attrEntry) {
		sort.Slice(s, func(i, j int) bool { return s[i].tfName < s[j].tfName })
	}
	byTFName(required)
	byTFName(optional)
	byTFName(computed)

	// Assemble the attributes map in the desired order:
	//   parent_id, name, location?, <required>, <optional>, <computed>, id, timeouts
	var b strings.Builder
	b.WriteString(specialParentIDAttribute())
	b.WriteString(specialNameAttribute())
	if hasLocation {
		b.WriteString(specialLocationAttribute())
	}
	writeEntries := func(entries []attrEntry) {
		for _, a := range entries {
			if a.comment != "" {
				fmt.Fprintf(&b, "// %s\n", a.comment)
			}
			fmt.Fprintf(&b, "%q: %s,\n", a.tfName, a.code)
		}
	}
	writeEntries(required)
	writeEntries(optional)
	writeEntries(computed)
	b.WriteString(specialIDAttribute())
	b.WriteString(specialTimeoutsAttribute())

	return g.renderFile(b.String(), rt)
}

// -----------------------------------------------------------------------------
// Naming
// -----------------------------------------------------------------------------

// tfName converts the last segment of the given (camelCase) API path to snake
// case, recording an override when the naive and smart conversions disagree.
func (g *resourceGenerator) tfName(camelPath []string) string {
	name := camelPath[len(camelPath)-1]
	naive := modelconv.ToSnakeCaseNaive(name)
	smart := modelconv.ToSnakeCaseSmart(name)
	if naive != smart {
		g.nameOverrides[strings.Join(camelPath, ".")] = smart
	}
	return smart
}

// -----------------------------------------------------------------------------
// Attribute rendering
// -----------------------------------------------------------------------------

// renderAttribute renders a schema.Attribute value for the given property.
// apiPath is the full API (camelCase) path to this property, including its own
// name as the last element ("*" is used for array element boundaries).
// The returned comment, when non-empty, explains why a DynamicAttribute was
// emitted and should be placed on the line above the attribute key/value pair.
func (g *resourceGenerator) renderAttribute(ctx attributeContext) (renderedAttribute, error) {
	typeInfo, err := g.loader.resolve(ctx.file, ctx.property.Type)
	if err != nil {
		return renderedAttribute{}, err
	}
	resolved := resolvedAttribute{
		typeInfo: typeInfo,
		mode:     modeLine(ctx.property.Flags),
		desc:     descLine(ctx.property.Description),
		apiPath:  ctx.apiPath,
	}
	var result renderedAttribute

	// If this attribute is Computed-only, anything rendered underneath it must
	// also be Computed-only. Track this via computedDepth; renderChildren
	// consults it and rewrites each child's flags accordingly.
	if modeOf(ctx.property.Flags) == modeComputed {
		g.computedDepth++
		defer func() { g.computedDepth-- }()
	}

	switch tt := typeInfo.t.(type) {
	case *types.StringType:
		result.code = g.renderString(resolved, tt)
	case *types.StringLiteralType:
		result.code = g.renderStringLiteral(resolved, tt)
	case *types.IntegerType:
		result.code = g.renderInteger(resolved, tt)
	case *types.BooleanType:
		result.code = "schema.BoolAttribute{\n" + resolved.mode + resolved.desc + "}"
	case *types.UnionType:
		var ok bool
		result.code, ok = g.renderUnionAsStringish(resolved, tt)
		if !ok {
			log.Printf("[WARN] Unexpected union type of non-stringish variants in file %q: %v", typeInfo.key.file, ctx.apiPath)
			result.comment = "dynamic: union type with non-stringish variants"
			result.code = "schema.DynamicAttribute{\n" + resolved.mode + resolved.desc + "}"
		}
	case *types.AnyType:
		result.comment = "dynamic: bicep any type"
		result.code = "schema.DynamicAttribute{\n" + resolved.mode + resolved.desc + "}"
	case *types.BuiltInType:
		// There is no BuiltInType in the bicep generated types any more.
		log.Printf("[WARN] Unexpected BuiltInType found in file %q: %v", ctx.file, ctx.apiPath)
		result.comment = "dynamic: unexpected BuiltInType"
		result.code = "schema.DynamicAttribute{\n" + resolved.mode + resolved.desc + "}"
	case *types.ObjectType:
		result, err = g.renderObject(resolved, tt)
	case *types.ArrayType:
		result.code, err = g.renderArray(resolved, tt)
	case *types.DiscriminatedObjectType:
		// TODO: Discriminator needs a different solution instead of dynamic attribute, but more thoughts needed.
		panic(fmt.Sprintf("DiscriminatedObjectType not supported: %s at %s", ctx.file, ctx.apiPath))
	default:
		log.Printf("[WARN] Unexpected type %T found in file %q: %v", tt, ctx.file, ctx.apiPath)
		result.comment = fmt.Sprintf("dynamic: unexpected type %T", tt)
		result.code = "schema.DynamicAttribute{\n" + resolved.mode + resolved.desc + "}"
	}
	if err != nil {
		return renderedAttribute{}, err
	}
	return result, nil
}

func (g *resourceGenerator) renderStringLiteral(ctx resolvedAttribute, st *types.StringLiteralType) string {
	g.goImports.add(importStringValidator)
	var b strings.Builder
	b.WriteString("schema.StringAttribute{\n")
	b.WriteString(ctx.mode)
	b.WriteString(ctx.desc)
	if st.Sensitive {
		b.WriteString("Sensitive: true,\n")
	}
	fmt.Fprintf(&b, "Validators: []validator.String{\nstringvalidator.OneOf(%q),\n},\n", st.Value)
	b.WriteString("}")
	return b.String()
}

func (g *resourceGenerator) renderString(ctx resolvedAttribute, st *types.StringType) string {
	var b strings.Builder
	b.WriteString("schema.StringAttribute{\n")
	b.WriteString(ctx.mode)
	b.WriteString(ctx.desc)
	if st.Sensitive {
		b.WriteString("Sensitive: true,\n")
	}
	var vs []string
	if st.Pattern != "" {
		g.goImports.add(importStringValidator)
		g.goImports.add(importRegexp)
		vs = append(vs, fmt.Sprintf("stringvalidator.RegexMatches(regexp.MustCompile(%q), \"\"),", st.Pattern))
	}
	switch {
	case st.MinLength != nil && st.MaxLength != nil:
		g.goImports.add(importStringValidator)
		vs = append(vs, fmt.Sprintf("stringvalidator.LengthBetween(%d, %d),", *st.MinLength, *st.MaxLength))
	case st.MinLength != nil:
		g.goImports.add(importStringValidator)
		vs = append(vs, fmt.Sprintf("stringvalidator.LengthAtLeast(%d),", *st.MinLength))
	case st.MaxLength != nil:
		g.goImports.add(importStringValidator)
		vs = append(vs, fmt.Sprintf("stringvalidator.LengthAtMost(%d),", *st.MaxLength))
	}
	if len(vs) > 0 {
		b.WriteString("Validators: []validator.String{\n")
		for _, v := range vs {
			b.WriteString(v + "\n")
		}
		b.WriteString("},\n")
	}
	b.WriteString("}")
	return b.String()
}

func (g *resourceGenerator) renderInteger(ctx resolvedAttribute, it *types.IntegerType) string {
	var b strings.Builder
	b.WriteString("schema.Int64Attribute{\n")
	b.WriteString(ctx.mode)
	b.WriteString(ctx.desc)
	var v string
	switch {
	case it.MinValue != nil && it.MaxValue != nil:
		v = fmt.Sprintf("int64validator.Between(%d, %d),", *it.MinValue, *it.MaxValue)
	case it.MinValue != nil:
		v = fmt.Sprintf("int64validator.AtLeast(%d),", *it.MinValue)
	case it.MaxValue != nil:
		v = fmt.Sprintf("int64validator.AtMost(%d),", *it.MaxValue)
	}
	if v != "" {
		g.goImports.add(importInt64Validator)
		b.WriteString("Validators: []validator.Int64{\n" + v + "\n},\n")
	}
	b.WriteString("}")
	return b.String()
}

func (g *resourceGenerator) renderUnionAsStringish(ctx resolvedAttribute, ut *types.UnionType) (string, bool) {
	literals, stringish := g.unionStrings(ctx.typeInfo.key.file, ut)
	if !stringish {
		return "", false
	}
	var b strings.Builder
	b.WriteString("schema.StringAttribute{\n")
	b.WriteString(ctx.mode)
	b.WriteString(ctx.desc)
	if len(literals) > 0 {
		g.goImports.add(importStringValidator)
		quoted := make([]string, len(literals))
		for i, l := range literals {
			quoted[i] = fmt.Sprintf("%q", l)
		}
		fmt.Fprintf(&b, "Validators: []validator.String{\nstringvalidator.OneOf(%s),\n},\n", strings.Join(quoted, ", "))
	}
	b.WriteString("}")
	return b.String(), true
}

// unionStrings returns the string-literal values of a union and whether every
// element is string-ish (a StringLiteralType or StringType). literals is only
// fully populated (and OneOf-worthy) when every element is a StringLiteralType.
func (g *resourceGenerator) unionStrings(file string, ut *types.UnionType) (literals []string, stringish bool) {
	allLiteral := true
	stringish = true
	for _, e := range ut.Elements {
		typeInfo, err := g.loader.resolve(file, e)
		if err != nil {
			return nil, false
		}
		switch et := typeInfo.t.(type) {
		case *types.StringLiteralType:
			literals = append(literals, et.Value)
		case *types.StringType:
			allLiteral = false
		default:
			return nil, false
		}
	}
	if !allLiteral {
		literals = nil
	}
	return literals, stringish
}

func (g *resourceGenerator) renderObject(ctx resolvedAttribute, ot *types.ObjectType) (renderedAttribute, error) {
	var sens string
	if ot.Sensitive != nil && *ot.Sensitive {
		sens = "Sensitive: true,\n"
	}

	key := ctx.typeInfo.key
	if g.visiting[key] {
		// Self-referential type. Emit a dynamic attribute to break the cycle.
		return renderedAttribute{
			code:    "schema.DynamicAttribute{\n" + ctx.mode + ctx.desc + sens + "}",
			comment: "dynamic: self-referential object type",
		}, nil
	}
	g.visiting[key] = true
	defer delete(g.visiting, key)

	// Map-like object (no declared properties, only additionalProperties).
	if len(ot.Properties) == 0 && ot.AdditionalProperties != nil {
		element, err := g.loader.resolve(ctx.typeInfo.key.file, ot.AdditionalProperties)
		if err != nil {
			return renderedAttribute{}, err
		}
		if eo, ok := element.t.(*types.ObjectType); ok && len(eo.Properties) > 0 {
			// Guard the element too.
			ekey := element.key
			if !g.visiting[ekey] {
				g.visiting[ekey] = true
				defer delete(g.visiting, ekey)
				children, err := g.renderChildren(element.key.file, eo, append(slices.Clone(ctx.apiPath), "*"))
				if err != nil {
					return renderedAttribute{}, err
				}
				var b strings.Builder
				b.WriteString("schema.MapNestedAttribute{\n")
				b.WriteString(ctx.mode)
				b.WriteString(ctx.desc)
				b.WriteString(sens)
				b.WriteString("NestedObject: schema.NestedAttributeObject{\nAttributes: map[string]schema.Attribute{\n")
				b.WriteString(children)
				b.WriteString("},\n},\n}")
				return renderedAttribute{code: b.String()}, nil
			}
		}
		elemType, err := g.attrType(element.key.file, element.t)
		if err != nil {
			return renderedAttribute{}, err
		}
		var b strings.Builder
		b.WriteString("schema.MapAttribute{\n")
		b.WriteString(ctx.mode)
		b.WriteString(fmt.Sprintf("ElementType: %s,\n", elemType))
		b.WriteString(ctx.desc)
		b.WriteString(sens)
		b.WriteString("}")
		return renderedAttribute{code: b.String()}, nil
	}

	// Plain object with no properties at all -> dynamic.
	if len(ot.Properties) == 0 {
		return renderedAttribute{
			code:    "schema.DynamicAttribute{\n" + ctx.mode + ctx.desc + sens + "}",
			comment: "dynamic: object type with no properties",
		}, nil
	}

	children, err := g.renderChildren(ctx.typeInfo.key.file, ot, ctx.apiPath)
	if err != nil {
		return renderedAttribute{}, err
	}
	var b strings.Builder
	b.WriteString("schema.SingleNestedAttribute{\n")
	b.WriteString(ctx.mode)
	b.WriteString(ctx.desc)
	b.WriteString(sens)
	b.WriteString("Attributes: map[string]schema.Attribute{\n")
	b.WriteString(children)
	b.WriteString("},\n}")
	return renderedAttribute{code: b.String()}, nil
}

func (g *resourceGenerator) renderArray(ctx resolvedAttribute, at *types.ArrayType) (string, error) {
	item, err := g.loader.resolve(ctx.typeInfo.key.file, at.ItemType)
	if err != nil {
		return "", err
	}
	listVals := g.listValidators(at)

	if io, ok := item.t.(*types.ObjectType); ok && len(io.Properties) > 0 {
		key := item.key
		if g.visiting[key] {
			// Cycle: fall back to a dynamic list.
			g.goImports.add(importFrameworkTypes)
			var b strings.Builder
			b.WriteString("schema.ListAttribute{\n")
			b.WriteString(ctx.mode)
			b.WriteString("ElementType: types.DynamicType,\n")
			b.WriteString(ctx.desc)
			b.WriteString(listVals)
			b.WriteString("}")
			return b.String(), nil
		}
		g.visiting[key] = true
		defer delete(g.visiting, key)

		children, err := g.renderChildren(item.key.file, io, append(slices.Clone(ctx.apiPath), "*"))
		if err != nil {
			return "", err
		}
		var b strings.Builder
		b.WriteString("schema.ListNestedAttribute{\n")
		b.WriteString(ctx.mode)
		b.WriteString(ctx.desc)
		b.WriteString(listVals)
		b.WriteString("NestedObject: schema.NestedAttributeObject{\nAttributes: map[string]schema.Attribute{\n")
		b.WriteString(children)
		b.WriteString("},\n},\n}")
		return b.String(), nil
	}

	elemType, err := g.attrType(item.key.file, item.t)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("schema.ListAttribute{\n")
	b.WriteString(ctx.mode)
	b.WriteString(fmt.Sprintf("ElementType: %s,\n", elemType))
	b.WriteString(ctx.desc)
	b.WriteString(listVals)
	b.WriteString("}")
	return b.String(), nil
}

func (g *resourceGenerator) listValidators(at *types.ArrayType) string {
	var v string
	switch {
	case at.MinLength != nil && at.MaxLength != nil:
		v = fmt.Sprintf("listvalidator.SizeBetween(%d, %d),", *at.MinLength, *at.MaxLength)
	case at.MinLength != nil:
		v = fmt.Sprintf("listvalidator.SizeAtLeast(%d),", *at.MinLength)
	case at.MaxLength != nil:
		v = fmt.Sprintf("listvalidator.SizeAtMost(%d),", *at.MaxLength)
	}
	if v == "" {
		return ""
	}
	g.goImports.add(importListValidator)
	return "Validators: []validator.List{\n" + v + "\n},\n"
}

// renderChildren renders the attributes of an object's properties, ordered as
// Required -> Optional -> Computed, alphabetically within each group.
func (g *resourceGenerator) renderChildren(file string, ot *types.ObjectType, camelPath []string) (string, error) {
	var required, optional, computed []attrEntry
	for name, prop := range ot.Properties {
		childPath := append(slices.Clone(camelPath), name)
		if !g.includePath(childPath) {
			continue
		}
		// Under a Computed-only ancestor, force every descendant to be
		// Computed-only regardless of its own bicep flags. This handles
		// shared models that are reused between configurable and Computed
		// contexts (e.g. a status/props sub-object referenced from a
		// read-only parent).
		if g.computedDepth > 0 {
			prop.Flags = types.TypePropertyFlagsReadOnly
		}
		tfName := g.tfName(childPath)
		rendered, err := g.renderAttribute(attributeContext{file: file, property: prop, apiPath: childPath})
		if err != nil {
			return "", err
		}
		entry := attrEntry{tfName: tfName, code: rendered.code, comment: rendered.comment, mode: modeOf(prop.Flags)}
		switch entry.mode {
		case modeRequired:
			required = append(required, entry)
		case modeOptional:
			optional = append(optional, entry)
		case modeComputed:
			computed = append(computed, entry)
		}
	}
	byTFName := func(s []attrEntry) {
		sort.Slice(s, func(i, j int) bool { return s[i].tfName < s[j].tfName })
	}
	byTFName(required)
	byTFName(optional)
	byTFName(computed)

	var b strings.Builder
	for _, group := range [][]attrEntry{required, optional, computed} {
		for _, e := range group {
			if e.comment != "" {
				fmt.Fprintf(&b, "// %s\n", e.comment)
			}
			fmt.Fprintf(&b, "%q: %s,\n", e.tfName, e.code)
		}
	}
	return b.String(), nil
}

// -----------------------------------------------------------------------------
// attr.Type expressions (for ElementType of list/map attributes)
// -----------------------------------------------------------------------------

func (g *resourceGenerator) attrType(file string, t types.Type) (string, error) {
	return g.attrTypeGuarded(file, t, map[bicepTypeKey]bool{})
}

func (g *resourceGenerator) attrTypeGuarded(file string, t types.Type, seen map[bicepTypeKey]bool) (string, error) {
	switch tt := t.(type) {
	case *types.StringType, *types.StringLiteralType:
		g.goImports.add(importFrameworkTypes)
		return "types.StringType", nil
	case *types.IntegerType:
		g.goImports.add(importFrameworkTypes)
		return "types.Int64Type", nil
	case *types.BooleanType:
		g.goImports.add(importFrameworkTypes)
		return "types.BoolType", nil
	case *types.AnyType, *types.BuiltInType, *types.DiscriminatedObjectType:
		g.goImports.add(importFrameworkTypes)
		return "types.DynamicType", nil
	case *types.UnionType:
		if _, stringish := g.unionStrings(file, tt); stringish {
			g.goImports.add(importFrameworkTypes)
			return "types.StringType", nil
		}
		g.goImports.add(importFrameworkTypes)
		return "types.DynamicType", nil
	case *types.ArrayType:
		item, err := g.loader.resolve(file, tt.ItemType)
		if err != nil {
			return "", err
		}
		et, err := g.attrTypeGuarded(item.key.file, item.t, seen)
		if err != nil {
			return "", err
		}
		g.goImports.add(importFrameworkTypes)
		return fmt.Sprintf("types.ListType{ElemType: %s}", et), nil
	case *types.ObjectType:
		if len(tt.Properties) == 0 && tt.AdditionalProperties != nil {
			element, err := g.loader.resolve(file, tt.AdditionalProperties)
			if err != nil {
				return "", err
			}
			et, err := g.attrTypeGuarded(element.key.file, element.t, seen)
			if err != nil {
				return "", err
			}
			g.goImports.add(importFrameworkTypes)
			return fmt.Sprintf("types.MapType{ElemType: %s}", et), nil
		}
		if len(tt.Properties) == 0 {
			g.goImports.add(importFrameworkTypes)
			return "types.DynamicType", nil
		}
		names := make([]string, 0, len(tt.Properties))
		for name := range tt.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		g.goImports.add(importFrameworkTypes)
		g.goImports.add(importAttr)
		b.WriteString("types.ObjectType{AttrTypes: map[string]attr.Type{\n")
		for _, name := range names {
			child, err := g.loader.resolve(file, tt.Properties[name].Type)
			if err != nil {
				return "", err
			}
			// Cycle protection for nested object attrType construction.
			ckey := child.key
			if seen[ckey] {
				fmt.Fprintf(&b, "%q: types.DynamicType,\n", modelconv.ToSnakeCaseSmart(name))
				continue
			}
			seen[ckey] = true
			et, err := g.attrTypeGuarded(child.key.file, child.t, seen)
			delete(seen, ckey)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "%q: %s,\n", modelconv.ToSnakeCaseSmart(name), et)
		}
		b.WriteString("}}")
		return b.String(), nil
	default:
		g.goImports.add(importFrameworkTypes)
		return "types.DynamicType", nil
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func modeLine(flags types.TypePropertyFlags) string {
	switch modeOf(flags) {
	case modeRequired:
		return "Required: true,\n"
	case modeComputed:
		return "Computed: true,\n"
	case modeOptional:
		return "Optional: true,\n"
	default:
		panic("unreachable modeLine")
	}
}

func descLine(desc string) string {
	if desc == "" {
		return ""
	}
	return fmt.Sprintf("MarkdownDescription: %q,\n", desc)
}
