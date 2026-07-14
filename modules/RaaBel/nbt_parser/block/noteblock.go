package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/mitchellh/mapstructure"
)

// NoteBlockNBT ..
type NoteBlockNBT struct {
	Note byte `mapstructure:"note"`
}

// 音符盒
type NoteBlock struct {
	DefaultBlock
	NBT NoteBlockNBT
}

func (n NoteBlock) NeedSpecialHandle() bool {
	return n.NBT.Note != 0
}

func (NoteBlock) NeedCheckCompletely() bool {
	return true
}

func (n NoteBlock) formatNBT(prefix string) string {
	return prefix + fmt.Sprintf("音高: %d\n", n.NBT.Note)
}

func (n *NoteBlock) Format(prefix string) string {
	result := n.DefaultBlock.Format(prefix)
	if n.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += n.formatNBT(prefix + "\t")
	}
	return result
}

func (n *NoteBlock) Parse(nbtMap map[string]any) error {
	var result NoteBlockNBT
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	result.Note %= 25
	n.NBT = result
	return nil
}

func (n NoteBlock) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	note := n.NBT.Note % 25
	w.Uint8(&note)

	return buf.Bytes()
}

func (n *NoteBlock) FullStableBytes() []byte {
	return append(n.DefaultBlock.FullStableBytes(), n.NBTStableBytes()...)
}
