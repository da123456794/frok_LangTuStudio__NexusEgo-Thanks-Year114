package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 炼药锅
type Cauldron struct {
	console *nbt_console.Console
	data    nbt_parser_block.Cauldron
}

func (Cauldron) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (c *Cauldron) Make() error {
	api := c.console.API()

	// 清空操作台中心处的方块
	err := c.console.API().SetBlock().SetBlock(c.console.Center(), "minecraft:air", "[]")
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.Air{})

	// 对炼药锅进行染色或倒入药水操作
	if c.data.NBT.CustomColor != 0 {
		// 生成有水的炼药锅
		err = api.SetBlock().SetBlock(c.console.Center(), c.data.BlockName(), c.data.BlockStatesString())
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
			KnownStates: true,
			Name:        c.data.BlockName(),
			States:      c.data.BlockStates(),
		})

		// 获取染色所需的颜料配方
		color, _ := utils.DecodeVarRGBA(c.data.NBT.CustomColor)
		dyeIDs, found := utils.SearchCauldronDyeIDsByColor(color)
		if !found {
			panic("Make: Should nerver happened")
		}

		// 遍历染料进行染色
		for _, dyeID := range dyeIDs {
			// 取得生成染色炼药锅所需要的染料
			dyeItemName := mapping.RGBToDyeItemName[mapping.DefaultDyeColor[dyeID]]
			err = c.console.API().Replaceitem().ReplaceitemInInventory(
				"@s",
				game_interface.ReplacePathHotbarOnly,
				game_interface.ReplaceitemInfo{
					Name:  dyeItemName,
					Count: 1,
					Slot:  c.console.HotbarSlotID(),
				},
				"",
				true,
			)
			if err != nil {
				return fmt.Errorf("Make: %v", err)
			}

			// 点击炼药锅
			err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
				HotbarSlotID: c.console.HotbarSlotID(),
				BotPos:       c.console.Position(),
				BlockPos:     c.console.Center(),
				BlockName:    c.data.BlockName(),
				BlockStates:  c.data.BlockStates(),
			})
			if err != nil {
				return fmt.Errorf("Make: %v", err)
			}
		}
	} else if c.data.NBT.PotionID != -1 && c.data.NBT.PotionType != -1 {
		// 生成空的炼药锅
		err = api.SetBlock().SetBlock(c.console.Center(), c.data.BlockName(), "")
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		c.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
			KnownStates: false,
			Name:        c.data.BlockName(),
		})

		// 获取炼药锅中的药水
		err = c.console.API().Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     mapping.PotionTypeToItemName[c.data.NBT.PotionType],
				Count:    1,
				MetaData: c.data.NBT.PotionID,
				Slot:     c.console.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		// 倒入药水
		time, ok := c.data.States["fill_level"].(int32)
		if !ok {
			time = 3
		} else {
			time = time / 2
		}

		blockStates := utils.DeepCopyNBT(c.data.BlockStates())
		for i := range time {
			blockStates["fill_level"] = int32(i * 2)
			// 点击炼药锅
			err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
				HotbarSlotID: c.console.HotbarSlotID(),
				BotPos:       c.console.Position(),
				BlockPos:     c.console.Center(),
				BlockName:    c.data.BlockName(),
				BlockStates:  blockStates,
			})
			if err != nil {
				return fmt.Errorf("Make: %v", err)
			}
		}
	}

	return nil
}
