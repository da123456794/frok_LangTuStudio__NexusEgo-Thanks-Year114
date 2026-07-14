package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"

	"github.com/mitchellh/mapstructure"
)

// SkullNBT ..
type SkullNBT struct {
	Rotation  float32 `mapstructure:"Rotation"`
	SkullType byte    `mapstructure:"SkullType"`
}

// 头颅
type Skull struct {
	DefaultBlock
	NBT SkullNBT
}

func (s Skull) NeedSpecialHandle() bool {
	if s.NBT.Rotation != 0 {
		return true
	}
	if s.NBT.SkullType != 255 {
		return true
	}
	return false
}

func (s Skull) NeedCheckCompletely() bool {
	return true
}

func (s Skull) formatNBT(prefix string) string {
	result := prefix + fmt.Sprintf("旋转角度: %v\n", s.NBT.Rotation)
	return result
}

func (s *Skull) Format(prefix string) string {
	result := s.DefaultBlock.Format(prefix)
	if s.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += s.formatNBT(prefix + "\t")
	}
	return result
}

func (s *Skull) Parse(nbtMap map[string]any) error {
	var result SkullNBT

	err := mapstructure.Decode(&nbtMap, &result)
	if err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	s.NBT = result
	if s.NBT.Rotation == -180 {
		s.NBT.Rotation = 180
	}
	if s.NBT.SkullType != 255 {
		switch s.NBT.SkullType {
		case 0:
			s.DefaultBlock.Name = "minecraft:skeleton_skull"
		case 1:
			s.DefaultBlock.Name = "minecraft:wither_skeleton_skull"
		case 2:
			s.DefaultBlock.Name = "minecraft:zombie_head"
		case 3:
			s.DefaultBlock.Name = "minecraft:player_head"
		case 4:
			s.DefaultBlock.Name = "minecraft:creeper_head"
		case 5:
			s.DefaultBlock.Name = "minecraft:dragon_head"
		case 6:
			s.DefaultBlock.Name = "minecraft:piglin_head"
		default:
			return fmt.Errorf("Parse: Should never happened")
		}
		s.NBT.SkullType = 255
	}

	return nil
}

func (s Skull) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.Float32(&s.NBT.Rotation)
	w.Uint8(&s.NBT.SkullType)

	return buf.Bytes()
}

func (s *Skull) FullStableBytes() []byte {
	return append(s.DefaultBlock.FullStableBytes(), s.NBTStableBytes()...)
}
