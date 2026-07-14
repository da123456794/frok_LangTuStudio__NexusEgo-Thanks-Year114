package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 头颅
type Skull struct {
	console *nbt_console.Console
	data    nbt_parser_block.Skull
}

func (Skull) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (s *Skull) Make() error {
	api := s.console.API()

	// 前置准备
	helperSkullBlock := s.data.BlockName()
	if helperSkullBlock == "minecraft:skull" {
		switch s.data.NBT.SkullType {
		case 0:
			helperSkullBlock = "minecraft:skeleton_skull"
		case 1:
			helperSkullBlock = "minecraft:wither_skeleton_skull"
		case 2:
			helperSkullBlock = "minecraft:zombie_head"
		case 3:
			helperSkullBlock = "minecraft:player_head"
		case 4:
			helperSkullBlock = "minecraft:creeper_head"
		case 5:
			helperSkullBlock = "minecraft:dragon_head"
		case 6:
			helperSkullBlock = "minecraft:piglin_head"
		}
	}

	// 清空操作台中心处的方块
	err := s.console.API().SetBlock().SetBlock(s.console.Center(), "minecraft:air", "[]")
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	s.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.Air{})

	// 取得生成头颅所需要的头颅物品
	err = s.console.API().Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathHotbarOnly,
		game_interface.ReplaceitemInfo{
			Name:  helperSkullBlock,
			Count: 1,
			Slot:  s.console.HotbarSlotID(),
		},
		"",
		true,
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 放置头颅前视角准备
	inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	inputData.Set(packet.InputFlagStartFlying)
	err = api.Resources().WritePacket(&packet.PlayerAuthInput{
		Yaw:       s.data.NBT.Rotation,
		InputData: inputData,
		Position:  s.console.Position(),
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 放置头颅
	_, offsetPos, err := api.BotClick().PlaceBlockHighLevel(
		s.console.Center(),
		s.console.Position(),
		s.console.HotbarSlotID(),
		1,
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	s.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: false,
		Name:        helperSkullBlock,
	})
	*s.console.NearBlockByIndex(nbt_console.ConsoleIndexCenterBlock, offsetPos) = block_helper.NearBlock{
		Name: game_interface.BasePlaceBlock,
	}

	return nil
}
