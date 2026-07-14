package item_stack_operation

import (
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
)

// BeaconPayment 指示信标支付操作。
// 它会将支付物品移动到信标支付槽，并提交主/副效果选择。
type BeaconPayment struct {
	// PaymentPath 指示支付物品在打开信标前所在的位置。
	PaymentPath resources_control.SlotLocation
	// PrimaryEffect 与 SecondaryEffect 指示信标主/副效果 ID。
	PrimaryEffect, SecondaryEffect int32
}

func (BeaconPayment) ID() uint8 {
	return IDItemStackOperationHighLevelBeaconPayment
}

func (BeaconPayment) CanInline() bool {
	return false
}

func (b BeaconPayment) Make(runtimeData MakingRuntime) []protocol.StackRequestAction {
	data := runtimeData.(BeaconPaymentRuntime)

	movePayment := protocol.PlaceStackRequestAction{}
	movePayment.Count = 1
	movePayment.Source = protocol.StackRequestSlotInfo{
		Container:      protocol.FullContainerName{ContainerID: data.MovePaymentSrcContainerID},
		Slot:           byte(b.PaymentPath.SlotID),
		StackNetworkID: data.MovePaymentSrcStackNetworkID,
	}
	movePayment.Destination = protocol.StackRequestSlotInfo{
		Container:      protocol.FullContainerName{ContainerID: protocol.ContainerBeaconPayment},
		Slot:           0x1b,
		StackNetworkID: data.BeaconInputStackNetworkID,
	}

	return []protocol.StackRequestAction{
		&movePayment,
		&protocol.BeaconPaymentStackRequestAction{
			PrimaryEffect:   b.PrimaryEffect,
			SecondaryEffect: b.SecondaryEffect,
		},
		&protocol.DestroyStackRequestAction{
			Count: 1,
			Source: protocol.StackRequestSlotInfo{
				Container: protocol.FullContainerName{ContainerID: protocol.ContainerBeaconPayment},
				Slot:      0x1b,
				// 约定使用请求 ID 指向当前请求中该槽位的更新结果。
				StackNetworkID: data.RequestID,
			},
		},
	}
}
