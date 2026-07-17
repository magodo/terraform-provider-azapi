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

// Bicep API attributes to ignore at any level.
var ignoreAttributesAnyLevel = map[string]bool{
	"etag":              true,
	"provisioningState": true,
	"systemData":        true,
}

// Bicep API attributes to ignore at top level.
// Note some of them are ignored because they have a specialized & fixed schema.
var ignoreAttributesTopLevel = map[string]bool{
	"type":       true,
	"apiVersion": true,
	"name":       true, // rendered as the fixed "name" attribute
	"id":         true, // rendered as the fixed "id" attribute
	"location":   true, // rendered as the fixed "location" attribute (when present)
}

// includePath reports whether the attribute at the given API path should be
// rendered, given the ordered rules.
func (g *resourceGenerator) includePath(apiPath []string) bool {
	// Always ignore certain attributes at any nesting level.
	if len(apiPath) > 0 {
		if ignoreAttributesAnyLevel[apiPath[len(apiPath)-1]] {
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

// modeOf categorises a property by its bicep flags.
func modeOf(flags types.TypePropertyFlags) Mode {
	switch {
	case flags&types.TypePropertyFlagsRequired != 0:
		return Required
	case flags&types.TypePropertyFlagsReadOnly != 0:
		return Computed
	default:
		return Optional
	}
}

// attributeContext is the input to buildAttribute: an unresolved property at a
// given API path.
type attributeContext struct {
	file     string
	property types.ObjectTypeProperty
	apiPath  []string
	// computed reports whether this attribute lives under a Computed-only
	// ancestor. When true, the attribute (and everything below it) is forced
	// to Computed-only regardless of its own bicep flags. This is control-flow
	// state that flows down the traversal, not generator-wide state.
	computed bool
}

// Generate builds the schema IR and renders it to formatted Go source.
func (g *resourceGenerator) Generate() ([]byte, error) {
	schema, rt, err := g.build()
	if err != nil {
		return nil, err
	}
	return g.renderFile(schema, rt)
}

// build walks the bicep types and produces the schema IR together with the
// resolved ResourceType (needed by the render phase for the example ID).
func (g *resourceGenerator) build() (*Schema, *types.ResourceType, error) {
	indexFile := "generated/index.json"

	indexContent, err := azure.StaticFiles.ReadFile(indexFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load schema index: %w", err)
	}
	var idx index.TypeIndex
	if err := json.Unmarshal(indexContent, &idx); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal schema index: %w", err)
	}

	ref, ok := idx.GetResource(g.apiResourceType, g.apiVersion)
	if !ok {
		return nil, nil, fmt.Errorf("resource type %s@%s not found in the bicep index", g.apiResourceType, g.apiVersion)
	}

	resourceType, err := g.loader.resolve(indexFile, ref)
	if err != nil {
		return nil, nil, err
	}
	rt, ok := resourceType.t.(*types.ResourceType)
	if !ok {
		return nil, nil, fmt.Errorf("index entry for %s@%s is not a ResourceType (got %T)", g.apiResourceType, g.apiVersion, resourceType.t)
	}

	bodyType, err := g.loader.resolve(resourceType.key.file, rt.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to resolve resource body: %w", err)
	}
	body, ok := bodyType.t.(*types.ObjectType)
	if !ok {
		return nil, nil, fmt.Errorf("resource body is not an object type (got %T)", bodyType.t)
	}
	g.visiting[bodyType.key] = true

	_, hasLocation := body.Properties["location"]

	// Build the body-derived attributes (everything except the special ones),
	// bucketed by behavior so that we can emit them in the desired order.
	var required, optional, computed []*Attribute
	for name, prop := range body.Properties {
		if ignoreAttributesTopLevel[name] {
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
		attrType, comment, err := g.buildAttribute(attributeContext{file: bodyType.key.file, property: prop, apiPath: apiPath})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to build attribute %q: %w", name, err)
		}
		attr := &Attribute{Name: g.tfName(apiPath), Comment: comment, Type: attrType}

		switch modeOf(prop.Flags) {
		case Required:
			required = append(required, attr)
		case Optional:
			optional = append(optional, attr)
		case Computed:
			computed = append(computed, attr)
		}
	}
	sortByName(required)
	sortByName(optional)
	sortByName(computed)

	// Assemble the attributes in the desired order:
	//   parent_id, name, location?, <required>, <optional>, <computed>, id, timeouts
	schema := &Schema{}
	schema.Attributes = append(schema.Attributes, specialParentID(), specialName())
	if hasLocation {
		schema.Attributes = append(schema.Attributes, specialLocation())
	}
	schema.Attributes = append(schema.Attributes, required...)
	schema.Attributes = append(schema.Attributes, optional...)
	schema.Attributes = append(schema.Attributes, computed...)
	schema.Attributes = append(schema.Attributes, specialID(), specialTimeouts())

	return schema, rt, nil
}

func sortByName(s []*Attribute) {
	sort.Slice(s, func(i, j int) bool { return s[i].Name < s[j].Name })
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
// Attribute building (bicep property -> IR AttributeType)
// -----------------------------------------------------------------------------

// buildAttribute builds the IR AttributeType for the given property, together
// with an optional leading comment (used to explain dynamic fallbacks).
// apiPath is the full API (camelCase) path to this property, including its own
// name as the last element ("*" is used for array element boundaries).
func (g *resourceGenerator) buildAttribute(ctx attributeContext) (AttributeType, string, error) {
	typeInfo, err := g.loader.resolve(ctx.file, ctx.property.Type)
	if err != nil {
		return nil, "", err
	}

	mode := modeOf(ctx.property.Flags)
	desc := ctx.property.Description

	// Whether the subtree rooted at this attribute must be Computed-only: it is
	// so if some ancestor was Computed-only, or if this attribute itself is.
	// buildChildren consults this to rewrite each child's flags accordingly.
	computedSubtree := ctx.computed || mode == Computed

	switch tt := typeInfo.t.(type) {
	case *types.StringType:
		return g.buildString(mode, desc, tt), "", nil
	case *types.StringLiteralType:
		return StringAttr{
			Mode:        mode,
			Description: desc,
			Sensitive:   tt.Sensitive,
			Validators:  []StringValidator{OneOf{Values: []string{tt.Value}}},
		}, "", nil
	case *types.IntegerType:
		return g.buildInteger(mode, desc, tt), "", nil
	case *types.BooleanType:
		return BoolAttr{Mode: mode, Description: desc}, "", nil
	case *types.UnionType:
		if attr, ok := g.buildUnionAsStringish(mode, desc, typeInfo, tt); ok {
			return attr, "", nil
		}
		log.Printf("[WARN] Unexpected union type of non-stringish variants in file %q: %v", typeInfo.key.file, ctx.apiPath)
		return DynamicAttr{Mode: mode, Description: desc}, "dynamic: union type with non-stringish variants", nil
	case *types.AnyType:
		return DynamicAttr{Mode: mode, Description: desc}, "dynamic: bicep any type", nil
	case *types.BuiltInType:
		// There is no BuiltInType in the bicep generated types any more.
		log.Printf("[WARN] Unexpected BuiltInType found in file %q: %v", ctx.file, ctx.apiPath)
		return DynamicAttr{Mode: mode, Description: desc}, "dynamic: unexpected BuiltInType", nil
	case *types.ObjectType:
		return g.buildObject(mode, desc, typeInfo, tt, ctx.apiPath, computedSubtree)
	case *types.ArrayType:
		attr, err := g.buildArray(mode, desc, typeInfo, tt, ctx.apiPath, computedSubtree)
		return attr, "", err
	case *types.DiscriminatedObjectType:
		// TODO: Discriminator needs a different solution instead of dynamic attribute, but more thoughts needed.
		panic(fmt.Sprintf("DiscriminatedObjectType not supported: %s at %s", ctx.file, ctx.apiPath))
	default:
		log.Printf("[WARN] Unexpected type %T found in file %q: %v", tt, ctx.file, ctx.apiPath)
		return DynamicAttr{Mode: mode, Description: desc}, fmt.Sprintf("dynamic: unexpected type %T", tt), nil
	}
}

func (g *resourceGenerator) buildString(behavior Mode, desc string, st *types.StringType) StringAttr {
	attr := StringAttr{Mode: behavior, Description: desc, Sensitive: st.Sensitive}
	if st.Pattern != "" {
		attr.Validators = append(attr.Validators, RegexMatches{Pattern: st.Pattern})
	}
	switch {
	case st.MinLength != nil && st.MaxLength != nil:
		attr.Validators = append(attr.Validators, LengthBetween{Min: *st.MinLength, Max: *st.MaxLength})
	case st.MinLength != nil:
		attr.Validators = append(attr.Validators, LengthAtLeast{Min: *st.MinLength})
	case st.MaxLength != nil:
		attr.Validators = append(attr.Validators, LengthAtMost{Max: *st.MaxLength})
	}
	return attr
}

func (g *resourceGenerator) buildInteger(behavior Mode, desc string, it *types.IntegerType) Int64Attr {
	attr := Int64Attr{Mode: behavior, Description: desc}
	switch {
	case it.MinValue != nil && it.MaxValue != nil:
		attr.Validators = append(attr.Validators, IntBetween{Min: *it.MinValue, Max: *it.MaxValue})
	case it.MinValue != nil:
		attr.Validators = append(attr.Validators, IntAtLeast{Min: *it.MinValue})
	case it.MaxValue != nil:
		attr.Validators = append(attr.Validators, IntAtMost{Max: *it.MaxValue})
	}
	return attr
}

func (g *resourceGenerator) buildUnionAsStringish(behavior Mode, desc string, typeInfo bicepType, ut *types.UnionType) (StringAttr, bool) {
	literals, stringish := g.unionStrings(typeInfo.key.file, ut)
	if !stringish {
		return StringAttr{}, false
	}
	attr := StringAttr{Mode: behavior, Description: desc}
	if len(literals) > 0 {
		attr.Validators = append(attr.Validators, OneOf{Values: literals})
	}
	return attr, true
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

func (g *resourceGenerator) buildObject(behavior Mode, desc string, typeInfo bicepType, ot *types.ObjectType, apiPath []string, computed bool) (AttributeType, string, error) {
	sensitive := ot.Sensitive != nil && *ot.Sensitive

	key := typeInfo.key
	if g.visiting[key] {
		log.Printf("[WARN] visited type %v at %v", typeInfo.key, apiPath)
		// Self-referential type. Emit a dynamic attribute to break the cycle.
		return DynamicAttr{Mode: behavior, Description: desc, Sensitive: sensitive},
			"dynamic: self-referential object type", nil
	}
	g.visiting[key] = true
	defer delete(g.visiting, key)

	// Map-like object (no declared properties, only additionalProperties).
	if len(ot.Properties) == 0 && ot.AdditionalProperties != nil {
		element, err := g.loader.resolve(typeInfo.key.file, ot.AdditionalProperties)
		if err != nil {
			return nil, "", err
		}
		if eo, ok := element.t.(*types.ObjectType); ok && len(eo.Properties) > 0 {
			// Guard the element too.
			ekey := element.key
			if !g.visiting[ekey] {
				g.visiting[ekey] = true
				defer delete(g.visiting, ekey)
				children, err := g.buildChildren(element.key.file, eo, append(slices.Clone(apiPath), "*"), computed)
				if err != nil {
					return nil, "", err
				}
				return MapNestedAttr{
					Mode:        behavior,
					Description: desc,
					Sensitive:   sensitive,
					Attributes:  children,
				}, "", nil
			}
		}
		elemType, err := g.buildElemType(element.key.file, element.t)
		if err != nil {
			return nil, "", err
		}
		return MapAttr{
			Mode:        behavior,
			Description: desc,
			Sensitive:   sensitive,
			ElementType: elemType,
		}, "", nil
	}

	// Plain object with no properties at all -> dynamic.
	if len(ot.Properties) == 0 {
		return DynamicAttr{Mode: behavior, Description: desc, Sensitive: sensitive},
			"dynamic: object type with no properties", nil
	}

	children, err := g.buildChildren(typeInfo.key.file, ot, apiPath, computed)
	if err != nil {
		return nil, "", err
	}
	return SingleNestedAttr{
		Mode:        behavior,
		Description: desc,
		Sensitive:   sensitive,
		Attributes:  children,
	}, "", nil
}

func (g *resourceGenerator) buildArray(behavior Mode, desc string, typeInfo bicepType, at *types.ArrayType, apiPath []string, computed bool) (AttributeType, error) {
	item, err := g.loader.resolve(typeInfo.key.file, at.ItemType)
	if err != nil {
		return nil, err
	}
	validators := g.listValidators(at)

	if io, ok := item.t.(*types.ObjectType); ok && len(io.Properties) > 0 {
		key := item.key
		if g.visiting[key] {
			log.Printf("[WARN] visited type %v at %v", key, apiPath)
			// Cycle: fall back to a dynamic list.
			return ListAttr{
				Mode:        behavior,
				Description: desc,
				ElementType: DynamicElem{},
				Validators:  validators,
			}, nil
		}
		g.visiting[key] = true
		defer delete(g.visiting, key)

		children, err := g.buildChildren(item.key.file, io, append(slices.Clone(apiPath), "*"), computed)
		if err != nil {
			return nil, err
		}
		return ListNestedAttr{
			Mode:        behavior,
			Description: desc,
			Validators:  validators,
			Attributes:  children,
		}, nil
	}

	elemType, err := g.buildElemType(item.key.file, item.t)
	if err != nil {
		return nil, err
	}
	return ListAttr{
		Mode:        behavior,
		Description: desc,
		ElementType: elemType,
		Validators:  validators,
	}, nil
}

func (g *resourceGenerator) listValidators(at *types.ArrayType) []ListValidator {
	switch {
	case at.MinLength != nil && at.MaxLength != nil:
		return []ListValidator{SizeBetween{Min: *at.MinLength, Max: *at.MaxLength}}
	case at.MinLength != nil:
		return []ListValidator{SizeAtLeast{Min: *at.MinLength}}
	case at.MaxLength != nil:
		return []ListValidator{SizeAtMost{Max: *at.MaxLength}}
	}
	return nil
}

// buildChildren builds the attributes of an object's properties, ordered as
// Required -> Optional -> Computed, alphabetically within each group. When
// computed is true, this object lives under a Computed-only ancestor, so every
// child is forced to Computed-only.
func (g *resourceGenerator) buildChildren(file string, ot *types.ObjectType, camelPath []string, computed bool) ([]*Attribute, error) {
	var required, optional, computedAttrs []*Attribute
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
		if computed {
			prop.Flags = types.TypePropertyFlagsReadOnly
		}
		attrType, comment, err := g.buildAttribute(attributeContext{file: file, property: prop, apiPath: childPath, computed: computed})
		if err != nil {
			return nil, err
		}
		entry := &Attribute{Name: g.tfName(childPath), Comment: comment, Type: attrType}
		switch modeOf(prop.Flags) {
		case Required:
			required = append(required, entry)
		case Optional:
			optional = append(optional, entry)
		case Computed:
			computedAttrs = append(computedAttrs, entry)
		}
	}
	sortByName(required)
	sortByName(optional)
	sortByName(computedAttrs)

	out := make([]*Attribute, 0, len(required)+len(optional)+len(computedAttrs))
	out = append(out, required...)
	out = append(out, optional...)
	out = append(out, computedAttrs...)
	return out, nil
}

// -----------------------------------------------------------------------------
// attr.Type expressions (ElementType of list/map attributes)
// -----------------------------------------------------------------------------

func (g *resourceGenerator) buildElemType(file string, t types.Type) (ElemType, error) {
	return g.buildElemTypeGuarded(file, t, map[bicepTypeKey]bool{})
}

func (g *resourceGenerator) buildElemTypeGuarded(file string, t types.Type, seen map[bicepTypeKey]bool) (ElemType, error) {
	switch tt := t.(type) {
	case *types.StringType, *types.StringLiteralType:
		return StringElem{}, nil
	case *types.IntegerType:
		return Int64Elem{}, nil
	case *types.BooleanType:
		return BoolElem{}, nil
	case *types.AnyType, *types.BuiltInType, *types.DiscriminatedObjectType:
		return DynamicElem{}, nil
	case *types.UnionType:
		if _, stringish := g.unionStrings(file, tt); stringish {
			return StringElem{}, nil
		}
		return DynamicElem{}, nil
	case *types.ArrayType:
		item, err := g.loader.resolve(file, tt.ItemType)
		if err != nil {
			return nil, err
		}
		et, err := g.buildElemTypeGuarded(item.key.file, item.t, seen)
		if err != nil {
			return nil, err
		}
		return ListElem{Elem: et}, nil
	case *types.ObjectType:
		if len(tt.Properties) == 0 && tt.AdditionalProperties != nil {
			element, err := g.loader.resolve(file, tt.AdditionalProperties)
			if err != nil {
				return nil, err
			}
			et, err := g.buildElemTypeGuarded(element.key.file, element.t, seen)
			if err != nil {
				return nil, err
			}
			return MapElem{Elem: et}, nil
		}
		if len(tt.Properties) == 0 {
			return DynamicElem{}, nil
		}
		names := make([]string, 0, len(tt.Properties))
		for name := range tt.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		obj := ObjectElem{}
		for _, name := range names {
			child, err := g.loader.resolve(file, tt.Properties[name].Type)
			if err != nil {
				return nil, err
			}
			// Cycle protection for nested object attrType construction.
			ckey := child.key
			if seen[ckey] {
				obj.Attributes = append(obj.Attributes, ObjectElemAttr{Name: modelconv.ToSnakeCaseSmart(name), Type: DynamicElem{}})
				continue
			}
			seen[ckey] = true
			et, err := g.buildElemTypeGuarded(child.key.file, child.t, seen)
			delete(seen, ckey)
			if err != nil {
				return nil, err
			}
			obj.Attributes = append(obj.Attributes, ObjectElemAttr{Name: modelconv.ToSnakeCaseSmart(name), Type: et})
		}
		return obj, nil
	default:
		return DynamicElem{}, nil
	}
}
