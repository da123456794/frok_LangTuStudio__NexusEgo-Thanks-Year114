package command

import (
	"encoding/binary"
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
// PlaceBlockWithBlockStates | u16(blockConstantStringID) u16(blockStatesConstantStringID)

type PlaceBlockWithBlockStates struct {
	BlockConstantStringID       uint16
	BlockStatesConstantStringID uint16
}

func (cmd *PlaceBlockWithBlockStates) ID() uint8 {
	return 14
}

func (cmd *PlaceBlockWithBlockStates) Name() string {
	return "PlaceBlockWithBlockStates"
}

func (cmd *PlaceBlockWithBlockStates) Marshal(writer io.Writer) (err error) {
	err = binary.Write(writer, binary.BigEndian, cmd.BlockConstantStringID)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, cmd.BlockStatesConstantStringID)
	if err != nil {
		return err
	}
	return
}

func (cmd *PlaceBlockWithBlockStates) Unmarshal(reader io.Reader) (err error) {
	err = binary.Read(reader, binary.BigEndian, &cmd.BlockConstantStringID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &cmd.BlockStatesConstantStringID)
	if err != nil {
		return err
	}
	return
}
