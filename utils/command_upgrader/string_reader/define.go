package string_reader

type StringReader struct {
	ptr int
	ctx *string
}

func NewStringReader(content *string) *StringReader {
	reader := StringReader{}
	reader.Reset(content)
	return &reader
}
