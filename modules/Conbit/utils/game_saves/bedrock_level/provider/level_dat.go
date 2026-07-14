package provider

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/LangTuStudio/Conbit/minecraft/nbt"
	"github.com/mitchellh/mapstructure"
)

type LevelDat struct {
	hdr  levelDatHeader
	data []byte
}

type levelDatHeader struct {
	StorageVersion int32
	FileLength     int32
}

func ReadLevelDatFile(dir string) (*LevelDat, error) {
	path := filepath.Join(dir, "level.dat")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening level.dat file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()
	if info, err := f.Stat(); err == nil && info.Size() == 0 {
		return nil, errors.New("level.dat exists but has no data")
	}
	return readLevelDat(bufio.NewReader(f))
}

func readLevelDat(r io.Reader) (*LevelDat, error) {
	var ldat LevelDat
	if err := binary.Read(r, binary.LittleEndian, &ldat.hdr); err != nil {
		return nil, fmt.Errorf("error opening level.dat file: %w", err)
	}
	if ldat.hdr.FileLength <= 0 {
		return nil, errors.New("level.dat exists but has no data")
	}
	ldat.data = make([]byte, ldat.hdr.FileLength)
	if n, err := r.Read(ldat.data); err != nil || int32(n) != ldat.hdr.FileLength {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("error opening level.dat file: %w", err)
	}
	return &ldat, nil
}

func (ld *LevelDat) Unmarshal(dst any) error {
	levelDatTempNBT := make(map[string]any)
	if err := nbt.UnmarshalEncoding(ld.data, &levelDatTempNBT, nbt.LittleEndian); err != nil {
		return fmt.Errorf("error decoding level.dat NBT: %w", err)
	}

	levelDatNBT := lowerMapKeyName(levelDatTempNBT)
	config := &mapstructure.DecoderConfig{
		TagName:          "nbt",
		Result:           dst,
		WeaklyTypedInput: true,
		MatchName:        strings.EqualFold,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return fmt.Errorf("error decoding level.dat NBT: %w", err)
	}
	if err := decoder.Decode(levelDatNBT); err != nil {
		return fmt.Errorf("error decoding level.dat NBT: %w", err)
	}

	return nil
}

func (ld *LevelDat) Marshal(src any) error {
	var err error
	ld.data, err = nbt.MarshalEncoding(src, nbt.LittleEndian)
	if err != nil {
		return fmt.Errorf("error encoding level.dat to NBT: %w", err)
	}
	ld.hdr = levelDatHeader{
		StorageVersion: Version,
		FileLength:     int32(len(ld.data)),
	}
	return nil
}

func (ld *LevelDat) WriteFile(dir string) error {
	path := filepath.Join(dir, "level.dat")
	f, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("error closing level.dat: %w", err)
	}
	w := bufio.NewWriter(f)
	defer func() {
		_ = w.Flush()
		_ = f.Close()
	}()
	if err := binary.Write(w, binary.LittleEndian, ld.hdr); err != nil {
		return fmt.Errorf("error closing level.dat: %w", err)
	}
	if _, err := w.Write(ld.data); err != nil {
		return fmt.Errorf("error closing level.dat: %w", err)
	}
	return nil
}

func lowerMapKeyName(src map[string]any) map[string]any {
	dst := make(map[string]any)
	for key, value := range src {
		if subMap, ok := value.(map[string]any); ok {
			dst[strings.ToLower(key)] = lowerMapKeyName(subMap)
			continue
		}
		dst[strings.ToLower(key)] = value
	}
	return dst
}
