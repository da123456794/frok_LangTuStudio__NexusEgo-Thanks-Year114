package nbt_console

import (
	"fmt"
	"math/rand"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/utils"
)

// FindAir 从操作台的帮助方块中寻找一个空气方块。
// includeCenter 指示要查找的方块是否也包括操作台
// 中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// 如果返回的 block 不为空，则说明找到，
// 否则没有找到。找到的方块可以通过修改
// 其指向的值从而将它变成其他方块
func (c Console) FindAir(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		if _, ok := (*value).(block_helper.Air); ok {
			return index, helperBlockMapping[index], value
		}
	}
	return 0, protocol.BlockPos{}, nil
}

// FindAir 从操作台的帮助方块中寻找一个铁砧方块。
// includeCenter 指示要查找的方块是否也包括操作
// 台中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// 如果返回的 block 不为空，则说明找到，
// 否则没有找到。找到的方块可以通过修改
// 其指向的值从而将它变成其他方块
func (c Console) FindAnvil(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		if _, ok := (*value).(block_helper.AnvilBlockHelper); ok {
			return index, helperBlockMapping[index], value
		}
	}
	return 0, protocol.BlockPos{}, nil
}

// FindLoom 从操作台的帮助方块中寻找一个织布机方块。
// includeCenter 指示要查找的方块是否也包括操作台
// 中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// 如果返回的 block 不为空，则说明找到，
// 否则没有找到。找到的方块可以通过修改
// 其指向的值从而将它变成其他方块
func (c Console) FindLoom(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		if _, ok := (*value).(block_helper.LoomBlockHelper); ok {
			return index, helperBlockMapping[index], value
		}
	}
	return 0, protocol.BlockPos{}, nil
}

// FindCraftingTable 从操作台的帮助方块中寻找一个工作台方块。
// includeCenter 指示要查找的方块是否也包括操作台
// 中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// 如果返回的 block 不为空，则说明找到，
// 否则没有找到。找到的方块可以通过修改
// 其指向的值从而将它变成其他方块
func (c Console) FindCraftingTable(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		if _, ok := (*value).(block_helper.CraftingTableBlockHelper); ok {
			return index, helperBlockMapping[index], value
		}
	}
	return 0, protocol.BlockPos{}, nil
}

// FindFrame 从操作台的帮助方块中寻找一个物品展示框方块。
// includeCenter 指示要查找的方块是否也包括操作台
// 中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// 如果返回的 block 不为空，则说明找到，
// 否则没有找到。找到的方块可以通过修改
// 其指向的值从而将它变成其他方块
func (c Console) FindFrame(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		if _, ok := (*value).(block_helper.FrameBlockHelper); ok {
			return index, helperBlockMapping[index], value
		}
	}
	return 0, protocol.BlockPos{}, nil
}

// FindCrafter 从操作台的帮助方块中寻找一个合成器方块。
// includeCenter 指示要查找的方块是否也包括操作台
// 中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// 如果返回的 block 不为空，则说明找到，
// 否则没有找到。找到的方块可以通过修改
// 其指向的值从而将它变成其他方块
func (c Console) FindCrafter(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		if _, ok := (*value).(block_helper.CrafterBlockHelper); ok {
			return index, helperBlockMapping[index], value
		}
	}
	return 0, protocol.BlockPos{}, nil
}

// FindCauldron 从操作台的帮助方块中寻找一个炼药锅方块。
// includeCenter 指示要查找的方块是否也包括操作台
// 中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// 如果返回的 block 不为空，则说明找到，
// 否则没有找到。找到的方块可以通过修改
// 其指向的值从而将它变成其他方块
func (c Console) FindCauldron(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		if _, ok := (*value).(block_helper.CauldronBlockHelper); ok {
			return index, helperBlockMapping[index], value
		}
	}
	return 0, protocol.BlockPos{}, nil
}

