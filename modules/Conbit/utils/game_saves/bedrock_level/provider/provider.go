package provider

import (
	"time"

	"github.com/LangTuStudio/Conbit/Conbit/chunks"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/chunk"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	blob_hash_packet "github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash/packet"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
)

type Provider struct {
	BedrockWorld World
}

func (p *Provider) get(dimension define.Dimension, pos define.ChunkPos) *chunks.ChunkWithAuxInfo {
	info := &chunks.ChunkWithAuxInfo{
		ChunkPos: pos,
	}
	ch, ok, err := p.BedrockWorld.LoadChunk(dimension, pos)
	if err != nil || !ok {
		return nil
	}
	info.Chunk = ch

	nbts, err := p.BedrockWorld.LoadNBT(dimension, pos)
	if err == nil {
		info.BlockNbts = make(map[define.CubePos]map[string]interface{})
		for _, nbtData := range nbts {
			if nbtPos, ok := define.GetCubePosFromNBT(nbtData); ok {
				info.BlockNbts[nbtPos] = nbtData
			}
		}
	}

	info.SyncTime = p.BedrockWorld.LoadTimeStamp(dimension, pos)
	return info
}

func (p *Provider) save(dimension define.Dimension, info *chunks.ChunkWithAuxInfo) error {
	if err := p.BedrockWorld.SaveChunk(dimension, info.ChunkPos, info.Chunk); err != nil {
		return err
	}

	var nbtList []map[string]interface{}
	if len(info.BlockNbts) > 0 {
		nbtList = make([]map[string]interface{}, 0, len(info.BlockNbts))
		for _, nbtData := range info.BlockNbts {
			nbtList = append(nbtList, nbtData)
		}
	}
	if err := p.BedrockWorld.SaveNBT(dimension, info.ChunkPos, nbtList); err != nil {
		return err
	}
	if err := p.BedrockWorld.SaveTimeStamp(dimension, info.ChunkPos, info.SyncTime); err != nil {
		return err
	}
	return nil
}

func (p *Provider) Close(force bool) error {
	if force {
		return p.BedrockWorld.Close()
	}
	return p.BedrockWorld.CloseWorld()
}

func (p *Provider) Get(dimension define.Dimension, pos define.ChunkPos) *chunks.DimensiondChunkWithAuxInfo {
	info := p.get(dimension, pos)
	return &chunks.DimensiondChunkWithAuxInfo{
		ChunkWithAuxInfo: info,
		Dim:              dimension,
	}
}

func (p *Provider) Save(chunk *chunks.DimensiondChunkWithAuxInfo) error {
	return p.save(chunk.Dim, chunk.ChunkWithAuxInfo)
}

func (p *Provider) SaveAuxInfo() error {
	p.BedrockWorld.LevelDat().LastPlayed = time.Now().Unix()
	return p.BedrockWorld.UpdateLevelDat()
}

func (p *Provider) LevelDat() *Data {
	return p.BedrockWorld.LevelDat()
}

func (p *Provider) LoadChunk(dimension define.Dimension, pos define.ChunkPos) (*chunk.Chunk, bool, error) {
	return p.BedrockWorld.LoadChunk(dimension, pos)
}

func (p *Provider) LoadChunkPayloadOnly(dimension define.Dimension, pos define.ChunkPos) ([][]byte, bool, error) {
	return p.BedrockWorld.LoadChunkPayloadOnly(dimension, pos)
}

func (p *Provider) LoadDeltaUpdate(dimension define.Dimension, pos define.ChunkPos) ([]byte, error) {
	return p.BedrockWorld.LoadDeltaUpdate(dimension, pos)
}

func (p *Provider) LoadDeltaUpdateTimeStamp(dimension define.Dimension, pos define.ChunkPos) int64 {
	return p.BedrockWorld.LoadDeltaUpdateTimeStamp(dimension, pos)
}

func (p *Provider) LoadFullSubChunkBlobHash(dimension define.Dimension, pos define.ChunkPos) []blob_hash_packet.HashWithPosY {
	return toBlobHashWithPosYList(p.BedrockWorld.LoadFullSubChunkBlobHash(dimension, pos))
}

func (p *Provider) LoadNBT(dimension define.Dimension, pos define.ChunkPos) ([]map[string]interface{}, error) {
	return p.BedrockWorld.LoadNBT(dimension, pos)
}

