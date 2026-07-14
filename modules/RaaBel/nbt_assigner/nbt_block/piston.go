package nbt_block

import (
	"fmt"
	"time"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 活塞
type Piston struct {
	console *nbt_console.Console
	data    nbt_parser_block.Piston
}

func (Piston) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (p *Piston) Make() error {
	api := p.console.API()

	// 放置活塞
	err := api.SetBlock().SetBlock(p.console.Center(), p.data.BlockName(), p.data.BlockStatesString())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	p.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        p.data.BlockName(),
		States:      p.data.BlockStates(),
	})

	// 仅支持活塞状态 2 的特殊制作
	if p.data.NBT.State != 2 {
		return nil
	}

	redstoneOffset := protocol.BlockPos{0, -1, 0}
	direction, _ := p.data.BlockStates()["facing_direction"].(int32)
	if direction == 0 {
		redstoneOffset = protocol.BlockPos{0, 1, 0}
	}
	redstonePos := p.console.NearBlockPosByIndex(nbt_console.ConsoleIndexCenterBlock, redstoneOffset)

	// 放置红石方块激活活塞
	err = api.SetBlock().SetBlock(redstonePos, "minecraft:redstone_block", "[]")
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	*p.console.NearBlockByIndex(nbt_console.ConsoleIndexCenterBlock, redstoneOffset) = block_helper.NearBlock{
		Name: "minecraft:redstone_block",
	}

	// 等待活塞伸出完成
	time.Sleep(500 * time.Millisecond)
	return nil
}
