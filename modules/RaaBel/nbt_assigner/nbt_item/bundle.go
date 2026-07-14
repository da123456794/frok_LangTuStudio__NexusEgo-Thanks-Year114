package nbt_item

import (
	"fmt"
	"time"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	nbt_assigner_interface "github.com/LangTuStudio/RaaBel/nbt_assigner/interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_hash "github.com/LangTuStudio/RaaBel/nbt_parser/hash"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
	"github.com/LangTuStudio/RaaBel/utils"
)

const (
	bundleWaitTimeout = time.Second
)

type bundleMoveInfo struct {
	SourceSlot resources_control.SlotID
	BundleSlot uint8
	Count      uint8
}

// 收纳袋
type Bundle struct {
	api   *nbt_console.Console
	cache *nbt_cache.NBTCacheSystem
	items []nbt_parser_item.Bundle
}

func (b *Bundle) Append(item ...nbt_parser_interface.Item) {
	for _, value := range item {
		val, ok := value.(*nbt_parser_item.Bundle)
		if !ok {
			continue
		}
		b.items = append(b.items, *val)
	}
}

func (b *Bundle) Make() (resultSlot map[uint64]resources_control.SlotID, err error) {
	if len(b.items) == 0 {
		return nil, nil
	}

	resultSlot = make(map[uint64]resources_control.SlotID)
	for _, target := range b.items {
		slot, err := b.makeOne(target)
		if err != nil {
			return nil, fmt.Errorf("Make: %v", err)
		}
		resultSlot[nbt_hash.NBTItemNBTHash(&target)] = slot
	}

	b.items = nil
	return resultSlot, nil
}

func (b *Bundle) makeOne(target nbt_parser_item.Bundle) (resources_control.SlotID, error) {
	moves, usedSlots, err := b.prepareStorageItems(target.NBT.StorageItems)
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}

	bundleSlot, err := b.findBundleSlotInHotbar(usedSlots)
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}

	underlying := target.UnderlyingItem().(*nbt_parser_item.DefaultItem)
	err = b.api.API().Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathInventory,
		game_interface.ReplaceitemInfo{
			Name:     target.ItemName(),
			Count:    1,
			MetaData: target.ItemMetadata(),
			Slot:     bundleSlot,
		},
		utils.MarshalItemComponent(underlying.Enhance.ItemComponent),
		true,
	)
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}
	b.api.UseInventorySlot(nbt_console.RequesterUser, bundleSlot, true)
	bundleItem, err := b.waitInventorySlot(bundleSlot, 1)
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}
	err = b.api.API().Commands().AwaitChangesGeneral()
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}

	if len(moves) == 0 {
		return bundleSlot, nil
	}

	bundleWindowID, found := bundleIDFromItem(*bundleItem)
	if !found || bundleWindowID < 0 {
		return 0, fmt.Errorf("makeOne: Dynamic container ID is missing")
	}
	dynamicContainerID := resources_control.DynamicContainerID(bundleWindowID)

	for _, moveInfo := range moves {
		success, _, _, err := b.api.API().ItemStackOperation().OpenTransaction().
			MoveToDynamicContainer(moveInfo.SourceSlot, dynamicContainerID, resources_control.SlotID(moveInfo.BundleSlot), moveInfo.Count).
			Commit()
		if err != nil {
			return 0, fmt.Errorf("makeOne: %v", err)
		}
		if !success {
			return 0, fmt.Errorf("makeOne: Move item to bundle failed")
		}
		b.api.UseInventorySlot(nbt_console.RequesterUser, moveInfo.SourceSlot, false)
	}

	return bundleSlot, nil
}

func (b *Bundle) prepareStorageItems(storageItems []nbt_parser_item.BundleItemWithSlot) ([]bundleMoveInfo, []resources_control.SlotID, error) {
	usedSlots := make([]resources_control.SlotID, 0)
	moves := make([]bundleMoveInfo, 0, len(storageItems))

	for _, value := range storageItems {
		if value.Item == nil || value.Item.ItemCount() == 0 {
			continue
		}

		var sourceSlot resources_control.SlotID
		var err error
		if value.Item.IsComplex() {
			sourceSlot, err = b.makeComplexStorageItem(value.Item, usedSlots)
		} else {
			sourceSlot, err = b.makeNormalStorageItem(value.Item, usedSlots)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("prepareStorageItems: %v", err)
		}
		_, err = b.waitInventorySlot(sourceSlot, value.Item.ItemCount())
		if err != nil {
			return nil, nil, fmt.Errorf("prepareStorageItems: %v", err)
		}
		usedSlots = append(usedSlots, sourceSlot)
		moves = append(moves, bundleMoveInfo{
			SourceSlot: sourceSlot,
			BundleSlot: value.Slot,
			Count:      value.Item.ItemCount(),
		})
	}

	return moves, usedSlots, nil
}

