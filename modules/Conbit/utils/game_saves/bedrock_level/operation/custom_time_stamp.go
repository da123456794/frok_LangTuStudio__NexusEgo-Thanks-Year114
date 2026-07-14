package operation

import (
	"encoding/binary"

	chunk_define "github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/define"
)

func (b *BedrockWorld) loadTimeStampByKey(dm chunk_define.Dimension, position chunk_define.ChunkPos, key ...byte) int64 {
	keyBytes := define.Sum(dm, position, key...)
	data, err := b.Get(keyBytes)
	if err != nil || len(data) == 0 {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(data))
}

func (b *BedrockWorld) LoadDeltaUpdateTimeStamp(dm chunk_define.Dimension, position chunk_define.ChunkPos) int64 {
	return b.loadTimeStampByKey(dm, position, []byte(define.KeyDeltaUpdateTimeStamp)...)
}

func (b *BedrockWorld) LoadTimeStamp(dm chunk_define.Dimension, position chunk_define.ChunkPos) int64 {
	return b.loadTimeStampByKey(dm, position, define.KeyChunkTimeStamp)
}

func (b *BedrockWorld) saveTimeStampByKey(dm chunk_define.Dimension, position chunk_define.ChunkPos, timeStamp int64, key ...byte) error {
	keyBytes := define.Sum(dm, position, key...)
	if timeStamp == 0 {
		return b.Delete(keyBytes)
	}
	timeStampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timeStampBytes, uint64(timeStamp))
	return b.Put(keyBytes, timeStampBytes)
}

func (b *BedrockWorld) SaveTimeStamp(dm chunk_define.Dimension, position chunk_define.ChunkPos, timeStamp int64) error {
	return b.saveTimeStampByKey(dm, position, timeStamp, define.KeyChunkTimeStamp)
}

func (b *BedrockWorld) SaveDeltaUpdateTimeStamp(dm chunk_define.Dimension, position chunk_define.ChunkPos, timeStamp int64) error {
	return b.saveTimeStampByKey(dm, position, timeStamp, []byte(define.KeyDeltaUpdateTimeStamp)...)
}
