package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	nbt_assigner_interface "github.com/LangTuStudio/RaaBel/nbt_assigner/interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_assigner_utils "github.com/LangTuStudio/RaaBel/nbt_assigner/utils"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
	nbt_hash "github.com/LangTuStudio/RaaBel/nbt_parser/hash"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 营火
type Campfire struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.Campfire
}

func (Campfire) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

// processComplex 处理复杂的物品
func (c *Campfire) processComplex(item nbt_parser_interface.Item) (canUseCommand bool, resultSlot resources_control.SlotID, err error) {
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
			panic("processComplex: Should never happened")
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

	// 复杂NBT物品制作
	methods := nbt_assigner_interface.MakeNBTItemMethod(c.console, c.cache, item)
	if len(methods) != 1 {
		panic("Make: Should never happened")
	}
	resultSlotMapping, err := methods[0].Make()
	if err != nil {
		return false, 0, fmt.Errorf("processComplex: %v", err)
	}
	if len(resultSlotMapping) != 1 {
		panic("Make: Should never happened")
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

// processCampfireItem 处理单个营火物品
func (c *Campfire) processCampfireItem(
	blockStates map[string]any,
	item nbt_parser_interface.Item,
) (err error) {
	api := c.console.API()
	if item == nil {
		return nil
	}

	var canUseCommand bool
	var resultSlot resources_control.SlotID

	// 如果这是一个复杂的物品
	if item.IsComplex() {
		canUseCommand, resultSlot, err = c.processComplex(item)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	} else {
		canUseCommand = true
	}

	// canUseCommand 指示可以先使用命令获取目标物品
	if canUseCommand {
		underlying := item.UnderlyingItem()
		defaultItem := underlying.(*nbt_parser_item.DefaultItem)

		err = api.Replaceitem().ReplaceitemInInventory(
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
			return fmt.Errorf("processCampfireItem: %v", err)
		}
		c.console.UseInventorySlot(nbt_console.RequesterUser, c.console.HotbarSlotID(), true)
		resultSlot = c.console.HotbarSlotID()
	}

	// 切换物品栏
	if resultSlot != c.console.HotbarSlotID() {
		err = c.console.ChangeAndUpdateHotbarSlotID(resultSlot)
		if err != nil {
			return fmt.Errorf("processCampfireItem: %v", err)
		}
	}

	if item.NeedEnchRenameDyeOrLore() {
		err = nbt_assigner_interface.EnchRenameDyeOrLoreSingle(c.console, item, resultSlot)
		if err != nil {
			return fmt.Errorf("processCampfireItem: %v", err)
		}
	}

	// 移动到可交互位置
	err = c.console.CanReachOrMove(c.console.Center())
	if err != nil {
		return fmt.Errorf("processCampfireItem: %v", err)
	}

	// 放入营火
	err = c.console.API().BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: c.console.HotbarSlotID(),
		BotPos:       c.console.Position(),
		BlockPos:     c.console.Center(),
		BlockName:    c.data.BlockName(),
		BlockStates:  blockStates,
	})
	if err != nil {
		return fmt.Errorf("processCampfireItem: %v", err)
	}

	return nil
}

func (c *Campfire) Make() error {
	api := c.console.API()
	blockStates := utils.DeepCopyNBT(c.data.BlockStates())
	targetExtinguishedByte, _ := blockStates["extinguished"].(byte)
	targetExtinguished := targetExtinguishedByte == 1

	// 需要先点燃再放食物
	if c.data.NBT.HaveAnyItem && targetExtinguished {
		blockStates["extinguished"] = byte(0)
	}

	// 生成营火
	err := nbt_assigner_utils.SpawnNewEmptyBlock(
		c.console,
		c.cache,
		nbt_assigner_utils.EmptyBlockData{
			Name:               c.data.BlockName(),
			States:             blockStates,
			IsCanOpenConatiner: false,
			BlockCustomName:    c.data.NBT.CustomName,
		},
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 如果营火上没有物品，则应当直接返回值
	if !c.data.NBT.HaveAnyItem {
		return nil
	}

	// 移动到位置
	err = c.console.CanReachOrMove(c.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 逐个放入物品
	for _, campfireItem := range c.data.NBT.Items {
		err = c.processCampfireItem(blockStates, campfireItem)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		// 同步修改方块状态
		c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
			KnownStates: true,
			Name:        c.data.BlockName(),
			States:      blockStates,
		})
	}

	// 熄灭营火
	if targetExtinguished {
		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:  "minecraft:iron_shovel",
				Count: 1,
				Slot:  c.console.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		c.console.UseInventorySlot(nbt_console.RequesterUser, c.console.HotbarSlotID(), true)

		err = c.console.API().BotClick().ClickBlock(game_interface.UseItemOnBlocks{
			HotbarSlotID: c.console.HotbarSlotID(),
			BotPos:       c.console.Position(),
			BlockPos:     c.console.Center(),
			BlockName:    c.data.BlockName(),
			BlockStates:  blockStates,
		})
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		blockStates["extinguished"] = byte(1)
		c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
			KnownStates: true,
			Name:        c.data.BlockName(),
			States:      blockStates,
		})
	}

	return nil
}
