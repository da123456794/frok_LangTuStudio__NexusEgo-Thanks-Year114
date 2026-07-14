package command

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/LangTuStudio/Conbit/minecraft/nbt"
)

// | 21 | NBTData | u16(blockConstantStringID) u16(blockStatesConstantStringID) void(*buffer) | 设置当前方块实体的 NBT 数据 |
/*
type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}
*/

// NBTData is a command that sets the data of the current block entity.
type NBTData struct {
	BlockConstantStringID       uint16
	BlockStatesConstantStringID uint16
	BlockNBT                    map[string]interface{}
}

// ID returns the command ID for the NBTData command.
func (NBTData) ID() uint8 {
	return 21
}

// Name returns the command name for the NBTData command.
func (NBTData) Name() string {
	return "NBTData"
}

// Marshal serializes the NBTData command into a binary format.
func (c NBTData) Marshal(writer io.Writer) error {
	err := binary.Write(writer, binary.BigEndian, c.BlockConstantStringID)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, c.BlockStatesConstantStringID)
	if err != nil {
		return err
	}
	var bytesIO = new(bytes.Buffer)
	err = nbt.NewEncoderWithEncoding(bytesIO, nbt.BigEndian).Encode(&c.BlockNBT)
	if err != nil {
		return err
	}
	nbtData, err := io.ReadAll(bytesIO)
	if err != nil {
		return err
	}
	var lenNBT uint64 = uint64(len(nbtData))
	err = binary.Write(writer, binary.BigEndian, lenNBT)
	if err != nil {
		return err
	}
	_, err = writer.Write(nbtData)
	if err != nil {
		return err
	}
	return nil
}

// Unmarshal deserializes the NBTData command from a binary format.
func (c *NBTData) Unmarshal(reader io.Reader) error {
	err := binary.Read(reader, binary.BigEndian, &c.BlockConstantStringID)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &c.BlockStatesConstantStringID)
	if err != nil {
		return err
	}
	var byteLen uint64
	err = binary.Read(reader, binary.BigEndian, &byteLen)
	if err != nil {
		return err
	}
	byteData := make([]byte, byteLen)
	_, err = io.ReadFull(reader, byteData)
	if err != nil {
		return err
	}
	reader = bytes.NewReader(byteData)
	err = nbt.NewDecoderWithEncoding(reader, nbt.BigEndian).Decode(&c.BlockNBT)
	if err != nil {
		return err
	}
	return nil
}

