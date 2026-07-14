package nbt_parser_block

import (
	"bytes"
	"fmt"

	//	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/mitchellh/mapstructure"
)

// PistonNBT ..
type PistonNBT struct {
	State byte `mapstructure:"State"`
}

// 活塞
type Piston struct {
	DefaultBlock
	NBT PistonNBT
}

func (p Piston) NeedSpecialHandle() bool {
	return false //p.NBT.State == 2
}

func (Piston) NeedCheckCompletely() bool {
	return true
}

func (p Piston) formatNBT(prefix string) string {
	return prefix + fmt.Sprintf("状态: %d\n", p.NBT.State)
}

func (p *Piston) Format(prefix string) string {
	result := p.DefaultBlock.Format(prefix)
	if p.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += p.formatNBT(prefix + "\t")
	}
	return result
}

func (p *Piston) Parse(nbtMap map[string]any) error {
	result := PistonNBT{}
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	p.NBT = result
	return nil
}

func (p Piston) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	//	w := protocol.NewWriter(buf, 0)

	//	w.Uint8(&p.NBT.State)

	return buf.Bytes()
}

func (p *Piston) FullStableBytes() []byte {
	return append(p.DefaultBlock.FullStableBytes(), p.NBTStableBytes()...)
}
