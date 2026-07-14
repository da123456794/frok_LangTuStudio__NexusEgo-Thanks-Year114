package bedrock_level

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LangTuStudio/Conbit/Conbit/chunks"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	bedrock_level_provider "github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/provider"
	"github.com/df-mc/goleveldb/leveldb/opt"
)

type ProviderInterface interface {
	Close(force bool) error
	Get(dimension define.Dimension, pos define.ChunkPos) *chunks.DimensiondChunkWithAuxInfo
	LevelDat() *Data
	LoadChunk(dimension define.Dimension, pos define.ChunkPos) (*chunk.Chunk, bool, error)
	LoadChunkPayloadOnly(dimension define.Dimension, pos define.ChunkPos) ([][]byte, bool, error)
	LoadDeltaUpdate(dimension define.Dimension, pos define.ChunkPos) ([]byte, error)
	LoadDeltaUpdateTimeStamp(dimension define.Dimension, pos define.ChunkPos) int64
	LoadFullSubChunkBlobHash(dimension define.Dimension, pos define.ChunkPos) []blob_hash_packet.HashWithPosY
	LoadNBT(dimension define.Dimension, pos define.ChunkPos) ([]map[string]interface{}, error)
	LoadNBTPayloadOnly(dimension define.Dimension, pos define.ChunkPos) []byte
	LoadSubChunk(dimension define.Dimension, pos protocol.SubChunkPos) *chunk.SubChunk
	LoadSubChunkBlobHash(dimension define.Dimension, pos protocol.SubChunkPos) (uint64, bool)
	LoadTimeStamp(dimension define.Dimension, pos define.ChunkPos) int64
	Save(chunk *chunks.DimensiondChunkWithAuxInfo) error
	SaveAuxInfo() error
	SaveChunk(dimension define.Dimension, pos define.ChunkPos, c *chunk.Chunk) error
	SaveChunkPayloadOnly(dimension define.Dimension, pos define.ChunkPos, payload [][]byte) error
	SaveDeltaUpdate(dimension define.Dimension, pos define.ChunkPos, payload []byte) error
	SaveDeltaUpdateTimeStamp(dimension define.Dimension, pos define.ChunkPos, ts int64) error
	SaveFullSubChunkBlobHash(dimension define.Dimension, pos define.ChunkPos, hashes []blob_hash_packet.HashWithPosY) error
	SaveNBT(dimension define.Dimension, pos define.ChunkPos, nbtData []map[string]interface{}) error
	SaveNBTPayloadOnly(dimension define.Dimension, pos define.ChunkPos, payload []byte) error
	SaveSubChunk(dimension define.Dimension, pos protocol.SubChunkPos, c *chunk.SubChunk) error
	SaveSubChunkBlobHash(dimension define.Dimension, pos protocol.SubChunkPos, hash uint64) error
	SaveTimeStamp(dimension define.Dimension, pos define.ChunkPos, ts int64) error
}

func New(dir string, compression opt.Compression, readOnly bool, options *opt.Options) (ProviderInterface, error) {
	_ = os.MkdirAll(filepath.Join(dir, "db"), 0o777)

	if options == nil {
		options = &opt.Options{
			Compression: compression,
		}
	}
	if readOnly {
		options.ReadOnly = true
	}

	bedrockWorld, err := OpenWorld(dir, options)
	if err != nil {
		return nil, fmt.Errorf("error opening world: %w", err)
	}

	return &Provider{
		BedrockWorld: bedrockWorld,
	}, nil
}

var OpenWorld = bedrock_level_provider.OpenWorld

func CheckIsMCDBDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "db")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "level.dat")); err != nil {
		return false
	}
	return true
}

func MCDBDirRedirect(dir string) string {
	if CheckIsMCDBDir(dir) {
		return dir
	}
	_ = filepath.Walk(dir, func(path string, _ os.FileInfo, _ error) error {
		if CheckIsMCDBDir(path) {
			fmt.Fprintln(os.Stdout, "re-target path to "+path)
			dir = path
		}
		return nil
	})
	return dir
}