// FindNonAnvilNonLoomNonCraftingTableNonCrafterNonCauldronAndNonFrame 从操作台的帮助方块
// 中寻找一个既不是铁砧，也不是织布机，且
// 不是工作台、合成器、炼药锅或物品展示框的方块。
//
// 这意味目标方块将可以是空气、容器或其他方块。
//
// includeCenter 指示要查找的方块是否也包括操
// 作台中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// FindNonAnvilNonLoomNonCraftingTableNonCrafterNonCauldronAndNonFrame 在设计上认为一定
// 可以找到目标的方块。
//
// 找到的方块可以通过修改其指向的值从而将它变成其他方块
func (c Console) FindNonAnvilNonLoomNonCraftingTableNonCrafterNonCauldronAndNonFrame(includeCenter bool) (index int, offset protocol.BlockPos, block *block_helper.BlockHelper) {
	idxs := make([]int, 0)

	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		switch (*value).(type) {
		case block_helper.AnvilBlockHelper, block_helper.LoomBlockHelper, block_helper.CraftingTableBlockHelper, block_helper.CrafterBlockHelper, block_helper.CauldronBlockHelper, block_helper.FrameBlockHelper:
		default:
			idxs = append(idxs, index)
		}
	}

	if len(idxs) == 0 {
		panic("FindNonAnvilNonLoomNonCraftingTableNonCrafterNonCauldronAndNonFrame: Should nerver happened")
	}

	randIndex := rand.Intn(len(idxs))
	index = idxs[randIndex]
	offset = helperBlockMapping[index]
	block = c.helperBlocks[index]

	return
}

// FindSpaceToPlaceNewBlock 尝试从操作台
// 找到一个位置以便于使用者放置一个新的方块。
// 它可以是帮助方块、容器，或者其他方块。
//
// includeCenter 指示要查找的方块是否也包括操
// 作台中心处的方块。
//
// 返回的 index 可用于 BlockByIndex，
// 而返回的 offset 可用于 BlockByOffset。
//
// FindSpaceToPlaceNewBlock 在设计上认为
// 一定可以找到目标的方块。
//
// 找到的方块可以通过修改其指向的值从而将它变成其他方块
func (c Console) FindSpaceToPlaceNewBlock(includeCenter bool) (
	index int,
	offset protocol.BlockPos,
	block *block_helper.BlockHelper,
) {
	index, offset, block = c.FindAir(includeCenter)
	if block != nil {
		return
	}

	index, offset, block = c.FindNonAnvilNonLoomNonCraftingTableNonCrafterNonCauldronAndNonFrame(includeCenter)
	if block == nil {
		panic("FindSpaceToPlaceNewBlock: Should nerver happened")
	}

	return
}

// FindMutipleSpaceToPlaceNewBlock 从操作台
// 找到所有可供放置新方块的位置。
//
// includeCenter 指示要查找的方块是否也包括操
// 作台中心处的方块。
//
// FindMutipleSpaceToPlaceNewBlock 在设计上
// 认为 blockIndexs 的长度必定大于 0。
//
// 找到的方块可以通过修改其指向的值从而将它变成其他方块
func (c Console) FindMutipleSpaceToPlaceNewBlock(includeCenter bool) (blockIndexs []int) {
	for index, value := range c.helperBlocks {
		if !includeCenter && index == 0 {
			continue
		}
		switch (*value).(type) {
		case block_helper.AnvilBlockHelper, block_helper.LoomBlockHelper, block_helper.CraftingTableBlockHelper, block_helper.CrafterBlockHelper, block_helper.CauldronBlockHelper, block_helper.FrameBlockHelper:
		default:
			blockIndexs = append(blockIndexs, index)
		}
	}
	return
}

// FindOrGenerateNewAnvil 寻找操作台的 8 个帮助方块中
// 是否有一个是铁砧。如果没有，则生成一个铁砧及其承重方块。
// index 指示找到或生成的铁砧在操作台上的索引
func (c *Console) FindOrGenerateNewAnvil() (index int, err error) {
	var block *block_helper.BlockHelper
	var needFloorBlock bool

	index, _, block = c.FindAnvil(false)
	if block != nil {
		return
	}

	index, _, block = c.FindSpaceToPlaceNewBlock(false)
	if block == nil {
		panic("FindOrGenerateNewAnvil: Should nerver happened")
	}

	nearBlock := c.NearBlockByIndex(index, protocol.BlockPos{0, -1, 0})
	switch (*nearBlock).(type) {
	case block_helper.Air, block_helper.ComplexBlock:
		needFloorBlock = true
	}

	states, err := c.api.SetBlock().SetAnvil(c.BlockPosByIndex(index), needFloorBlock)
	if err != nil {
		return 0, fmt.Errorf("FindOrGenerateNewAnvil: %v", err)
	}

	anvil := block_helper.AnvilBlockHelper{States: states}
	c.UseHelperBlock(RequesterSystemCall, index, anvil)
	if needFloorBlock {
		var floorBlock block_helper.BlockHelper = block_helper.NearBlock{
			Name: game_interface.BaseAnvil,
		}
		*c.NearBlockByIndex(index, protocol.BlockPos{0, -1, 0}) = floorBlock
	}

	return index, nil
}

