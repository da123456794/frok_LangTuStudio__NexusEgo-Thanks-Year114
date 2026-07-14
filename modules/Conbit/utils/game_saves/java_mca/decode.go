package java_mca

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/LangTuStudio/Conbit/Conbit/chunks"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/minecraft/nbt"
)

var (
	ErrNoSector             = errors.New("no sector")
	ErrNoData               = errors.New("no data")
	ErrTooLarge             = errors.New("chunk data too large")
	ErrSectorNegativeLength = errors.New("sector negative length")
)

type BiomeState string

type PaletteContainer[T any] struct {
	Palette []T     `nbt:"palette"`
	Data    []int64 `nbt:"data"`
}

type BlockState struct {
	Name       string
	Properties map[string]interface{}
}

type Section struct {
	Y           int8
	BlockStates PaletteContainer[BlockState] `nbt:"block_states"`
	Biomes      PaletteContainer[BiomeState] `nbt:"biomes"`
	SkyLight    []uint8
	BlockLight  []uint8
}

type JavaChunkData struct {
	BlockEntities []map[string]interface{} `nbt:"block_entities"`
	DataVersion   int32
	LastUpdate    int64
	SectionsLower []Section `nbt:"sections"`
	SectionsUpper []Section `nbt:"Sections"`
	Level         *JavaChunkLevelOld
	Status        string
	XPos          int32 `nbt:"xPos"`
	YPos          int32 `nbt:"yPos"`
	ZPos          int32 `nbt:"zPos"`
}

type JavaChunkLevelOld struct {
	SectionsLower []JavaChunkLevelSectionsOld `nbt:"sections"`
	SectionsUpper []JavaChunkLevelSectionsOld `nbt:"Sections"`
}

type JavaChunkLevelSectionsOld struct {
	Data       []uint8 `nbt:"Data"`
	Blocks     []uint8 `nbt:"Blocks"`
	LastUpdate int64
	Y          int8
}

type BitStorage struct {
	data          []int64
	mask          uint64
	bits          int
	length        int
	valuesPerLong int
}

func calcBitsPerValue(paletteSize int) int {
	if paletteSize <= 0 {
		return 0
	}
	return bits.Len(uint(paletteSize - 1))
}

func calcBitStorageSize(length int, bitsPerValue int) int {
	if bitsPerValue <= 0 {
		return 0
	}
	valuesPerLong := 64 / bitsPerValue
	return (length + valuesPerLong - 1) / valuesPerLong
}

func NewBitStorage(bitsPerValue int, length int, data []int64) (*BitStorage, error) {
	if bitsPerValue <= 0 {
		return &BitStorage{
			bits:          bitsPerValue,
			length:        length,
			valuesPerLong: 0,
		}, nil
	}
	if bitsPerValue > 64 {
		return nil, fmt.Errorf("bitsPerValue too large: %d", bitsPerValue)
	}
	valuesPerLong := 64 / bitsPerValue
	mask := uint64(0)
	if bitsPerValue < 64 {
		mask = (uint64(1) << uint(bitsPerValue)) - 1
	} else {
		mask = ^uint64(0)
	}
	expected := calcBitStorageSize(length, bitsPerValue)
	if data != nil && len(data) != expected {
		return nil, fmt.Errorf("bit storage length mismatch: %d != %d", len(data), expected)
	}
	if data == nil {
		data = make([]int64, expected)
	}
	return &BitStorage{
		data:          data,
		mask:          mask,
		bits:          bitsPerValue,
		length:        length,
		valuesPerLong: valuesPerLong,
	}, nil
}

func (b *BitStorage) calcIndex(index int) (int, int) {
	if b.valuesPerLong == 0 {
		return 0, 0
	}
	return index / b.valuesPerLong, (index % b.valuesPerLong) * b.bits
}

func (b *BitStorage) Get(index int) uint64 {
	dataIndex, bitOffset := b.calcIndex(index)
	if dataIndex < 0 || dataIndex >= len(b.data) {
		return 0
	}
	return (uint64(b.data[dataIndex]) >> uint(bitOffset)) & b.mask
}

func javaPaletteStorageToBitStorage(data []int64, bitsPerValue int, length int) (*BitStorage, error) {
	return NewBitStorage(bitsPerValue, length, data)
}

func javaChunkDataToChunk(_ *JavaChunkData) (*chunks.ChunkWithAuxInfo, error) {
	return nil, fmt.Errorf("javaChunkDataToChunk: TODO")
}

func javaChunkOldDataToChunk(_ *JavaChunkLevelOld) (*chunks.ChunkWithAuxInfo, error) {
	return nil, fmt.Errorf("javaChunkOldDataToChunk: TODO")
}

func javaOldSectionToSubChunk(_ JavaChunkLevelSectionsOld) *chunk.SubChunk {
	return nil
}

func javaSectionToSubChunk(_ Section) *chunk.SubChunk {
	return nil
}

func quickLegacyMapping(_ map[string]interface{}) map[string]interface{} {
	return nil
}

func decodeChunkNBT(payload []byte) (*JavaChunkData, *JavaChunkLevelOld, error) {
	var data JavaChunkData
	if err := nbt.UnmarshalEncoding(payload, &data, nbt.BigEndian); err == nil {
		return &data, nil, nil
	}
	var old JavaChunkLevelOld
	if err := nbt.UnmarshalEncoding(payload, &old, nbt.BigEndian); err == nil {
		return nil, &old, nil
	}
	return nil, nil, fmt.Errorf("decode chunk nbt failed")
}

func convertChunk(pos define.ChunkPos, payload []byte) (*chunks.ChunkWithAuxInfo, error) {
	javaData, oldData, err := decodeChunkNBT(payload)
	if err != nil {
		return nil, err
	}
	if javaData != nil {
		chunkData, err := javaChunkDataToChunk(javaData)
		if err != nil {
			return nil, err
		}
		if chunkData != nil {
			chunkData.ChunkPos = pos
		}
		return chunkData, nil
	}
	if oldData != nil {
		chunkData, err := javaChunkOldDataToChunk(oldData)
		if err != nil {
			return nil, err
		}
		if chunkData != nil {
			chunkData.ChunkPos = pos
		}
		return chunkData, nil
	}
	return nil, fmt.Errorf("no chunk data decoded")
}
