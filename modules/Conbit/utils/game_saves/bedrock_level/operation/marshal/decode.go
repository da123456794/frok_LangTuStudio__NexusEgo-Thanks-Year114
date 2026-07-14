package marshal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/LangTuStudio/Conbit/Conbit/blocks"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/nbt"
)

func DecodeSubChunkBlocksNoNBT(data []byte, r define.Range) (*chunk.SubChunk, int, error) {
	buf := bytes.NewBuffer(data)
	ver, err := buf.ReadByte()
	if err != nil {
		return nil, -1, fmt.Errorf("error reading version: %w", err)
	}

	sub := chunk.NewSubChunk(blocks.NEMC_AIR_RUNTIMEID)
	switch ver {
	case 1:
		storage, err := decodePalettedStorage(buf)
		if err != nil {
			return nil, -1, err
		}
		sub.Storages = append(sub.Storages, storage)
		return sub, 0, nil
	case 8, 9:
		storageCount, err := buf.ReadByte()
		if err != nil {
			return nil, -1, fmt.Errorf("error reading storage count: %w", err)
		}
		index := 0
		if ver == 9 {
			uIndex, err := buf.ReadByte()
			if err != nil {
				return nil, -1, fmt.Errorf("error reading subchunk index: %w", err)
			}
			index = int(int8(uIndex) - int8(r.Min()>>4))
		}
		sub.Storages = make([]*chunk.PalettedStorage, storageCount)
		for i := byte(0); i < storageCount; i++ {
			storage, err := decodePalettedStorage(buf)
			if err != nil {
				return nil, -1, err
			}
			sub.Storages[i] = storage
		}
		return sub, index, nil
	default:
		return nil, -1, fmt.Errorf("unknown sub chunk version %v: can't decode", ver)
	}
}

func DecodeChunkBlocksNoNBT(subChunks [][]byte, r define.Range) (*chunk.Chunk, error) {
	c := chunk.New(blocks.NEMC_AIR_RUNTIMEID, r)
	for _, subData := range subChunks {
		if len(subData) == 0 {
			continue
		}
		sub, index, err := DecodeSubChunkBlocksNoNBT(subData, r)
		if err != nil {
			return c, err
		}
		c.AssignSub(index, sub)
	}
	return c, nil
}

func decodePalettedStorage(buf *bytes.Buffer) (*chunk.PalettedStorage, error) {
	blockSize, err := buf.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("error reading block size: %w", err)
	}
	blockSize >>= 1
	if blockSize == 0x7f {
		return nil, nil
	}

	size := chunk.PaletteSize(blockSize)
	uint32Count := size.Uint32s()
	uint32s := make([]uint32, uint32Count)
	byteCount := uint32Count * 4

	data := buf.Next(byteCount)
	if len(data) != byteCount {
		return nil, fmt.Errorf(
			"cannot read paletted storage (size=%v) not enough block data present: expected %v bytes, got %v",
			blockSize,
			byteCount,
			len(data),
		)
	}
	for i := 0; i < uint32Count; i++ {
		uint32s[i] = uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
	}
	palette, err := decodePalette(buf, size)
	return chunk.NewPalettedStorage(uint32s, palette), err
}

func decodePalette(buf *bytes.Buffer, blockSize chunk.PaletteSize) (*chunk.Palette, error) {
	paletteCount := uint32(1)
	if blockSize != 0 {
		if err := binary.Read(buf, binary.LittleEndian, &paletteCount); err != nil {
			return nil, fmt.Errorf("error reading palette entry count: %w", err)
		}
	}

	palette := chunk.NewPalette(blockSize, make([]uint32, paletteCount))
	for i := uint32(0); i < paletteCount; i++ {
		value, err := decodePaletteBlock(buf)
		palette.Values[i] = value
		if err != nil {
			_, _ = fmt.Fprintf(os.Stdout, "decode bedrock level db palette error: %v", err)
		}
	}
	return palette, nil
}

func decodePaletteBlock(buf *bytes.Buffer) (uint32, error) {
	var block paletteBlock
	dec := nbt.NewDecoderWithEncoding(buf, nbt.LittleEndian)
	if err := dec.Decode(&block); err != nil {
		return 0, fmt.Errorf("error decoding block palette entry: %w", err)
	}
	runtimeID, _ := blocks.BlockNameAndStateToRuntimeID(block.Name, block.States)
	return runtimeID, nil
}
