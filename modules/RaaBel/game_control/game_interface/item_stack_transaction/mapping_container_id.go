package item_stack_transaction

import (
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/mapping"
)

// slotLocationToContainerName 根据 slotLocation 查找本地持有(或打开)的库存，
// 并找到对应 slotLocation 的完整容器名。api 指示容器资源的管理器
func slotLocationToContainerName(
	api *resources_control.ContainerManager,
	slotLocation resources_control.SlotLocation,
) (
	result protocol.FullContainerName,
	found bool,
) {
	switch slotLocation.WindowID {
	case protocol.WindowIDInventory:
		return protocol.FullContainerName{ContainerID: protocol.ContainerCombinedHotBarAndInventory}, true
	case protocol.WindowIDOffHand:
		return protocol.FullContainerName{ContainerID: protocol.ContainerOffhand}, true
	case protocol.WindowIDArmour:
		return protocol.FullContainerName{ContainerID: protocol.ContainerArmor}, true
	case protocol.WindowIDDynamic:
		return protocol.FullContainerName{ContainerID: protocol.ContainerDynamic, DynamicContainerID: protocol.Option(uint32(slotLocation.DynamicContainerID))}, true
	case protocol.WindowIDCrafting:
		return protocol.FullContainerName{ContainerID: protocol.ContainerCraftingInput}, true
	case protocol.WindowIDUI:
		return protocol.FullContainerName{}, false // TODO: Figure out what WindowIDUI means
	}

	containerData, containerID, existed := api.ContainerData()
	if !existed {
		return protocol.FullContainerName{}, false
	}
	if containerData.WindowID != byte(slotLocation.WindowID) {
		return protocol.FullContainerName{}, false
	}
	if containerID != mapping.ContainerIDUnknown {
		return protocol.FullContainerName{ContainerID: byte(containerID)}, true
	}

	containerTypeWithSlot := mapping.ContainerTypeWithSlot{
		ContainerType: int(containerData.ContainerType),
	}
	if mapping.ContainerNeedSlotIDMapping[containerTypeWithSlot.ContainerType] {
		containerTypeWithSlot.SlotID = uint8(slotLocation.SlotID)
	}
	if result, ok := mapping.ContainerIDMapping[containerTypeWithSlot]; ok {
		if result == mapping.ContainerIDUnknown || result == mapping.ContainerIDCanNotOpen {
			return protocol.FullContainerName{}, false
		}
		return protocol.FullContainerName{ContainerID: byte(result)}, true
	}

	return protocol.FullContainerName{}, false
}
