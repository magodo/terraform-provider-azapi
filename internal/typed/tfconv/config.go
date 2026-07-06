package tfconv

import (
	"strings"
	"unicode"
)

// SnakeCamelNameMapper is the a name mapper: snake_case <-> camelCase.
type SnakeCamelNameMapper struct {
	overrides map[string]string
	reverse   map[string]string
}

// NewSnakeCamelNameMapper creates a new SnakeCamelNameMapper, with an override map (snake -> camel)
func NewSnakeCamelNameMapper(overrides map[string]string) SnakeCamelNameMapper {
	if overrides == nil {
		overrides = make(map[string]string)
	}
	reverse := make(map[string]string, len(overrides))
	for tf, api := range overrides {
		reverse[api] = tf
	}
	return SnakeCamelNameMapper{
		overrides: overrides,
		reverse:   reverse,
	}
}

// ToCamelCase is a naive while override-able conversion from snake case to camel case.
// It takes each underscore as a boundary.
func (m *SnakeCamelNameMapper) ToCamelCase(input string) string {
	if v, ok := m.overrides[input]; ok {
		return v
	}

	if input == "" {
		return input
	}
	var b strings.Builder
	upperNext := false
	for _, r := range input {
		if r == '_' {
			upperNext = true
			continue
		}
		if upperNext {
			b.WriteRune(unicode.ToUpper(r))
			upperNext = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ToSnakeCase is a naive while override-able conversion from camel case to snake case.
// It takes each capitalized letter as a boundary.
func (m *SnakeCamelNameMapper) ToSnakeCase(input string) string {
	if v, ok := m.reverse[input]; ok {
		return v
	}

	if input == "" {
		return input
	}
	var b strings.Builder
	for i, r := range input {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteRune('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type Option struct {
	NameMapper SnakeCamelNameMapper
}

func NewDefaultOption() Option {
	return Option{
		NameMapper: NewSnakeCamelNameMapper(nil),
	}
}
