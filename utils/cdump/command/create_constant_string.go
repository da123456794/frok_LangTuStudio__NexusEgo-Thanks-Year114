package command

import (
	"io"
)

/*
type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}
*/
//  string(constantString)
type CreateConstantString struct {
	ConstantString string
}

func (c *CreateConstantString) ID() uint8 {
	return 13
}

func (c *CreateConstantString) Name() string {
	return "CreateConstantString"
}

func (c *CreateConstantString) Marshal(writer io.Writer) error {
	return writeString(writer, c.ConstantString)
}

func (c *CreateConstantString) Unmarshal(reader io.Reader) error {
	var err error
	c.ConstantString, err = readString(reader)
	return err
}
