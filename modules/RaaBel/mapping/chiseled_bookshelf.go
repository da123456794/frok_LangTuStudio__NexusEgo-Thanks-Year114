package mapping

import "github.com/go-gl/mathgl/mgl32"

// 此表描述了雕纹书架方块状态中 direction 字段到 PlaceBlock 中 BlockFace 字段的映射。
var ChiseledBookshelfDirectionToBlockFace = map[int32]int32{
	0: 3,
	1: 4,
	2: 2,
	3: 5,
}

// 此表描述了雕纹书架方块状态中 direction 字段和书在雕纹书架中的卡槽 ID
// 到 UseItemOnBlocks 中 ClickedPosition 字段的映射。
var ChiseledBookshelfDirectionAndBookSlotIDToClickedPosition = map[int32]map[uint8]mgl32.Vec3{
	0: {
		0: {0, 1, 0},
		1: {0.5, 1, 0},
		2: {1, 1, 0},
		3: {0, 0, 0},
		4: {0.5, 0, 0},
		5: {1, 0, 0},
	},
	1: {
		0: {0, 1, 0},
		1: {0, 1, 0.5},
		2: {0, 1, 1},
		3: {0, 0, 0},
		4: {0, 0, 0.5},
		5: {0, 0, 1},
	},
	2: {
		0: {1, 1, 0},
		1: {0.5, 1, 0},
		2: {0, 1, 0},
		3: {1, 0, 0},
		4: {0.5, 0, 0},
		5: {0, 0, 1},
	},
	3: {
		0: {0, 1, 1},
		1: {0, 1, 0.5},
		2: {0, 1, 0},
		3: {0, 0, 1},
		4: {0, 0, 0.5},
		5: {0, 0, 0},
	},
}