func (b *Bundle) makeComplexStorageItem(item nbt_parser_interface.Item, exclusion []resources_control.SlotID) (resources_control.SlotID, error) {
	underlying := item.UnderlyingItem().(*nbt_parser_item.DefaultItem)
	if underlying.Block.SubBlock != nil {
		slot, err := b.makeSubBlockStorageItem(underlying.Block.SubBlock)
		if err != nil {
			return 0, fmt.Errorf("makeComplexStorageItem: %v", err)
		}
		return b.moveItemIfSlotConflict(slot, exclusion, item.ItemCount())
	}

	slot, err := b.makeSupportedComplexStorageItem(item)
	if err != nil {
		return 0, fmt.Errorf("makeComplexStorageItem: %v", err)
	}

	return b.moveItemIfSlotConflict(slot, exclusion, item.ItemCount())
}

func (b *Bundle) makeSupportedComplexStorageItem(item nbt_parser_interface.Item) (resources_control.SlotID, error) {

	hashNumber := nbt_hash.NBTItemNBTHash(item)
	makers := nbt_assigner_interface.MakeNBTItemMethod(b.api, b.cache, item)
	if len(makers) == 0 {
		return 0, fmt.Errorf("makeSupportedComplexStorageItem: Can not find maker for %s", item.ItemName())
	}

	for _, maker := range makers {
		for {
			resultSlot, err := maker.Make()
			if err != nil {
				return 0, fmt.Errorf("makeSupportedComplexStorageItem: %v", err)
			}
			if len(resultSlot) == 0 {
				break
			}
			slot, ok := resultSlot[hashNumber]
			if ok {
				return slot, nil
			}
		}
	}

	return 0, fmt.Errorf("makeSupportedComplexStorageItem: Failed to make complex item %s", item.ItemName())
}

func (b *Bundle) makeSubBlockStorageItem(subBlock nbt_parser_interface.Block) (resources_control.SlotID, error) {
	api := b.api.API()
	if !subBlock.NeedSpecialHandle() {
		return 0, fmt.Errorf("makeSubBlockStorageItem: Sub block %s does not need special handle", subBlock.BlockName())
	}
	if !nbt_assigner_interface.NBTBlockIsSupported(subBlock) {
		return 0, fmt.Errorf("makeSubBlockStorageItem: Unsupported sub block %s", subBlock.BlockName())
	}

	err := b.api.CanReachOrMove(b.api.Center())
	if err != nil {
		return 0, fmt.Errorf("makeSubBlockStorageItem: %v", err)
	}

	_, _, _, err = nbt_assigner_interface.PlaceNBTBlock(b.api, b.cache, subBlock)
	if err != nil {
		return 0, fmt.Errorf("makeSubBlockStorageItem: %v", err)
	}

	success, currentSlot, err := api.BotClick().PickBlock(b.api.Center(), true)
	if err != nil {
		return 0, fmt.Errorf("makeSubBlockStorageItem: %v", err)
	}
	if !success {
		return 0, fmt.Errorf("makeSubBlockStorageItem: Failed to pick block due to unknown reason")
	}

	b.api.UpdateHotbarSlotID(currentSlot)
	b.api.UseInventorySlot(nbt_console.RequesterUser, currentSlot, true)
	return currentSlot, nil
}

func (b *Bundle) moveItemIfSlotConflict(
	sourceSlot resources_control.SlotID,
	exclusion []resources_control.SlotID,
	count uint8,
) (resources_control.SlotID, error) {
	if !slotInExclusion(sourceSlot, exclusion) {
		return sourceSlot, nil
	}

	targetSlot := b.api.FindInventorySlot(exclusion)
	if slotInExclusion(targetSlot, exclusion) {
		return 0, fmt.Errorf("moveItemIfSlotConflict: No available inventory slot")
	}

	api := b.api.API()
	success, err := api.ContainerOpenAndClose().OpenInventory()
	if err != nil {
		return 0, fmt.Errorf("moveItemIfSlotConflict: %v", err)
	}
	if !success {
		return 0, fmt.Errorf("moveItemIfSlotConflict: Failed to open the inventory")
	}
	defer api.ContainerOpenAndClose().CloseContainer()

	success, _, _, err = api.ItemStackOperation().OpenTransaction().
		MoveBetweenInventory(sourceSlot, targetSlot, count).
		Commit()
	if err != nil {
		return 0, fmt.Errorf("moveItemIfSlotConflict: %v", err)
	}
	if !success {
		return 0, fmt.Errorf("moveItemIfSlotConflict: The server rejected the item stack request actions")
	}

	b.api.UseInventorySlot(nbt_console.RequesterUser, sourceSlot, false)
	b.api.UseInventorySlot(nbt_console.RequesterUser, targetSlot, true)
	return targetSlot, nil
}

