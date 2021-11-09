package types

import (
	"fmt"

	"github.com/ms-henglu/terraform-provider-azurermg/utils"
)

var _ TypeBase = &StringLiteralType{}

type StringLiteralType struct {
	Value string `json:"Value"`
}

func (t *StringLiteralType) Validate(body interface{}, path string) []error {
	if t == nil || body == nil {
		return []error{}
	}
	errors := make([]error, 0)
	if stringValue, ok := body.(string); ok {
		if stringValue != t.Value {
			errors = append(errors, utils.ErrorMismatch(path, t.Value, stringValue))
		}
	} else {
		errors = append(errors, utils.ErrorMismatch(path, "string", fmt.Sprintf("%T", body)))
	}
	return errors
}

func (t *StringLiteralType) AsTypeBase() *TypeBase {
	typeBase := TypeBase(t)
	return &typeBase
}
