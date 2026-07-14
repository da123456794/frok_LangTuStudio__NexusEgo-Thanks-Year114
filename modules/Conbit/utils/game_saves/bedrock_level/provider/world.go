package provider

import (
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation"
)

type HashWithPosY = operation.HashWithPosY

type IterAllFunc = operation.IterAllFunc

type World interface {
	Close() error
	CloseWorld() error
	Delete(key []byte) error
	Get(key []byte) ([]byte, error)
	Has(key []byte) (bool, error)
	IterAll(fn IterAllFunc) error
	LevelDat() *Data
	LoadChunk(dimension define.Dimension, pos define.ChunkPos) (*chunk.Chunk, bool, error)
	LoadChunkPayloadOnly(dimension define.Dimension, pos define.ChunkPos) ([][]byte, bool, error)
	LoadDeltaUpdate(dimension define.Dimension, pos define.ChunkPos) ([]byte, error)
	LoadDeltaUpdateTimeStamp(dimension define.Dimension, pos define.ChunkPos) int64
	LoadFullSubChunkBlobHash(dimension define.Dimension, pos define.ChunkPos) []HashWithPosY
	LoadNBT(dimension define.Dimension, pos define.ChunkPos) ([]map[string]interface{}, error)
	LoadNBTPayloadOnly(dimension define.Dimension, pos define.ChunkPos) []byte
	LoadSubChunk(dimension define.Dimension, pos protocol.SubChunkPos) *chunk.SubChunk
	LoadSubChunkBlobHash(dimension define.Dimension, pos protocol.SubChunkPos) (uint64, bool)
	LoadTimeStamp(dimension define.Dimension, pos define.ChunkPos) int64
	Put(key []byte, value []byte) error
	SaveChunk(dimension define.Dimension, pos define.ChunkPos, c *chunk.Chunk) error
	SaveChunkPayloadOnly(dimension define.Dimension, pos define.ChunkPos, payload [][]byte) error
	SaveDeltaUpdate(dimension define.Dimension, pos define.ChunkPos, payload []byte) error
	SaveDeltaUpdateTimeStamp(dimension define.Dimension, pos define.ChunkPos, ts int64) error
	SaveFullSubChunkBlobHash(dimension define.Dimension, pos define.ChunkPos, hashes []HashWithPosY) error
	SaveNBT(dimension define.Dimension, pos define.ChunkPos, nbtData []map[string]interface{}) error
	SaveNBTPayloadOnly(dimension define.Dimension, pos define.ChunkPos, payload []byte) error
	SaveSubChunk(dimension define.Dimension, pos protocol.SubChunkPos, c *chunk.SubChunk) error
	SaveSubChunkBlobHash(dimension define.Dimension, pos protocol.SubChunkPos, hash uint64) error
	SaveTimeStamp(dimension define.Dimension, pos define.ChunkPos, ts int64) error
	UpdateLevelDat() error
}