func (b *Bundle) makeNormalStorageItem(item nbt_parser_interface.Item, exclusion []resources_control.SlotID) (resources_control.SlotID, error) {
	underlying := item.UnderlyingItem().(*nbt_parser_item.DefaultItem)
	if underlying.NeedEnchRenameDyeOrLore() {
		return b.makeEnhancedStorageItem(item, exclusion)
	}

	slot := b.api.FindInventorySlot(exclusion)

	err := b.api.API().Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathInventory,
		game_interface.ReplaceitemInfo{
			Name:     item.ItemName(),
			Count:    item.ItemCount(),
			MetaData: item.ItemMetadata(),
			Slot:     slot,
		},
		utils.MarshalItemComponent(underlying.Enhance.ItemComponent),
		true,
	)
	if err != nil {
		return 0, fmt.Errorf("makeNormalStorageItem: %v", err)
	}
	b.api.UseInventorySlot(nbt_console.RequesterUser, slot, true)

	return slot, nil
}

func (b *Bundle) makeEnhancedStorageItem(item nbt_parser_interface.Item, exclusion []resources_control.SlotID) (resources_control.SlotID, error) {
	api := b.api.API()
	underlying := item.UnderlyingItem().(*nbt_parser_item.DefaultItem)

	slot, err := b.findBundleSlotInHotbar(exclusion)
	if err != nil {
		return 0, fmt.Errorf("makeEnhancedStorageItem: %v", err)
	}

	err = api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathInventory,
		game_interface.ReplaceitemInfo{
			Name:     item.ItemName(),
			Count:    item.ItemCount(),
			MetaData: item.ItemMetadata(),
			Slot:     slot,
		},
		utils.MarshalItemComponent(underlying.Enhance.ItemComponent),
		true,
	)
	if err != nil {
		return 0, fmt.Errorf("makeEnhancedStorageItem: %v", err)
	}
	b.api.UseInventorySlot(nbt_console.RequesterUser, slot, true)

	err = api.Commands().AwaitChangesGeneral()
	if err != nil {
		return 0, fmt.Errorf("makeEnhancedStorageItem: %v", err)
	}

	if b.api.HotbarSlotID() != slot {
		err = b.api.ChangeAndUpdateHotbarSlotID(slot)
		if err != nil {
			return 0, fmt.Errorf("makeEnhancedStorageItem: %v", err)
		}
	}

	err = nbt_assigner_interface.EnchRenameDyeOrLoreSingle(b.api, item, slot)
	if err != nil {
		return 0, fmt.Errorf("makeEnhancedStorageItem: %v", err)
	}

	return slot, nil
}

func slotInExclusion(slot resources_control.SlotID, exclusion []resources_control.SlotID) bool {
	for _, value := range exclusion {
		if value == slot {
			return true
		}
	}
	return false
}

func (b *Bundle) findBundleSlotInHotbar(exclusion []resources_control.SlotID) (resources_control.SlotID, error) {
	exclusionMapping := make(map[resources_control.SlotID]bool)
	for _, slotID := range exclusion {
		exclusionMapping[slotID] = true
	}

	for index := range 9 {
		slotID := resources_control.SlotID(index)
		if exclusionMapping[slotID] {
			continue
		}
		return slotID, nil
	}

	return 0, fmt.Errorf("findBundleSlotInHotbar: No available hotbar slot")
}

func (b *Bundle) waitInventorySlot(slot resources_control.SlotID, expectCount uint8) (*protocol.ItemInstance, error) {
	api := b.api.API()
	resources := api.Resources()

	timer := time.NewTimer(bundleWaitTimeout)
	defer timer.Stop()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		item, inventoryExisted := resources.Inventories().GetItemStack(resources_control.WindowNameInventory, slot)
		if inventoryExisted && item != nil {
			if item.Stack.Count >= uint16(expectCount) && item.StackNetworkID != 0 {
				return item, nil
			}
		}

		select {
		case <-timer.C:
			return nil, fmt.Errorf("waitInventorySlot: Timeout waiting slot %d update", slot)
		case <-ticker.C:
		}
	}
}

func bundleIDFromItem(item protocol.ItemInstance) (int32, bool) {
	if item.Stack.NBTData == nil {
		return 0, false
	}
	value, ok := item.Stack.NBTData["bundle_id"]
	if !ok {
		return 0, false
	}
	return parseBundleID(value)
}

func parseBundleID(value any) (int32, bool) {
	switch typed := value.(type) {
	case int:
		return int32(typed), true
	case int8:
		return int32(typed), true
	case int16:
		return int32(typed), true
	case int32:
		return typed, true
	case int64:
		return int32(typed), true
	case uint8:
		return int32(typed), true
	case uint16:
		return int32(typed), true
	case uint32:
		return int32(typed), true
	case uint64:
		return int32(typed), true
	case float32:
		return int32(typed), true
	case float64:
		return int32(typed), true
	default:
		return 0, false
	}
}
