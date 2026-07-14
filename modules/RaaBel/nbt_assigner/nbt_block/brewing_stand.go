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
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 酿造台
type BrewingStand struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.BrewingStand
}

func (BrewingStand) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (b *BrewingStand) Make() error {
	api := b.console.API()
	usedSyncReplaceitemCommand := false
	existItemNeedRename := false

	brewingStandStates := map[string]any{
		"brewing_stand_slot_a_bit": byte(0),
		"brewing_stand_slot_b_bit": byte(0),
		"brewing_stand_slot_c_bit": byte(0),
	}
	updateBlockStates := func() {
		b.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ContainerBlockHelper{
			OpenInfo: block_helper.ContainerBlockOpenInfo{
				Name:                  b.data.BlockName(),
				States:                brewingStandStates,
				ConsiderOpenDirection: false,
			},
		})
	}

	// 生成酿造台方块
	err := nbt_assigner_utils.SpawnNewEmptyBlock(
		b.console,
		b.cache,
		nbt_assigner_utils.EmptyBlockData{
			Name: b.data.BlockName(),
			States: map[string]any{
				"brewing_stand_slot_a_bit": byte(0),
				"brewing_stand_slot_b_bit": byte(0),
				"brewing_stand_slot_c_bit": byte(0),
			},
			IsCanOpenConatiner:    true,
			ConsiderOpenDirection: false,
			ShulkerFacing:         0,
			BlockCustomName:       b.data.NBT.CustomName,
		},
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 针对酿造台燃料槽位的特殊处理
	for _, item := range b.data.NBT.Items {
		if item.Slot != 4 {
			continue
		}

		err = api.Replaceitem().ReplaceitemInContainerAsync(
			b.console.Center(),
			game_interface.ReplaceitemInfo{
				Name:     "minecraft:blaze_powder",
				Count:    1,
				MetaData: 0,
				Slot:     4,
			},
			"",
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		break
	}

	// 处理可以直接 Replaceitem 处理的物品
	for _, item := range b.data.NBT.Items {
		underlaying := item.Item.UnderlyingItem()
		defaultItem := underlaying.(*nbt_parser_item.DefaultItem)

		if item.Item.NeedEnchRenameDyeOrLore() {
			existItemNeedRename = true
			continue
		}

		usedSyncReplaceitemCommand = true
		switch item.Slot {
		case 1:
			brewingStandStates["brewing_stand_slot_a_bit"] = byte(1)
		case 2:
			brewingStandStates["brewing_stand_slot_b_bit"] = byte(1)
		case 3:
			brewingStandStates["brewing_stand_slot_c_bit"] = byte(1)
		}

		err = b.console.API().Replaceitem().ReplaceitemInContainerAsync(
			b.console.Center(),
			game_interface.ReplaceitemInfo{
				Name:     item.Item.ItemName(),
				Count:    item.Item.ItemCount(),
				MetaData: item.Item.ItemMetadata(),
				Slot:     resources_control.SlotID(item.Slot),
			},
			utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		updateBlockStates()
	}

	// 如果使用了 Replaceitem 命令，
	// 则需要等待更改
	if usedSyncReplaceitemCommand {
		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	// 如果没有物品存在自定义物品名称，
	// 则可以直接返回值
	if !existItemNeedRename {
		return nil
	}

	// 先将需要特殊处理的物品放入快捷栏并执行增强处理
	for _, item := range b.data.NBT.Items {
		underlaying := item.Item.UnderlyingItem()
		defaultItem := underlaying.(*nbt_parser_item.DefaultItem)

		if !item.Item.NeedEnchRenameDyeOrLore() {
			continue
		}

		err = b.console.API().Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     item.Item.ItemName(),
				Count:    item.Item.ItemCount(),
				MetaData: item.Item.ItemMetadata(),
				Slot:     resources_control.SlotID(item.Slot),
			},
			utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
			true,
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		b.console.UseInventorySlot(nbt_console.RequesterUser, resources_control.SlotID(item.Slot), true)

		err = nbt_assigner_interface.EnchRenameDyeOrLoreSingle(b.console, item.Item, resources_control.SlotID(item.Slot))
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	// 打开酿造台
	success, err := b.console.OpenContainerByIndex(nbt_console.ConsoleIndexCenterBlock)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	if !success {
		return fmt.Errorf("Make: Failed to open the brewing stand")
	}
	defer api.ContainerOpenAndClose().CloseContainer()

	// 移动已增强物品到酿造台
	transaction := api.ItemStackOperation().OpenTransaction()
	for _, item := range b.data.NBT.Items {
		if !item.Item.NeedEnchRenameDyeOrLore() {
			continue
		}
		_ = transaction.MoveToContainer(
			resources_control.SlotID(item.Slot),
			resources_control.SlotID(item.Slot),
			item.Item.ItemCount(),
		)
	}

	// 提交更改
	success, _, _, err = transaction.Commit()
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	if !success {
		return fmt.Errorf("Make: The server rejected the stack request action")
	}

	// 更新方块状态
	brewingStandStates = b.data.BlockStates()
	updateBlockStates()

	// 返回值
	return nil
}
