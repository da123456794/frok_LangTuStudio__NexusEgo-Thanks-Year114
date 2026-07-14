package GameInterface

import (
	"fmt"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

/*
鎵撳紑 pos 澶勫悕涓?blockName 涓旀柟鍧楃姸鎬佷负 blockStates 鐨勫鍣ㄣ€?hotBarSlotID 瀛楁浠ｈ〃鐜╁姝ゆ椂鎵嬫寔鐨勭墿鍝佹爮锛?鍥犱负鎵撳紑瀹瑰櫒瀹為檯涓婃槸涓€娆℃柟鍧楃偣鍑讳簨浠躲€?杩斿洖鍊肩殑绗竴椤逛唬琛ㄦ墽琛岀粨鏋滐紝涓虹湡鏃跺鍣ㄨ鎴愬姛鎵撳紑锛屽惁鍒欏弽涔嬨€?
瀹瑰櫒涓嶄竴瀹氭€昏兘鎵撳紑锛屽彲鑳借瀹瑰櫒宸茶绉婚櫎鎴栨満鍣ㄤ汉宸茶绉诲姩銆?鍥犳锛屽崟娆℃墦寮€鎿嶄綔鍦ㄦ姷杈炬渶闀挎埅姝㈡椂闂村悗灏嗕細鍦ㄥ唴閮ㄨ楠岃瘉涓鸿秴鏃讹紝
姝ゆ椂灏嗕細閲嶆柊鎻愪氦涓€娆″鍣ㄦ墦寮€鎿嶄綔锛?鐩村埌鎬绘搷浣滄鏁版姷杈?ContainerOperationsReTryMaximumCounts 鏃舵銆?
璇风‘淇濆湪浣跨敤姝ゅ嚱鏁板墠鍗犵敤浜嗗鍣ㄨ祫婧愶紝鍚﹀垯浼氶€犳垚绋嬪簭 panic
*/
func (g *GameInterface) OpenContainer(
	pos [3]int32,
	blockName string,
	blockStates map[string]interface{},
	hotBarSlotID uint8,
) (bool, error) {
	if g.Resources.Container.GetContainerOpeningData() != nil {
		return false, ErrContainerHasBeenOpened
	}
	// if the container has been opened
	for i := 0; i < ContainerOperationsReTryMaximumCounts; i++ {
		g.Resources.Container.AwaitChangesBeforeSendingPacket()
		// await responce before send packet
		err := g.ClickBlock(
			UseItemOnBlocks{
				HotbarSlotID: hotBarSlotID,
				BlockPos:     pos,
				BlockName:    blockName,
				BlockStates:  blockStates,
			},
		)
		if err != nil {
			return false, fmt.Errorf("OpenContainer: %v", err)
		}
		// open container
		g.Resources.Container.AwaitChangesAfterSendingPacket()
		// wait changes
		if g.Resources.Container.GetContainerOpeningData() == nil {
			continue
		}
		// if unsuccess
		return true, nil
		// return
	}
	// open container.
	// try a maximum of ContainerOperationsReTryMaximumCounts times
	return false, nil
	// return
}

/*
鍏抽棴宸茬粡鎵撳紑鐨勫鍣紝涓斿彧鏈夊綋瀹瑰櫒琚叧闂悗鎵嶄細杩斿洖鍊笺€?鎮ㄥ簲璇ョ‘淇濆鍣ㄨ鍏抽棴鍚庯紝瀵瑰簲鐨勫鍣ㄥ叕鐢ㄨ祫婧愯閲婃斁銆?
杩斿洖鍊肩殑绗竴椤逛唬琛ㄦ墽琛岀粨鏋滐紝涓虹湡鏃跺鍣ㄨ鎴愬姛鍏抽棴锛屽惁鍒欏弽涔嬨€?
瀹瑰櫒涓嶄竴瀹氭€昏兘鍏抽棴锛屽彲鑳界璧佹湇宸茬粡鍗℃銆?鍥犳锛屽崟娆″叧闂搷浣滃湪鎶佃揪鏈€闀挎埅姝㈡椂闂村悗灏嗕細鍦ㄥ唴閮ㄨ楠岃瘉涓鸿秴鏃讹紝
姝ゆ椂灏嗕細閲嶆柊鎻愪氦涓€娆″鍣ㄦ墦寮€鎿嶄綔锛?鐩村埌鎬绘搷浣滄鏁版姷杈?ContainerOperationsReTryMaximumCounts 鏃舵銆?*/
func (g *GameInterface) CloseContainer() (bool, error) {
	if g.Resources.Container.GetContainerOpeningData() == nil {
		return false, ErrContainerNerverOpened
	}
	// if the container is not opened
	for i := 0; i < ContainerOperationsReTryMaximumCounts; i++ {
		g.Resources.Container.AwaitChangesBeforeSendingPacket()
		// await responce before send packet
		err := g.WritePacket(&packet.ContainerClose{
			WindowID:   g.Resources.Container.GetContainerOpeningData().WindowID,
			ServerSide: false,
		})
		if err != nil {
			return false, fmt.Errorf("CloseContainer: %v", err)
		}
		// close container
		g.Resources.Container.AwaitChangesAfterSendingPacket()
		// wait changes
		if g.Resources.Container.GetContainerClosingData() == nil {
			continue
		}
		// if unsuccess
		return true, nil
		// return
	}
	// close container.
	// try a maximum of ContainerOperationsReTryMaximumCounts times
	return false, nil
	// return
}

