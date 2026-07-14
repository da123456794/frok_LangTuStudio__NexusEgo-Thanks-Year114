package GameInterface

import (
	"fmt"

	ResourcesControl "nexus/utils/api/resources_control"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

type ItemLocation struct {
	WindowID    uint8
	ContainerID uint8
	Slot        uint8
}

func stackSlotInfo(containerID, slot uint8, stackNetworkID int32) protocol.StackRequestSlotInfo {
	return protocol.StackRequestSlotInfo{
		Container: protocol.FullContainerName{
			ContainerID: containerID,
		},
		Slot:           slot,
		StackNetworkID: stackNetworkID,
	}
}

func (g *GameInterface) MoveItem(
	source ItemLocation,
	destination ItemLocation,
	moveCount uint8,
	sourceResult protocol.ItemInstance,
	destResult protocol.ItemInstance,
) ([]protocol.ItemStackResponse, error) {
	itemOnSource, err := g.Resources.Inventory.GetItemStackInfo(uint32(source.WindowID), source.Slot)
	if err != nil {
		return nil, fmt.Errorf("MoveItem: %v", err)
	}
	itemOnDestination, _ := g.Resources.Inventory.GetItemStackInfo(uint32(destination.WindowID), destination.Slot)
	if itemOnSource.Stack.NetworkID == 0 {
		return nil, ErrMoveItemCheckFailure
	}

	count := moveCount
	if count > uint8(itemOnSource.Stack.Count) {
		count = uint8(itemOnSource.Stack.Count)
	}

	action := &protocol.PlaceStackRequestAction{}
	action.Count = count
	action.Source = stackSlotInfo(
		source.ContainerID,
		source.Slot,
		itemOnSource.StackNetworkID,
	)
	action.Destination = stackSlotInfo(
		destination.ContainerID,
		destination.Slot,
		itemOnDestination.StackNetworkID,
	)

	changeDetails := map[ResourcesControl.ContainerID]ResourcesControl.StackRequestContainerInfo{
		ResourcesControl.ContainerID(source.ContainerID): {
			WindowID: uint32(source.WindowID),
			ChangeResult: map[uint8]protocol.ItemInstance{
				source.Slot: sourceResult,
			},
		},
	}
	if source.ContainerID == destination.ContainerID {
		changeDetails[ResourcesControl.ContainerID(source.ContainerID)] = ResourcesControl.StackRequestContainerInfo{
			WindowID: uint32(source.WindowID),
			ChangeResult: map[uint8]protocol.ItemInstance{
				source.Slot:      sourceResult,
				destination.Slot: destResult,
			},
		}
	} else {
		changeDetails[ResourcesControl.ContainerID(destination.ContainerID)] = ResourcesControl.StackRequestContainerInfo{
			WindowID: uint32(destination.WindowID),
			ChangeResult: map[uint8]protocol.ItemInstance{
				destination.Slot: destResult,
			},
		}
	}

	ans, err := g.SendItemStackRequestWithResponse(
		&packet.ItemStackRequest{
			Requests: []protocol.ItemStackRequest{
				{
					Actions: []protocol.StackRequestAction{action},
				},
			},
		},
		[]ItemChangingDetails{{Details: changeDetails}},
	)
	if err != nil {
		return nil, fmt.Errorf("MoveItem: %v", err)
	}
	return ans, nil
}

func (g *GameInterface) DropItemAll(
	source protocol.StackRequestSlotInfo,
	windowID uint32,
) (bool, error) {
	ans, err := g.SendItemStackRequestWithResponse(
		&packet.ItemStackRequest{
			Requests: []protocol.ItemStackRequest{
				{
					Actions: []protocol.StackRequestAction{
						&protocol.DropStackRequestAction{
							Count:    64,
							Source:   source,
							Randomly: false,
						},
					},
				},
			},
		},
		[]ItemChangingDetails{
			{
				Details: map[ResourcesControl.ContainerID]ResourcesControl.StackRequestContainerInfo{
					ResourcesControl.ContainerID(source.Container.ContainerID): {
						WindowID: windowID,
						ChangeResult: map[uint8]protocol.ItemInstance{
							source.Slot: AirItem,
						},
					},
				},
			},
		},
	)
	if err != nil {
		return false, fmt.Errorf("DropItemAll: %v", err)
	}
	if len(ans) == 0 || ans[0].Status != protocol.ItemStackResponseStatusOK {
		return false, nil
	}
	return true, nil
}