// FindOrGenerateNewLoom 寻找操作台的 8 个帮助方块中
// 是否有一个是织布机。如果没有，则生成一个新的织布机。
// index 指示找到或生成的织布机在操作台上的索引
func (c *Console) FindOrGenerateNewLoom() (index int, err error) {
	var block *block_helper.BlockHelper

	index, _, block = c.FindLoom(false)
	if block != nil {
		return
	}

	index, _, block = c.FindSpaceToPlaceNewBlock(false)
	if block == nil {
		panic("FindOrGenerateNewLoom: Should nerver happened")
	}

	loom := block_helper.LoomBlockHelper{}
	err = c.api.SetBlock().SetBlock(
		c.BlockPosByIndex(index),
		loom.BlockName(),
		loom.BlockStatesString(),
	)
	if err != nil {
		return 0, fmt.Errorf("FindOrGenerateNewLoom: %v", err)
	}
	c.UseHelperBlock(RequesterSystemCall, index, loom)

	return index, nil
}

// FindOrGenerateNewCraftingTable 寻找操作台的 8 个帮助方块中
// 是否有一个是工作台。如果没有，则生成一个新的工作台。
// index 指示找到或生成的工作台在操作台上的索引
func (c *Console) FindOrGenerateNewCraftingTable() (index int, err error) {
	var block *block_helper.BlockHelper

	index, _, block = c.FindCraftingTable(false)
	if block != nil {
		return
	}

	index, _, block = c.FindSpaceToPlaceNewBlock(false)
	if block == nil {
		panic("FindOrGenerateNewCraftingTable: Should nerver happened")
	}

	craftingTable := block_helper.CraftingTableBlockHelper{}
	err = c.api.SetBlock().SetBlock(
		c.BlockPosByIndex(index),
		craftingTable.BlockName(),
		craftingTable.BlockStatesString(),
	)
	if err != nil {
		return 0, fmt.Errorf("FindOrGenerateNewCraftingTable: %v", err)
	}
	c.UseHelperBlock(RequesterSystemCall, index, craftingTable)

	return index, nil
}

// FindOrGenerateNewFrame 寻找操作台的 8 个帮助方块中
// 是否有一个是物品展示框。如果没有，则生成一个新的物品展示框。
// index 指示找到或生成的物品展示框在操作台上的索引
func (c *Console) FindOrGenerateNewFrame() (index int, err error) {
	var block *block_helper.BlockHelper

	index, _, block = c.FindFrame(false)
	if block != nil {
		return
	}

	index, _, block = c.FindSpaceToPlaceNewBlock(false)
	if block == nil {
		panic("FindOrGenerateNewFrame: Should nerver happened")
	}

	frame := block_helper.FrameBlockHelper{}
	err = c.api.SetBlock().SetBlock(
		c.BlockPosByIndex(index),
		frame.BlockName(),
		frame.BlockStatesString(),
	)
	if err != nil {
		return 0, fmt.Errorf("FindOrGenerateNewFrame: %v", err)
	}
	c.UseHelperBlock(RequesterSystemCall, index, frame)

	return index, nil
}

