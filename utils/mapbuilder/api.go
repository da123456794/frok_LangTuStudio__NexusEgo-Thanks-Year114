package mapbuilder

import "image/color"

// BlockPos 表示方块坐标。
type BlockPos [3]int32

// SubChunkPos 表示子区块坐标（X/Z 为区块坐标，Y 为子区块索引）。
type SubChunkPos [3]int32

// SubChunkOffset 表示相对中心子区块的偏移。
type SubChunkOffset [3]int8

// SubChunkEntry 表示子区块数据条目。
type SubChunkEntry struct {
	Offset     SubChunkOffset
	Result     byte
	RawPayload []byte
}

// SubChunkResponse 表示子区块响应数据。
type SubChunkResponse struct {
	Position        SubChunkPos
	SubChunkEntries []SubChunkEntry
}

// PixelRequest 表示地图像素更新请求。
type PixelRequest struct {
	Colour color.RGBA
	Index  uint16
}

const (
	SubChunkResultSuccess byte = iota + 1
	SubChunkResultChunkNotFound
	SubChunkResultInvalidDimension
	SubChunkResultPlayerNotFound
	SubChunkResultIndexOutOfBounds
	SubChunkResultSuccessAllAir
)

// MapAPI 约束地图播放器与不同版本连接的最小能力集合。
type MapAPI interface {
	LockMap(mapID int64) error
	SendMapPixels(mapID int64, pixels []PixelRequest) error
	GetSubChunksInArea(dimension int32, start, end BlockPos) (*SubChunkResponse, error)
	Dimension() (int32, bool)
}

// StructureNBTAPI 是可选扩展：走 structure save + 结构包请求拿完整方块实体 NBT，
// 不受客户端可见性限制。
type StructureNBTAPI interface {
	RequestStructureNBTs(start, end BlockPos) (map[[3]int32]map[string]interface{}, error)
}
