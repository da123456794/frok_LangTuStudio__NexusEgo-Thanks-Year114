package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
)

// DecoratedPotNBT ..
type DecoratedPotNBT struct {
	HaveItem         bool
	Item             nbt_parser_interface.Item
	HaveSpecialSherd bool
	Sherds           [4]string // 固定 4 个: 后、左、右、前
}

// 纹饰陶罐
type DecoratedPot struct {
	DefaultBlock
	NBT DecoratedPotNBT
}

func (d DecoratedPot) NeedSpecialHandle() bool {
	return d.NBT.HaveItem || d.NBT.HaveSpecialSherd
}

func (DecoratedPot) NeedCheckCompletely() bool {
	return true
}

func (d *DecoratedPot) formatNBT(prefix string) string {
	result := ""

	if d.NBT.HaveItem && d.NBT.Item != nil {
		result += prefix + "存储物品: \n"
		result += d.NBT.Item.Format(prefix + "\t")
	} else {
		result += prefix + "无存储物品\n"
	}

	result += prefix + "陶片(后 左 右 前): \n"
	for i, s := range d.NBT.Sherds {
		result += prefix + fmt.Sprintf("\t[%d] %s\n", i, s)
	}

	return result
}

func (d *DecoratedPot) Format(prefix string) string {
	result := d.DefaultBlock.Format(prefix)
	if d.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += d.formatNBT(prefix + "\t")
	}
	return result
}

func (d *DecoratedPot) Parse(nbtMap map[string]any) error {
	// 陶片 sherds [4]
	if sherdsArr, ok := nbtMap["sherds"].([]any); ok && len(sherdsArr) >= 4 {
		for i := range 4 {
			s, _ := sherdsArr[i].(string)
			if s != "minecraft:brick" {
				d.NBT.HaveSpecialSherd = true
			}
			d.NBT.Sherds[i] = s
		}
	}

	// 物品 item
	itemMap, ok := nbtMap["item"].(map[string]any)
	if ok {
		itemName, _ := itemMap["Name"].(string)

		if len(itemName) == 0 {
			return nil
		}

		item, canGetByCommand, err := nbt_parser_interface.ParseItemNormal(d.NameChecker, itemMap)
		if err != nil {
			return fmt.Errorf("Parse: %v", err)
		}
		if canGetByCommand {
			d.NBT.HaveItem = true
			d.NBT.Item = item
		}
	}

	return nil
}

func (d DecoratedPot) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	// 物品
	w.Bool(&d.NBT.HaveItem)
	if d.NBT.HaveItem {
		itemBytes := d.NBT.Item.FullStableBytes()
		w.ByteSlice(&itemBytes)
	}

	// 4 个陶片
	w.Bool(&d.NBT.HaveSpecialSherd)
	if d.NBT.HaveSpecialSherd {
		for i := range 4 {
			s := d.NBT.Sherds[i]
			w.String(&s)
		}
	}

	return buf.Bytes()
}

func (d *DecoratedPot) FullStableBytes() []byte {
	return append(d.DefaultBlock.FullStableBytes(), d.NBTStableBytes()...)
}
