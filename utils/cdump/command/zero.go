package command

import "io"

/*
type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}
*/
type Zero struct {
}

func (z *Zero) ID() uint8 {
	return 0
}

func (z *Zero) Name() string {
	return "zero"
}

func (z *Zero) Marshal(writer io.Writer) error {
	return nil
}

func (z *Zero) Unmarshal(reader io.Reader) error {
	return nil
}
