package operation

import (
	"encoding/binary"

	"github.com/LangTuStudio/Conbit/Conbit/blocks"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	chunk_define "github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/define"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/marshal"
)

func (b *BedrockWorld) LoadSubChunk(dm chunk_define.Dimension, position protocol.SubChunkPos) *chunk.SubChunk {
	chunkPos := chunk_define.ChunkPos{position[0], position[2]}
	keyBytes := define.Sum(
		dm, chunkPos,
		define.KeySubChunkData, byte(position[1]),
	)

	r := dm.RangeUpperInclude()
	if position[1] < int32(r[0]>>4) || position[1] > int32(r[1]>>4) {
		return nil
	}

	subChunkData, _ := b.Get(keyBytes)
	if len(subChunkData) == 0 {
		has, err := b.Has(define.Sum(dm, chunkPos, define.KeyVersion))
		if err == nil && !has {
			has, err = b.Has(define.Sum(dm, chunkPos, define.KeyVersionOld))
		}
		if err == nil && has {
			return chunk.NewSubChunk(blocks.NEMC_AIR_RUNTIMEID)
		}
		return nil
	}

	subChunk, _, err := marshal.DecodeSubChunkBlocksNoNBT(subChunkData, r)
	if err != nil {
		return nil
	}
	return subChunk
}

func (b *BedrockWorld) SaveSubChunk(dm chunk_define.Dimension, position protocol.SubChunkPos, c *chunk.SubChunk) error {
	chunkPos := chunk_define.ChunkPos{position[0], position[2]}
	subChunkKey := define.Sum(dm, chunkPos, define.KeySubChunkData, byte(position[1]))
	if c == nil {
		return b.Delete(subChunkKey)
	}

	r := dm.RangeUpperInclude()
	finalisation := make([]byte, 4)
	binary.LittleEndian.PutUint32(finalisation, define.FinalisationGenerated)
	_ = b.Put(
		define.Sum(dm, chunkPos, define.KeyVersion),
		[]byte{define.ChunkVersion},
	)
	_ = b.Put(
		define.Sum(dm, chunkPos, define.KeyFinalisation),
		finalisation,
	)

	fixedYPos := (position[1]<<4 - int32(r[0])) >> 4
	subChunkData := marshal.EncodeSubChunkBlocksNoNBT(c, r, int(fixedYPos))
	return b.Put(subChunkKey, subChunkData)
}