func (p *Provider) LoadNBTPayloadOnly(dimension define.Dimension, pos define.ChunkPos) []byte {
	return p.BedrockWorld.LoadNBTPayloadOnly(dimension, pos)
}

func (p *Provider) LoadSubChunk(dimension define.Dimension, pos protocol.SubChunkPos) *chunk.SubChunk {
	return p.BedrockWorld.LoadSubChunk(dimension, pos)
}

func (p *Provider) LoadSubChunkBlobHash(dimension define.Dimension, pos protocol.SubChunkPos) (uint64, bool) {
	return p.BedrockWorld.LoadSubChunkBlobHash(dimension, pos)
}

func (p *Provider) LoadTimeStamp(dimension define.Dimension, pos define.ChunkPos) int64 {
	return p.BedrockWorld.LoadTimeStamp(dimension, pos)
}

func (p *Provider) SaveChunk(dimension define.Dimension, pos define.ChunkPos, c *chunk.Chunk) error {
	return p.BedrockWorld.SaveChunk(dimension, pos, c)
}

func (p *Provider) SaveChunkPayloadOnly(dimension define.Dimension, pos define.ChunkPos, payload [][]byte) error {
	return p.BedrockWorld.SaveChunkPayloadOnly(dimension, pos, payload)
}

func (p *Provider) SaveDeltaUpdate(dimension define.Dimension, pos define.ChunkPos, payload []byte) error {
	return p.BedrockWorld.SaveDeltaUpdate(dimension, pos, payload)
}

func (p *Provider) SaveDeltaUpdateTimeStamp(dimension define.Dimension, pos define.ChunkPos, ts int64) error {
	return p.BedrockWorld.SaveDeltaUpdateTimeStamp(dimension, pos, ts)
}

func (p *Provider) SaveFullSubChunkBlobHash(dimension define.Dimension, pos define.ChunkPos, hashes []blob_hash_packet.HashWithPosY) error {
	return p.BedrockWorld.SaveFullSubChunkBlobHash(dimension, pos, toWorldHashWithPosYList(hashes))
}

func (p *Provider) SaveNBT(dimension define.Dimension, pos define.ChunkPos, nbtData []map[string]interface{}) error {
	return p.BedrockWorld.SaveNBT(dimension, pos, nbtData)
}

func (p *Provider) SaveNBTPayloadOnly(dimension define.Dimension, pos define.ChunkPos, payload []byte) error {
	return p.BedrockWorld.SaveNBTPayloadOnly(dimension, pos, payload)
}

func (p *Provider) SaveSubChunk(dimension define.Dimension, pos protocol.SubChunkPos, c *chunk.SubChunk) error {
	return p.BedrockWorld.SaveSubChunk(dimension, pos, c)
}

func (p *Provider) SaveSubChunkBlobHash(dimension define.Dimension, pos protocol.SubChunkPos, hash uint64) error {
	return p.BedrockWorld.SaveSubChunkBlobHash(dimension, pos, hash)
}

func (p *Provider) SaveTimeStamp(dimension define.Dimension, pos define.ChunkPos, ts int64) error {
	return p.BedrockWorld.SaveTimeStamp(dimension, pos, ts)
}

func (p *Provider) Put(key []byte, value []byte) error {
	return p.BedrockWorld.Put(key, value)
}

func (p *Provider) Delete(key []byte) error {
	return p.BedrockWorld.Delete(key)
}

func (p *Provider) IterAll(fn IterAllFunc) error {
	return p.BedrockWorld.IterAll(fn)
}

func toBlobHashWithPosYList(src []HashWithPosY) []blob_hash_packet.HashWithPosY {
	if len(src) == 0 {
		return nil
	}
	dst := make([]blob_hash_packet.HashWithPosY, 0, len(src))
	for _, item := range src {
		dst = append(dst, blob_hash_packet.HashWithPosY{
			Hash: blob_hash_packet.Hash(item.Hash),
			PosY: item.PosY,
		})
	}
	return dst
}

func toWorldHashWithPosYList(src []blob_hash_packet.HashWithPosY) []HashWithPosY {
	if len(src) == 0 {
		return nil
	}
	dst := make([]HashWithPosY, 0, len(src))
	for _, item := range src {
		dst = append(dst, HashWithPosY{
			Hash: uint64(item.Hash),
			PosY: item.PosY,
		})
	}
	return dst
}
