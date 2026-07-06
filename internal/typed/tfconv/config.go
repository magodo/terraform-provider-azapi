package tfconv

type Option struct {
	NameMapper SnakeCamelNameMapper
}

func NewDefaultOption() Option {
	return Option{
		NameMapper: NewSnakeCamelNameMapper(nil),
	}
}