// FindOrGenerateNewCrafter 寻找操作台的 8 个帮助方块中
// 是否有一个是合成器。如果没有，则生成一个新的合成器。
// index 指示找到或生成的合成器在操作台上的索引
func (c *Console) FindOrGenerateNewCrafter() (index int, err error) {
	var block *block_helper.BlockHelper

	index, _, block = c.FindCrafter(false)
	if block != nil {
		return
	}

	index, _, block = c.FindSpaceToPlaceNewBlock(false)
	if block == nil {
		panic("FindOrGenerateNewCrafter: Should nerver happened")
	}

	crafter := block_helper.CrafterBlockHelper{}
	err = c.api.SetBlock().SetBlock(
		c.BlockPosByIndex(index),
		crafter.BlockName(),
		crafter.BlockStatesString(),
	)
	if err != nil {
		return 0, fmt.Errorf("FindOrGenerateNewCrafter: %v", err)
	}
	c.UseHelperBlock(RequesterSystemCall, index, crafter)

	return index, nil
}

// FindOrGenerateNewCauldron 寻找操作台的 8 个帮助方块中
// 是否有一个是炼药锅。如果没有，则生成一个新的炼药锅。
// customColor 可选，用于指定炼药锅的目标 RGB 颜色。
// index 指示找到或生成的炼药锅在操作台上的索引
func (c *Console) FindOrGenerateNewCauldron(customColor ...[3]uint8) (index int, err error) {
	var block *block_helper.BlockHelper
	var targetColor [3]uint8
	haveTargetColor := len(customColor) > 0
	if haveTargetColor {
		targetColor = customColor[0]
	}

	index, _, block = c.FindCauldron(false)
	if block != nil {
		if !haveTargetColor {
			return
		}
		err = c.colorCauldronByHelper(index, targetColor)
		if err != nil {
			return 0, fmt.Errorf("FindOrGenerateNewCauldron: %v", err)
		}
		return index, nil
	}

	index, _, block = c.FindSpaceToPlaceNewBlock(false)
	if block == nil {
		panic("FindOrGenerateNewCauldron: Should nerver happened")
	}

	if haveTargetColor {
		err = c.colorCauldronByHelper(index, targetColor)
		if err != nil {
			return 0, fmt.Errorf("FindOrGenerateNewCauldron: %v", err)
		}
		return index, nil
	}

	cauldron := block_helper.CauldronBlockHelper{}
	err = c.api.SetBlock().SetBlock(
		c.BlockPosByIndex(index),
		cauldron.BlockName(),
		cauldron.BlockStatesString(),
	)
	if err != nil {
		return 0, fmt.Errorf("FindOrGenerateNewCauldron: %v", err)
	}
	c.UseHelperBlock(RequesterSystemCall, index, cauldron)

	return index, nil
}

// colorCauldronByHelper 在 index 处生成并染色一个炼药锅。
func (c *Console) colorCauldronByHelper(index int, customColor [3]uint8) error {
	dyeIDs, found := utils.SearchCauldronDyeIDsByColor(customColor)
	if !found {
		return fmt.Errorf("colorCauldronByHelper: Can't found dye IDs by color %#v", customColor)
	}

	err := c.API().SetBlock().SetBlock(c.BlockPosByIndex(index), "minecraft:air", "[]")
	if err != nil {
		return fmt.Errorf("colorCauldronByHelper: %v", err)
	}
	c.UseHelperBlock(RequesterSystemCall, index, block_helper.Air{})

	cauldron := block_helper.CauldronBlockHelper{States: map[string]any{
		"cauldron_liquid": "water",
		"fill_level":      int32(6),
	}}
	err = c.api.SetBlock().SetBlock(
		c.BlockPosByIndex(index),
		cauldron.BlockName(),
		cauldron.BlockStatesString(),
	)
	if err != nil {
		return fmt.Errorf("colorCauldronByHelper: %v", err)
	}
	c.UseHelperBlock(RequesterSystemCall, index, cauldron)

	for _, dyeID := range dyeIDs {
		dyeItemName := mapping.RGBToDyeItemName[mapping.DefaultDyeColor[dyeID]]
		err = c.api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:  dyeItemName,
				Count: 1,
				Slot:  c.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return fmt.Errorf("colorCauldronByHelper: %v", err)
		}

		err = c.api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
			HotbarSlotID: c.HotbarSlotID(),
			BotPos:       c.Position(),
			BlockPos:     c.BlockPosByIndex(index),
			BlockName:    cauldron.BlockName(),
			BlockStates:  cauldron.BlockStates(),
		})
		if err != nil {
			return fmt.Errorf("colorCauldronByHelper: %v", err)
		}
	}

	return nil
}
