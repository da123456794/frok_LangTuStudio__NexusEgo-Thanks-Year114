package command

import (
	"encoding/binary"
	"io"
)

/*
| 17 | PlaceRuntimeBlock | u16(runtimeId) | 使用特定的 `运行时ID` 在当前画笔的位置放置方块 |
| 18 | PlaceRuntimeBlockU32| u32(runtimeId) | 使用特定的 `运行时ID` 在当前画笔的位置放置方块 |
*/
/*type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}
*/

type PlaceRuntimeBlock struct {
	RuntimeID uint16
}

func (c *PlaceRuntimeBlock) ID() uint8 {
	return 17
}

func (c *PlaceRuntimeBlock) Name() string {
	return "PlaceRuntimeBlock"
}

func (c *PlaceRuntimeBlock) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, c.RuntimeID)
}

func (c *PlaceRuntimeBlock) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.RuntimeID)

}

type PlaceRuntimeBlockU32 struct {
	RuntimeID uint32
}

func (c *PlaceRuntimeBlockU32) ID() uint8 {
	return 18
}

func (c *PlaceRuntimeBlockU32) Name() string {
	return "PlaceRuntimeBlockU32"
}

func (c *PlaceRuntimeBlockU32) Marshal(writer io.Writer) error {
	return binary.Write(writer, binary.BigEndian, c.RuntimeID)
}

func (c *PlaceRuntimeBlockU32) Unmarshal(reader io.Reader) error {
	return binary.Read(reader, binary.BigEndian, &c.RuntimeID)
}
