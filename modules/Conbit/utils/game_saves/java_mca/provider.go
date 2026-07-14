package java_mca

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/LangTuStudio/Conbit/Conbit/chunks"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
)

type JavaStorageProvider struct {
	getRegionFileData func(int, int) []uint8
	lastLoadRegionPos *struct {
		X int
		Z int
	}
	lastLoadRegion *Region
}

func NewJavaSaveReaderFromZip(path string) (chunks.ChunkReader, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}

	fileMap := make(map[string]*zip.File)
	for _, file := range reader.File {
		name := file.Name
		if strings.HasPrefix(name, "region/") {
			name = strings.TrimPrefix(name, "region/")
		}
		fileMap[name] = file
	}

	getRegionData := func(regionX int, regionZ int) []uint8 {
		name := fmt.Sprintf("r.%v.%v.mca", regionX, regionZ)
		file := fileMap[name]
		if file == nil {
			return nil
		}
		rc, err := file.Open()
		if err != nil {
			return nil
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil
		}
		return data
	}

	return &JavaStorageProvider{
		getRegionFileData: getRegionData,
	}, nil
}

func (p *JavaStorageProvider) Get(pos define.ChunkPos) *chunks.ChunkWithAuxInfo {
	regionX, regionZ := chunkInWhichRegion(pos)
	region, err := p.getRegion(int(regionX), int(regionZ))
	if err != nil || region == nil {
		return nil
	}
	localX, localZ := whichChunkInsideRegion(pos)
	payload, err := region.ReadSector(int(localX), int(localZ))
	if err != nil {
		return nil
	}
	chunkData, err := convertChunk(pos, payload)
	if err != nil {
		return nil
	}
	return chunkData
}

func (p *JavaStorageProvider) getRegion(regionX int, regionZ int) (*Region, error) {
	if p.lastLoadRegionPos != nil {
		if p.lastLoadRegionPos.X == regionX && p.lastLoadRegionPos.Z == regionZ {
			return p.lastLoadRegion, nil
		}
	}
	if p.getRegionFileData == nil {
		return nil, ErrNoData
	}
	data := p.getRegionFileData(regionX, regionZ)
	if len(data) == 0 {
		return nil, ErrNoData
	}
	region, err := LoadRegion(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	p.lastLoadRegionPos = &struct {
		X int
		Z int
	}{X: regionX, Z: regionZ}
	p.lastLoadRegion = region
	return region, nil
}

func chunkInWhichRegion(pos define.ChunkPos) (int32, int32) {
	return pos[0] >> 5, pos[1] >> 5
}

func whichChunkInsideRegion(pos define.ChunkPos) (int32, int32) {
	return pos[0] & 31, pos[1] & 31
}

func regionFilePath(base string, regionX int32, regionZ int32) string {
	return filepath.Join(base, fmt.Sprintf("r.%v.%v.mca", regionX, regionZ))
}
