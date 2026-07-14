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
// | 19 | CommandBlockData | u32(mode) string(command) string(customName) string(lastOutput) i32(tickdelay) bool(executeOnFirstTick) bool(trackOutput) bool(conditional) bool(needRedstone) | 设置当前命令方块的数据 |

type CommandBlockData struct {
	Mode               uint32 // 0: Sequence, 1: Repeat, 2: Chain
	Command            string
	CustomName         string
	LastOutput         string
	TickDelay          int32
	ExecuteOnFirstTick bool
	TrackOutput        bool
	Conditional        bool
	NeedRedstone       bool
}

func (c *CommandBlockData) ID() uint8 {
	return 19
}

func (c *CommandBlockData) Name() string {
	return "CommandBlockData"
}

func (c *CommandBlockData) Marshal(writer io.Writer) error {

	err := binary.Write(writer, binary.BigEndian, c.Mode)
	if err != nil {
		return err
	}
	err = writeString(writer, c.Command)
	if err != nil {
		return err
	}
	err = writeString(writer, c.CustomName)
	if err != nil {
		return err
	}
	err = writeString(writer, c.LastOutput)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, c.TickDelay)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, c.ExecuteOnFirstTick)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, c.TrackOutput)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, c.Conditional)
	if err != nil {
		return err
	}
	err = binary.Write(writer, binary.BigEndian, c.NeedRedstone)
	if err != nil {
		return err
	}
	return nil
}

func (c *CommandBlockData) Unmarshal(reader io.Reader) error {
	err := binary.Read(reader, binary.BigEndian, &c.Mode)
	if err != nil {
		return err
	}
	c.Command, err = readString(reader)
	if err != nil {
		return err
	}
	c.CustomName, err = readString(reader)
	if err != nil {
		return err
	}
	c.LastOutput, err = readString(reader)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &c.TickDelay)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &c.ExecuteOnFirstTick)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &c.TrackOutput)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &c.Conditional)
	if err != nil {
		return err
	}
	err = binary.Read(reader, binary.BigEndian, &c.NeedRedstone)
	if err != nil {
		return err
	}
	return nil
}
