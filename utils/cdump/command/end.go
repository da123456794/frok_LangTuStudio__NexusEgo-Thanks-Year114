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
type End struct {
}

func (z *End) ID() uint8 {
	return 22
}

func (z *End) Name() string {
	return "End"
}

func (z *End) Marshal(writer io.Writer) error {
	return nil
}

func (z *End) Unmarshal(reader io.Reader) error {
	return nil
}
