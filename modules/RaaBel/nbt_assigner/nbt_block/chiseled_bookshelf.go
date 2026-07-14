package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	nbt_assigner_interface "github.com/LangTuStudio/RaaBel/nbt_assigner/interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
	nbt_hash "github.com/LangTuStudio/RaaBel/nbt_parser/hash"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 雕纹书架
type ChiseledBookshelf struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.ChiseledBookshelf
}

func (ChiseledBookshelf) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

// clickChiseledBookshelfSlot 点击雕纹书架的某个槽位
func (c *ChiseledBookshelf) clickChiseledBookshelfSlot(request game_interface.UseItemOnBlocks, slot resources_control.SlotID) (err error) {
	direction, _ := request.BlockStates["direction"].(int32)
	blockFace, ok := mapping.ChiseledBookshelfDirectionToBlockFace[direction]
	if !ok {
		return fmt.Errorf("clickChiseledBookshelfSlot: Should never happened")
	}
	clickedPosition, ok := mapping.ChiseledBookshelfDirectionAndBookSlotIDToClickedPosition[direction][uint8(slot)]
	if !ok {
		return fmt.Errorf("clickChiseledBookshelfSlot: Should never happened")
	}

	request.ClickedPosition = clickedPosition
	err = c.console.API().BotClick().PlaceBlock(request, blockFace)
	if err != nil {
		return fmt.Errorf("clickChiseledBookshelfSlot: %v", err)
	}

	return nil
}

// processComplex 处理复杂的物品
func (c *ChiseledBookshelf) processComplex(item nbt_parser_interface.Item) (canUseCommand bool, resultSlot resources_control.SlotID, err error) {
	api := c.console.API()
	underlying := item.UnderlyingItem()
	defaultItem := underlying.(*nbt_parser_item.DefaultItem)

	// 子方块
	if defaultItem.Block.SubBlock != nil {
		if !defaultItem.Block.SubBlock.NeedSpecialHandle() {
			return true, 0, nil
		}
		_, _, _, err = nbt_assigner_interface.PlaceNBTBlock(c.console, c.cache, defaultItem.Block.SubBlock)
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}

		_, hit, partHit, err := c.cache.NBTBlockCache().LoadCache(nbt_hash.CompletelyHashNumber{
			HashNumber:    nbt_hash.NBTBlockFullHash(defaultItem.Block.SubBlock),
			SetHashNumber: nbt_hash.ContainerSetHash(defaultItem.Block.SubBlock),
		})
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !hit || partHit {
			panic("processComplex: Should nerver happened")
		}

		_, err = c.console.API().Commands().SendWSCommandWithResp("clear")
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		c.console.CleanInventory()

		success, currentSlot, err := api.BotClick().PickBlock(c.console.Center(), true)
		if err != nil || !success {
			_ = c.console.ChangeAndUpdateHotbarSlotID(nbt_console.DefaultHotbarSlot)
		}
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !success {
			return false, 0, fmt.Errorf("processComplex: Failed to pick block due to unknown reason")
		}
		c.console.UpdateHotbarSlotID(currentSlot)
		c.console.UseInventorySlot(nbt_console.RequesterUser, currentSlot, true)

		return false, currentSlot, nil
	}

	// 复杂 NBT 物品制作
	methods := nbt_assigner_interface.MakeNBTItemMethod(c.console, c.cache, item)
	if len(methods) != 1 {
		panic("Make: Should nerver happened")
	}
	resultSlotMapping, err := methods[0].Make()
	if err != nil {
		return false, 0, fmt.Errorf("processComplex: %v", err)
	}
	if len(resultSlotMapping) != 1 {
		panic("Make: Should nerver happened")
	}

	// 将复杂 NBT 物品移动到快捷栏
	for _, slotID := range resultSlotMapping {
		resultSlot = slotID
	}
	if resultSlot > 8 {
		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     "minecraft:air",
				Count:    1,
				MetaData: 0,
				Slot:     c.console.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		c.console.UseInventorySlot(nbt_console.RequesterUser, c.console.HotbarSlotID(), false)

		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !success {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			MoveBetweenInventory(resultSlot, c.console.HotbarSlotID(), 1).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return false, 0, fmt.Errorf("processComplex: The server rejected the stack request action")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}

		resultSlot = c.console.HotbarSlotID()
	}

	return false, resultSlot, nil
}

