package operation

import (
	"bytes"
	"fmt"

	chunk_define "github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/nbt"
	"github.com/LangTuStudio/Conbit/utils/game_saves/bedrock_level/operation/define"
)

func (b *BedrockWorld) LoadNBTPayloadOnly(dm chunk_define.Dimension, position chunk_define.ChunkPos) []byte {
	key := define.Sum(dm, position, define.KeyBlockEntities)
	data, err := b.Get(key)
	if err != nil {
		return nil
	}
	return data
}

func (b *BedrockWorld) LoadNBT(dm chunk_define.Dimension, position chunk_define.ChunkPos) ([]map[string]any, error) {
	data := b.LoadNBTPayloadOnly(dm, position)
	if len(data) == 0 {
		return make([]map[string]any, 0), nil
	}

	var result []map[string]any
	buf := bytes.NewBuffer(data)
	dec := nbt.NewDecoderWithEncoding(buf, nbt.LittleEndian)

	for buf.Len() != 0 {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("decode nbt: %w", err)
		}
		result = append(result, m)
	}
	return result, nil
}

func (b *BedrockWorld) SaveNBTPayloadOnly(dm chunk_define.Dimension, position chunk_define.ChunkPos, data []byte) error {
	key := define.Sum(dm, position, define.KeyBlockEntities)
	if len(data) == 0 {
		return b.Delete(key)
	}
	return b.Put(key, data)
}

func (b *BedrockWorld) SaveNBT(dm chunk_define.Dimension, position chunk_define.ChunkPos, data []map[string]any) error {
	buf := bytes.NewBuffer(nil)
	enc := nbt.NewEncoderWithEncoding(buf, nbt.LittleEndian)
	for _, d := range data {
		if err := enc.Encode(d); err != nil {
			return fmt.Errorf("store block entities: encode nbt: %w", err)
		}
	}
	return b.SaveNBTPayloadOnly(dm, position, buf.Bytes())
}
