package convert

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	midiconvert "nexus/utils/convert/midi"
	"nexus/utils/log"

	bwochunk "github.com/TriM-Organization/bedrock-world-operator/chunk"
	bwodefine "github.com/TriM-Organization/bedrock-world-operator/define"
	"github.com/TriM-Organization/bedrock-world-operator/world"
	"github.com/Yeah114/WaterStructure/define"
	"github.com/Yeah114/WaterStructure/structure"
	"github.com/mholt/archiver/v3"
	"github.com/schollz/progressbar/v3"
)

func ConvertToMCWorld(inputPath, outputDir, nexusPassword string) (string, error) {
	ext := strings.ToLower(filepath.Ext(inputPath))
	if IsImageFile(inputPath) {
		return ConvertImageToMCWorld(inputPath, outputDir, 0)
	}
	if ext == ".mid" || ext == ".midi" {
		return midiconvert.ConvertFileToMCWorld(inputPath, outputDir, 0)
	}

	file, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("打开结构文件失败: %w", err)
	}
	defer file.Close()

	var reader structure.Structure
	if ext == ".nexus" {
		nexus := &structure.Nexus{Password: nexusPassword}
		if err := nexus.FromFile(file); err != nil {
			return "", fmt.Errorf("read nexus failed: %w", err)
		}
		reader = nexus
	} else {
		reader, err = structure.StructureFromFile(file)
		if err != nil {
			return "", fmt.Errorf("read structure failed: %w", err)
		}
	}
	if reader != nil {
		defer reader.Close()
	}

	tempDir, err := os.MkdirTemp("", "ws_world_*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	bedrockWorld, err := world.Open(tempDir, nil)
	if err != nil {
		return "", fmt.Errorf("打开世界失败: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = bedrockWorld.CloseWorld()
		}
	}()

	structureName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	size := reader.GetSize()
	worldName := fmt.Sprintf("%s@[0,-64,0]~[%d,%d,%d]", structureName, size.Width-1, size.Height-64-1, size.Length-1)
	bedrockWorld.LevelDat().LevelName = worldName

	log.Log.Info(fmt.Sprintf("结构类型: %s", reader.Name()))

	maxThreads := runtime.NumCPU() * 2
	if maxThreads > 32 {
		maxThreads = 32
	}

	xCount, zCount := size.GetChunkXCount(), size.GetChunkZCount()
	totalChunks := xCount * zCount

	chunksPerTask := 32
	if totalChunks > 1000 {
		chunksPerTask = 64
	} else if totalChunks < 100 {
		chunksPerTask = 16
	}

	num, _ := reader.CountNonAirBlocks()
	log.Log.Info(fmt.Sprintf("区块总数: %d (X:%d Z:%d)", totalChunks, xCount, zCount))
	log.Log.Info(fmt.Sprintf("非空气方块: %d", num))

	totalStart := time.Now()
	bar := buildBar(totalChunks, "转换进度")

	var produceWG sync.WaitGroup
	var saveWG sync.WaitGroup
	semaphore := make(chan struct{}, maxThreads)
	chunkChan := make(chan map[define.ChunkPos]*bwochunk.Chunk, maxThreads*2)
	nbtChan := make(chan map[define.ChunkPos][]map[string]any, maxThreads*2)

	saveWG.Add(1)
	go func() {
		defer saveWG.Done()
		for chunks := range chunkChan {
			for pos, chunkData := range chunks {
				targetPos := bwodefine.ChunkPos{pos[0], pos[1]}
				if err := bedrockWorld.SaveChunk(bwodefine.Dimension(0), targetPos, chunkData); err != nil {
					fmt.Printf("\n警告: 保存区块失败: %v\n", err)
				}
			}
		}
	}()

	saveWG.Add(1)
	go func() {
		defer saveWG.Done()
		for nbtByChunk := range nbtChan {
			for pos, data := range nbtByChunk {
				targetPos := bwodefine.ChunkPos{pos[0], pos[1]}
				if err := bedrockWorld.SaveNBT(bwodefine.Dimension(0), targetPos, data); err != nil {
					fmt.Printf("\n警告: 保存NBT失败: %v\n", err)
				}
			}
		}
	}()

	err = forEachChunkBatch(xCount, zCount, chunksPerTask, func(chunkSubset []define.ChunkPos) error {
		taskSize := len(chunkSubset)

		produceWG.Add(1)
		semaphore <- struct{}{}
		go func(chunks []define.ChunkPos, size int) {
			defer produceWG.Done()
			defer func() { <-semaphore }()

			processedChunks, err := reader.GetChunks(chunks)
			if err != nil {
				fmt.Printf("\n警告: 获取区块失败: %v\n", err)
				return
			}
			if len(processedChunks) > 0 {
				chunkChan <- processedChunks
			}

			chunkNBTs, err := reader.GetChunksNBT(chunks)
			if err != nil {
				fmt.Printf("\n警告: 获取区块NBT失败: %v\n", err)
			} else {
				nbtBatch := buildChunkNBTBatch(chunkNBTs)
				if len(nbtBatch) > 0 {
					nbtChan <- nbtBatch
				}
			}

			if bar != nil {
				_ = bar.Add(size)
			}
		}(chunkSubset, taskSize)
		return nil
	})
	if err != nil {
		return "", err
	}

	produceWG.Wait()
	close(chunkChan)
	close(nbtChan)
	saveWG.Wait()

	if bar != nil {
		_ = bar.Finish()
	}

	if err := bedrockWorld.CloseWorld(); err != nil {
		return "", fmt.Errorf("关闭世界失败: %w", err)
	}
	closed = true

	mcworldPath, err := archiveWorldAsMCWorld(tempDir, worldName, outputDir)
	if err != nil {
		return "", err
	}

	log.Log.Info(fmt.Sprintf("转换完成! 总耗时: %.1fs", time.Since(totalStart).Seconds()))
	log.Log.Info(fmt.Sprintf("MCWorld 文件已保存到: %s", mcworldPath))
	return mcworldPath, nil
}

func IsImportCandidatePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	if IsImageFile(path) || ext == ".mid" || ext == ".midi" {
		return true
	}
	return isOneDragonStructureFile(path)
}

func buildChunkNBTBatch(chunkNBTs map[define.ChunkPos]map[define.BlockPos]map[string]any) map[define.ChunkPos][]map[string]any {
	if len(chunkNBTs) == 0 {
		return nil
	}

	nbtByChunk := make(map[define.ChunkPos][]map[string]any, len(chunkNBTs))
	for cpos, blockMap := range chunkNBTs {
		if len(blockMap) == 0 {
			continue
		}

		list := make([]map[string]any, 0, len(blockMap))
		for bpos, nbtData := range blockMap {
			if nbtData == nil {
				continue
			}

			entry := make(map[string]any, len(nbtData)+3)
			for key, value := range nbtData {
				entry[key] = value
			}
			entry["x"] = int32(cpos[0]*16) + bpos.X()
			entry["y"] = bpos.Y()
			entry["z"] = int32(cpos[1]*16) + bpos.Z()
			list = append(list, entry)
		}

		if len(list) > 0 {
			nbtByChunk[cpos] = list
		}
	}
	return nbtByChunk
}

func forEachChunkBatch(xCount, zCount, batchSize int, fn func([]define.ChunkPos) error) error {
	if batchSize <= 0 {
		batchSize = 1
	}

	batch := make([]define.ChunkPos, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		current := make([]define.ChunkPos, len(batch))
		copy(current, batch)
		batch = batch[:0]
		return fn(current)
	}

	for x := 0; x < xCount; x++ {
		for z := 0; z < zCount; z++ {
			batch = append(batch, define.ChunkPos{int32(x), int32(z)})
			if len(batch) == batchSize {
				if err := flush(); err != nil {
					return err
				}
			}
		}
	}
	return flush()
}

func archiveWorldAsMCWorld(worldPath, worldName, outputDir string) (string, error) {
	files, err := os.ReadDir(worldPath)
	if err != nil {
		return "", fmt.Errorf("读取世界目录失败: %w", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("世界目录为空: %s", worldPath)
	}

	filePaths := make([]string, 0, len(files))
	for _, file := range files {
		filePaths = append(filePaths, filepath.Join(worldPath, file.Name()))
	}

	if outputDir == "" {
		outputDir = "."
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("创建输出目录失败: %w", err)
	}

	zipPath := filepath.Join(outputDir, fmt.Sprintf("%s.zip", worldName))
	mcworldPath := filepath.Join(outputDir, fmt.Sprintf("%s.mcworld", worldName))

	_ = os.RemoveAll(zipPath)
	_ = os.RemoveAll(mcworldPath)

	z := archiver.Zip{}
	if err := z.Archive(filePaths, zipPath); err != nil {
		return "", fmt.Errorf("压缩世界失败: %w", err)
	}
	if err := os.Rename(zipPath, mcworldPath); err != nil {
		return "", fmt.Errorf("重命名为 mcworld 失败: %w", err)
	}
	return mcworldPath, nil
}

func buildBar(total int, desc string) *progressbar.ProgressBar {
	if total <= 0 {
		return nil
	}
	return progressbar.NewOptions(
		total,
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerHead:    ">",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionClearOnFinish(),
	)
}
