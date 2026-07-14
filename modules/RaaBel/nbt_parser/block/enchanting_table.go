package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/mitchellh/mapstructure"
)

// EnchantingTableNBT...
type EnchantingTableNBT struct {
	CustomName string `mapstructure:"CustomName"`
}

// 附魔台
type EnchantingTable struct {
	DefaultBlock
	NBT EnchantingTableNBT
}

func (e EnchantingTable) NeedSpecialHandle() bool {
	return len(e.NBT.CustomName) != 0
}

func (EnchantingTable) NeedCheckCompletely() bool {
	return true
}

func (e EnchantingTable) formatNBT(prefix string) string {
	return prefix + fmt.Sprintf("自定义名称: %s\n", e.NBT.CustomName)
}

func (e *EnchantingTable) Format(prefix string) string {
	result := e.DefaultBlock.Format(prefix)
	if e.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += e.formatNBT(prefix + "\t")
	}
	return result
}

func (e *EnchantingTable) Parse(nbtMap map[string]any) error {
	var result EnchantingTableNBT
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	e.NBT = result
	return nil
}

func (e EnchantingTable) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.String(&e.NBT.CustomName)

	return buf.Bytes()
}

func (e *EnchantingTable) FullStableBytes() []byte {
	return append(e.DefaultBlock.FullStableBytes(), e.NBTStableBytes()...)
}
