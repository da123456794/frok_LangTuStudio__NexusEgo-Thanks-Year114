package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 刷怪笼
type MobSpawner struct {
	console *nbt_console.Console
	data    nbt_parser_block.MobSpawner
}

func (MobSpawner) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (m *MobSpawner) Make() error {
	api := m.console.API()

	// 放置刷怪笼
	err := api.SetBlock().SetBlock(m.console.Center(), m.data.BlockName(), m.data.BlockStatesString())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	m.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        m.data.BlockName(),
		States:      m.data.BlockStates(),
	})

	// 获取刷怪笼中的生物蛋
	err = api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathHotbarOnly,
		game_interface.ReplaceitemInfo{
			Name:  fmt.Sprintf("%s_spawn_egg", m.data.NBT.EntityIdentifier),
			Count: 1,
			Slot:  m.console.HotbarSlotID(),
		},
		"",
		true,
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	m.console.UseInventorySlot(nbt_console.RequesterUser, m.console.HotbarSlotID(), true)

	// 前往操作台中心处
	err = m.console.CanReachOrMove(m.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 点击刷怪笼放置生物
	err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: m.console.HotbarSlotID(),
		BotPos:       m.console.Position(),
		BlockPos:     m.console.Center(),
		BlockName:    m.data.BlockName(),
		BlockStates:  m.data.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
