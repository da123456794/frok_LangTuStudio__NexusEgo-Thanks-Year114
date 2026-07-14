package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/mitchellh/mapstructure"
)

// BeaconNBT...
type BeaconNBT struct {
	CustomName string `mapstructure:"CustomName"`
	Primary    int32  `mapstructure:"primary"`
	Secondary  int32  `mapstructure:"secondary"`
}

// 信标
type Beacon struct {
	DefaultBlock
	NBT BeaconNBT
}

func (b Beacon) NeedSpecialHandle() bool {
	return len(b.NBT.CustomName) != 0 || b.NBT.Primary != 0 || b.NBT.Secondary != 0
}

func (Beacon) NeedCheckCompletely() bool {
	return true
}

func (b Beacon) formatNBT(prefix string) string {
	result := ""
	if len(b.NBT.CustomName) > 0 {
		result += prefix + fmt.Sprintf("自定义名称: %s\n", b.NBT.CustomName)
	}
	if b.NBT.Primary != 0 {
		result += prefix + fmt.Sprintf("主效果: %v\n", mapping.BeaconFormat[b.NBT.Primary])
	}
	if b.NBT.Secondary != 0 {
		result += prefix + fmt.Sprintf("副效果: %v\n", mapping.BeaconFormat[b.NBT.Secondary])
	}

	return result
}

func (b *Beacon) Format(prefix string) string {
	result := b.DefaultBlock.Format(prefix)
	if b.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += b.formatNBT(prefix + "\t")
	}
	return result
}

func (b *Beacon) Parse(nbtMap map[string]any) error {
	var result BeaconNBT
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	b.NBT = result
	return nil
}

func (b Beacon) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.String(&b.NBT.CustomName)
	w.Int32(&b.NBT.Primary)
	w.Int32(&b.NBT.Secondary)

	return buf.Bytes()
}

func (b *Beacon) FullStableBytes() []byte {
	return append(b.DefaultBlock.FullStableBytes(), b.NBTStableBytes()...)
}
