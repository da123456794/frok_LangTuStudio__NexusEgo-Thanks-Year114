package GameInterface

import (
	"fmt"

	types "nexus/defines"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
)

func (g *GameInterface) CopyItem(
	hotBarSlot uint8,
	blockPos [3]int32,
	requestCount uint8,
) error {
	if requestCount <= 1 {
		return nil
	}

	success, err := g.OpenContainer(
		blockPos,
		"minecraft:barrel",
		map[string]any{
			"facing_direction": int32(0),
			"open_bit":         byte(1),
		},
		hotBarSlot,
	)
	if err != nil {
		return fmt.Errorf("CopyItem: %v", err)
	}
	if !success {
		return fmt.Errorf("CopyItem: failed to open barrel")
	}
	defer func() {
		_, _ = g.CloseContainer()
	}()

	sourceData, err := g.Resources.Inventory.GetItemStackInfo(0, hotBarSlot)
	if err != nil {
		return fmt.Errorf("CopyItem: %v", err)
	}
	if sourceData.Stack.NetworkID == 0 {
		return fmt.Errorf("CopyItem: source item is air")
	}

	container := g.Resources.Container.GetContainerOpeningData()
	if container == nil {
		return fmt.Errorf("CopyItem: container closed unexpectedly")
	}

	targetCount := uint16(requestCount)
	for sourceData.Stack.Count < targetCount {
		moveResp, err := g.MoveItem(
			ItemLocation{WindowID: 0, ContainerID: ContainerIDInventory, Slot: hotBarSlot},
			ItemLocation{WindowID: container.WindowID, ContainerID: ContainerIDBarrel, Slot: 13},
			uint8(sourceData.Stack.Count),
			AirItem,
			sourceData,
		)
		if err != nil {
			return fmt.Errorf("CopyItem: %v", err)
		}
		if len(moveResp) == 0 || moveResp[0].Status != protocol.ItemStackResponseStatusOK {
			return fmt.Errorf("CopyItem: move to container rejected")
		}

		if err := g.ReplaceItemInInventory(
			TargetMySelf,
			ItemGenerateLocation{Path: "slot.hotbar", Slot: hotBarSlot},
			ChestSlotAir(),
			"",
			true,
		); err != nil {
			return fmt.Errorf("CopyItem: %v", err)
		}

		restoreResp, err := g.MoveItem(
			ItemLocation{WindowID: container.WindowID, ContainerID: ContainerIDBarrel, Slot: 13},
			ItemLocation{WindowID: 0, ContainerID: ContainerIDInventory, Slot: hotBarSlot},
			uint8(sourceData.Stack.Count),
			AirItem,
			sourceData,
		)
		if err != nil {
			return fmt.Errorf("CopyItem: %v", err)
		}
		if len(restoreResp) == 0 || restoreResp[0].Status != protocol.ItemStackResponseStatusOK {
			return fmt.Errorf("CopyItem: restore from container rejected")
		}

		sourceData, err = g.Resources.Inventory.GetItemStackInfo(0, hotBarSlot)
		if err != nil {
			return fmt.Errorf("CopyItem: %v", err)
		}
		if sourceData.Stack.Count >= targetCount {
			break
		}
	}
	return nil
}

func ChestSlotAir() types.ChestSlot {
	return types.ChestSlot{
		Name:   "air",
		Count:  1,
		Damage: 0,
	}
}

