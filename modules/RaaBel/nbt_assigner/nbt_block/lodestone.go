package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 磁石
type Lodestone struct {
	console *nbt_console.Console
	data    nbt_parser_block.Lodestone
}

func (Lodestone) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (l *Lodestone) Make() error {
	api := l.console.API()

	// 放置磁石
	err := api.SetBlock().SetBlock(l.console.Center(), l.data.BlockName(), l.data.BlockStatesString())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	l.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        l.data.BlockName(),
		States:      l.data.BlockStates(),
	})

	// 获取指南针
	err = api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathHotbarOnly,
		game_interface.ReplaceitemInfo{
			Name:  "compass",
			Count: 1,
			Slot:  l.console.HotbarSlotID(),
		},
		"",
		true,
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	l.console.UseInventorySlot(nbt_console.RequesterUser, l.console.HotbarSlotID(), true)

	// 前往操作台中心处
	err = l.console.CanReachOrMove(l.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 点击磁石
	err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: l.console.HotbarSlotID(),
		BotPos:       l.console.Position(),
		BlockPos:     l.console.Center(),
		BlockName:    l.data.BlockName(),
		BlockStates:  l.data.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
