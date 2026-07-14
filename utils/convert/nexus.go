package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/TriM-Organization/bedrock-world-operator/world"
	"github.com/Yeah114/WaterStructure/define"
	"github.com/Yeah114/WaterStructure/structure"
	"github.com/mholt/archiver/v3"
)

func ConvertMCWorldToNexus(inputPath, outputPath, author, password string) (string, error) {
	inputPath = strings.TrimSpace(inputPath)
	if inputPath == "" {
		return "", fmt.Errorf("mcworld path is empty")
	}
	ext := strings.ToLower(filepath.Ext(inputPath))
	if ext != ".mcworld" && ext != ".zip" {
		return "", fmt.Errorf("only mcworld files are supported")
	}

	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		outputPath = strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + ".nexus"
	}
	if filepath.Ext(outputPath) == "" {
		outputPath += ".nexus"
	} else if strings.ToLower(filepath.Ext(outputPath)) != ".nexus" {
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".nexus"
	}

	outputDir := filepath.Dir(outputPath)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return "", fmt.Errorf("create output dir failed: %w", err)
		}
	}

	tempDir, err := os.MkdirTemp("", "mcworld_nexus_*")
	if err != nil {
		return "", fmt.Errorf("create temp dir failed: %w", err)
	}
	defer os.RemoveAll(tempDir)

	z := archiver.Zip{}
	if err := z.Unarchive(inputPath, tempDir); err != nil {
		return "", fmt.Errorf("unarchive mcworld failed: %w", err)
	}

	bw, err := world.Open(tempDir, nil)
	if err != nil {
		return "", fmt.Errorf("open mcworld failed: %w", err)
	}
	defer func() {
		bw.CloseWorld()
		bw.Close()
	}()

	minPos, maxPos, err := parseMCWorldBounds(inputPath, bw.LevelDat().LevelName)
	if err != nil {
		return "", err
	}

	target, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create nexus failed: %w", err)
	}
	defer target.Close()

	nexus := &structure.Nexus{
		Author:   strings.TrimSpace(author),
		Password: strings.TrimSpace(password),
	}
	if err := nexus.FromMCWorld(bw, target, minPos, maxPos, nil, func() {}); err != nil {
		return "", fmt.Errorf("build nexus failed: %w", err)
	}

	return outputPath, nil
}

func parseMCWorldBounds(filePath, levelName string) (define.BlockPos, define.BlockPos, error) {
	parse := func(target string) (define.BlockPos, define.BlockPos, bool) {
		re := regexp.MustCompile(`@\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]~\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]`)
		matches := re.FindStringSubmatch(target)
		if len(matches) != 7 {
			return define.BlockPos{}, define.BlockPos{}, false
		}
		x1, _ := strconv.ParseInt(matches[1], 10, 32)
		y1, _ := strconv.ParseInt(matches[2], 10, 32)
		z1, _ := strconv.ParseInt(matches[3], 10, 32)
		x2, _ := strconv.ParseInt(matches[4], 10, 32)
		y2, _ := strconv.ParseInt(matches[5], 10, 32)
		z2, _ := strconv.ParseInt(matches[6], 10, 32)

		minX, maxX := minInt32(int32(x1), int32(x2)), maxInt32(int32(x1), int32(x2))
		minY, maxY := minInt32(int32(y1), int32(y2)), maxInt32(int32(y1), int32(y2))
		minZ, maxZ := minInt32(int32(z1), int32(z2)), maxInt32(int32(z1), int32(z2))

		return define.BlockPos{minX, minY, minZ}, define.BlockPos{maxX, maxY, maxZ}, true
	}

	if minPos, maxPos, ok := parse(filepath.Base(filePath)); ok {
		return minPos, maxPos, nil
	}
	if minPos, maxPos, ok := parse(levelName); ok {
		return minPos, maxPos, nil
	}

	return define.BlockPos{}, define.BlockPos{}, fmt.Errorf("cannot parse bounds from mcworld name")
}

func minInt32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
