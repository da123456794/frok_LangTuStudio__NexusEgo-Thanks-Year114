package midi

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"nexus/utils/log"

	bwoblock "github.com/TriM-Organization/bedrock-world-operator/block"
	bwoworld "github.com/TriM-Organization/bedrock-world-operator/world"
	wsutils "github.com/Yeah114/WaterStructure/utils"
)

type blockKind int

const (
	blockImpulse blockKind = iota
	blockRepeating
	blockChain
)

const (
	facingDown  = int32(0)
	facingUp    = int32(1)
	facingNorth = int32(2)
	facingSouth = int32(3)
	facingWest  = int32(4)
	facingEast  = int32(5)
)

type commandBlock struct {
	Kind       blockKind
	Command    string
	X, Y, Z    int32
	Facing     int32
	TickDelay  int32
	Auto       byte
	CustomName string
}

type timedCommand struct {
	tick    int
	command string
}

func ConvertFileToMCWorld(inputPath, outputDir string, squareSize int32) (string, error) {
	song, err := ParseFile(inputPath)
	if err != nil {
		return "", err
	}
	opts := DefaultOptions()
	if squareSize > 0 {
		opts.SquareSize = squareSize
	}
	timeline, err := BuildTimeline(song, opts)
	if err != nil {
		return "", err
	}
	midiName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return ExportToMCWorld(timeline, outputDir, opts.SquareSize, midiName)
}

// ExportToMCWorld 将 MIDI timeline 导出为 .mcworld 文件
func ExportToMCWorld(timeline *Timeline, outputDir string, squareSize int32, midiName string) (string, error) {
	if timeline == nil {
		return "", fmt.Errorf("timeline is nil")
	}
	startedAt := time.Now()
	if squareSize <= 0 {
		squareSize = defaultSquareSize
	}
	if midiName == "" {
		midiName = "midi"
	}
	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	blocks := buildChainBlocks(timeline, squareSize, midiName)
	if len(blocks) == 0 {
		return "", fmt.Errorf("no command blocks generated")
	}

	minPos, maxPos := boundsForBlocks(blocks)
	xCount := countChunkSpan(minPos[0], maxPos[0])
	zCount := countChunkSpan(minPos[2], maxPos[2])
	totalChunks := xCount * zCount

	log.Log.Info("结构类型: MID")
	log.Log.Info(fmt.Sprintf("区块总数: %d (X:%d Z:%d)", totalChunks, xCount, zCount))
	log.Log.Info(fmt.Sprintf("非空气方块: %d", len(blocks)))

	worldPath, err := os.MkdirTemp(outputDir, "nexusego-midi-")
	if err != nil {
		return "", fmt.Errorf("create temp world: %w", err)
	}
	defer os.RemoveAll(worldPath)

	bw, err := bwoworld.Open(worldPath, nil)
	if err != nil {
		return "", fmt.Errorf("open temp world: %w", err)
	}

	mcworld, err := wsutils.NewMCWorld(bw, context.Background())
	if err != nil {
		return "", fmt.Errorf("init temp world: %w", err)
	}

	for _, blk := range blocks {
		runtimeID, err := commandBlockRuntimeID(blk.Kind, blk.Facing)
		if err != nil {
			return "", err
		}
		if err := mcworld.SetBlock(blk.X, int16(blk.Y), blk.Z, runtimeID); err != nil {
			return "", fmt.Errorf("set block: %w", err)
		}
		nbt := commandBlockNBT(blk.Command, blk.X, blk.Y, blk.Z, blk.TickDelay, blk.Auto, blk.CustomName)
		if err := mcworld.SetBlockNBT(blk.X, blk.Y, blk.Z, nbt); err != nil {
			return "", fmt.Errorf("set block nbt: %w", err)
		}
	}

	mcworld.Flush()
	levelName := fmt.Sprintf("%s@[0,-64,0]~[%d,%d,%d]", midiName, maxPos[0], maxPos[1], maxPos[2])
	bw.LevelDat().LevelName = levelName
	if err := mcworld.Close(); err != nil {
		return "", fmt.Errorf("close world: %w", err)
	}

	outputPath := filepath.Join(outputDir, levelName+".mcworld")
	if err := zipWorldDir(worldPath, outputPath); err != nil {
		return "", err
	}
	log.Log.Info(fmt.Sprintf("转换完成! 总耗时: %.1fs", time.Since(startedAt).Seconds()))
	log.Log.Info(fmt.Sprintf("MCWorld 文件已保存到: %s", outputPath))
	return outputPath, nil
}

func buildChainBlocks(timeline *Timeline, squareSize int32, midiName string) []commandBlock {
	entries := flattenCommands(timeline.CommandsByTick)
	if len(entries) == 0 {
		return nil
	}

	positions := buildPositions(len(entries), squareSize)
	blocks := make([]commandBlock, 0, len(entries))
	prevTick := 0

	for i, entry := range entries {
		delay := entry.tick
		if i > 0 {
			delay = entry.tick - prevTick
		}
		if delay < 0 {
			delay = 0
		}
		kind := blockChain
		auto := byte(1)
		customName := ""
		if i == 0 {
			kind = blockImpulse
			auto = byte(0)
			customName = midiName
		}
		pos := positions[i]
		blocks = append(blocks, commandBlock{
			Kind:       kind,
			Command:    entry.command,
			X:          pos[0],
			Y:          pos[1],
			Z:          pos[2],
			TickDelay:  int32(delay),
			Auto:       auto,
			CustomName: customName,
		})
		prevTick = entry.tick
	}

	applyFacing(blocks)
	return blocks
}

