package servicehooks

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// UpdateSchemaAttribute walks the given dotted path (e.g.
// "properties.address_space.address_prefixes") in the schema, invokes fn on
// the target attribute typed as T (e.g. schema.SetAttribute), and writes the
// returned value back into the parent attributes map.
func UpdateSchemaAttribute[T schema.Attribute](s schema.Schema, path string, fn func(T) T) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		panic("UpdateSchemaAttribute: empty path")
	}

	attrs := s.Attributes
	for i, p := range parts {
		attr, ok := attrs[p]
		if !ok {
			panic(fmt.Sprintf("UpdateSchemaAttribute: attribute %q not found at segment %d of %q", p, i, path))
		}
		if i == len(parts)-1 {
			target, ok := attr.(T)
			if !ok {
				var zero T
				panic(fmt.Sprintf("UpdateSchemaAttribute: attribute at %q is %T, want %T", path, attr, zero))
			}
			// Attributes maps in nested attributes are reference types shared
			// with the parent schema, so writing back at the leaf is sufficient.
			attrs[p] = fn(target)
			return
		}
		next := nestedAttributes(attr)
		if next == nil {
			panic(fmt.Sprintf("UpdateSchemaAttribute: attribute at segment %q (%T) is not a nested attribute", p, attr))
		}
		attrs = next
	}
}

// RenameSchemaAttribute walks the given dotted path in the schema and renames
// the target attribute (the last segment of the path) to newName in its
// parent attributes map.
func RenameSchemaAttribute(s schema.Schema, path string, newName string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		panic("RenameSchemaAttribute: empty path")
	}

	attrs := s.Attributes
	for i, p := range parts {
		attr, ok := attrs[p]
		if !ok {
			panic(fmt.Sprintf("RenameSchemaAttribute: attribute %q not found at segment %d of %q", p, i, path))
		}
		if i == len(parts)-1 {
			if p == newName {
				return
			}
			if _, exists := attrs[newName]; exists {
				panic(fmt.Sprintf("RenameSchemaAttribute: cannot rename %q to %q, target already exists", path, newName))
			}
			// Attributes maps in nested attributes are reference types shared
			// with the parent schema, so mutating at the leaf is sufficient.
			attrs[newName] = attr
			delete(attrs, p)
			return
		}
		next := nestedAttributes(attr)
		if next == nil {
			panic(fmt.Sprintf("RenameSchemaAttribute: attribute at segment %q (%T) is not a nested attribute", p, attr))
		}
		attrs = next
	}
}

func nestedAttributes(a schema.Attribute) map[string]schema.Attribute {
	switch v := a.(type) {
	case schema.SingleNestedAttribute:
		return v.Attributes
	case schema.ListNestedAttribute:
		return v.NestedObject.Attributes
	case schema.SetNestedAttribute:
		return v.NestedObject.Attributes
	case schema.MapNestedAttribute:
		return v.NestedObject.Attributes
	}
	return nil
}
