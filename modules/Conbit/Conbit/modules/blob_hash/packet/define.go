package blob_hash_packet

import "github.com/LangTuStudio/Conbit/minecraft/protocol"

const (
	DimensionOverWorld uint8 = iota
	DimensionNether
	DimensionTheEnd
)

// Hash 指示 blob hash 的值，
// 它是通过对数据荷载的 xxHash 得到的
type Hash uint64

// HashWithPosY ..
type HashWithPosY struct {
	Hash Hash
	PosY int8
}

// HashWithPosition ..
type HashWithPosition struct {
	Hash        Hash
	SubChunkPos protocol.SubChunkPos
	Dimension   uint8
}

func (h *HashWithPosition) Marshal(io protocol.IO) {
	io.Uint64((*uint64)(&h.Hash))
	io.Int32(&h.SubChunkPos[0])
	io.Int32(&h.SubChunkPos[1])
	io.Int32(&h.SubChunkPos[2])
	io.Uint8(&h.Dimension)
}

// PayloadByHash ..
type PayloadByHash struct {
	Hash    HashWithPosition
	Payload []byte
}

func (h *PayloadByHash) Marshal(io protocol.IO) {
	protocol.Single(io, &h.Hash)
	protocol.FuncSliceUint32Length(io, &h.Payload, io.Uint8)
}