// processEmptySlot 处理雕纹书架的一个空槽位
func (c *ChiseledBookshelf) processEmptySlot(
	blockStates map[string]any,
	slot resources_control.SlotID,
) (err error) {
	api := c.console.API()

	// 获取书
	err = c.console.API().Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathHotbarOnly,
		game_interface.ReplaceitemInfo{
			Name:  "minecraft:book",
			Count: 1,
			Slot:  c.console.HotbarSlotID(),
		},
		"",
		true,
	)
	if err != nil {
		return fmt.Errorf("processEmptySlot: %v", err)
	}

	c.console.UseInventorySlot(nbt_console.RequesterUser, c.console.HotbarSlotID(), true)

	// 点击雕纹书架
	err = c.clickChiseledBookshelfSlot(game_interface.UseItemOnBlocks{
		HotbarSlotID: c.console.HotbarSlotID(),
		BotPos:       c.console.Position(),
		BlockPos:     c.console.Center(),
		BlockName:    c.data.BlockName(),
		BlockStates:  blockStates,
	}, slot)
	if err != nil {
		return fmt.Errorf("processEmptySlot: %v", err)
	}
	/*
		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("processEmptySlot: %v", err)
		}

		_, err = c.console.API().Commands().SendWSCommandWithResp("clear")
		if err != nil {
			return fmt.Errorf("processEmptySlot: %v", err)
		}
		c.console.CleanInventory()

		// 置空
		err = c.console.API().Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     "minecraft:air",
				Count:    1,
				Slot:     c.console.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return fmt.Errorf("processEmptySlot: %v", err)
		}

		c.console.UseInventorySlot(nbt_console.RequesterUser, c.console.HotbarSlotID(), true)
	*/
	// 点击雕纹书架
	err = c.clickChiseledBookshelfSlot(game_interface.UseItemOnBlocks{
		HotbarSlotID: c.console.HotbarSlotID(),
		BotPos:       c.console.Position(),
		BlockPos:     c.console.Center(),
		BlockName:    c.data.BlockName(),
		BlockStates:  blockStates,
	}, slot)
	if err != nil {
		return fmt.Errorf("processEmptySlot: %v", err)
	}

	err = api.SetBlock().SetBlock(c.console.Center(), c.data.BlockName(), utils.MarshalBlockStates(c.data.BlockStates()))
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        c.data.BlockName(),
		States:      c.data.BlockStates(),
	})
	/*
		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("processEmptySlot: %v", err)
		}*/
	/*
		err = api.Replaceitem().ReplaceitemInContainerAsync(
			c.console.Center(),
			game_interface.ReplaceitemInfo{
				Name:     "minecraft:air",
				Count:    1,
				Slot:     slot,
			},
			"",
		)
		if err != nil {
			return fmt.Errorf("processEmptySlot: %v", err)
		}

		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("processEmptySlot: %v", err)
		}
	*/
	return nil
}

