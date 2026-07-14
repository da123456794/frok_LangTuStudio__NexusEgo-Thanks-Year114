package GameInterface

import (
	"encoding/gob"
	"fmt"

	ResourcesControl "nexus/utils/api/resources_control"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

func (g *GameInterface) RenameItem(
	name string,
	slot uint8,
) (*AnvilOperationResponse, error) {
	containerOpeningData := g.Resources.Container.GetContainerOpeningData()
	if containerOpeningData == nil {
		return nil, fmt.Errorf("RenameItem: anvil has not opened")
	}

	get, err := g.Resources.Inventory.GetItemStackInfo(uint32(containerOpeningData.WindowID), 1)
	if err != nil {
		return nil, fmt.Errorf("RenameItem: %v", err)
	}
	if get.Stack.ItemType.NetworkID == 0 {
		return nil, fmt.Errorf("RenameItem: item provided is air")
	}

	var itemDatas protocol.ItemInstance
	ResourcesControl.DeepCopy(&get, &itemDatas, func() {
		gob.Register(map[string]any{})
		gob.Register([]any{})
	})
	var backup protocol.ItemInstance
	ResourcesControl.DeepCopy(&get, &backup, func() {
		gob.Register(map[string]any{})
		gob.Register([]any{})
	})

	g.Resources.ItemStackOperation.SetItemName(&itemDatas, name)
	if _, ok := itemDatas.Stack.NBTData["RepairCost"]; !ok {
		itemDatas.Stack.NBTData["RepairCost"] = int32(0)
	}

	newRequestID := g.Resources.ItemStackOperation.GetNewRequestID()

	moveBack := &protocol.PlaceStackRequestAction{}
	moveBack.Count = byte(itemDatas.Stack.Count)
	moveBack.Source = stackSlotInfo(protocol.ContainerCreatedOutput, 0x32, newRequestID)
	moveBack.Destination = stackSlotInfo(ContainerIDInventory, slot, 0)

	request := &packet.ItemStackRequest{
		Requests: []protocol.ItemStackRequest{
			{
				RequestID: newRequestID,
				Actions: []protocol.StackRequestAction{
					&protocol.CraftRecipeOptionalStackRequestAction{
						RecipeNetworkID:   0,
						FilterStringIndex: 0,
					},
					&protocol.ConsumeStackRequestAction{
						DestroyStackRequestAction: protocol.DestroyStackRequestAction{
							Count: byte(itemDatas.Stack.Count),
							Source: stackSlotInfo(
								0,
								1,
								itemDatas.StackNetworkID,
							),
						},
					},
					moveBack,
				},
				FilterStrings: []string{name},
				FilterCause:   protocol.FilterCauseAnvilText,
			},
		},
	}

	err = g.Resources.ItemStackOperation.WriteRequest(
		newRequestID,
		map[ResourcesControl.ContainerID]ResourcesControl.StackRequestContainerInfo{
			ResourcesControl.ContainerID(ContainerIDInventory): {
				WindowID: 0,
				ChangeResult: map[uint8]protocol.ItemInstance{
					slot: itemDatas,
				},
			},
			0: {
				WindowID: uint32(containerOpeningData.WindowID),
				ChangeResult: map[uint8]protocol.ItemInstance{
					1: AirItem,
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("RenameItem: %v", err)
	}

	if err := g.WritePacket(request); err != nil {
		return nil, fmt.Errorf("RenameItem: %v", err)
	}

	ans, err := g.Resources.ItemStackOperation.LoadResponseAndDelete(newRequestID)
	if err != nil {
		return nil, fmt.Errorf("RenameItem: %v", err)
	}
	if ans.Status != protocol.ItemStackResponseStatusOK {
		return &AnvilOperationResponse{
			Successful: false,
			Destination: &ItemLocation{
				WindowID:    uint8(containerOpeningData.WindowID),
				ContainerID: 0,
				Slot:        1,
			},
		}, nil
	}

	return &AnvilOperationResponse{
		Successful: true,
		Destination: &ItemLocation{
			WindowID:    0,
			ContainerID: ContainerIDInventory,
			Slot:        slot,
		},
	}, nil
}

