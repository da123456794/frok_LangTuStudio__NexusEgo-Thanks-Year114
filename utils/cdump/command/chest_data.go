package command

import (
	"encoding/binary"
	"io"

	types "nexus/defines"
)

/*type Command interface {
	ID() uint8 // Extra ID spaces (uint8) may be allocated in the future.
	Name() string
	Marshal(writer io.Writer) error
	Unmarshal(reader io.Reader) error
}*/
/*
| 20 | ChestData | u8(slotCount) chestData(data) | 设置当前箱子的槽位数据 |
*/
type ChestData struct {
	SlotCount uint8
	Data      []types.ChestSlot
}

func (c *ChestData) ID() uint8 {
	return 20
}

func (c *ChestData) Name() string {
	return "ChestData"
}

func (c *ChestData) Marshal(writer io.Writer) error {
	if err := binary.Write(writer, binary.BigEndian, c.SlotCount); err != nil {
		return err
	}
	_, err := writer.Write([]byte{uint8(len(c.Data))})
	if err != nil {
		return err
	}
	for _, slot := range c.Data {
		// Name   string
		// Count  uint8
		// Damage uint16
		// Slot   uint8
		if err := writeString(writer, slot.Name); err != nil {
			return err
		}
		if err := binary.Write(writer, binary.BigEndian, slot.Count); err != nil {
			return err
		}
		if err := binary.Write(writer, binary.BigEndian, slot.Damage); err != nil {
			return err
		}
		if err := binary.Write(writer, binary.BigEndian, slot.Slot); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChestData) Unmarshal(reader io.Reader) error {
	if err := binary.Read(reader, binary.BigEndian, &c.SlotCount); err != nil {
		return err
	}
	dataLen := make([]byte, 1)
	_, err := reader.Read(dataLen)
	if err != nil {
		return err
	}
	dataLen = dataLen[:1]
	dataLenInt := int(dataLen[0])
	c.Data = make([]types.ChestSlot, dataLenInt)
	for i := 0; i < dataLenInt; i++ {
		slot := types.ChestSlot{}
		slot.Name, err = readString(reader)
		if err != nil {
			return err
		}
		if err := binary.Read(reader, binary.BigEndian, &slot.Count); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.BigEndian, &slot.Damage); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.BigEndian, &slot.Slot); err != nil {
			return err
		}

		c.Data[i] = slot
	}
	return nil
}
