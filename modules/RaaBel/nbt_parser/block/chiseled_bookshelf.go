package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
)

// ChiseledBookshelfNBT ..
type ChiseledBookshelfNBT struct {
	HaveAnyItem            bool
	Items                  [6]nbt_parser_interface.Item
	HaveLastInteractedSlot bool
	LastInteractedSlot     resources_control.SlotID
}

// 雕纹书架
type ChiseledBookshelf struct {
	DefaultBlock
	NBT ChiseledBookshelfNBT
}

func (c ChiseledBookshelf) NeedSpecialHandle() bool {
	return c.NBT.HaveLastInteractedSlot
}

func (c ChiseledBookshelf) NeedCheckCompletely() bool {
	return true
}

func (c ChiseledBookshelf) formatNBT(prefix string) string {
	result := ""
	if c.NBT.HaveLastInteractedSlot {
		result += prefix + fmt.Sprintf("上次交互的卡槽: %d\n", c.NBT.LastInteractedSlot)
	}
	for index, item := range c.NBT.Items {
		if item == nil {
			result += prefix + fmt.Sprintf("卡槽 %d: 空\n", index)
			continue
		}
		result += prefix + fmt.Sprintf("卡槽 %d: \n", index)
		result += item.Format(prefix + "\t")
	}
	return result
}

func (c *ChiseledBookshelf) Format(prefix string) string {
	result := c.DefaultBlock.Format(prefix)
	if c.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += c.formatNBT(prefix + "\t")
	}
	return result
}

func (c *ChiseledBookshelf) Parse(nbtMap map[string]any) error {
	var lastInteractedSlot int32
	lastInteractedSlot, c.NBT.HaveLastInteractedSlot = nbtMap["LastInteractedSlot"].(int32)
	// 雕纹书架的 slot 是从 1 开始的
	c.NBT.LastInteractedSlot = resources_control.SlotID(lastInteractedSlot - 1)

	items, _ := nbtMap["Items"].([]any)
	for index := 0; index < len(c.NBT.Items) && index < len(items); index++ {
		itemMap, ok := items[index].(map[string]any)
		if !ok {
			continue
		}

		if name, _ := itemMap["Name"].(string); len(name) == 0 {
			continue
		}

		item, canGetByCommand, err := nbt_parser_interface.ParseItemNormal(c.NameChecker, itemMap)
		if err != nil {
			return fmt.Errorf("Parse: %v", err)
		}
		if !canGetByCommand {
			continue
		}

		c.NBT.HaveAnyItem = true
		c.NBT.Items[index] = item
	}

	return nil
}

func (c ChiseledBookshelf) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.Bool(&c.NBT.HaveAnyItem)
	if c.NBT.HaveAnyItem {
		w.Bool(&c.NBT.HaveLastInteractedSlot)
		w.Uint8((*uint8)(&c.NBT.LastInteractedSlot))
	}
	for _, item := range c.NBT.Items {
		haveItem := item != nil
		w.Bool(&haveItem)
		if haveItem {
			stableBytes := item.FullStableBytes()
			w.ByteSlice(&stableBytes)
		}
	}

	return buf.Bytes()
}

func (c *ChiseledBookshelf) FullStableBytes() []byte {
	return append(c.DefaultBlock.FullStableBytes(), c.NBTStableBytes()...)
}
