package main

// Schema is the IR of a generated resource schema
type Schema struct {
	Attributes []*Attribute
}

// Attribute is a single entry in an attributes map.
type Attribute struct {
	// Name is the Terraform (snake_case) attribute name.
	Name string
	// Comment, when non-empty, is emitted as a leading // comment above the
	// attribute (used to explain e.g. why a DynamicAttribute was chosen).
	Comment string
	// Type is the schema.XxxAttribute value.
	Type AttributeType
}

// Mode is the required/optional/computed nature of an attribute.
type Mode int

const (
	Required Mode = iota
	Optional
	Computed
)

// -----------------------------------------------------------------------------
// Attribute types
// -----------------------------------------------------------------------------

// AttributeType is the schema.XxxAttribute value of an attribute.
type AttributeType interface{ attributeType() }

// StringAttr -> schema.StringAttribute.
type StringAttr struct {
	Mode        Mode
	Description string
	Sensitive   bool
	// CustomType, when non-empty, is the Go expression for the attribute's
	// CustomType (e.g. "customtypes.LocationType{}"). It maps to the
	// schema.StringAttribute.CustomType field.
	CustomType    string
	PlanModifiers []StringPlanModifier
	Validators    []StringValidator
}

// BoolAttr -> schema.BoolAttribute.
type BoolAttr struct {
	Mode        Mode
	Description string
	Sensitive   bool
}

// Int64Attr -> schema.Int64Attribute.
type Int64Attr struct {
	Mode        Mode
	Description string
	Validators  []Int64Validator
}

// DynamicAttr -> schema.DynamicAttribute.
type DynamicAttr struct {
	Mode        Mode
	Description string
	Sensitive   bool
}

// SingleNestedAttr -> schema.SingleNestedAttribute.
type SingleNestedAttr struct {
	Mode        Mode
	Description string
	Sensitive   bool
	Attributes  []*Attribute
}

// ListNestedAttr -> schema.ListNestedAttribute.
type ListNestedAttr struct {
	Mode        Mode
	Description string
	Validators  []ListValidator
	Attributes  []*Attribute
}

// MapNestedAttr -> schema.MapNestedAttribute.
type MapNestedAttr struct {
	Mode        Mode
	Description string
	Sensitive   bool
	Attributes  []*Attribute
}

// ListAttr -> schema.ListAttribute.
type ListAttr struct {
	Mode        Mode
	Description string
	ElementType ElemType
	Validators  []ListValidator
}

// MapAttr -> schema.MapAttribute.
type MapAttr struct {
	Mode        Mode
	Description string
	Sensitive   bool
	ElementType ElemType
}

// RawAttr is an attribute whose value is an opaque, pre-rendered Go expression
// (used for the special "timeouts" attribute).
type RawAttr struct {
	Code string
}

func (StringAttr) attributeType()       {}
func (BoolAttr) attributeType()         {}
func (Int64Attr) attributeType()        {}
func (DynamicAttr) attributeType()      {}
func (SingleNestedAttr) attributeType() {}
func (ListNestedAttr) attributeType()   {}
func (MapNestedAttr) attributeType()    {}
func (ListAttr) attributeType()         {}
func (MapAttr) attributeType()          {}
func (RawAttr) attributeType()          {}

// -----------------------------------------------------------------------------
// Validators
// -----------------------------------------------------------------------------

// StringValidator is an entry in a []validator.String.
type StringValidator interface{ stringValidator() }

type (
	OneOf              struct{ Values []string }
	RegexMatches       struct{ Pattern string }
	LengthBetween      struct{ Min, Max int64 }
	LengthAtLeast      struct{ Min int64 }
	LengthAtMost       struct{ Max int64 }
	StringIsResourceID struct{}
)

func (OneOf) stringValidator()              {}
func (RegexMatches) stringValidator()       {}
func (LengthBetween) stringValidator()      {}
func (LengthAtLeast) stringValidator()      {}
func (LengthAtMost) stringValidator()       {}
func (StringIsResourceID) stringValidator() {}

// Int64Validator is an entry in a []validator.Int64.
type Int64Validator interface{ int64Validator() }

type (
	IntBetween struct{ Min, Max int64 }
	IntAtLeast struct{ Min int64 }
	IntAtMost  struct{ Max int64 }
)

func (IntBetween) int64Validator() {}
func (IntAtLeast) int64Validator() {}
func (IntAtMost) int64Validator()  {}

// ListValidator is an entry in a []validator.List.
type ListValidator interface{ listValidator() }

type (
	SizeBetween struct{ Min, Max int64 }
	SizeAtLeast struct{ Min int64 }
	SizeAtMost  struct{ Max int64 }
)

func (SizeBetween) listValidator() {}
func (SizeAtLeast) listValidator() {}
func (SizeAtMost) listValidator()  {}

// -----------------------------------------------------------------------------
// Plan modifiers
// -----------------------------------------------------------------------------

// StringPlanModifier is an entry in a []planmodifier.String.
type StringPlanModifier interface{ stringPlanModifier() }

type (
	RequiresReplace    struct{}
	UseStateForUnknown struct{}
)

func (RequiresReplace) stringPlanModifier()    {}
func (UseStateForUnknown) stringPlanModifier() {}

// -----------------------------------------------------------------------------
// Element types (attr.Type expressions for list/map ElementType)
// -----------------------------------------------------------------------------

// ElemType is an attr.Type expression used as the ElementType of a list or map.
type ElemType interface{ elemType() }

type (
	StringElem  struct{}
	Int64Elem   struct{}
	BoolElem    struct{}
	DynamicElem struct{}
	ListElem    struct{ Elem ElemType }
	MapElem     struct{ Elem ElemType }
	ObjectElem  struct{ Attributes []ObjectElemAttr }
)

// ObjectElemAttr is one field of an ObjectElem's AttrTypes map.
type ObjectElemAttr struct {
	Name string
	Type ElemType
}

func (StringElem) elemType()  {}
func (Int64Elem) elemType()   {}
func (BoolElem) elemType()    {}
func (DynamicElem) elemType() {}
func (ListElem) elemType()    {}
func (MapElem) elemType()     {}
func (ObjectElem) elemType()  {}
