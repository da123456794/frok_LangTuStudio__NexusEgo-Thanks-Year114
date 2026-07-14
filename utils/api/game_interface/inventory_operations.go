package GameInterface

import (
	"fmt"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

// 鍒囨崲瀹㈡埛绔殑鎵嬫寔鐗╁搧鏍忎负 hotBarSlotID 銆?// 鑻ユ彁渚涚殑 hotBarSlotID 澶т簬 8 锛屽垯浼氶噸瀹氬悜涓?0
func (g *GameInterface) ChangeSelectedHotbarSlot(hotbarSlotID uint8) error {
	if hotbarSlotID > 8 {
		hotbarSlotID = 0
	}
	// init var
	err := g.WritePacket(&packet.PlayerHotBar{
		SelectedHotBarSlot: uint32(hotbarSlotID),
		WindowID:           0,
		SelectHotBarSlot:   true,
	})
	if err != nil {
		return fmt.Errorf("ChangeSelectedHotbarSlot: %v", err)
	}
	// change selected hotbar slot
	return nil
	// return
}

/*
鎵撳紑鑳屽寘銆?杩斿洖鍊肩殑绗竴椤逛唬琛ㄦ墽琛岀粨鏋滐紝涓虹湡鏃惰儗鍖呰鎴愬姛鎵撳紑锛屽惁鍒欏弽涔嬨€?濡傞渶瑕佸叧闂凡鎵撳紑鐨勮儗鍖咃紝璇风洿鎺ヤ娇鐢ㄥ嚱鏁?CloseContainer 銆?
璇风‘淇濇墦寮€鍓嶅崰鐢ㄤ簡瀹瑰櫒璧勬簮锛屽惁鍒欎細閫犳垚绋嬪簭 panic 銆?*/
func (g *GameInterface) OpenInventory() (bool, error) {
	g.Resources.Container.AwaitChangesBeforeSendingPacket()
	// await responce before send packet
	err := g.WritePacket(&packet.Interact{
		ActionType:            packet.InteractActionOpenInventory,
		TargetEntityRuntimeID: g.ClientInfo.EntityRuntimeID,
	})
	if err != nil {
		return false, fmt.Errorf("OpenInventory: %v", err)
	}
	// open inventory
	g.Resources.Container.AwaitChangesAfterSendingPacket()
	// wait changes
	if g.Resources.Container.GetContainerOpeningData() == nil {
		return false, nil
	}
	// if unsuccess
	return true, nil
	// return
}

