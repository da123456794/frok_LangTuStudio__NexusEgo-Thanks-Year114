package item_stack_operation

import (
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
)

// CreativeItem 指示创造物品获取操作
type CreativeItem struct {
	CINI  uint32 // CreativeItemNetworkID
	Path  resources_control.SlotLocation
	Count uint8
}

func (CreativeItem) ID() uint8 {
	return IDItemStackOperationCreativeItem
}

func (CreativeItem) CanInline() bool {
	return false
}

func (d CreativeItem) Make(runtimeData MakingRuntime) []protocol.StackRequestAction {
	data := runtimeData.(CreativeItemRuntime)

	move := protocol.PlaceStackRequestAction{}
	move.Count = d.Count
	move.Source = protocol.StackRequestSlotInfo{
		Container: protocol.FullContainerName{
			ContainerID: protocol.ContainerCreatedOutput,
		},
		Slot:           0x32,
		StackNetworkID: data.RequestID,
	}
	move.Destination = protocol.StackRequestSlotInfo{
		Container:      data.DstContainer,
		Slot:           byte(d.Path.SlotID),
		StackNetworkID: data.DstItemStackID,
	}

	return []protocol.StackRequestAction{
		&protocol.CraftCreativeStackRequestAction{CreativeItemNetworkID: data.CreativeItemNetworkID, NumberOfCrafts: 1},
		&move,
	}
}
