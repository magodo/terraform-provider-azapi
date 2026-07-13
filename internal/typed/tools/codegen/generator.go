package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Azure/bicep-types/src/bicep-types-go/types"
	"github.com/Azure/terraform-provider-azapi/internal/typed/modelconv"
)

// generator holds the state required to render a single typed resource.
type generator struct {
	loader *typeLoader

	apiType      string // "Microsoft.Network/virtualNetworks@2025-01-01"
	resourceType string // "Microsoft.Network/virtualNetworks"
	apiVersion   string // "2025-01-01"
	tfType       string // "azapi_virtual_network"

	// nameOverrides records API(camel) path -> TF(snake) name, only when the
	// naive snake conversion differs from the smart one (so expand/flatten can
	// reverse the mapping).
	nameOverrides map[string]string

	imports *importSet

	// visiting tracks bicep types currently in the render stack so we can break
	// self-referential cycles (e.g. Subnet.ipConfigurations[*].subnet -> Subnet).
	visiting map[typeKey]bool
}

type typeKey struct {
	file string
	ref  int
}

// Top-level ARM envelope properties that receive special handling and are never
// rendered from the bicep body directly.
var skipTopLevel = map[string]bool{
	"type":       true,
	"apiVersion": true,
	"name":       true, // rendered as the fixed "name" attribute
	"id":         true, // rendered as the fixed "id" attribute
	"location":   true, // rendered as the fixed "location" attribute (when present)
}

func (g *generator) Generate(res *resolvedResource) ([]byte, error) {
	bodyType, bodyFile, bodyRef, err := g.loader.Resolve(res.file, res.resource.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource body: %w", err)
	}
	body, ok := bodyType.(*types.ObjectType)
	if !ok {
		return nil, fmt.Errorf("resource body is not an object type (got %T)", bodyType)
	}
	g.visiting = map[typeKey]bool{{file: bodyFile, ref: bodyRef}: true}

	_, hasLocation := body.Properties["location"]

	// Render the body-derived attributes (everything except the special ones).
	type attrEntry struct {
		tfName string
		code   string
	}
	var bodyAttrs []attrEntry
	for name, prop := range body.Properties {
		if skipTopLevel[name] {
			continue
		}
		camelPath := []string{name}
		tfName := g.tfName(camelPath)
		code, err := g.renderAttribute(bodyFile, prop, camelPath)
		if err != nil {
			return nil, fmt.Errorf("failed to render attribute %q: %w", name, err)
		}
		bodyAttrs = append(bodyAttrs, attrEntry{tfName: tfName, code: code})
	}
	sort.Slice(bodyAttrs, func(i, j int) bool { return bodyAttrs[i].tfName < bodyAttrs[j].tfName })

	// Assemble the attributes map, in a deterministic, readable order.
	var attrs strings.Builder
	attrs.WriteString(specialNameAttribute())
	attrs.WriteString(specialParentIDAttribute())
	if hasLocation {
		attrs.WriteString(specialLocationAttribute())
	}
	for _, a := range bodyAttrs {
		fmt.Fprintf(&attrs, "%q: %s,\n", a.tfName, a.code)
	}
	attrs.WriteString(specialIDAttribute())
	attrs.WriteString(specialTimeoutsAttribute())

	return g.renderFile(attrs.String(), res.resource)
}

// -----------------------------------------------------------------------------
// Naming
// -----------------------------------------------------------------------------

