package operation

import (
	chunk_define "github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/define"
)

func (b *BedrockWorld) LoadDeltaUpdate(dm chunk_define.Dimension, position chunk_define.ChunkPos) ([]byte, error) {
	key := define.Sum(dm, position, []byte(define.KeyDeltaUpdate)...)
	data, err := b.Get(key)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (b *BedrockWorld) SaveDeltaUpdate(dm chunk_define.Dimension, position chunk_define.ChunkPos, payload []byte) error {
	key := define.Sum(dm, position, []byte(define.KeyDeltaUpdate)...)
	if len(payload) == 0 {
		return b.Delete(key)
	}
	return b.Put(key, payload)
}
