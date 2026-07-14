package nbt_parser_block

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/LangTuStudio/RaaBel/utils"
	"github.com/mitchellh/mapstructure"
)

// CauldronNBT ..
type CauldronNBT struct {
	CustomColor int32 `mapstructure:"CustomColor"`
	PotionID    int16 `mapstructure:"PotionId"`
	PotionType  int16 `mapstructure:"PotionType"`
	// Items           []ItemWithSlot 我不知道这个值应该从何而来
}

// 炼药锅
type Cauldron struct {
	DefaultBlock
	NBT CauldronNBT
}

func (c Cauldron) NeedSpecialHandle() bool {
	return c.NBT.CustomColor != 0 || c.NBT.PotionID != -1 || c.NBT.PotionType != -1
}

func (Cauldron) NeedCheckCompletely() bool {
	return true
}

func (c Cauldron) formatNBT(prefix string) string {
	var result strings.Builder
	if c.NBT.CustomColor != 0 {
		rgb, _ := utils.DecodeVarRGBA(c.NBT.CustomColor)
		dyeIDs, found := utils.SearchCauldronDyeIDsByColor(rgb)
		if found {
			result.WriteString(prefix + "颜色配方:")
		}
		for _, dyeID := range dyeIDs {
			name := mapping.RGBFormat[mapping.DefaultDyeColor[dyeID]]
			result.WriteString(" " + name)
		}
		result.WriteString("\n")
	}
	if c.NBT.PotionID != -1 {
		result.WriteString(prefix + fmt.Sprintf("药水 ID: %d\n", c.NBT.PotionID))
	}
	if c.NBT.PotionType != -1 {
		result.WriteString(prefix + fmt.Sprintf("药水类型: %s\n", mapping.PotionTypeFormat[c.NBT.PotionType]))
	}

	return result.String()
}

func (c *Cauldron) Format(prefix string) string {
	result := c.DefaultBlock.Format(prefix)
	if c.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += c.formatNBT(prefix + "\t")
	}
	return result
}

func (c *Cauldron) Parse(nbtMap map[string]any) error {
	var result CauldronNBT

	err := mapstructure.Decode(&nbtMap, &result)
	if err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	c.NBT = result

	return nil
}

func (c Cauldron) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	haveCustomColor := c.NBT.CustomColor != 0
	w.Bool(&haveCustomColor)
	if haveCustomColor {
		color, _ := utils.DecodeVarRGBA(c.NBT.CustomColor)
		dyeIDs, _ := utils.SearchCauldronDyeIDsByColor(color)
		w.ByteSlice(&dyeIDs)
	}
	w.Int16(&c.NBT.PotionID)
	w.Int16(&c.NBT.PotionType)

	return buf.Bytes()
}

func (c *Cauldron) FullStableBytes() []byte {
	return append(c.DefaultBlock.FullStableBytes(), c.NBTStableBytes()...)
}
