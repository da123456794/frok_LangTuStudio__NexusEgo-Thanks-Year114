package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 音符盒
type NoteBlock struct {
	console *nbt_console.Console
	data    nbt_parser_block.NoteBlock
}

func (NoteBlock) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (n *NoteBlock) Make() error {
	api := n.console.API()

	// 清空操作台中心处的方块
	err := n.console.API().SetBlock().SetBlock(n.console.Center(), "minecraft:air", "[]")
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	n.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.Air{})

	// 放置音符盒
	err = api.SetBlock().SetBlock(n.console.Center(), n.data.BlockName(), n.data.BlockStatesString())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	n.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        n.data.BlockName(),
		States:      n.data.BlockStates(),
	})

	// 默认音高为 0
	if n.data.NBT.Note == 0 {
		return nil
	}

	// 前往操作台中心处
	err = n.console.CanReachOrMove(n.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 每次点击音符盒会使音高加一
	request := game_interface.UseItemOnBlocks{
		HotbarSlotID: n.console.HotbarSlotID(),
		BotPos:       n.console.Position(),
		BlockPos:     n.console.Center(),
		BlockName:    n.data.BlockName(),
		BlockStates:  n.data.BlockStates(),
	}
	for range n.data.NBT.Note {
		err = api.BotClick().ClickBlock(request)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	return nil
}
