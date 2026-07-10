package modelconv

type Option struct {
	NameMapper NameMapper

	// ExpandSkipNull modifies the expand behavior when the TF value is null.
	// By default, it will be converted to an explicit JSON null.
	// This modifier changes it to be omit in the JSON value.
	// The key here is the TF attribute path.
	ExpandSkipNull map[string]bool
}

func NewDefaultOption() Option {
	return Option{
		NameMapper:     NewCamelSnakeNameMapper(nil),
		ExpandSkipNull: map[string]bool{},
	}
}
