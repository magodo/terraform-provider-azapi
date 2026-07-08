package tfconv

type Option struct {
	NameMapper NameMapper
}

func NewDefaultOption() Option {
	return Option{
		NameMapper: NewSnakeCamelNameMapper(nil),
	}
}
