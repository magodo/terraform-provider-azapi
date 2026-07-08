package tfconv

import (
	"strings"
	"unicode"
)

type NameMapper interface {
	ToCamelCase(string) string
	ToSnakeCase(string) string
}

// NoopNameMapper is a name mapper that keeps the original name
type NoopNameMapper struct{}

func (NoopNameMapper) ToCamelCase(in string) string {
	return in
}

func (NoopNameMapper) ToSnakeCase(in string) string {
	return in
}

// SnakeCamelNameMapper is a name mapper: snake_case <-> camelCase.
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
func (m SnakeCamelNameMapper) ToCamelCase(input string) string {
	if v, ok := m.overrides[input]; ok {
		return v
	}

	return ToCamelCaseNaive(input)
}

// ToSnakeCase is a naive while override-able conversion from camel case to snake case.
// It takes each capitalized letter as a boundary.
func (m SnakeCamelNameMapper) ToSnakeCase(input string) string {
	if v, ok := m.reverse[input]; ok {
		return v
	}

	return ToSnakeCaseNaive(input)
}

// ToCamelCaseNaive takes each underscore as a boundary.
func ToCamelCaseNaive(input string) string {
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

// ToSnakeCaseNaive takes each capitalized letter as a boundary.
func ToSnakeCaseNaive(input string) string {
	if input == "" {
		return input
	}
	var b strings.Builder
	for _, r := range input {
		if unicode.IsUpper(r) {
			b.WriteRune('_')
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ToSnakeCaseSmart considers continuous capital cased letters and keep them as one word.
func ToSnakeCaseSmart(input string) string {
	if input == "" {
		return input
	}
	runes := []rune(input)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) {
			// Insert '_' before this upper rune when:
			//   - previous rune is lowercase/digit (word boundary), OR
			//   - previous rune is upper AND next rune is lowercase
			//     (start of a new word after an acronym, e.g. "HTTPServer").
			if i > 0 {
				prev := runes[i-1]
				var next rune
				if i+1 < len(runes) {
					next = runes[i+1]
				}
				if unicode.IsLower(prev) || unicode.IsDigit(prev) ||
					(unicode.IsUpper(prev) && next != 0 && unicode.IsLower(next)) {
					b.WriteRune('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}
