package marshal

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/LangTuStudio/Conbit/Conbit/blocks"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/nbt"
)

func EncodeSubChunkBlocksNoNBT(sub *chunk.SubChunk, r define.Range, index int) []byte {
	if sub == nil {
		return nil
	}

	buf := bytes.NewBuffer(nil)
	storageCount := byte(len(sub.Storages))
	subChunkIndex := byte(index + (r.Min() >> 4))
	_, _ = buf.Write([]byte{0x9, storageCount, subChunkIndex})

	for _, storage := range sub.Storages {
		encodePalettedStorage(buf, storage)
	}

	return buf.Bytes()
}

func EncodeChunkBlocksNoNBT(c *chunk.Chunk) ([][]byte, error) {
	subs := c.Sub()
	output := make([][]byte, len(subs))
	for i, sub := range subs {
		if sub == nil {
			return nil, fmt.Errorf("EncodeChunkBlocksNoNBT: c.sub()[%d] = nil, c.sub() = %v", i, subs)
		}
		output[i] = EncodeSubChunkBlocksNoNBT(sub, c.Range(), i)
	}
	return output, nil
}

func encodePalettedStorage(buf *bytes.Buffer, storage *chunk.PalettedStorage) {
	indices := storage.Indices()
	b := make([]byte, len(indices)*4+1)
	b[0] = byte(storage.BitsPerIndex()) << 1

	for i, v := range indices {
		b[i*4+1], b[i*4+2], b[i*4+3], b[i*4+4] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24)
	}
	_, _ = buf.Write(b)

	encodePalette(buf, storage.Palette())
}

func encodePalette(buf *bytes.Buffer, palette *chunk.Palette) {
	if palette.Size != 0 {
		_ = binary.Write(buf, binary.LittleEndian, uint32(len(palette.Values)))
	}
	for _, runtimeID := range palette.Values {
		name, states, _ := blocks.RuntimeIDToState(runtimeID)
		block := paletteBlock{
			Name:    name,
			States:  states,
			Version: blockStateVersion,
		}
		_ = nbt.NewEncoderWithEncoding(buf, nbt.LittleEndian).Encode(block)
	}
}
