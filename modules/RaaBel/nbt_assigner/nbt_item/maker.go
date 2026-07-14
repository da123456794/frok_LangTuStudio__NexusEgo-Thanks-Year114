package nbt_item

import (
	"fmt"
	"sort"

	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	nbt_assigner_interface "github.com/LangTuStudio/RaaBel/nbt_assigner/interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
)

func init() {
	nbt_assigner_interface.MakeNBTItemMethod = MakeNBTItemMethod
	nbt_assigner_interface.EnchMultiple = EnchMultiple
	nbt_assigner_interface.RenameMultiple = RenameMultiple
	nbt_assigner_interface.DyeMultiple = DyeMultiple
	nbt_assigner_interface.LoreMultiple = LoreMultiple
	nbt_assigner_interface.EnchRenameDyeOrLoreMultiple = EnchRenameOrDyeMultiple
	nbt_assigner_interface.EnchRenameDyeOrLoreSingle = EnchRenameOrDyeSingle
}

// NBTItemIsSupported 检查 item 是否是受支持的复杂物品
func NBTItemIsSupported(item nbt_parser_interface.Item) bool {
	switch item.(type) {
	case *nbt_parser_item.Book:
	case *nbt_parser_item.Banner:
	case *nbt_parser_item.Shield:
	case *nbt_parser_item.FireworkRocket:
	case *nbt_parser_item.Bundle:
	default:
		return false
	}
	return true
}

// MakeNBTItemMethod 根据传入的操作台、缓存命中系统和多个物品，
// 将它们归类为每种复杂物品。对于 result 中的每个元素，可以使用
// Make 制作它们
func MakeNBTItemMethod(
	console *nbt_console.Console,
	cache *nbt_cache.NBTCacheSystem,
	multipleItems ...nbt_parser_interface.Item,
) (result []nbt_assigner_interface.Item) {
	if len(multipleItems) == 0 {
		return nil
	}

	books := make([]nbt_parser_interface.Item, 0)
	banners := make([]nbt_parser_interface.Item, 0)
	shields := make([]nbt_parser_interface.Item, 0)
	fireworks := make([]nbt_parser_interface.Item, 0)
	bundles := make([]nbt_parser_interface.Item, 0)

	for _, item := range multipleItems {
		switch item.(type) {
		case *nbt_parser_item.Book:
			books = append(books, item)
		case *nbt_parser_item.Banner:
			banners = append(banners, item)
		case *nbt_parser_item.Shield:
			shields = append(shields, item)
		case *nbt_parser_item.FireworkRocket:
			fireworks = append(fireworks, item)
		case *nbt_parser_item.Bundle:
			bundles = append(bundles, item)
		}
	}

	if len(books) > 0 {
		element := &Book{api: console}
		element.Append(books...)
		result = append(result, element)
	}
	if len(banners) > 0 {
		element := &Banner{
			api:             console,
			maxSlotCanUse:   BannerMaxSlotCanUse,
			maxBannerToMake: BannerMaxBannerToMake,
		}
		element.Append(banners...)
		result = append(result, element)
	}
	if len(shields) > 0 {
		element := &Shield{api: console}
		element.Append(shields...)
		result = append(result, element)
	}
	if len(fireworks) > 0 {
		element := &Firework{api: console}
		element.Append(fireworks...)
		result = append(result, element)
	}
	if len(bundles) > 0 {
		element := &Bundle{api: console, cache: cache}
		element.Append(bundles...)
		result = append(result, element)
	}

	return result
}

// EnchMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 将它们进行一一附魔处理。应当说明的是，这些物品应当置于非快捷栏的物品栏，
// 并且对于无需处理的物品，应当简单的置为 nil
func EnchMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	api := console.API()

	enchItems := make([]resources_control.SlotID, 0)
	enchItemsCount := make(map[resources_control.SlotID]uint8)

	for index, value := range multipleItems {
		if value == nil {
			continue
		}

		slotID := resources_control.SlotID(index + 9)
		defaultItem := (*value).UnderlyingItem().(*nbt_parser_item.DefaultItem)

		if len(defaultItem.Enhance.EnchList) > 0 {
			enchItems = append(enchItems, slotID)
			enchItemsCount[slotID] = defaultItem.ItemCount()
		}
	}

	if len(enchItems) > 0 {
		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return fmt.Errorf("EnchMultiple: %v", err)
		}
		if !success {
			return fmt.Errorf("EnchMultiple: Failed to open the inventory")
		}
		defer api.ContainerOpenAndClose().CloseContainer()
	}

	for {
		if len(enchItems) == 0 {
			break
		}

		currentRound := enchItems[0:min(len(enchItems), 9)]
		transaction := api.ItemStackOperation().OpenTransaction()

		for dstSlotID, srcSlotID := range currentRound {
			_ = transaction.MoveBetweenInventory(
				srcSlotID,
				resources_control.SlotID(dstSlotID),
				enchItemsCount[srcSlotID],
			)
		}

		success, _, _, err := transaction.Commit()
		if err != nil {
			return fmt.Errorf("EnchMultiple: %v", err)
		}
		if !success {
			return fmt.Errorf("EnchMultiple: The server rejected the item stack operation (Ench stage 1)")
		}

		for index, originSlotID := range currentRound {
			item := multipleItems[originSlotID-9]
			defaultItem := (*item).UnderlyingItem().(*nbt_parser_item.DefaultItem)

			currentSlotID := resources_control.SlotID(index)
			if console.HotbarSlotID() != currentSlotID {
				err = console.ChangeAndUpdateHotbarSlotID(currentSlotID)
				if err != nil {
					return fmt.Errorf("EnchMultiple: %v", err)
				}
			}

			for _, ench := range defaultItem.Enhance.EnchList {
				err = api.Commands().SendSettingsCommand(fmt.Sprintf("enchant @s %d %d", ench.ID, ench.Level), true)
				if err != nil {
					return fmt.Errorf("EnchMultiple: %v", err)
				}
			}

			err = api.Commands().AwaitChangesGeneral()
			if err != nil {
				return fmt.Errorf("EnchMultiple: %v", err)
			}
		}

		for currentSlotID, originSlotID := range currentRound {
			_ = transaction.MoveBetweenInventory(
				resources_control.SlotID(currentSlotID),
				originSlotID,
				enchItemsCount[originSlotID],
			)
		}

		success, _, _, err = transaction.Commit()
		if err != nil {
			return fmt.Errorf("EnchMultiple: %v", err)
		}
		if !success {
			return fmt.Errorf("EnchMultiple: The server rejected the item stack operation (Ench stage 2)")
		}

		enchItems = enchItems[len(currentRound):]
	}

	return nil
}

// RenameMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 将它们进行集中性物品改名处理。应当说明的是，这些物品应当置于非快捷栏的物品栏，
// 并且对于无需处理的物品，应当简单的置为 nil
func RenameMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	api := console.API()

	renameItems := make([]resources_control.SlotID, 0)
	renameItemsNewName := make([]string, 0)

	for index, value := range multipleItems {
		if value == nil {
			continue
		}

		slotID := resources_control.SlotID(index + 9)
		defaultItem := (*value).UnderlyingItem().(*nbt_parser_item.DefaultItem)
		displayName := defaultItem.Enhance.DisplayName

		if len(displayName) > 0 {
			renameItems = append(renameItems, slotID)
			renameItemsNewName = append(renameItemsNewName, displayName)
		}
	}

	if len(renameItems) == 0 {
		return nil
	}

	index, err := console.FindOrGenerateNewAnvil()
	if err != nil {
		return fmt.Errorf("RenameMultiple: %v", err)
	}

	success, err := console.OpenContainerByIndex(index)
	if err != nil {
		return fmt.Errorf("RenameMultiple: %v", err)
	}
	if !success {
		return fmt.Errorf("RenameMultiple: Failed to open the anvil")
	}
	defer api.ContainerOpenAndClose().CloseContainer()

	transaction := api.ItemStackOperation().OpenTransaction()
	for index, slotID := range renameItems {
		_ = transaction.RenameInventoryItem(
			slotID,
			renameItemsNewName[index],
		)
	}

	success, _, _, err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("RenameMultiple: %v", err)
	}
	if !success {
		return fmt.Errorf("RenameMultiple: The server rejected the renaming operation")
	}

	return nil
}

