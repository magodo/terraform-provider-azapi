package modelconv

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

// CamelSnakeNameMapper is a name mapper: camelCase <-> snake_case.
type CamelSnakeNameMapper struct {
	overrides map[string]string
}

// NewCamelSnakeNameMapper creates a new SnakeCamelNameMapper, with an override map (camel -> snake)
func NewCamelSnakeNameMapper(overrides map[string]string) CamelSnakeNameMapper {
	if overrides == nil {
		overrides = make(map[string]string)
	}
	return CamelSnakeNameMapper{
		overrides: overrides,
	}
}

// ToCamelCase is a naive while override-able conversion from snake case to camel case.
func (m CamelSnakeNameMapper) ToCamelCase(snakeCasePath string) string {
	var camelSteps []string
Step:
	for snakeStep := range strings.SplitSeq(snakeCasePath, ".") {
		if snakeStep == "*" {
			camelSteps = append(camelSteps, "*")
			continue
		}
		for camelOv, snakeOv := range m.overrides {
			if snakeOv == snakeStep {
				camelOvPrefix := ""
				camelOvLastStep := camelOv
				if idx := strings.LastIndex(camelOv, "."); idx != -1 {
					camelOvPrefix = camelOv[:idx]
					camelOvLastStep = camelOv[idx+1:]
				}

				// The target override record is found
				if camelOvPrefix == strings.Join(camelSteps, ".") {
					camelSteps = append(camelSteps, camelOvLastStep)
					continue Step
				}
			}
		}
		// No override record found, we simply use the naive mapping rule.
		camelSteps = append(camelSteps, ToCamelCaseNaive(snakeStep))
	}

	return camelSteps[len(camelSteps)-1]
}

// ToSnakeCase is a naive while override-able conversion from camel case to snake case.
func (m CamelSnakeNameMapper) ToSnakeCase(camelCasePath string) string {
	if v, ok := m.overrides[camelCasePath]; ok {
		return v
	}
	steps := strings.Split(camelCasePath, ".")
	if len(steps) == 0 {
		return ""
	}
	return ToSnakeCaseNaive(steps[len(steps)-1])
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
