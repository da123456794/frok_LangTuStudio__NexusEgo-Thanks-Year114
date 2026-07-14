package nbt_parser_item

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
)

// BundleItemWithSlot ..
type BundleItemWithSlot struct {
	Item nbt_parser_interface.Item
	Slot uint8
}

// Format ..
func (i BundleItemWithSlot) Format(prefix string) string {
	result := prefix + fmt.Sprintf("- 所在收纳槽位: %d\n", i.Slot+1)
	result += prefix + "  物品数据: \n"
	result += i.Item.Format(prefix + "  \t")
	return result
}

// BundleNBT ..
type BundleNBT struct {
	Weight       int32
	StorageItems []BundleItemWithSlot
}

// 收纳袋
type Bundle struct {
	DefaultItem
	NBT BundleNBT
}

func (b Bundle) formatNBT(prefix string) string {
	result := ""

	if b.NBT.Weight > 0 {
		result += prefix + fmt.Sprintf("当前负重: %d\n", b.NBT.Weight)
	}

	if itemCount := len(b.NBT.StorageItems); itemCount > 0 {
		result += prefix + fmt.Sprintf("收纳内容 (合计 %d 个槽位): \n", itemCount)
	} else {
		result += prefix + "无收纳物品\n"
	}

	for _, item := range b.NBT.StorageItems {
		result += item.Format(prefix + "\t")
	}

	return result
}

func (b *Bundle) Format(prefix string) string {
	result := b.DefaultItem.Format(prefix)
	if b.IsComplex() {
		result += prefix + "附加数据: \n"
		result += b.formatNBT(prefix + "\t")
	}
	return result
}

func (b *Bundle) parse(tag map[string]any) error {
	b.DefaultItem.Enhance.ItemComponent.LockInInventory = false
	b.DefaultItem.Enhance.ItemComponent.LockInSlot = false
	b.DefaultItem.Enhance.EnchList = nil
	b.DefaultItem.Block = ItemBlockData{}

	if len(tag) == 0 {
		return nil
	}

	b.NBT.Weight, _ = tag["bundle_weight"].(int32)
	itemList, _ := tag["storage_item_component_content"].([]any)
	slotMap := make(map[uint8]nbt_parser_interface.Item)

	for idx, v := range itemList {
		value, ok := v.(map[string]any)
		if !ok {
			continue
		}

		name, _ := value["Name"].(string)
		if len(name) == 0 {
			continue
		}

		slot, ok := value["Slot"].(byte)
		if !ok {
			slot = byte(idx)
		}

		item, canGetByCommand, err := nbt_parser_interface.ParseItemNormal(b.NameChecker, value)
		if err != nil {
			return fmt.Errorf("Parse: %v", err)
		}
		if !canGetByCommand {
			continue
		}

		slotMap[slot] = item
	}

	// 排序槽位
	slots := make([]uint8, 0, len(slotMap))
	for s := range slotMap {
		slots = append(slots, s)
	}
	slices.SortStableFunc(slots, cmp.Compare)

	// 构造结果
	for _, s := range slots {
		b.NBT.StorageItems = append(b.NBT.StorageItems, BundleItemWithSlot{
			Slot: s,
			Item: slotMap[s],
		})
	}
	return nil
}

func (b *Bundle) ParseNormal(nbtMap map[string]any) error {
	tag, _ := nbtMap["tag"].(map[string]any)
	err := b.parse(tag)
	if err != nil {
		return fmt.Errorf("ParseNormal: %v", err)
	}
	return nil
}

func (b *Bundle) ParseNetwork(item protocol.ItemStack, itemName string) error {
	err := b.parse(item.NBTData)
	if err != nil {
		return fmt.Errorf("ParseNetwork: %v", err)
	}
	return nil
}

func (b Bundle) IsComplex() bool {
	return len(b.NBT.StorageItems) > 0
}

func (b Bundle) complexFieldsOnly() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.Int32(&b.NBT.Weight)
	itemCount := uint8(len(b.NBT.StorageItems))
	w.Uint8(&itemCount)
	for _, item := range b.NBT.StorageItems {
		stableItemBytes := append(item.Item.FullStableBytes(), item.Slot)
		w.ByteSlice(&stableItemBytes)
	}

	return buf.Bytes()
}

func (b *Bundle) NBTStableBytes() []byte {
	return append(b.DefaultItem.NBTStableBytes(), b.complexFieldsOnly()...)
}

func (b *Bundle) TypeStableBytes() []byte {
	return append(b.DefaultItem.TypeStableBytes(), b.complexFieldsOnly()...)
}

func (b *Bundle) FullStableBytes() []byte {
	return append(b.TypeStableBytes(), b.Basic.Count)
}
