package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/Happy2018new/worldupgrader/blockupgrader"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/utils"
	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/mitchellh/mapstructure"
)

// PlantBlock ..
type PlantBlock struct {
	Name    string         `mapstructure:"name"`
	States  map[string]any `mapstructure:"states"`
	Version int32          `mapstructure:"version"`
}

// FlowerPotNBT ..
type FlowerPotNBT struct {
	HavePlant  bool       `mapstructure:"-"`
	PlantBlock PlantBlock `mapstructure:"PlantBlock"`
}

// 花盆
type FlowerPot struct {
	DefaultBlock
	NBT FlowerPotNBT
}

func (f FlowerPot) NeedSpecialHandle() bool {
	return f.NBT.HavePlant
}

func (FlowerPot) NeedCheckCompletely() bool {
	return true
}

func (f FlowerPot) formatNBT(prefix string) string {
	if !f.NBT.HavePlant {
		return ""
	}

	result := prefix + "植物数据: \n"
	result += prefix + "\t" + fmt.Sprintf("方块名称: %s\n", f.NBT.PlantBlock.Name)
	result += prefix + "\t" + fmt.Sprintf("方块状态: %s\n", utils.MarshalBlockStates(f.NBT.PlantBlock.States))
	result += prefix + "\t" + fmt.Sprintf("方块版本: %d\n", f.NBT.PlantBlock.Version)
	return result
}

func (f *FlowerPot) Format(prefix string) string {
	result := f.DefaultBlock.Format(prefix)
	if f.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += f.formatNBT(prefix + "\t")
	}
	return result
}

func (f *FlowerPot) Parse(nbtMap map[string]any) error {
	var result FlowerPotNBT

	err := mapstructure.Decode(&nbtMap, &result)
	if err != nil {
		return fmt.Errorf("Parse: %v", err)
	}

	if result.PlantBlock.Name == "" {
		f.NBT = FlowerPotNBT{}
		return nil
	}

	newBlock := blockupgrader.Upgrade(blockupgrader.BlockState{
		Name:       result.PlantBlock.Name,
		Properties: result.PlantBlock.States,
		Version:    result.PlantBlock.Version,
	})

	result.HavePlant = true
	result.PlantBlock.Name = newBlock.Name
	result.PlantBlock.States = newBlock.Properties
	result.PlantBlock.Version = newBlock.Version
	f.NBT = result

	return nil
}

func (f FlowerPot) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	havePlant := f.NBT.HavePlant
	w.Bool(&havePlant)
	if havePlant {
		blockRuntimeID, _ := block.StateToRuntimeID(f.NBT.PlantBlock.Name, f.NBT.PlantBlock.States)
		// version := f.NBT.PlantBlock.Version

		w.Uint32(&blockRuntimeID)
		// w.Int32(&version)
	}

	return buf.Bytes()
}

func (f *FlowerPot) FullStableBytes() []byte {
	return append(f.DefaultBlock.FullStableBytes(), f.NBTStableBytes()...)
}