// dyeTask 描述一次待染色的物品任务。
type dyeTask struct {
	Item  nbt_parser_interface.Item
	Slot  resources_control.SlotID
	Color [3]uint8
}

func refillFrame(console *nbt_console.Console, frameIndex int) error {
	frame := block_helper.FrameBlockHelper{}
	blockPos := console.BlockPosByIndex(frameIndex)
	api := console.API()

	err := api.SetBlock().SetBlock(blockPos, "minecraft:air", "[]")
	if err != nil {
		return fmt.Errorf("refillFrame: %v", err)
	}
	err = api.SetBlock().SetBlock(blockPos, frame.BlockName(), frame.BlockStatesString())
	if err != nil {
		return fmt.Errorf("refillFrame: %v", err)
	}
	console.UseHelperBlock(nbt_console.RequesterSystemCall, frameIndex, frame)
	return nil
}

func loreSingleByFrame(
	console *nbt_console.Console,
	slot resources_control.SlotID,
	frameIndex int,
) error {
	api := console.API()
	targetSlot := slot
	originSlot := slot
	hotbarSlot := console.HotbarSlotID()
	moved := false

	err := refillFrame(console, frameIndex)
	if err != nil {
		return fmt.Errorf("loreSingleByFrame: %v", err)
	}

	if targetSlot > 8 {
		err := api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:  "minecraft:air",
				Count: 1,
				Slot:  hotbarSlot,
			},
			"",
			true,
		)
		if err != nil {
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}
		console.UseInventorySlot(nbt_console.RequesterUser, hotbarSlot, false)

		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}
		if !success {
			return fmt.Errorf("loreSingleByFrame: Failed to open the inventory")
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			MoveBetweenInventory(originSlot, hotbarSlot, 1).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("loreSingleByFrame: The server rejected the item stack request actions when move item to hotbar")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}

		console.UseInventorySlot(nbt_console.RequesterUser, originSlot, false)
		console.UseInventorySlot(nbt_console.RequesterUser, hotbarSlot, true)
		targetSlot = hotbarSlot
		moved = true
	}

	if console.HotbarSlotID() != targetSlot {
		err := console.ChangeAndUpdateHotbarSlotID(targetSlot)
		if err != nil {
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}
	}

	frameAny := *console.BlockByIndex(frameIndex)
	frameBlock, ok := frameAny.(block_helper.FrameBlockHelper)
	if !ok {
		return fmt.Errorf("loreSingleByFrame: Block block_helper.%T is not a frame", frameAny)
	}

	err = console.CanReachOrMove(console.BlockPosByIndex(frameIndex))
	if err != nil {
		return fmt.Errorf("loreSingleByFrame: %v", err)
	}

	err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: targetSlot,
		BotPos:       console.Position(),
		BlockPos:     console.BlockPosByIndex(frameIndex),
		BlockName:    frameBlock.BlockName(),
		BlockStates:  frameBlock.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("loreSingleByFrame: %v", err)
	}
	defer refillFrame(console, frameIndex)

	success, pickedSlot, err := api.BotClick().PickBlock(console.BlockPosByIndex(frameIndex), true)
	if err != nil {
		return fmt.Errorf("loreSingleByFrame: %v", err)
	}
	if !success {
		return fmt.Errorf("loreSingleByFrame: Failed to pick block due to unknown reason")
	}

	console.UpdateHotbarSlotID(pickedSlot)
	console.UseInventorySlot(nbt_console.RequesterUser, pickedSlot, true)

	if pickedSlot != originSlot {
		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}
		if !success {
			return fmt.Errorf("loreSingleByFrame: Failed to open the inventory")
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			MoveBetweenInventory(pickedSlot, originSlot, 1).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("loreSingleByFrame: The server rejected the item stack request actions when move item to origin")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}

		console.UseInventorySlot(nbt_console.RequesterUser, pickedSlot, false)
		console.UseInventorySlot(nbt_console.RequesterUser, originSlot, true)
	}

	if moved {
		console.UpdateHotbarSlotID(hotbarSlot)
	}

	if console.HotbarSlotID() != hotbarSlot {
		err = console.ChangeAndUpdateHotbarSlotID(hotbarSlot)
		if err != nil {
			return fmt.Errorf("loreSingleByFrame: %v", err)
		}
	}

	return nil
}

