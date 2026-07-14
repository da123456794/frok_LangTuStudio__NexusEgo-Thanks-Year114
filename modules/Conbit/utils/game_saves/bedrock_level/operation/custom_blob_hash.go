package operation

import (
	"encoding/binary"

	chunk_define "github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/define"
)

type HashWithPosY struct {
	Hash uint64
	PosY int8
}

func (b *BedrockWorld) LoadFullSubChunkBlobHash(dm chunk_define.Dimension, position chunk_define.ChunkPos) []HashWithPosY {
	result := make([]HashWithPosY, 0)

	key := define.Sum(dm, position, []byte(define.KeyBlobHash)...)
	data, err := b.Get(key)
	if err != nil || len(data) == 0 {
		return nil
	}

	for len(data) > 0 {
		result = append(result, HashWithPosY{
			Hash: binary.LittleEndian.Uint64(data[1:9]),
			PosY: int8(data[0]),
		})
		data = data[9:]
	}

	return result
}

func (b *BedrockWorld) LoadSubChunkBlobHash(dm chunk_define.Dimension, position protocol.SubChunkPos) (uint64, bool) {
	key := define.Sum(dm, chunk_define.ChunkPos{position[0], position[2]}, []byte(define.KeyBlobHash)...)
	data, err := b.Get(key)
	if err != nil || len(data) == 0 {
		return 0, false
	}

	for len(data) > 0 {
		if int8(data[0]) == int8(position[1]) {
			return binary.LittleEndian.Uint64(data[1:9]), true
		}
		data = data[9:]
	}

	return 0, false
}

func (b *BedrockWorld) SaveFullSubChunkBlobHash(dm chunk_define.Dimension, position chunk_define.ChunkPos, newHash []HashWithPosY) error {
	key := define.Sum(dm, position, []byte(define.KeyBlobHash)...)
	data := make([]byte, 0)

	for _, value := range newHash {
		hashBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(hashBytes, uint64(value.Hash))
		data = append(data, uint8(value.PosY))
		data = append(data, hashBytes...)
	}

	if len(data) == 0 {
		return b.Delete(key)
	}
	return b.Put(key, data)
}

func (b *BedrockWorld) SaveSubChunkBlobHash(dm chunk_define.Dimension, position protocol.SubChunkPos, hash uint64) error {
	diskHasHash := false
	modified := make([]byte, 0)

	hashByte := make([]byte, 8)
	binary.LittleEndian.PutUint64(hashByte, hash)

	key := define.Sum(dm, chunk_define.ChunkPos{position[0], position[2]}, []byte(define.KeyBlobHash)...)
	data, err := b.Get(key)

	if err == nil && len(data) != 0 {
		for len(data) > 0 {
			if int8(data[0]) == int8(position[1]) {
				modified = append(modified, data[0])
				modified = append(modified, hashByte...)
				diskHasHash = true
			} else {
				modified = append(modified, data[0:9]...)
			}
			data = data[9:]
		}
	}

	if !diskHasHash {
		modified = append(modified, uint8(position[1]))
		modified = append(modified, hashByte...)
	}

	if len(modified) == 0 {
		return b.Delete(key)
	}
	return b.Put(key, modified)
}
