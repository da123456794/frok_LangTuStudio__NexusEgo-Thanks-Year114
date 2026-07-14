package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/mitchellh/mapstructure"
)

// BeehiveNBT...
type BeehiveNBT struct {
	Occupants []any `mapstructure:"Occupants"`
}

// 蜂巢
type Beehive struct {
	DefaultBlock
	NBT BeehiveNBT
}

func (b Beehive) NeedSpecialHandle() bool {
	return len(b.NBT.Occupants) != 0
}

func (Beehive) NeedCheckCompletely() bool {
	return true
}

func (b Beehive) formatNBT(prefix string) string {
	return prefix + fmt.Sprintf("蜜蜂数量: %d\n", len(b.NBT.Occupants))
}

func (b *Beehive) Format(prefix string) string {
	result := b.DefaultBlock.Format(prefix)
	if b.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += b.formatNBT(prefix + "\t")
	}
	return result
}

func (b *Beehive) Parse(nbtMap map[string]any) error {
	var result BeehiveNBT
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	b.NBT = result
	return nil
}

func (b Beehive) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	count := uint8(len(b.NBT.Occupants))
	w.Uint8(&count)

	return buf.Bytes()
}

func (b *Beehive) FullStableBytes() []byte {
	return append(b.DefaultBlock.FullStableBytes(), b.NBTStableBytes()...)
}
