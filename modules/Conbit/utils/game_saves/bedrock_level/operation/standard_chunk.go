package operation

import (
	"encoding/binary"
	"fmt"

	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	chunk_define "github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/define"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/marshal"
)

type IterAllFunc func(key, value []byte) bool

type DB interface {
	Close() error
	Delete(key []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	IterAll(fn IterAllFunc) error
	Put(key []byte, value []byte) error
}

type BedrockWorld struct {
	DB
}

func (b *BedrockWorld) Close() error {
	return b.DB.Close()
}

func (b *BedrockWorld) Delete(key []byte) error {
	return b.DB.Delete(key)
}

func (b *BedrockWorld) Get(key []byte) ([]byte, error) {
	return b.DB.Get(key)
}

func (b *BedrockWorld) Has(key []byte) (bool, error) {
	return b.DB.Has(key)
}

func (b *BedrockWorld) Put(key []byte, value []byte) error {
	return b.DB.Put(key, value)
}

func (b *BedrockWorld) IterAll(fn IterAllFunc) error {
	return b.DB.IterAll(fn)
}

func (b *BedrockWorld) LoadBiomes(dm chunk_define.Dimension, position chunk_define.ChunkPos) ([]byte, error) {
	biomes, err := b.Get(define.Sum(dm, position, define.Key3DData))
	if err != nil {
		return nil, err
	}
	if len(biomes) == 0 {
		return nil, nil
	}
	if n := len(biomes); n <= 512 {
		return nil, fmt.Errorf("expected at least 513 bytes for 3D data, got %v", n)
	}
	return biomes[512:], nil
}

func (b *BedrockWorld) SaveBiomes(dm chunk_define.Dimension, position chunk_define.ChunkPos, payload []byte) error {
	key := define.Sum(dm, position, define.Key3DData)
	if len(payload) == 0 {
		return b.Delete(key)
	}
	return b.Put(key, append(make([]byte, 512), payload...))
}

func (b *BedrockWorld) LoadChunkPayloadOnly(dm chunk_define.Dimension, position chunk_define.ChunkPos) ([][]byte, bool, error) {
	r := dm.RangeUpperInclude()
	height := r.Height() + 1
	subchunksBytes := make([][]byte, height>>4)

	has, err := b.Has(define.Sum(dm, position, define.KeyVersion))
	if err == nil && !has {
		has, err = b.Has(define.Sum(dm, position, define.KeyVersionOld))
		if err == nil && !has {
			return nil, false, nil
		}
	} else if err != nil {
		return nil, true, fmt.Errorf("error reading version: %w", err)
	}

	for i := range subchunksBytes {
		subchunksBytes[i], err = b.Get(
			define.Sum(
				dm, position,
				define.KeySubChunkData, uint8(i+(r.Min()>>4)),
			),
		)
		if len(subchunksBytes[i]) == 0 && err == nil {
			continue
		} else if err != nil {
			return nil, true, fmt.Errorf("sub chunk %v: %w", i, err)
		}
	}

	return subchunksBytes, true, err
}

func (b *BedrockWorld) LoadChunk(dm chunk_define.Dimension, position chunk_define.ChunkPos) (*chunk.Chunk, bool, error) {
	subchunksBytes, exists, err := b.LoadChunkPayloadOnly(dm, position)
	if !exists || err != nil {
		return nil, exists, err
	}
	_, _ = b.LoadBiomes(dm, position)
	c, err := marshal.DecodeChunkBlocksNoNBT(subchunksBytes, dm.RangeUpperInclude())
	return c, true, err
}

func (b *BedrockWorld) SaveChunkPayloadOnly(dm chunk_define.Dimension, position chunk_define.ChunkPos, subchunksBytes [][]byte) error {
	_ = b.Put(
		define.Sum(dm, position, define.KeyVersion),
		[]byte{define.ChunkVersion},
	)

	finalisation := make([]byte, 4)
	binary.LittleEndian.PutUint32(finalisation, define.FinalisationGenerated)
	_ = b.Put(
		define.Sum(dm, position, define.KeyFinalisation),
		finalisation,
	)

	for i, sub := range subchunksBytes {
		if len(sub) == 0 {
			_ = b.Delete(
				define.Sum(
					dm, position,
					define.KeySubChunkData, byte(i+(dm.RangeUpperInclude().Min()>>4)),
				),
			)
			continue
		}
		_ = b.Put(
			define.Sum(
				dm, position,
				define.KeySubChunkData, byte(i+(dm.RangeUpperInclude().Min()>>4)),
			),
			sub,
		)
	}
	return nil
}

func (b *BedrockWorld) SaveChunk(dm chunk_define.Dimension, position chunk_define.ChunkPos, c *chunk.Chunk) error {
	if c == nil {
		return nil
	}
	subchunksBytes, err := marshal.EncodeChunkBlocksNoNBT(c)
	if err != nil {
		return fmt.Errorf("SaveChunk: %v", err)
	}

	err = b.SaveChunkPayloadOnly(dm, position, subchunksBytes)
	if err != nil {
		return fmt.Errorf("SaveChunk: %v", err)
	}
	return nil
}
