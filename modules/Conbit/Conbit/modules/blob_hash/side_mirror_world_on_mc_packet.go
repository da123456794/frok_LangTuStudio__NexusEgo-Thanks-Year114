package blob_hash

import (
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

// onSubChunk 从 pk 中检索全为空气的条目，
// 这意味着该条目的结果是 protocol.SubChunkResultSuccessAllAir。
// 然后，将检索结果应用到镜像存档，因为全为空气的条目不存在 blob cache
func (b *BlobHashMirrorWorldSide) onSubChunk(pk *packet.SubChunk) {
	if !pk.CacheEnabled || !b.bbhh.isDiskHolder {
		return
	}

	pos := make([]blob_hash_packet.HashWithPosition, 0)

	for _, value := range pk.SubChunkEntries {
		if value.Result == protocol.SubChunkResultSuccessAllAir {
			pos = append(pos, blob_hash_packet.HashWithPosition{
				SubChunkPos: protocol.SubChunkPos{
					pk.Position[0] + int32(value.Offset[0]),
					pk.Position[1] + int32(value.Offset[1]),
					pk.Position[2] + int32(value.Offset[2]),
				},
				Dimension: uint8(pk.Dimension),
			})
		}
	}

	if len(pos) > 0 && b.bbhh.handler.handleCleanBlobHashAndApplyToWorld != nil {
		b.bbhh.handler.handleCleanBlobHashAndApplyToWorld(pos)
	}
}

// onLevelChunk 从 pk 中检索全为空气的条目，
// 然后，将检索结果应用到镜像存档，因为全为空气的条目不存在 blob cache
func (b *BlobHashMirrorWorldSide) onLevelChunk(pk *packet.LevelChunk) {
	if pk.SubChunkCount != protocol.SubChunkRequestModeLimited || !b.bbhh.isDiskHolder {
		return
	}

	pos := make([]blob_hash_packet.HashWithPosition, 0)
	r := define.Dimension(pk.Dimension).RangeUpperExclude()
	subChunkPosStartY := int(pk.HighestSubChunk) + (r[0] >> 4) + 1
	subChunkPosEndY := r[1] >> 4

	for i := subChunkPosStartY; i <= subChunkPosEndY; i++ {
		pos = append(pos, blob_hash_packet.HashWithPosition{
			SubChunkPos: protocol.SubChunkPos{
				pk.Position[0],
				int32(i),
				pk.Position[1],
			},
			Dimension: uint8(pk.Dimension),
		})
	}

	if len(pos) > 0 && b.bbhh.handler.handleCleanBlobHashAndApplyToWorld != nil {
		b.bbhh.handler.handleCleanBlobHashAndApplyToWorld(pos)
	}
}