// tfName converts the last segment of the given (camelCase) API path to snake
// case, recording an override when the naive and smart conversions disagree.
func (g *generator) tfName(camelPath []string) string {
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
// camelPath is the full API (camelCase) path to this property, including its own
// name as the last element ("*" is used for array element boundaries).
func (g *generator) renderAttribute(file string, prop types.ObjectTypeProperty, camelPath []string) (string, error) {
	t, tfile, tref, err := g.loader.Resolve(file, prop.Type)
	if err != nil {
		return "", err
	}
	mode := modeLine(prop.Flags)
	desc := descLine(prop.Description)

	switch tt := t.(type) {
	case *types.StringType:
		return g.renderString(mode, desc, tt), nil
	case *types.StringLiteralType:
		return "schema.StringAttribute{\n" + mode + desc + "}", nil
	case *types.IntegerType:
		return g.renderInteger(mode, desc, tt), nil
	case *types.BooleanType:
		return "schema.BoolAttribute{\n" + mode + desc + "}", nil
	case *types.UnionType:
		return g.renderUnion(tfile, mode, desc, tt), nil
	case *types.AnyType:
		return "schema.DynamicAttribute{\n" + mode + desc + "}", nil
	case *types.BuiltInType:
		return "schema.DynamicAttribute{\n" + mode + desc + "}", nil
	case *types.ObjectType:
		return g.renderObject(tfile, tref, mode, desc, tt, camelPath)
	case *types.ArrayType:
		return g.renderArray(tfile, mode, desc, tt, camelPath)
	case *types.DiscriminatedObjectType:
		// Discriminated unions do not have a clean typed representation; fall
		// back to a dynamic attribute so the resource remains usable.
		return "schema.DynamicAttribute{\n" + mode + desc + "}", nil
	default:
		return "schema.DynamicAttribute{\n" + mode + desc + "}", nil
	}
}

func (g *generator) renderString(mode, desc string, st *types.StringType) string {
	var b strings.Builder
	b.WriteString("schema.StringAttribute{\n")
	b.WriteString(mode)
	b.WriteString(desc)
	if st.Sensitive {
		b.WriteString("Sensitive: true,\n")
	}
	var vs []string
	if st.Pattern != "" {
		g.imports.add("github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator", "")
		g.imports.add("regexp", "")
		vs = append(vs, fmt.Sprintf("stringvalidator.RegexMatches(regexp.MustCompile(%q), \"\"),", st.Pattern))
	}
	switch {
	case st.MinLength != nil && st.MaxLength != nil:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator", "")
		vs = append(vs, fmt.Sprintf("stringvalidator.LengthBetween(%d, %d),", *st.MinLength, *st.MaxLength))
	case st.MinLength != nil:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator", "")
		vs = append(vs, fmt.Sprintf("stringvalidator.LengthAtLeast(%d),", *st.MinLength))
	case st.MaxLength != nil:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator", "")
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

func (g *generator) renderInteger(mode, desc string, it *types.IntegerType) string {
	var b strings.Builder
	b.WriteString("schema.Int64Attribute{\n")
	b.WriteString(mode)
	b.WriteString(desc)
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
		g.imports.add("github.com/hashicorp/terraform-plugin-framework-validators/int64validator", "")
		b.WriteString("Validators: []validator.Int64{\n" + v + "\n},\n")
	}
	b.WriteString("}")
	return b.String()
}

func (g *generator) renderUnion(file, mode, desc string, ut *types.UnionType) string {
	literals, stringish := g.unionStrings(file, ut)
	if !stringish {
		return "schema.DynamicAttribute{\n" + mode + desc + "}"
	}
	var b strings.Builder
	b.WriteString("schema.StringAttribute{\n")
	b.WriteString(mode)
	b.WriteString(desc)
	if len(literals) > 0 {
		g.imports.add("github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator", "")
		quoted := make([]string, len(literals))
		for i, l := range literals {
			quoted[i] = fmt.Sprintf("%q", l)
		}
		fmt.Fprintf(&b, "Validators: []validator.String{\nstringvalidator.OneOf(%s),\n},\n", strings.Join(quoted, ", "))
	}
	b.WriteString("}")
	return b.String()
}

// unionStrings returns the string-literal values of a union and whether every
// element is string-ish (a StringLiteralType or StringType). literals is only
// fully populated (and OneOf-worthy) when every element is a StringLiteralType.
func (g *generator) unionStrings(file string, ut *types.UnionType) (literals []string, stringish bool) {
	allLiteral := true
	stringish = true
	for _, e := range ut.Elements {
		t, _, _, err := g.loader.Resolve(file, e)
		if err != nil {
			return nil, false
		}
		switch et := t.(type) {
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

func (g *generator) renderObject(file string, ref int, mode, desc string, ot *types.ObjectType, camelPath []string) (string, error) {
	key := typeKey{file: file, ref: ref}
	if g.visiting[key] {
		// Self-referential type. Emit a dynamic attribute to break the cycle.
		return "schema.DynamicAttribute{\n" + mode + desc + "}", nil
	}
	g.visiting[key] = true
	defer delete(g.visiting, key)

	// Map-like object (no declared properties, only additionalProperties).
	if len(ot.Properties) == 0 && ot.AdditionalProperties != nil {
		elemT, elemFile, elemRef, err := g.loader.Resolve(file, ot.AdditionalProperties)
		if err != nil {
			return "", err
		}
		if eo, ok := elemT.(*types.ObjectType); ok && len(eo.Properties) > 0 {
			// Guard the element too.
			ekey := typeKey{file: elemFile, ref: elemRef}
			if !g.visiting[ekey] {
				g.visiting[ekey] = true
				defer delete(g.visiting, ekey)
				children, err := g.renderChildren(elemFile, eo, appendPath(camelPath, "*"))
				if err != nil {
					return "", err
				}
				var b strings.Builder
				b.WriteString("schema.MapNestedAttribute{\n")
				b.WriteString(mode)
				b.WriteString(desc)
				b.WriteString("NestedObject: schema.NestedAttributeObject{\nAttributes: map[string]schema.Attribute{\n")
				b.WriteString(children)
				b.WriteString("},\n},\n}")
				return b.String(), nil
			}
		}
		elemType, err := g.attrType(elemFile, elemT)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		b.WriteString("schema.MapAttribute{\n")
		b.WriteString(mode)
		b.WriteString(fmt.Sprintf("ElementType: %s,\n", elemType))
		b.WriteString(desc)
		b.WriteString("}")
		return b.String(), nil
	}

	// Plain object with no properties at all -> dynamic.
	if len(ot.Properties) == 0 {
		return "schema.DynamicAttribute{\n" + mode + desc + "}", nil
	}

	children, err := g.renderChildren(file, ot, camelPath)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("schema.SingleNestedAttribute{\n")
	b.WriteString(mode)
	b.WriteString(desc)
	b.WriteString("Attributes: map[string]schema.Attribute{\n")
	b.WriteString(children)
	b.WriteString("},\n}")
	return b.String(), nil
}

func (g *generator) renderArray(file, mode, desc string, at *types.ArrayType, camelPath []string) (string, error) {
	itemT, itemFile, itemRef, err := g.loader.Resolve(file, at.ItemType)
	if err != nil {
		return "", err
	}
	listVals := g.listValidators(at)

	if io, ok := itemT.(*types.ObjectType); ok && len(io.Properties) > 0 {
		key := typeKey{file: itemFile, ref: itemRef}
		if g.visiting[key] {
			// Cycle: fall back to a dynamic list.
			g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
			var b strings.Builder
			b.WriteString("schema.ListAttribute{\n")
			b.WriteString(mode)
			b.WriteString("ElementType: types.DynamicType,\n")
			b.WriteString(desc)
			b.WriteString(listVals)
			b.WriteString("}")
			return b.String(), nil
		}
		g.visiting[key] = true
		defer delete(g.visiting, key)

		children, err := g.renderChildren(itemFile, io, appendPath(camelPath, "*"))
		if err != nil {
			return "", err
		}
		var b strings.Builder
		b.WriteString("schema.ListNestedAttribute{\n")
		b.WriteString(mode)
		b.WriteString(desc)
		b.WriteString(listVals)
		b.WriteString("NestedObject: schema.NestedAttributeObject{\nAttributes: map[string]schema.Attribute{\n")
		b.WriteString(children)
		b.WriteString("},\n},\n}")
		return b.String(), nil
	}

	elemType, err := g.attrType(itemFile, itemT)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("schema.ListAttribute{\n")
	b.WriteString(mode)
	b.WriteString(fmt.Sprintf("ElementType: %s,\n", elemType))
	b.WriteString(desc)
	b.WriteString(listVals)
	b.WriteString("}")
	return b.String(), nil
}

func (g *generator) listValidators(at *types.ArrayType) string {
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
	g.imports.add("github.com/hashicorp/terraform-plugin-framework-validators/listvalidator", "")
	return "Validators: []validator.List{\n" + v + "\n},\n"
}

// renderChildren renders the ordered attributes of an object's properties.
func (g *generator) renderChildren(file string, ot *types.ObjectType, camelPath []string) (string, error) {
	names := make([]string, 0, len(ot.Properties))
	for name := range ot.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	type entry struct {
		tfName string
		code   string
	}
	entries := make([]entry, 0, len(names))
	for _, name := range names {
		childPath := appendPath(camelPath, name)
		tfName := g.tfName(childPath)
		code, err := g.renderAttribute(file, ot.Properties[name], childPath)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry{tfName: tfName, code: code})
	}
	// Sort by TF name for deterministic output.
	sort.Slice(entries, func(i, j int) bool { return entries[i].tfName < entries[j].tfName })

	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%q: %s,\n", e.tfName, e.code)
	}
	return b.String(), nil
}

// -----------------------------------------------------------------------------
// attr.Type expressions (for ElementType of list/map attributes)
// -----------------------------------------------------------------------------

func (g *generator) attrType(file string, t types.Type) (string, error) {
	return g.attrTypeGuarded(file, t, map[typeKey]bool{})
}

func (g *generator) attrTypeGuarded(file string, t types.Type, seen map[typeKey]bool) (string, error) {
	switch tt := t.(type) {
	case *types.StringType, *types.StringLiteralType:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		return "types.StringType", nil
	case *types.IntegerType:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		return "types.Int64Type", nil
	case *types.BooleanType:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		return "types.BoolType", nil
	case *types.AnyType, *types.BuiltInType, *types.DiscriminatedObjectType:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		return "types.DynamicType", nil
	case *types.UnionType:
		if _, stringish := g.unionStrings(file, tt); stringish {
			g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
			return "types.StringType", nil
		}
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		return "types.DynamicType", nil
	case *types.ArrayType:
		itemT, itemFile, _, err := g.loader.Resolve(file, tt.ItemType)
		if err != nil {
			return "", err
		}
		et, err := g.attrTypeGuarded(itemFile, itemT, seen)
		if err != nil {
			return "", err
		}
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		return fmt.Sprintf("types.ListType{ElemType: %s}", et), nil
	case *types.ObjectType:
		if len(tt.Properties) == 0 && tt.AdditionalProperties != nil {
			elemT, elemFile, _, err := g.loader.Resolve(file, tt.AdditionalProperties)
			if err != nil {
				return "", err
			}
			et, err := g.attrTypeGuarded(elemFile, elemT, seen)
			if err != nil {
				return "", err
			}
			g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
			return fmt.Sprintf("types.MapType{ElemType: %s}", et), nil
		}
		if len(tt.Properties) == 0 {
			g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
			return "types.DynamicType", nil
		}
		names := make([]string, 0, len(tt.Properties))
		for name := range tt.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		var b strings.Builder
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/attr", "")
		b.WriteString("types.ObjectType{AttrTypes: map[string]attr.Type{\n")
		for _, name := range names {
			ct, cfile, cref, err := g.loader.Resolve(file, tt.Properties[name].Type)
			if err != nil {
				return "", err
			}
			// Cycle protection for nested object attrType construction.
			ckey := typeKey{file: cfile, ref: cref}
			if seen[ckey] {
				fmt.Fprintf(&b, "%q: types.DynamicType,\n", modelconv.ToSnakeCaseSmart(name))
				continue
			}
			seen[ckey] = true
			et, err := g.attrTypeGuarded(cfile, ct, seen)
			delete(seen, ckey)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "%q: %s,\n", modelconv.ToSnakeCaseSmart(name), et)
		}
		b.WriteString("}}")
		return b.String(), nil
	default:
		g.imports.add("github.com/hashicorp/terraform-plugin-framework/types", "")
		return "types.DynamicType", nil
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

func modeLine(flags types.TypePropertyFlags) string {
	switch {
	case flags&types.TypePropertyFlagsRequired != 0:
		return "Required: true,\n"
	case flags&types.TypePropertyFlagsReadOnly != 0:
		return "Computed: true,\n"
	default:
		return "Optional: true,\n"
	}
}

func descLine(desc string) string {
	if desc == "" {
		return ""
	}
	return fmt.Sprintf("MarkdownDescription: %q,\n", desc)
}

func appendPath(path []string, seg string) []string {
	out := make([]string, len(path), len(path)+1)
	copy(out, path)
	return append(out, seg)
}