func flattenCommands(cmdsByTick map[int][]string) []timedCommand {
	if len(cmdsByTick) == 0 {
		return nil
	}
	ticks := make([]int, 0, len(cmdsByTick))
	for t := range cmdsByTick {
		ticks = append(ticks, t)
	}
	sort.Ints(ticks)
	var out []timedCommand
	for _, t := range ticks {
		for _, cmd := range cmdsByTick[t] {
			out = append(out, timedCommand{tick: t, command: cmd})
		}
	}
	return out
}

func buildPositions(count int, squareSize int32) [][3]int32 {
	const startY = int32(-64)
	layerPath := make([][3]int32, 0, squareSize*squareSize)
	for z := int32(0); z < squareSize; z++ {
		if z%2 == 0 {
			for x := int32(0); x < squareSize; x++ {
				layerPath = append(layerPath, [3]int32{x, 0, z})
			}
		} else {
			for x := squareSize - 1; x >= 0; x-- {
				layerPath = append(layerPath, [3]int32{x, 0, z})
			}
		}
	}
	layerSize := int32(len(layerPath))
	if layerSize <= 0 {
		layerSize = 1
	}
	out := make([][3]int32, 0, count)
	for i := 0; i < count; i++ {
		layer := int32(i) / layerSize
		offset := int32(i) % layerSize
		idx := offset
		if layer%2 == 1 {
			idx = layerSize - 1 - offset
		}
		pos := layerPath[idx]
		out = append(out, [3]int32{pos[2], layer + startY, pos[0]})
	}
	return out
}

func applyFacing(blocks []commandBlock) {
	if len(blocks) == 0 {
		return
	}
	for i := 0; i < len(blocks)-1; i++ {
		blocks[i].Facing = facingBetween(blocks[i], blocks[i+1])
	}
	if len(blocks) == 1 {
		blocks[0].Facing = facingEast
	} else {
		blocks[len(blocks)-1].Facing = blocks[len(blocks)-2].Facing
	}
}

func facingBetween(a, b commandBlock) int32 {
	dx, dy, dz := b.X-a.X, b.Y-a.Y, b.Z-a.Z
	switch {
	case dx == 1:
		return facingEast
	case dx == -1:
		return facingWest
	case dz == 1:
		return facingSouth
	case dz == -1:
		return facingNorth
	case dy == 1:
		return facingUp
	case dy == -1:
		return facingDown
	default:
		return facingEast
	}
}

func boundsForBlocks(blocks []commandBlock) ([3]int32, [3]int32) {
	minX, minY, minZ := blocks[0].X, blocks[0].Y, blocks[0].Z
	maxX, maxY, maxZ := minX, minY, minZ
	for _, b := range blocks {
		if b.X < minX {
			minX = b.X
		}
		if b.Y < minY {
			minY = b.Y
		}
		if b.Z < minZ {
			minZ = b.Z
		}
		if b.X > maxX {
			maxX = b.X
		}
		if b.Y > maxY {
			maxY = b.Y
		}
		if b.Z > maxZ {
			maxZ = b.Z
		}
	}
	return [3]int32{minX, minY, minZ}, [3]int32{maxX, maxY, maxZ}
}

func countChunkSpan(minPos, maxPos int32) int {
	return int(chunkCoord(maxPos)-chunkCoord(minPos)) + 1
}

func chunkCoord(pos int32) int32 {
	if pos >= 0 {
		return pos / 16
	}
	return (pos - 15) / 16
}

func commandBlockRuntimeID(kind blockKind, facing int32) (uint32, error) {
	name := "minecraft:command_block"
	switch kind {
	case blockRepeating:
		name = "minecraft:repeating_command_block"
	case blockChain:
		name = "minecraft:chain_command_block"
	}
	if facing < 0 || facing > 5 {
		facing = facingEast
	}
	props := map[string]any{
		"facing_direction": facing,
		"conditional_bit":  byte(0),
	}
	runtimeID, ok := bwoblock.StateToRuntimeID(name, props)
	if !ok {
		return 0, fmt.Errorf("unknown block state for %s", name)
	}
	return runtimeID, nil
}

func commandBlockNBT(command string, x, y, z, tickDelay int32, auto byte, customName string) map[string]any {
	if tickDelay < 0 {
		tickDelay = 0
	}
	return map[string]any{
		"id":                 "CommandBlock",
		"Command":            command,
		"CustomName":         customName,
		"TrackOutput":        byte(0),
		"LastOutput":         "",
		"TickDelay":          tickDelay,
		"ExecuteOnFirstTick": byte(1),
		"auto":               auto,
		"powered":            byte(0),
		"conditionalMode":    byte(0),
		"x":                  x,
		"y":                  y,
		"z":                  z,
	}
}

func zipWorldDir(worldPath, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	return filepath.WalkDir(worldPath, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(worldPath, path)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(w, src)
		return err
	})
}