func refillCauldron(console *nbt_console.Console, cauldronIndex int) error {
	cauldron := block_helper.CauldronBlockHelper{States: map[string]any{
		"cauldron_liquid": "water",
		"fill_level":      int32(6),
	}}
	err := console.API().SetBlock().SetBlock(
		console.BlockPosByIndex(cauldronIndex),
		cauldron.BlockName(),
		cauldron.BlockStatesString(),
	)
	if err != nil {
		return fmt.Errorf("refillCauldron: %v", err)
	}
	console.UseHelperBlock(nbt_console.RequesterSystemCall, cauldronIndex, cauldron)
	return nil
}

func dyeSingleByCauldron(
	console *nbt_console.Console,
	item nbt_parser_interface.Item,
	slot resources_control.SlotID,
	cauldronIndex int,
) error {
	api := console.API()
	targetSlot := slot
	originSlot := slot
	hotbarSlot := console.HotbarSlotID()
	moved := false

	if targetSlot > 8 {
		err := api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:  "minecraft:air",
				Count: 1,
				Slot:  hotbarSlot,
			},
			"",
			true,
		)
		if err != nil {
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}
		console.UseInventorySlot(nbt_console.RequesterUser, hotbarSlot, false)

		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}
		if !success {
			return fmt.Errorf("dyeSingleByCauldron: Failed to open the inventory")
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			MoveBetweenInventory(originSlot, hotbarSlot, item.ItemCount()).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("dyeSingleByCauldron: The server rejected the item stack request actions when move item to hotbar")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}

		console.UseInventorySlot(nbt_console.RequesterUser, originSlot, false)
		console.UseInventorySlot(nbt_console.RequesterUser, hotbarSlot, true)
		targetSlot = hotbarSlot
		moved = true
	}

	if console.HotbarSlotID() != targetSlot {
		err := console.ChangeAndUpdateHotbarSlotID(targetSlot)
		if err != nil {
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}
	}

	cauldronAny := *console.BlockByIndex(cauldronIndex)
	cauldronBlock, ok := cauldronAny.(block_helper.CauldronBlockHelper)
	if !ok {
		return fmt.Errorf("dyeSingleByCauldron: Block block_helper.%T is not a cauldron", cauldronAny)
	}

	err := api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: targetSlot,
		BotPos:       console.Position(),
		BlockPos:     console.BlockPosByIndex(cauldronIndex),
		BlockName:    cauldronBlock.BlockName(),
		BlockStates:  cauldronBlock.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("dyeSingleByCauldron: %v", err)
	}

	currentStates := cauldronBlock.BlockStates()
	fillLevel, ok := currentStates["fill_level"].(int32)
	if !ok {
		fillLevel = 6
	}
	if fillLevel > 0 {
		fillLevel--
	}
	currentStates["fill_level"] = fillLevel
	console.UseHelperBlock(nbt_console.RequesterSystemCall, cauldronIndex, block_helper.CauldronBlockHelper{States: currentStates})

	err = refillCauldron(console, cauldronIndex)
	if err != nil {
		return fmt.Errorf("dyeSingleByCauldron: %v", err)
	}

	if moved {
		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}
		if !success {
			return fmt.Errorf("dyeSingleByCauldron: Failed to open the inventory")
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			MoveBetweenInventory(targetSlot, originSlot, item.ItemCount()).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("dyeSingleByCauldron: The server rejected the item stack request actions when move item back to inventory")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return fmt.Errorf("dyeSingleByCauldron: %v", err)
		}

		console.UseInventorySlot(nbt_console.RequesterUser, targetSlot, false)
		console.UseInventorySlot(nbt_console.RequesterUser, originSlot, true)
	}

	return nil
}

// dyeSingle 根据操作台 console、目标物品 item 和其所在槽位 slot，
// 对这个物品执行一次炼药锅染色处理
func dyeSingle(
	console *nbt_console.Console,
	item nbt_parser_interface.Item,
	slot resources_control.SlotID,
) error {
	defaultItem := item.UnderlyingItem().(*nbt_parser_item.DefaultItem)
	if defaultItem.Enhance.CustomColor == nil {
		return nil
	}

	cauldronIndex, err := console.FindOrGenerateNewCauldron(*defaultItem.Enhance.CustomColor)
	if err != nil {
		return fmt.Errorf("dyeSingle: %v", err)
	}
	err = dyeSingleByCauldron(console, item, slot, cauldronIndex)
	if err != nil {
		return fmt.Errorf("dyeSingle: %v", err)
	}
	return nil
}

// DyeMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 对其中需要染色的物品执行炼药锅染色处理。
// multipleItems 的第 i 个元素对应背包槽位 i+9
func DyeMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	tasks := make([]dyeTask, 0)
	for index, value := range multipleItems {
		if value == nil {
			continue
		}

		slotID := resources_control.SlotID(index + 9)
		defaultItem := (*value).UnderlyingItem().(*nbt_parser_item.DefaultItem)
		if defaultItem.Enhance.CustomColor == nil {
			continue
		}
		customColor := *defaultItem.Enhance.CustomColor
		tasks = append(tasks, dyeTask{
			Item:  *value,
			Slot:  slotID,
			Color: customColor,
		})
	}

	sort.Slice(tasks, func(i, j int) bool {
		for idx := range 3 {
			if tasks[i].Color[idx] != tasks[j].Color[idx] {
				return tasks[i].Color[idx] < tasks[j].Color[idx]
			}
		}
		return tasks[i].Slot < tasks[j].Slot
	})

	haveCurrentColor := false
	var currentColor [3]uint8
	currentCauldronIndex := 0

	for _, task := range tasks {
		if !haveCurrentColor || task.Color != currentColor {
			index, err := console.FindOrGenerateNewCauldron(task.Color)
			if err != nil {
				return fmt.Errorf("DyeMultiple: %v", err)
			}
			currentCauldronIndex = index
			currentColor = task.Color
			haveCurrentColor = true
		}

		err := dyeSingleByCauldron(console, task.Item, task.Slot, currentCauldronIndex)
		if err != nil {
			return fmt.Errorf("DyeMultiple: %v", err)
		}
	}
	return nil
}

// LoreMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 对其中需要 Lore 的物品执行物品展示框处理。
// multipleItems 的第 i 个元素对应背包槽位 i+9
func LoreMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	frameIndex, err := console.FindOrGenerateNewFrame()
	if err != nil {
		return fmt.Errorf("LoreMultiple: %v", err)
	}

	for index, value := range multipleItems {
		if value == nil {
			continue
		}

		slotID := resources_control.SlotID(index + 9)
		defaultItem := (*value).UnderlyingItem().(*nbt_parser_item.DefaultItem)
		if len(defaultItem.Enhance.Lore) > 0 {
			err = loreSingleByFrame(console, slotID, frameIndex)
			if err != nil {
				return fmt.Errorf("LoreMultiple: %v", err)
			}
		}
	}

	return nil
}

