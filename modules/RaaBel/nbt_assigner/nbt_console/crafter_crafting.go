package nbt_console

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/utils"
)

// inventorySlotIsAir 检查背包 slotID 处是否是空气。
func (c *Console) inventorySlotIsAir(slotID resources_control.SlotID) (isAir bool, err error) {
	item, inventoryExisted := c.api.Resources().Inventories().GetItemStack(resources_control.WindowNameInventory, slotID)
	if !inventoryExisted {
		return false, fmt.Errorf("inventorySlotIsAir: Inventory 0 is not existed")
	}
	if item.Stack.NetworkID == 0 || item.Stack.NetworkID == -1 || item.Stack.Count == 0 {
		return true, nil
	}
	return false, nil
}

// CraftByCrafter 使用操作台 index 处的合成器制作一个物品。
// inventoryToCrafterSlotMapping 记录了背包物品栏到合成器 3*3 物品栏的映射关系，
// resultSlot 是期望产物出现的背包物品栏。
func (c *Console) CraftByCrafter(
	index int,
	inventoryToCrafterSlotMapping map[resources_control.SlotID]resources_control.SlotID,
	resultSlot resources_control.SlotID,
) error {
	api := c.api

	if len(inventoryToCrafterSlotMapping) == 0 {
		return fmt.Errorf("CraftByCrafter: inventoryToCrafterSlotMapping is empty")
	}

	resultSlotIsAir, err := c.inventorySlotIsAir(resultSlot)
	if err != nil {
		return fmt.Errorf("CraftByCrafter: %v", err)
	}
	if !resultSlotIsAir {
		return fmt.Errorf("CraftByCrafter: Expected result slot %d is not empty", resultSlot)
	}

	success, err := c.OpenContainerByIndex(index)
	if err != nil {
		return fmt.Errorf("CraftByCrafter: %v", err)
	}
	if !success {
		return fmt.Errorf("CraftByCrafter: Failed to open the crafter at index = %d", index)
	}
	containerOpened := true

	floorOffset := protocol.BlockPos{0, -1, 0}
	crafterPos := c.BlockPosByIndex(index)
	floorPos := c.NearBlockPosByIndex(index, floorOffset)
	originFloor := *c.NearBlockByIndex(index, floorOffset)
	filledBarrierSlots := make([]resources_control.SlotID, 0)
	redstonePlaced := false

	appendErr := func(originErr error, newErr error) error {
		if newErr == nil {
			return originErr
		}
		if originErr == nil {
			return newErr
		}
		return fmt.Errorf("%v; %v", originErr, newErr)
	}

	cleanup := func(originErr error) error {
		resultErr := originErr

		for _, slotID := range filledBarrierSlots {
			err := api.Replaceitem().ReplaceitemInInventory(
				"@s",
				game_interface.ReplacePathInventory,
				game_interface.ReplaceitemInfo{
					Name:     "minecraft:air",
					Count:    1,
					MetaData: 0,
					Slot:     slotID,
				},
				"",
				true,
			)
			if err != nil {
				resultErr = appendErr(resultErr, fmt.Errorf("CraftByCrafter: %v", err))
				continue
			}
			c.UseInventorySlot(RequesterUser, slotID, false)
		}

		if redstonePlaced {
			err := api.SetBlock().SetBlock(
				floorPos,
				originFloor.BlockName(),
				originFloor.BlockStatesString(),
			)
			if err != nil {
				resultErr = appendErr(resultErr, fmt.Errorf("CraftByCrafter: %v", err))
			} else {
				*c.NearBlockByIndex(index, floorOffset) = originFloor
			}
		}

		if containerOpened {
			err := api.ContainerOpenAndClose().CloseContainer()
			if err != nil {
				resultErr = appendErr(resultErr, fmt.Errorf("CraftByCrafter: %v", err))
			} else {
				containerOpened = false
			}
		}

		return resultErr
	}

	transaction := api.ItemStackOperation().OpenTransaction()
	for inventorySlot, crafterSlot := range inventoryToCrafterSlotMapping {
		_ = transaction.MoveToContainer(inventorySlot, crafterSlot, 1)
	}
	success, _, _, err = transaction.Commit()
	if err != nil {
		return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
	}
	if !success {
		return cleanup(fmt.Errorf("CraftByCrafter: The server rejected item stack request actions when move item to crafter"))
	}

	err = api.ContainerOpenAndClose().CloseContainer()
	if err != nil {
		return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
	}
	containerOpened = false

	for inventorySlot := range inventoryToCrafterSlotMapping {
		slotIsAir, err := c.inventorySlotIsAir(inventorySlot)
		if err != nil {
			return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
		}
		c.UseInventorySlot(RequesterUser, inventorySlot, !slotIsAir)
	}

	for slot := range 36 {
		slotID := resources_control.SlotID(slot)
		if slotID == resultSlot {
			continue
		}

		slotIsAir, err := c.inventorySlotIsAir(slotID)
		if err != nil {
			return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
		}
		if !slotIsAir {
			continue
		}

		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathInventory,
			game_interface.ReplaceitemInfo{
				Name:     "minecraft:barrier",
				Count:    1,
				MetaData: 0,
				Slot:     slotID,
			},
			"",
			true,
		)
		if err != nil {
			return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
		}

		filledBarrierSlots = append(filledBarrierSlots, slotID)
		c.UseInventorySlot(RequesterUser, slotID, true)
	}

	targetPos := protocol.BlockPos{crafterPos[0], crafterPos[1] + 1, crafterPos[2]}
	dimensionName := utils.DimensionNameByID(c.dimension)
	err = api.Commands().SendSettingsCommand(
		fmt.Sprintf("execute in %s run tp %d %d %d", dimensionName, targetPos[0], targetPos[1], targetPos[2]),
		true,
	)
	if err != nil {
		return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
	}
	err = api.Commands().AwaitChangesGeneral()
	if err != nil {
		return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
	}
	c.position = targetPos

	err = api.SetBlock().SetBlock(floorPos, "minecraft:redstone_block", "[]")
	if err != nil {
		return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
	}
	redstonePlaced = true

	var redstoneFloor block_helper.BlockHelper = block_helper.NearBlock{Name: "minecraft:redstone_block"}
	*c.NearBlockByIndex(index, floorOffset) = redstoneFloor

	for try := 0; ; try++ {
		resultSlotIsAir, err = c.inventorySlotIsAir(resultSlot)
		if err != nil {
			return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
		}
		if !resultSlotIsAir {
			break
		}
		if try >= 80 {
			return cleanup(fmt.Errorf("CraftByCrafter: Timeout waiting crafted item to appear in slot %d", resultSlot))
		}

		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return cleanup(fmt.Errorf("CraftByCrafter: %v", err))
		}
	}

	c.UseInventorySlot(RequesterUser, resultSlot, true)
	return cleanup(nil)
}
