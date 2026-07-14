package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
)

// CampfireNBT ..
type CampfireNBT struct {
	CustomName  string
	HaveAnyItem bool
	Items       [4]nbt_parser_interface.Item
}

// 营火
type Campfire struct {
	DefaultBlock
	NBT CampfireNBT
}

func (c Campfire) NeedSpecialHandle() bool {
	return len(c.NBT.CustomName) != 0 || c.NBT.HaveAnyItem
}

func (Campfire) NeedCheckCompletely() bool {
	return true
}

func (c Campfire) formatNBT(prefix string) string {
	result := ""

	if len(c.NBT.CustomName) > 0 {
		result += prefix + fmt.Sprintf("自定义名称: %s\n", c.NBT.CustomName)
	}

	for index, item := range c.NBT.Items {
		if item != nil {
			result += prefix + fmt.Sprintf("卡槽 %d: \n", index)
			result += item.Format(prefix + "\t")
		}
	}

	return result
}

func (c *Campfire) Format(prefix string) string {
	result := c.DefaultBlock.Format(prefix)
	if c.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += c.formatNBT(prefix + "\t")
	}
	return result
}

func (c *Campfire) Parse(nbtMap map[string]any) error {
	c.NBT.CustomName, _ = nbtMap["CustomName"].(string)
	for slot := range 4 {
		itemMap, ok := nbtMap[fmt.Sprintf("Item%d", slot+1)].(map[string]any)
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
		c.NBT.Items[slot] = item
	}

	return nil
}

func (c Campfire) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.String(&c.NBT.CustomName)
	w.Bool(&c.NBT.HaveAnyItem)
	for _, item := range c.NBT.Items {
		haveItem := item != nil
		w.Bool(&haveItem)
		if haveItem {
			stableItemBytes := item.FullStableBytes()
			w.ByteSlice(&stableItemBytes)
		}
	}

	return buf.Bytes()
}

func (c *Campfire) FullStableBytes() []byte {
	return append(c.DefaultBlock.FullStableBytes(), c.NBTStableBytes()...)
}
