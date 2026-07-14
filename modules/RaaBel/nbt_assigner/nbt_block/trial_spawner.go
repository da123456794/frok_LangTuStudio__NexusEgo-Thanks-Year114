package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 试炼刷怪笼
type TrialSpawner struct {
	console *nbt_console.Console
	data    nbt_parser_block.TrialSpawner
}

func (TrialSpawner) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (t *TrialSpawner) Make() error {
	api := t.console.API()

	// 放置试炼刷怪笼
	err := api.SetBlock().SetBlock(t.console.Center(), t.data.BlockName(), t.data.BlockStatesString())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	t.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        t.data.BlockName(),
		States:      t.data.BlockStates(),
	})

	// 获取试炼刷怪笼中的生物蛋
	err = api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathHotbarOnly,
		game_interface.ReplaceitemInfo{
			Name:  fmt.Sprintf("%s_spawn_egg", t.data.NBT.SpawnData.TypeID),
			Count: 1,
			Slot:  t.console.HotbarSlotID(),
		},
		"",
		true,
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	t.console.UseInventorySlot(nbt_console.RequesterUser, t.console.HotbarSlotID(), true)

	// 前往操作台中心处
	err = t.console.CanReachOrMove(t.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 点击试炼刷怪笼放置生物
	err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: t.console.HotbarSlotID(),
		BotPos:       t.console.Position(),
		BlockPos:     t.console.Center(),
		BlockName:    t.data.BlockName(),
		BlockStates:  t.data.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
