package nbt_parser_item

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/mitchellh/mapstructure"
)

// FireworkExplosion ..
type FireworkExplosion struct {
	FireworkType    byte   `mapstructure:"FireworkType"`
	FireworkColor   []byte `mapstructure:"FireworkColor"`
	FireworkFade    []byte `mapstructure:"FireworkFade"`
	FireworkFlicker bool   `mapstructure:"FireworkFlicker"`
	FireworkTrail   bool   `mapstructure:"FireworkTrail"`
}

// Fireworks ..
type Fireworks struct {
	Flight     byte                `mapstructure:"Flight"`
	Explosions []FireworkExplosion `mapstructure:"Explosions"`
}

// FireworkTag ..
type FireworkTag struct {
	Fireworks Fireworks `mapstructure:"Fireworks"`
}

// FireworkRocketNBT ..
type FireworkRocketNBT struct {
	HaveFireworks bool
	Flight        uint8
	Explosions    []FireworkExplosion
}

// 烟花火箭
type FireworkRocket struct {
	DefaultItem
	NBT FireworkRocketNBT
}

func fireworkColorFormat(colors []byte) string {
	if len(colors) == 0 {
		return "无"
	}

	formatted := make([]string, 0, len(colors))
	for _, colorID := range colors {
		colorName, ok := mapping.ColorFormat[int32(colorID)]
		if !ok {
			colorName = fmt.Sprintf("未知(%d)", colorID)
		}
		formatted = append(formatted, colorName)
	}

	return strings.Join(formatted, "、")
}

// formatNBT 输出格式化
func (f FireworkRocket) formatNBT(prefix string) string {
	flightName, ok := mapping.FireworkFlightFormat[f.NBT.Flight]
	if !ok {
		flightName = fmt.Sprintf("未知(%d)", f.NBT.Flight)
	}

	result := prefix + fmt.Sprintf("飞行时间: %d (%s)\n", f.NBT.Flight, flightName)
	result += prefix + fmt.Sprintf("爆炸数量: %d\n", len(f.NBT.Explosions))

	for i, e := range f.NBT.Explosions {
		typeName, ok := mapping.FireworkTypeFormat[e.FireworkType]
		if !ok {
			typeName = fmt.Sprintf("未知(%d)", e.FireworkType)
		}

		result += prefix + fmt.Sprintf("爆炸 #%d: \n", i)
		result += prefix + "\t" + fmt.Sprintf("形状: %s\n", typeName)
		result += prefix + "\t" + fmt.Sprintf("基础颜色: %s\n", fireworkColorFormat(e.FireworkColor))
		result += prefix + "\t" + fmt.Sprintf("褪色颜色: %s\n", fireworkColorFormat(e.FireworkFade))
		result += prefix + "\t" + fmt.Sprintf("闪烁: %s\n", mapping.BoolFormat[e.FireworkFlicker])
		result += prefix + "\t" + fmt.Sprintf("拖尾: %s\n", mapping.BoolFormat[e.FireworkTrail])
	}

	return result
}

func (f FireworkRocket) Format(prefix string) string {
	result := f.DefaultItem.Format(prefix)
	if f.IsComplex() {
		result += prefix + "附加数据: \n"
		result += f.formatNBT(prefix + "\t")
	}
	return result
}

// parse ..
func (f *FireworkRocket) parse(tag map[string]any) error {
	f.DefaultItem.Enhance.ItemComponent.LockInInventory = false
	f.DefaultItem.Enhance.ItemComponent.LockInSlot = false
	f.DefaultItem.Enhance.EnchList = nil
	f.DefaultItem.Block = ItemBlockData{}

	if len(tag) == 0 {
		return nil
	}

	var data FireworkTag
	if err := mapstructure.WeakDecode(tag, &data); err != nil {
		return fmt.Errorf("mapstructure decode: %w", err)
	}

	fw := data.Fireworks
	f.NBT.HaveFireworks = true
	f.NBT.Flight = uint8(fw.Flight)
	f.NBT.Explosions = fw.Explosions

	return nil
}

func (f *FireworkRocket) ParseNormal(nbtMap map[string]any) error {
	tag, _ := nbtMap["tag"].(map[string]any)
	if err := f.parse(tag); err != nil {
		return fmt.Errorf("ParseNormal: %v", err)
	}
	return nil
}

func (f *FireworkRocket) ParseNetwork(stack protocol.ItemStack, name string) error {
	if err := f.parse(stack.NBTData); err != nil {
		return fmt.Errorf("ParseNetwork: %v", err)
	}
	return nil
}

func (f FireworkRocket) IsComplex() bool {
	return f.NBT.HaveFireworks
}

func (f FireworkRocket) complexFieldsOnly() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	have := f.NBT.HaveFireworks
	w.Bool(&have)
	if !have {
		return buf.Bytes()
	}

	w.Uint8(&f.NBT.Flight)

	expCount := uint16(len(f.NBT.Explosions))
	w.Uint16(&expCount)

	for _, e := range f.NBT.Explosions {
		w.Uint8(&e.FireworkType)
		w.Bool(&e.FireworkFlicker)
		w.Bool(&e.FireworkTrail)
		w.ByteSlice(&e.FireworkColor)
		w.ByteSlice(&e.FireworkFade)
	}

	return buf.Bytes()
}

func (f *FireworkRocket) NBTStableBytes() []byte {
	return append(f.DefaultItem.NBTStableBytes(), f.complexFieldsOnly()...)
}

func (f *FireworkRocket) TypeStableBytes() []byte {
	return append(f.DefaultItem.TypeStableBytes(), f.complexFieldsOnly()...)
}

func (f *FireworkRocket) FullStableBytes() []byte {
	return append(f.TypeStableBytes(), f.Basic.Count)
}