// processComplex 处理雕纹书架的一个槽位
func (c *ChiseledBookshelf) processSlot(
	blockStates map[string]any,
	slot resources_control.SlotID,
) (newBlockStates map[string]any, err error) {
	api := c.console.API()
	booksStored, _ := blockStates["books_stored"].(int32)
	item := c.data.NBT.Items[slot]
	if item == nil {
		return blockStates, nil
	}
	underlying := item.UnderlyingItem()
	defaultItem := underlying.(*nbt_parser_item.DefaultItem)

	// 如果书可以直接使用命令放置
	if !item.IsComplex() && !item.NeedEnchRenameDyeOrLore() {
		err = api.Replaceitem().ReplaceitemInContainerAsync(
			c.console.Center(),
			game_interface.ReplaceitemInfo{
				Name:     item.ItemName(),
				Count:    item.ItemCount(),
				MetaData: item.ItemMetadata(),
				Slot:     slot,
			},
			utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
		)
		if err != nil {
			return blockStates, fmt.Errorf("processSlot: %v", err)
		}

		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return blockStates, fmt.Errorf("processSlot: %v", err)
		}

		booksStored |= 1 << slot
		blockStates["books_stored"] = booksStored
		return blockStates, nil
	}

	var canUseCommand bool
	var resultSlot resources_control.SlotID

	// 如果这是一个复杂的物品
	if item.IsComplex() {
		canUseCommand, resultSlot, err = c.processComplex(item)
		if err != nil {
			return blockStates, fmt.Errorf("processSlot: %v", err)
		}
	} else {
		canUseCommand = true
	}

	// canUseCommand 指示可以先使用命令获取目标物品
	if canUseCommand {
		underlying := item.UnderlyingItem()
		defaultItem := underlying.(*nbt_parser_item.DefaultItem)

		err = c.console.API().Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     item.ItemName(),
				Count:    1,
				MetaData: item.ItemMetadata(),
				Slot:     c.console.HotbarSlotID(),
			},
			utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
			true,
		)
		if err != nil {
			return blockStates, fmt.Errorf("processSlot: %v", err)
		}

		c.console.UseInventorySlot(nbt_console.RequesterUser, c.console.HotbarSlotID(), true)
		resultSlot = c.console.HotbarSlotID()
	}

	// 切换物品栏，如果需要的话
	if resultSlot != c.console.HotbarSlotID() {
		err = c.console.ChangeAndUpdateHotbarSlotID(resultSlot)
		if err != nil {
			return blockStates, fmt.Errorf("processSlot: %v", err)
		}
	}

	// 如果这个物品需要重命名、附魔、染色或 Lore 处理
	if item.NeedEnchRenameDyeOrLore() {
		err = nbt_assigner_interface.EnchRenameDyeOrLoreSingle(c.console, item, resultSlot)
		if err != nil {
			return blockStates, fmt.Errorf("processSlot: %v", err)
		}
	}
	// 前往操作台中心处
	err = c.console.CanReachOrMove(c.console.Center())
	if err != nil {
		return blockStates, fmt.Errorf("processSlot: %v", err)
	}

	// 点击雕纹书架
	err = c.clickChiseledBookshelfSlot(game_interface.UseItemOnBlocks{
		HotbarSlotID: c.console.HotbarSlotID(),
		BotPos:       c.console.Position(),
		BlockPos:     c.console.Center(),
		BlockName:    c.data.BlockName(),
		BlockStates:  blockStates,
	}, slot)
	if err != nil {
		return blockStates, fmt.Errorf("processSlot: %v", err)
	}

	booksStored |= 1 << slot
	blockStates["books_stored"] = booksStored
	return blockStates, nil
}

func (c *ChiseledBookshelf) Make() error {
	api := c.console.API()

	// 放置一个空雕纹书架
	states := utils.DeepCopyNBT(c.data.BlockStates())
	states["books_stored"] = int32(0)

	err := api.SetBlock().SetBlock(c.console.Center(), c.data.BlockName(), utils.MarshalBlockStates(states))
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        c.data.BlockName(),
		States:      states,
	})

	// 如果这个雕纹书架有任何物品
	if c.data.NBT.HaveAnyItem {
		// 进行物品处理
		for slot := range resources_control.SlotID(6) {
			if slot == c.data.NBT.LastInteractedSlot {
				continue
			}
			states, err = c.processSlot(states, slot)
			if err != nil {
				return fmt.Errorf("Make: %v", err)
			}
			c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
				KnownStates: true,
				Name:        c.data.BlockName(),
				States:      states,
			})
		}

		// 处理最后一个交互的槽位
		states, err = c.processSlot(states, c.data.NBT.LastInteractedSlot)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
			KnownStates: true,
			Name:        c.data.BlockName(),
			States:      states,
		})
	} else {
		// 如果雕纹书架中没有任何物品
		// 且最后一次交互的槽位不为空
		// 则设置该槽位为书后设置空气
		err = c.processEmptySlot(states, c.data.NBT.LastInteractedSlot)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	return nil
}