// EnchRenameOrDyeSingle 根据操作台 console、目标物品 item 和其所在槽位 slot，
// 对这个物品执行一次附魔、改名、染色和 Lore 处理
func EnchRenameOrDyeSingle(
	console *nbt_console.Console,
	item nbt_parser_interface.Item,
	slot resources_control.SlotID,
) error {
	if !item.NeedEnchRenameDyeOrLore() {
		return nil
	}
	defaultItem := item.UnderlyingItem().(*nbt_parser_item.DefaultItem)
	api := console.API()

	if len(defaultItem.Enhance.Lore) > 0 {
		index, err := console.FindOrGenerateNewFrame()
		if err != nil {
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}
		err = loreSingleByFrame(console, slot, index)
		if err != nil {
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}
	}

	if defaultItem.Enhance.CustomColor != nil {
		err := dyeSingle(console, item, slot)
		if err != nil {
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}
	}

	if len(defaultItem.Enhance.EnchList) > 0 {
		hotbarSlot := console.HotbarSlotID()
		targetSlot := slot
		moved := false

		if targetSlot > 8 {
			err := api.Replaceitem().ReplaceitemInInventory(
				"@s",
				game_interface.ReplacePathHotbarOnly,
				game_interface.ReplaceitemInfo{
					Name:  "minecraft:air",
					Count: 1,
					Slot:  hotbarSlot,
				},
				"",
				true,
			)
			if err != nil {
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}
			console.UseInventorySlot(nbt_console.RequesterUser, hotbarSlot, false)

			success, err := api.ContainerOpenAndClose().OpenInventory()
			if err != nil {
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}
			if !success {
				return fmt.Errorf("EnchRenameOrDyeSingle: Failed to open the inventory")
			}

			success, _, _, err = api.ItemStackOperation().OpenTransaction().
				MoveBetweenInventory(slot, hotbarSlot, item.ItemCount()).
				Commit()
			if err != nil {
				_ = api.ContainerOpenAndClose().CloseContainer()
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}
			if !success {
				_ = api.ContainerOpenAndClose().CloseContainer()
				return fmt.Errorf("EnchRenameOrDyeSingle: The server rejected the item stack request actions when move item to hotbar")
			}

			err = api.ContainerOpenAndClose().CloseContainer()
			if err != nil {
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}

			console.UseInventorySlot(nbt_console.RequesterUser, slot, false)
			console.UseInventorySlot(nbt_console.RequesterUser, hotbarSlot, true)
			targetSlot = hotbarSlot
			moved = true
		}

		if console.HotbarSlotID() != targetSlot {
			err := console.ChangeAndUpdateHotbarSlotID(targetSlot)
			if err != nil {
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}
		}

		for _, ench := range defaultItem.Enhance.EnchList {
			err := api.Commands().SendSettingsCommand(fmt.Sprintf("enchant @s %d %d", ench.ID, ench.Level), true)
			if err != nil {
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}
		}

		err := api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}

		if moved {
			success, err := api.ContainerOpenAndClose().OpenInventory()
			if err != nil {
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}
			if !success {
				return fmt.Errorf("EnchRenameOrDyeSingle: Failed to open the inventory")
			}

			success, _, _, err = api.ItemStackOperation().OpenTransaction().
				MoveBetweenInventory(targetSlot, slot, item.ItemCount()).
				Commit()
			if err != nil {
				_ = api.ContainerOpenAndClose().CloseContainer()
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}
			if !success {
				_ = api.ContainerOpenAndClose().CloseContainer()
				return fmt.Errorf("EnchRenameOrDyeSingle: The server rejected the item stack request actions when move item back to inventory")
			}

			err = api.ContainerOpenAndClose().CloseContainer()
			if err != nil {
				return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
			}

			console.UseInventorySlot(nbt_console.RequesterUser, targetSlot, false)
			console.UseInventorySlot(nbt_console.RequesterUser, slot, true)
		}
	}

	if len(defaultItem.Enhance.DisplayName) > 0 {
		index, err := console.FindOrGenerateNewAnvil()
		if err != nil {
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}

		success, err := console.OpenContainerByIndex(index)
		if err != nil {
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}
		if !success {
			return fmt.Errorf("EnchRenameOrDyeSingle: Failed to open the anvil")
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			RenameInventoryItem(slot, defaultItem.Enhance.DisplayName).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return fmt.Errorf("EnchRenameOrDyeSingle: The server rejected the renaming operation")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return fmt.Errorf("EnchRenameOrDyeSingle: %v", err)
		}
	}

	return nil
}

// EnchRenameOrDyeMultiple 根据操作台 console 和已放入背包的多个物品 multipleItems，
// 将它们进行集中性的物品附魔、物品改名和物品染色处理。应当说明的是，这些物品应当置于非快捷栏的物品栏，
// 并且对于无需处理的物品，应当简单的置为 nil
func EnchRenameOrDyeMultiple(
	console *nbt_console.Console,
	multipleItems [27]*nbt_parser_interface.Item,
) error {
	err := LoreMultiple(console, multipleItems)
	if err != nil {
		return fmt.Errorf("EnchRenameOrDyeMultiple: %v", err)
	}
	err = DyeMultiple(console, multipleItems)
	if err != nil {
		return fmt.Errorf("EnchRenameOrDyeMultiple: %v", err)
	}
	err = EnchMultiple(console, multipleItems)
	if err != nil {
		return fmt.Errorf("EnchRenameOrDyeMultiple: %v", err)
	}
	err = RenameMultiple(console, multipleItems)
	if err != nil {
		return fmt.Errorf("EnchRenameOrDyeMultiple: %v", err)
	}
	return nil
}
