package GameInterface

import (
	"fmt"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	mcstructure "nexus/utils/api/mcstructure"
)

type AnvilOperationResponse struct {
	Successful  bool
	Destination *ItemLocation
}

type ItemRenamingRequest struct {
	Slot uint8
	Name string
}

func (g *GameInterface) RenameItemByAnvil(
	pos [3]int32,
	blockStates string,
	hotBarSlotID uint8,
	request []ItemRenamingRequest,
) ([]AnvilOperationResponse, error) {
	responses := make([]AnvilOperationResponse, 0, len(request))

	if err := g.SendAICommand("gamemode 1", true); err != nil {
		return nil, fmt.Errorf("RenameItemByAnvil: %v", err)
	}

	uniqueID, correctPos, err := g.GenerateNewAnvil(pos, blockStates)
	if err != nil {
		return nil, fmt.Errorf("RenameItemByAnvil: %v", err)
	}

	if err := g.SendAICommand(
		fmt.Sprintf("tp %d %d %d", correctPos[0], correctPos[1], correctPos[2]),
		true,
	); err != nil {
		return nil, fmt.Errorf("RenameItemByAnvil: %v", err)
	}
	if err := g.AwaitChangesGeneral(); err != nil {
		return nil, fmt.Errorf("RenameItemByAnvil: %v", err)
	}

	holder := g.Resources.Container.Occupy()
	defer g.Resources.Container.Release(holder)

	blockStatesMap, err := mcstructure.UnmarshalBlockStates(blockStates)
	if err != nil {
		return nil, fmt.Errorf("RenameItemByAnvil: %v", err)
	}
	if err := g.ChangeSelectedHotbarSlot(hotBarSlotID); err != nil {
		return nil, fmt.Errorf("RenameItemByAnvil: %v", err)
	}

	success, err := g.OpenContainer(correctPos, "minecraft:anvil", blockStatesMap, hotBarSlotID)
	if err != nil {
		return nil, fmt.Errorf("RenameItemByAnvil: %v", err)
	}
	if !success {
		return nil, fmt.Errorf("RenameItemByAnvil: failed to open anvil at %v", correctPos)
	}
	defer func() {
		_, _ = g.CloseContainer()
		_ = g.RevertStructure(
			uniqueID,
			BlockPos{int(correctPos[0]), int(correctPos[1] - 1), int(correctPos[2])},
		)
	}()

	for _, req := range request {
		itemData, err := g.Resources.Inventory.GetItemStackInfo(0, req.Slot)
		if err != nil || itemData.Stack.ItemType.NetworkID == 0 {
			responses = append(responses, AnvilOperationResponse{
				Successful: false,
				Destination: &ItemLocation{
					WindowID:    0,
					ContainerID: ContainerIDInventory,
					Slot:        req.Slot,
				},
			})
			continue
		}

		containerOpen := g.Resources.Container.GetContainerOpeningData()
		if containerOpen == nil {
			return responses, fmt.Errorf("RenameItemByAnvil: anvil has been closed")
		}

		moveResp, err := g.MoveItem(
			ItemLocation{WindowID: 0, ContainerID: ContainerIDInventory, Slot: req.Slot},
			ItemLocation{WindowID: containerOpen.WindowID, ContainerID: 0, Slot: 1},
			uint8(itemData.Stack.Count),
			AirItem,
			itemData,
		)
		if err != nil {
			return responses, fmt.Errorf("RenameItemByAnvil: %v", err)
		}
		if len(moveResp) == 0 || moveResp[0].Status != protocol.ItemStackResponseStatusOK {
			responses = append(responses, AnvilOperationResponse{
				Successful: false,
				Destination: &ItemLocation{
					WindowID:    0,
					ContainerID: ContainerIDInventory,
					Slot:        req.Slot,
				},
			})
			continue
		}

		renameResp, err := g.RenameItem(req.Name, req.Slot)
		if err != nil {
			return responses, fmt.Errorf("RenameItemByAnvil: %v", err)
		}
		if renameResp == nil {
			responses = append(responses, AnvilOperationResponse{
				Successful:  false,
				Destination: nil,
			})
			continue
		}
		responses = append(responses, *renameResp)
	}

	return responses, nil
}

