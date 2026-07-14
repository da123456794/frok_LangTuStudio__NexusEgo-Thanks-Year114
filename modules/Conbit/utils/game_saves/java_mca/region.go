package java_mca

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"io"
	"sync"
	"time"
)

const (
	sectorSize    = 4096
	regionEntries = 32
	headerSectors = 2
)

type Region struct {
	data               []uint8
	chunkDataLocation  [regionEntries][regionEntries]int32
	Timestamps         [regionEntries][regionEntries]int32
	DataSectorUsed4096 map[int32]bool
	mu                 sync.RWMutex
	Changed            bool
}

func LoadRegion(reader io.Reader) (*Region, error) {
	buf := &bytes.Buffer{}
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	if len(data) < sectorSize*headerSectors {
		return nil, ErrNoData
	}

	r := &Region{
		data:               data,
		DataSectorUsed4096: make(map[int32]bool),
	}

	r.DataSectorUsed4096[0] = true
	r.DataSectorUsed4096[1] = true

	for i := 0; i < regionEntries; i++ {
		for j := 0; j < regionEntries; j++ {
			offset := 4 * (i + j*regionEntries)
			location := int32(binary.BigEndian.Uint32(data[offset : offset+4]))
			r.chunkDataLocation[i][j] = location
		}
	}

	tsBase := sectorSize
	for i := 0; i < regionEntries; i++ {
		for j := 0; j < regionEntries; j++ {
			offset := tsBase + 4*(i+j*regionEntries)
			ts := int32(binary.BigEndian.Uint32(data[offset : offset+4]))
			r.Timestamps[i][j] = ts
		}
	}

	for i := 0; i < regionEntries; i++ {
		for j := 0; j < regionEntries; j++ {
			start, count := sectorStartAndNumOccupied(r.chunkDataLocation[i][j])
			if start <= 0 || count <= 0 {
				continue
			}
			for k := int32(0); k < count; k++ {
				r.DataSectorUsed4096[start+k] = true
			}
		}
	}
	return r, nil
}

func (r *Region) ReadSector(x int, z int) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if x < 0 || x >= regionEntries || z < 0 || z >= regionEntries {
		return nil, ErrNoSector
	}
	location := r.chunkDataLocation[x][z]
	start, count := sectorStartAndNumOccupied(location)
	if start <= 0 || count <= 0 {
		return nil, ErrNoSector
	}
	if int64(start) < 0 || count <= 0 {
		return nil, ErrSectorNegativeLength
	}
	offset := int(start) * sectorSize
	size := int(count) * sectorSize
	if offset+size > len(r.data) {
		return nil, ErrNoData
	}
	if offset+5 > len(r.data) {
		return nil, ErrNoData
	}
	length := int(binary.BigEndian.Uint32(r.data[offset : offset+4]))
	if length <= 0 {
		return nil, ErrNoData
	}
	if length+4 > size {
		return nil, ErrTooLarge
	}
	compressType := r.data[offset+4]
	payload := r.data[offset+5 : offset+4+length]
	switch compressType {
	case 1:
		rc, err := gzip.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	case 2:
		rc, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	default:
		return payload, nil
	}
}

func (r *Region) WriteSector(x int, z int, payload []byte) error {
	compressed, compressType, err := compressPayload(payload)
	if err != nil {
		return err
	}
	length := len(compressed) + 1
	if length <= 0 {
		return ErrNoData
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	sectorCount := int32((length + 4 + sectorSize - 1) / sectorSize)
	if sectorCount <= 0 {
		return ErrNoData
	}

	if err := r.writeOrAppend(x, z, compressed, compressType, sectorCount); err != nil {
		return err
	}
	r.Changed = true
	return nil
}

func (r *Region) ExistSector(x int, z int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if x < 0 || x >= regionEntries || z < 0 || z >= regionEntries {
		return false
	}
	location := r.chunkDataLocation[x][z]
	start, count := sectorStartAndNumOccupied(location)
	return start > 0 && count > 0
}

func (r *Region) findSpace(required int32) int32 {
	if required <= 0 {
		return -1
	}
	var start int32 = -1
	var count int32
	totalSectors := int32(len(r.data) / sectorSize)
	for i := int32(headerSectors); i <= totalSectors; i++ {
		used := r.DataSectorUsed4096[i]
		if !used {
			if start < 0 {
				start = i
			}
			count++
			if count >= required {
				return start
			}
			continue
		}
		start = -1
		count = 0
	}
	return -1
}

func (r *Region) writeOrAppend(x int, z int, data []byte, compressType byte, sectorCount int32) error {
	location := r.chunkDataLocation[x][z]
	start, count := sectorStartAndNumOccupied(location)
	if count >= sectorCount && start > 0 {
		r.writeAt(start, data, compressType, sectorCount)
		r.setHead(x, z, start, sectorCount)
		return nil
	}

	start = r.findSpace(sectorCount)
	if start < 0 {
		start = int32(len(r.data) / sectorSize)
		extend := int(sectorCount) * sectorSize
		r.data = append(r.data, make([]byte, extend)...)
	}
	r.writeAt(start, data, compressType, sectorCount)
	r.setHead(x, z, start, sectorCount)
	return nil
}

func (r *Region) writeAt(start int32, data []byte, compressType byte, sectorCount int32) {
	offset := int(start) * sectorSize
	length := len(data) + 1
	binary.BigEndian.PutUint32(r.data[offset:offset+4], uint32(length))
	r.data[offset+4] = compressType
	copy(r.data[offset+5:], data)
	limit := int(sectorCount) * sectorSize
	for i := offset + 5 + len(data); i < offset+limit; i++ {
		r.data[i] = 0
	}
}

func (r *Region) setHead(x int, z int, start int32, count int32) {
	for i := int32(0); i < count; i++ {
		r.DataSectorUsed4096[start+i] = true
	}
	r.chunkDataLocation[x][z] = (start << 8) | (count & 0xff)
	r.Timestamps[x][z] = int32(time.Now().Unix())
}

func sectorStartAndNumOccupied(location int32) (int32, int32) {
	if location == 0 {
		return 0, 0
	}
	start := location >> 8
	count := location & 0xff
	return start, count
}

func compressPayload(payload []byte) ([]byte, byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(payload); err != nil {
		_ = w.Close()
		return nil, 0, err
	}
	if err := w.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), 2, nil
}
