package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 花盆
type FlowerPot struct {
	console *nbt_console.Console
	data    nbt_parser_block.FlowerPot
}

func (FlowerPot) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (f *FlowerPot) Make() error {
	api := f.console.API()

	// 生成花盆中的花
	err := f.console.API().SetBlock().SetBlock(f.console.Center(), f.data.NBT.PlantBlock.Name, utils.MarshalBlockStates(f.data.NBT.PlantBlock.States))
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	f.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        f.data.NBT.PlantBlock.Name,
		States:      f.data.NBT.PlantBlock.States,
	})

	// 获取花盆中的花
	_, err = f.console.API().Commands().SendWSCommandWithResp("clear")
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	f.console.CleanInventory()

	success, currentSlot, err := api.BotClick().PickBlock(f.console.Center(), true)
	if err != nil || !success {
		_ = f.console.ChangeAndUpdateHotbarSlotID(nbt_console.DefaultHotbarSlot)
	}
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	if !success {
		return fmt.Errorf("Make: Failed to pick block due to unknown reason")
	}
	f.console.UpdateHotbarSlotID(currentSlot)
	f.console.UseInventorySlot(nbt_console.RequesterUser, currentSlot, true)

	// 生成花盆
	err = api.SetBlock().SetBlock(f.console.Center(), f.data.BlockName(), f.data.BlockStatesString())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	f.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        f.data.BlockName(),
		States:      f.data.BlockStates(),
	})

	// 点击花盆
	err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: f.console.HotbarSlotID(),
		BotPos:       f.console.Position(),
		BlockPos:     f.console.Center(),
		BlockName:    f.data.BlockName(),
		BlockStates:  f.data.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
