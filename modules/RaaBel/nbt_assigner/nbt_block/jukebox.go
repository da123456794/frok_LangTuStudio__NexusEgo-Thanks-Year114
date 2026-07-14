package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	nbt_assigner_interface "github.com/LangTuStudio/RaaBel/nbt_assigner/interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_assigner_utils "github.com/LangTuStudio/RaaBel/nbt_assigner/utils"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 唱片机
type JukeBox struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.JukeBox
}

func (JukeBox) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (j *JukeBox) Make() error {
	var defaultItem *nbt_parser_item.DefaultItem
	api := j.console.API()

	// 生成唱片机
	err := nbt_assigner_utils.SpawnNewEmptyBlock(
		j.console,
		j.cache,
		nbt_assigner_utils.EmptyBlockData{
			Name:               j.data.BlockName(),
			States:             j.data.BlockStates(),
			IsCanOpenConatiner: false,
			BlockCustomName:    j.data.NBT.CustomName,
		},
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 如果唱片机中没有唱片，则应当直接返回值
	if j.data.NBT.HaveDisc {
		underlying := j.data.NBT.Disc.UnderlyingItem()
		defaultItem = underlying.(*nbt_parser_item.DefaultItem)
	} else {
		return nil
	}

	// 如果唱片可以直接使用命令放置
	if !j.data.NBT.Disc.NeedEnchRenameDyeOrLore() {
		err = api.Replaceitem().ReplaceitemInContainerAsync(
			j.console.Center(),
			game_interface.ReplaceitemInfo{
				Name:     j.data.NBT.Disc.ItemName(),
				Count:    j.data.NBT.Disc.ItemCount(),
				MetaData: j.data.NBT.Disc.ItemMetadata(),
				Slot:     0,
			},
			utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		return nil
	}

	// 获取唱片到物品栏
	err = api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathHotbarOnly,
		game_interface.ReplaceitemInfo{
			Name:     j.data.NBT.Disc.ItemName(),
			Count:    j.data.NBT.Disc.ItemCount(),
			MetaData: j.data.NBT.Disc.ItemMetadata(),
			Slot:     j.console.HotbarSlotID(),
		},
		utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
		true,
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	j.console.UseInventorySlot(nbt_console.RequesterUser, j.console.HotbarSlotID(), true)

	err = nbt_assigner_interface.EnchRenameDyeOrLoreSingle(j.console, j.data.NBT.Disc, j.console.HotbarSlotID())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 传送到操作台中心
	err = j.console.CanReachOrMove(j.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 放置唱片
	err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: j.console.HotbarSlotID(),
		BotPos:       j.console.Position(),
		BlockPos:     j.console.Center(),
		BlockName:    j.data.BlockName(),
		BlockStates:  j.data.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
