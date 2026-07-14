package mapbuilder

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/disintegration/imaging"

	consolepkg "nexus/utils/console"
	"nexus/utils/log"
	chunkdec "nexus/utils/mirror/chunk"
)

// MediaDir 是 MapBuilder 用来寻找媒体文件的目录。
// 由调用方在启动 MapBuilder 之前设置（一般指向 NexusEgo 的 file 目录）。
var MediaDir = ""

// inputConsole 由调用方通过 SetConsole 注入；mapbuilder 内部的所有交互式输入都走它，
// 避免与 NexusEgo 主控制台读 stdin 抢输入。
var inputConsole *consolepkg.Console_input

// SetConsole 让外部把 NexusEgo 的 Console_input 注入进来。
func SetConsole(c *consolepkg.Console_input) {
	inputConsole = c
}

// ========== 枚举定义 ==========
type OverlayMode string

const (
	ModeDirect OverlayMode = "direct"
	ModeClear  OverlayMode = "clear"
)

type ScaleMode string

const (
	ScaleStretch ScaleMode = "stretch"
	ScaleKeep    ScaleMode = "keep"
)

// ========== 结构体定义 ==========
type ImageInfo struct {
	ScaledImage   image.Image
	ContentWidth  int
	ContentHeight int
	OffsetX       int
	OffsetY       int
	TotalWidth    int
	TotalHeight   int
}

type MapConfig struct {
	StartPos     [3]int32
	EndPos       [3]int32
	XDirection   int
	ZDirection   int
	LockMap      bool
	IsConfigured bool
}

type MediaConfig struct {
	MediaType   string
	MediaPath   string
	VideoFPS    int
	VideoSpeed  float64
	OverlayMode OverlayMode
	ScaleMode   ScaleMode
	StartFrame  int
	Concurrency int
	BatchSize   int
}

type FileMapConfig struct {
	StartPos   [3]int32 `json:"start_pos"`
	EndPos     [3]int32 `json:"end_pos"`
	XDirection int      `json:"x_dir"`
	ZDirection int      `json:"z_dir"`
	LockMap    *bool    `json:"lock_map"`
}

type FileMediaConfig struct {
	Type        string  `json:"type"`
	Path        string  `json:"path"`
	Overlay     string  `json:"overlay"`
	Scale       string  `json:"scale"`
	FPS         int     `json:"fps"`
	Speed       float64 `json:"speed"`
	StartFrame  int     `json:"start_frame"`
	Concurrency int     `json:"concurrency"`
	BatchSize   int     `json:"batch_size"`
}

type FileConfig struct {
	Map   FileMapConfig   `json:"map"`
	Media FileMediaConfig `json:"media"`
}

type VideoPlayer struct {
	api           MapAPI
	mapCfg        MapConfig
	videoPath     string
	imagePath     string
	mapsPerRow    int
	mapsPerCol    int
	targetFPS     int
	playbackSpeed float64
	overlayMode   OverlayMode
	scaleMode     ScaleMode
	startFrame    int
	concurrency   int
	batchSize     int
	hasCleared    bool
	mapIDs        [][]int64
	frameBuffer   chan *ImageInfo
	stopChan      chan struct{}
	wg            sync.WaitGroup
	ffmpegCmd     *exec.Cmd
	ffmpegOutput  io.ReadCloser
	width         int
	height        int
	planeType     string
	isImageMode   bool
	isPaused      atomic.Bool
	mu            sync.Mutex
}

type ItemFrameData struct {
	PosX  int32
	PosY  int32
	PosZ  int32
	MapID int64
	Item  map[string]interface{}
}

func NewVideoPlayer(api MapAPI) *VideoPlayer {
	return &VideoPlayer{
		api:           api,
		frameBuffer:   make(chan *ImageInfo, 50),
		stopChan:      make(chan struct{}),
		mu:            sync.Mutex{},
		mapCfg:        MapConfig{IsConfigured: false},
		targetFPS:     10,
		playbackSpeed: 1.0,
		overlayMode:   ModeDirect,
		scaleMode:     ScaleKeep,
		hasCleared:    false,
		startFrame:    0,
		concurrency:   4,
		batchSize:     8,
	}
}

func (vp *VideoPlayer) safeCloseStopChan() {
	vp.mu.Lock()
	defer vp.mu.Unlock()
	select {
	case <-vp.stopChan:
		return
	default:
		close(vp.stopChan)
	}
}

func (vp *VideoPlayer) ResetForNewMedia(mediaCfg MediaConfig) {
	vp.mu.Lock()
	defer vp.mu.Unlock()

	select {
	case <-vp.stopChan:
	default:
		close(vp.stopChan)
	}
	vp.wg.Wait()

	vp.stopChan = make(chan struct{})
	vp.frameBuffer = make(chan *ImageInfo, 50)

	vp.isImageMode = mediaCfg.MediaType == "image"
	vp.isPaused.Store(false)
	vp.targetFPS = mediaCfg.VideoFPS
	vp.playbackSpeed = mediaCfg.VideoSpeed
	vp.overlayMode = mediaCfg.OverlayMode
	vp.scaleMode = mediaCfg.ScaleMode
	vp.startFrame = mediaCfg.StartFrame
	vp.concurrency = mediaCfg.Concurrency
	vp.batchSize = mediaCfg.BatchSize
	vp.hasCleared = false

	if vp.isImageMode {
		vp.imagePath = mediaCfg.MediaPath
		vp.videoPath = ""
	} else {
		vp.videoPath = mediaCfg.MediaPath
		vp.imagePath = ""
	}

	if vp.ffmpegCmd != nil && vp.ffmpegCmd.Process != nil {
		_ = vp.ffmpegCmd.Process.Kill()
	}
	vp.ffmpegCmd = nil
	vp.ffmpegOutput = nil
	vp.width = 0
	vp.height = 0
}

func (vp *VideoPlayer) LockMap(mapID int64) error {
	return vp.api.LockMap(mapID)
}

// ========== 地图配置逻辑 ==========
func (vp *VideoPlayer) ConfigureMap() error {
	log.Log.Info("开始配置地图区域")
	log.Log.Info("提示：在 Minecraft 中使用 F3 查看坐标，选择包含所有地图物品框的矩形区域")

	startPos := readPosInput("请输入地图区域起始坐标（x y z）：")
	endPos := readPosInput("请输入地图区域结束坐标（x y z）：")
	xDir := 1
	if endPos[0] < startPos[0] {
		xDir = -1
	}
	zDir := 1
	if endPos[2] < startPos[2] {
		zDir = -1
	}
	lockMap := readBool("是否锁定地图（锁定后不会被更新）？", true)

	vp.mapCfg = MapConfig{
		StartPos:     startPos,
		EndPos:       endPos,
		XDirection:   xDir,
		ZDirection:   zDir,
		LockMap:      lockMap,
		IsConfigured: true,
	}

	return vp.prepareMaps()
}

func (vp *VideoPlayer) ClearMaps() error {
	if len(vp.mapIDs) == 0 {
		return errors.New("地图 ID 未初始化，请先配置地图区域")
	}
	log.Log.Info("初始化地图缓存...")
	return nil
}

func (vp *VideoPlayer) prepareMaps() error {
	cfg := vp.mapCfg
	minX, maxX := minI32(cfg.StartPos[0], cfg.EndPos[0]), maxI32(cfg.StartPos[0], cfg.EndPos[0])
	minY, maxY := minI32(cfg.StartPos[1], cfg.EndPos[1]), maxI32(cfg.StartPos[1], cfg.EndPos[1])
	minZ, maxZ := minI32(cfg.StartPos[2], cfg.EndPos[2]), maxI32(cfg.StartPos[2], cfg.EndPos[2])

	if minY == maxY {
		vp.planeType = "horizontal"
	} else if minX == maxX {
		vp.planeType = "x_fixed"
	} else if minZ == maxZ {
		vp.planeType = "z_fixed"
	} else {
		return errors.New("区域不是二维平面")
	}

	itemFrames, err := vp.scanItemFrames(minX, minY, minZ, maxX, maxY, maxZ)
	if err != nil {
		return err
	}
	if len(itemFrames) == 0 {
		return errors.New("区域内未找到地图物品框")
	}

	vp.sortItemFrames(itemFrames)

	xSet, ySet, zSet := make(map[int32]struct{}), make(map[int32]struct{}), make(map[int32]struct{})
	for _, f := range itemFrames {
		xSet[f.PosX], ySet[f.PosY], zSet[f.PosZ] = struct{}{}, struct{}{}, struct{}{}
	}
	switch vp.planeType {
	case "horizontal":
		vp.mapsPerRow, vp.mapsPerCol = len(xSet), len(zSet)
	case "x_fixed":
		vp.mapsPerRow, vp.mapsPerCol = len(zSet), len(ySet)
	case "z_fixed":
		vp.mapsPerRow, vp.mapsPerCol = len(xSet), len(ySet)
	}

	if len(itemFrames) != vp.mapsPerRow*vp.mapsPerCol {
		return fmt.Errorf("地图物品框数量(%d)与网格(%dx%d=%d)不匹配", len(itemFrames), vp.mapsPerRow, vp.mapsPerCol, vp.mapsPerRow*vp.mapsPerCol)
	}
	log.Log.Info(fmt.Sprintf("检测到地图网格: %d行 x %d列", vp.mapsPerCol, vp.mapsPerRow))

	vp.initMapBuffers(itemFrames)

	log.Log.Success("地图准备完成")
	return nil
}

func (vp *VideoPlayer) sortItemFrames(itemFrames []*ItemFrameData) {
	cfg := vp.mapCfg
	switch vp.planeType {
	case "horizontal":
		sort.Slice(itemFrames, func(i, j int) bool {
			if itemFrames[i].PosZ != itemFrames[j].PosZ {
				return itemFrames[i].PosZ < itemFrames[j].PosZ == (cfg.ZDirection > 0)
			}
			return itemFrames[i].PosX < itemFrames[j].PosX == (cfg.XDirection > 0)
		})
	case "x_fixed":
		sort.Slice(itemFrames, func(i, j int) bool {
			if itemFrames[i].PosY != itemFrames[j].PosY {
				return itemFrames[i].PosY > itemFrames[j].PosY
			}
			return itemFrames[i].PosZ < itemFrames[j].PosZ == (cfg.ZDirection > 0)
		})
	case "z_fixed":
		sort.Slice(itemFrames, func(i, j int) bool {
			if itemFrames[i].PosY != itemFrames[j].PosY {
				return itemFrames[i].PosY > itemFrames[j].PosY
			}
			return itemFrames[i].PosX < itemFrames[j].PosX == (cfg.XDirection > 0)
		})
	}
}

func (vp *VideoPlayer) initMapBuffers(itemFrames []*ItemFrameData) {
	vp.mapIDs = make([][]int64, vp.mapsPerCol)
	for i := range vp.mapIDs {
		vp.mapIDs[i] = make([]int64, vp.mapsPerRow)
	}

	for i := 0; i < vp.mapsPerCol; i++ {
		for j := 0; j < vp.mapsPerRow; j++ {
			idx := i*vp.mapsPerRow + j
			vp.mapIDs[i][j] = itemFrames[idx].MapID
		}
	}
}

func (vp *VideoPlayer) scanItemFrames(minX, minY, minZ, maxX, maxY, maxZ int32) ([]*ItemFrameData, error) {
	dimensionID, ok := vp.api.Dimension()
	if !ok {
		return nil, errors.New("无法获取机器人所在维度")
	}
	_ = dimensionID
	start, end := BlockPos{minX, minY, minZ}, BlockPos{maxX, maxY, maxZ}

	// 优先走 structure save 的路径（能拿到完整方块实体 NBT，不受客户端可见性限制）。
	if structureAPI, ok := vp.api.(StructureNBTAPI); ok {
		blockEntities, err := structureAPI.RequestStructureNBTs(start, end)
		if err != nil {
			log.Log.Warn("structure save 路径失败，回退到 SubChunk 路径: " + err.Error())
		} else {
			return vp.extractItemFramesFromNBTs(blockEntities, minX, minY, minZ, maxX, maxY, maxZ)
		}
	}

	subChunkResp, err := vp.api.GetSubChunksInArea(dimensionID, start, end)
	if err != nil {
		return nil, fmt.Errorf("子区块请求失败: %w", err)
	}

	type subChunkKey [3]int32
	subChunks := make(map[subChunkKey]*chunkdec.SubChunk)
	blockEntities := make(map[[3]int32]map[string]interface{})
	basePos := subChunkResp.Position

	for _, entry := range subChunkResp.SubChunkEntries {
		if entry.Result == SubChunkResultChunkNotFound || entry.Result == SubChunkResultSuccessAllAir {
			continue
		}

		_, subChunk, nbts, decodeErr := chunkdec.NEMCSubChunkDecode(entry.RawPayload)
		if decodeErr != nil {
			log.Log.Warn(fmt.Sprintf("子区块解码失败(%d,%d,%d): %v", entry.Offset[0], entry.Offset[1], entry.Offset[2], decodeErr))
			continue
		}

		chunkX := basePos[0] + int32(entry.Offset[0])
		chunkZ := basePos[2] + int32(entry.Offset[2])
		subY := basePos[1] + int32(entry.Offset[1])
		subChunks[subChunkKey{chunkX, subY, chunkZ}] = subChunk

		for _, nbtData := range nbts {
			x, xOk := toInt32(nbtData["x"])
			y, yOk := toInt32(nbtData["y"])
			z, zOk := toInt32(nbtData["z"])
			if xOk && yOk && zOk {
				blockEntities[[3]int32{x, y, z}] = nbtData
			}
		}
	}

	log.Log.Info(fmt.Sprintf("已收到 %d 个子区块，%d 个方块实体 NBT", len(subChunks), len(blockEntities)))
	return vp.extractItemFramesFromNBTs(blockEntities, minX, minY, minZ, maxX, maxY, maxZ)
}

// extractItemFramesFromNBTs 从（坐标 → NBT）映射里挑出所有装着 filled_map 的物品展示框。
func (vp *VideoPlayer) extractItemFramesFromNBTs(
	blockEntities map[[3]int32]map[string]interface{},
	minX, minY, minZ, maxX, maxY, maxZ int32,
) ([]*ItemFrameData, error) {
	var itemFrames []*ItemFrameData
	frameWithNBTButWrongItem := 0
	frameWithMapButNoTag := 0
	idSeen := map[string]bool{}
	for pos, entityData := range blockEntities {
		x, y, z := pos[0], pos[1], pos[2]
		if x < minX || x > maxX || y < minY || y > maxY || z < minZ || z > maxZ {
			continue
		}
		idVal, _ := entityData["id"].(string)
		idClean := strings.TrimPrefix(idVal, "minecraft:")
		idSeen[idClean] = true
		item, ok := entityData["Item"].(map[string]interface{})
		if !ok {
			frameWithNBTButWrongItem++
			ucd, _ := entityData["UserCustomData"].(string)
			log.Log.Warn(fmt.Sprintf("物品框(%d,%d,%d) Item 类型异常: 字段=%v，Item=%T %#v，UserCustomData 长度=%d", x, y, z, mapKeys(entityData), entityData["Item"], entityData["Item"], len(ucd)))
			continue
		}
		itemName, _ := item["Name"].(string)
		if strings.TrimPrefix(itemName, "minecraft:") != "filled_map" {
			frameWithNBTButWrongItem++
			log.Log.Warn(fmt.Sprintf("物品框(%d,%d,%d) Item.Name=%q（不是 filled_map）", x, y, z, itemName))
			continue
		}
		tagData, ok := item["tag"].(map[string]interface{})
		if !ok {
			frameWithMapButNoTag++
			log.Log.Warn(fmt.Sprintf("物品框(%d,%d,%d) 的 filled_map 缺少 tag，Item 字段: %v", x, y, z, mapKeys(item)))
			continue
		}
		mapID, ok := toInt64(tagData["map_uuid"])
		if !ok {
			frameWithMapButNoTag++
			log.Log.Warn(fmt.Sprintf("物品框(%d,%d,%d) tag 缺少 map_uuid，tag 键: %v", x, y, z, mapKeys(tagData)))
			continue
		}
		itemFrames = append(itemFrames, &ItemFrameData{
			PosX: x, PosY: y, PosZ: z, MapID: mapID, Item: item,
		})
	}

	if len(itemFrames) == 0 {
		log.Log.Info(fmt.Sprintf("方块实体: %d，识别到的物品框 id 类型: %v，物品不对: %d，无map_uuid: %d", len(blockEntities), mapKeysFromBoolMap(idSeen), frameWithNBTButWrongItem, frameWithMapButNoTag))
		return nil, errors.New("区域内未找到地图物品框")
	}
	log.Log.Info(fmt.Sprintf("找到 %d 个地图物品框", len(itemFrames)))
	return itemFrames, nil
}

// mapKeys 返回 m 的所有键，按字典序排序（仅用于调试日志）。
func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func mapKeysFromBoolMap(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ========== 比例模式处理函数 ==========
func resizeByScaleMode(origImg image.Image, targetWidth, targetHeight int, scaleMode ScaleMode) (image.Image, int, int, int, int) {
	origBounds := origImg.Bounds()
	origWidth := origBounds.Dx()
	origHeight := origBounds.Dy()

	var scaledImg image.Image
	var contentWidth, contentHeight int
	var offsetX, offsetY int

	switch scaleMode {
	case ScaleStretch:
		scaledImg = imaging.Resize(origImg, targetWidth, targetHeight, imaging.Lanczos)
		contentWidth = targetWidth
		contentHeight = targetHeight
		offsetX = 0
		offsetY = 0
	case ScaleKeep:
		widthRatio := float64(targetWidth) / float64(origWidth)
		heightRatio := float64(targetHeight) / float64(origHeight)
		scaleRatio := widthRatio
		if heightRatio < widthRatio {
			scaleRatio = heightRatio
		}
		contentWidth = int(float64(origWidth) * scaleRatio)
		contentHeight = int(float64(origHeight) * scaleRatio)
		scaledImg = imaging.Resize(origImg, contentWidth, contentHeight, imaging.Lanczos)
		offsetX = (targetWidth - contentWidth) / 2
		offsetY = (targetHeight - contentHeight) / 2
	}

	return scaledImg, contentWidth, contentHeight, offsetX, offsetY
}

func loadAndResizeImage(filePath string, mapsPerRow, mapsPerCol int, scaleMode ScaleMode) (*ImageInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("无法打开图片: %w", err)
	}
	defer file.Close()
	origImg, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("图片解码失败: %w", err)
	}
	totalWidth := mapsPerRow * 128
	totalHeight := mapsPerCol * 128
	log.Log.Info(fmt.Sprintf("地图网格尺寸: %dx%d", totalWidth, totalHeight))
	scaledImg, contentWidth, contentHeight, offsetX, offsetY := resizeByScaleMode(origImg, totalWidth, totalHeight, scaleMode)
	log.Log.Info(fmt.Sprintf("处理后尺寸: %dx%d (偏移: X=%d, Y=%d)", contentWidth, contentHeight, offsetX, offsetY))
	return &ImageInfo{
		ScaledImage:   scaledImg,
		ContentWidth:  contentWidth,
		ContentHeight: contentHeight,
		OffsetX:       offsetX,
		OffsetY:       offsetY,
		TotalWidth:    totalWidth,
		TotalHeight:   totalHeight,
	}, nil
}

// ========== 地图处理逻辑 ==========
func processMapByModeAndClear(imgInfo *ImageInfo, row, col int, mode OverlayMode, hasCleared bool) []PixelRequest {
	xOffset := col * 128
	yOffset := row * 128
	pixels := make([]PixelRequest, 0, 128*128)
	transparentColor := color.RGBA{R: 0, G: 0, B: 0, A: 0}

	needClear := (mode == ModeClear) && !hasCleared

	for localY := 0; localY < 128; localY++ {
		for localX := 0; localX < 128; localX++ {
			absX := xOffset + localX
			absY := yOffset + localY
			imgX := absX - imgInfo.OffsetX
			imgY := absY - imgInfo.OffsetY

			var pixelColor color.RGBA
			if imgX >= 0 && imgX < imgInfo.ContentWidth && imgY >= 0 && imgY < imgInfo.ContentHeight {
				c := imgInfo.ScaledImage.At(imgX, imgY)
				r, g, b, a := c.RGBA()
				pixelColor = color.RGBA{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
					A: uint8(a >> 8),
				}
			} else {
				if needClear {
					pixelColor = transparentColor
				} else {
					continue
				}
			}

			index := uint16(localY*128 + localX)
			pixels = append(pixels, PixelRequest{
				Colour: pixelColor,
				Index:  index,
			})
		}
	}

	return pixels
}

func (vp *VideoPlayer) processFrame(imgInfo *ImageInfo) {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, vp.concurrency)
	mapsToProcess := make([][2]int, 0, vp.mapsPerCol*vp.mapsPerRow)
	for row := 0; row < vp.mapsPerCol; row++ {
		for col := 0; col < vp.mapsPerRow; col++ {
			mapsToProcess = append(mapsToProcess, [2]int{row, col})
		}
	}

	var mapMu sync.Mutex
	updatedMaps := make(map[[2]int]bool)
	for _, mc := range mapsToProcess {
		updatedMaps[mc] = false
	}

	for i := 0; i < len(mapsToProcess); i += vp.batchSize {
		end := i + vp.batchSize
		if end > len(mapsToProcess) {
			end = len(mapsToProcess)
		}
		batch := mapsToProcess[i:end]
		for _, mc := range batch {
			row, col := mc[0], mc[1]
			wg.Add(1)
			semaphore <- struct{}{}
			go func(row, col int, mc [2]int) {
				defer wg.Done()
				defer func() { <-semaphore }()
				mapID := vp.mapIDs[row][col]
				if vp.mapCfg.LockMap {
					_ = vp.LockMap(mapID)
				}
				pixels := processMapByModeAndClear(imgInfo, row, col, vp.overlayMode, vp.hasCleared)
				if len(pixels) > 0 {
					mapMu.Lock()
					err := vp.api.SendMapPixels(mapID, pixels)
					if err == nil {
						updatedMaps[mc] = true
					}
					mapMu.Unlock()
					if err != nil {
						log.Log.Warn(fmt.Sprintf("地图[%d,%d]发送像素失败: %v", row, col, err))
					}
					time.Sleep(30 * time.Millisecond)
				}
			}(row, col, mc)
		}
		wg.Wait()
		time.Sleep(100 * time.Millisecond)
	}

	if vp.overlayMode == ModeClear && !vp.hasCleared {
		vp.hasCleared = true
		log.Log.Info("首次清空完成，后续将直接覆盖内容")
	}
}

// ========== 媒体加载逻辑 ==========
func (vp *VideoPlayer) LoadImage() error {
	if !vp.isImageMode {
		return errors.New("当前不是图片模式")
	}
	log.Log.Info(fmt.Sprintf("尝试加载图片: %s", filepath.Base(vp.imagePath)))
	imgInfo, err := loadAndResizeImage(vp.imagePath, vp.mapsPerRow, vp.mapsPerCol, vp.scaleMode)
	if err != nil {
		return fmt.Errorf("加载图片失败: %w", err)
	}
	vp.frameBuffer <- imgInfo
	return nil
}

func (vp *VideoPlayer) StartDecoding() error {
	if vp.isImageMode {
		return vp.LoadImage()
	}
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("未找到ffmpeg，请加入系统环境变量: %w", err)
	}
	log.Log.Info(fmt.Sprintf("找到ffmpeg路径: %s", ffmpegPath))
	probeCmd := exec.Command(
		"ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=r_frame_rate", "-of", "csv=p=0",
		vp.videoPath,
	)
	rateOutput, err := probeCmd.Output()
	if err != nil {
		return fmt.Errorf("获取视频帧率失败: %w", err)
	}
	frameRateStr := strings.TrimSpace(string(rateOutput))
	rateParts := strings.Split(frameRateStr, "/")
	var frameRate float64
	if len(rateParts) == 2 {
		num, _ := strconv.ParseFloat(rateParts[0], 64)
		den, _ := strconv.ParseFloat(rateParts[1], 64)
		if den != 0 {
			frameRate = num / den
		} else {
			frameRate = 30.0
		}
	} else {
		frameRate, _ = strconv.ParseFloat(frameRateStr, 64)
		if frameRate == 0 {
			frameRate = 30.0
		}
	}
	log.Log.Info(fmt.Sprintf("视频原始帧率: %.2f FPS", frameRate))
	probeSizeCmd := exec.Command(
		"ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height", "-of", "csv=p=0",
		vp.videoPath,
	)
	output, err := probeSizeCmd.Output()
	if err != nil {
		return fmt.Errorf("获取视频信息失败: %w", err)
	}
	if _, err := fmt.Sscanf(string(output), "%d,%d", &vp.width, &vp.height); err != nil {
		return fmt.Errorf("解析视频尺寸失败: %w", err)
	}
	log.Log.Info(fmt.Sprintf("视频尺寸: %dx%d", vp.width, vp.height))
	startTime := float64(vp.startFrame) / frameRate
	log.Log.Info(fmt.Sprintf("从第 %d 帧开始播放（对应时间: %.2f秒）", vp.startFrame, startTime))
	ffmpegArgs := []string{
		"-v", "error",
		"-ss", fmt.Sprintf("%.2f", startTime),
		"-i", vp.videoPath,
		"-vf", fmt.Sprintf("select=gte(n\\,%d)", vp.startFrame),
		"-f", "image2pipe",
		"-pix_fmt", "rgba",
		"-sws_flags", "bicubic",
		"-vcodec", "rawvideo",
		"-",
	}
	vp.ffmpegCmd = exec.Command(ffmpegPath, ffmpegArgs...)
	stderr, err := vp.ffmpegCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建stderr管道失败: %w", err)
	}
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Log.Warn(fmt.Sprintf("FFMPEG 错误: %s", scanner.Text()))
		}
	}()
	vp.ffmpegOutput, err = vp.ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建stdout管道失败: %w", err)
	}
	if err := vp.ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("启动ffmpeg失败: %w", err)
	}
	log.Log.Success("FFMPEG 解码进程已启动")
	vp.wg.Add(1)
	go vp.readFrames()
	return nil
}

func (vp *VideoPlayer) readFrames() {
	defer vp.wg.Done()
	defer vp.ffmpegOutput.Close()
	frameSize := vp.width * vp.height * 4
	buffer := make([]byte, frameSize)
	bufReader := bufio.NewReaderSize(vp.ffmpegOutput, frameSize*2)
	for {
		select {
		case <-vp.stopChan:
			_ = vp.ffmpegCmd.Process.Kill()
			return
		default:
		}
		n, err := io.ReadFull(bufReader, buffer)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				log.Log.Info("视频流读取完成")
				return
			}
			log.Log.Warn(fmt.Sprintf("读取帧失败: %v (读取长度: %d/预期: %d)", err, n, frameSize))
			continue
		}
		if n != frameSize {
			log.Log.Warn("帧数据不完整，跳过")
			continue
		}
		img := image.NewRGBA(image.Rect(0, 0, vp.width, vp.height))
		copy(img.Pix, buffer)
		totalWidth := vp.mapsPerRow * 128
		totalHeight := vp.mapsPerCol * 128
		scaledImg, contentWidth, contentHeight, offsetX, offsetY := resizeByScaleMode(img, totalWidth, totalHeight, vp.scaleMode)
		imgInfo := &ImageInfo{
			ScaledImage:   scaledImg,
			ContentWidth:  contentWidth,
			ContentHeight: contentHeight,
			OffsetX:       offsetX,
			OffsetY:       offsetY,
			TotalWidth:    totalWidth,
			TotalHeight:   totalHeight,
		}
		select {
		case vp.frameBuffer <- imgInfo:
		case <-vp.stopChan:
			return
		default:
		}
	}
}

func (vp *VideoPlayer) TogglePause() {
	if vp.isImageMode {
		log.Log.Warn("图片模式不支持暂停/继续")
		return
	}
	old := vp.isPaused.Swap(!vp.isPaused.Load())
	if !old {
		log.Log.Warn("视频已暂停（按空格继续，按q退出当前播放）")
	} else {
		log.Log.Success("视频已继续（按空格暂停，按q退出当前播放）")
	}
}

func (vp *VideoPlayer) listenKeyPress() {
	if runtime.GOOS != "windows" {
		_ = exec.Command("stty", "-echo").Run()
		defer func() { _ = exec.Command("stty", "echo").Run() }()
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-vp.stopChan:
			return
		default:
		}
		key, _, err := reader.ReadRune()
		if err != nil {
			continue
		}
		switch key {
		case ' ':
			vp.TogglePause()
		case 'q', 'Q':
			log.Log.Info("检测到退出指令，停止当前播放...")
			vp.safeCloseStopChan()
			vp.wg.Wait()
			vp.isPaused.Store(false)
			return
		}
	}
}

func (vp *VideoPlayer) Play() error {
	if vp.isImageMode {
		log.Log.Info("开始显示图片...")
		select {
		case <-vp.stopChan:
			return nil
		case imgInfo := <-vp.frameBuffer:
			vp.processFrame(imgInfo)
			log.Log.Success("图片显示完成")
			return nil
		}
	}
	go vp.listenKeyPress()
	log.Log.Info("视频即将开始播放，请稍候...")
	log.Log.Info("播放控制：【空格】暂停/继续 | 【q】退出当前播放")
	time.Sleep(2 * time.Second)
	frameDuration := time.Duration(float64(time.Second) / (float64(vp.targetFPS) * vp.playbackSpeed))
	ticker := time.NewTicker(frameDuration)
	defer ticker.Stop()
	frameCount := 0
	startTime := time.Now()
	for {
		select {
		case <-vp.stopChan:
			log.Log.Info("当前播放已停止")
			return nil
		case <-ticker.C:
			for vp.isPaused.Load() {
				select {
				case <-vp.stopChan:
					return nil
				case <-time.After(100 * time.Millisecond):
				}
			}
			select {
			case imgInfo := <-vp.frameBuffer:
				vp.processFrame(imgInfo)
				frameCount++
				if frameCount%15 == 0 {
					elapsed := time.Since(startTime)
					avgFps := float64(frameCount) / elapsed.Seconds()
					log.Log.Info(fmt.Sprintf("已播放 %d 帧 | 平均帧率 %.1f FPS | 【空格】暂停/继续 | 【q】退出", frameCount, avgFps))
				}
			case <-vp.stopChan:
				return nil
			default:
			}
		}
	}
}

func (vp *VideoPlayer) Stop() {
	vp.safeCloseStopChan()
	vp.wg.Wait()
	vp.isPaused.Store(false)
	if vp.ffmpegCmd != nil && vp.ffmpegCmd.Process != nil {
		_ = vp.ffmpegCmd.Process.Kill()
	}
	log.Log.Info("播放已停止")
}

// ========== 工具函数 ==========
func toInt32(v interface{}) (int32, bool) {
	switch val := v.(type) {
	case int32:
		return val, true
	case int64:
		return int32(val), true
	case int:
		return int32(val), true
	case int16:
		return int32(val), true
	case int8:
		return int32(val), true
	case uint32:
		return int32(val), true
	case uint64:
		return int32(val), true
	case uint16:
		return int32(val), true
	case uint8:
		return int32(val), true
	case float32:
		return int32(val), true
	case float64:
		return int32(val), true
	default:
		return 0, false
	}
}

func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int32:
		return int64(val), true
	case int:
		return int64(val), true
	case int16:
		return int64(val), true
	case int8:
		return int64(val), true
	case uint64:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint16:
		return int64(val), true
	case uint8:
		return int64(val), true
	case float32:
		return int64(val), true
	case float64:
		return int64(val), true
	default:
		return 0, false
	}
}

func minI32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// ========== 交互函数 ==========
func readInput(prompt string) string {
	if inputConsole == nil {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print(prompt)
		scanner.Scan()
		return strings.TrimSpace(scanner.Text())
	}
	s, _, _ := inputConsole.InputInfo(prompt)
	return strings.TrimSpace(s)
}

func readRequiredInput(prompt string) string {
	for {
		s := readInput(prompt)
		if s != "" {
			return s
		}
		log.Log.Error("输入不能为空，请重新输入")
	}
}

func readIntInput(prompt string, def int) int {
	s := readInput(prompt)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		log.Log.Warn(fmt.Sprintf("输入无效，使用默认值 %d", def))
		return def
	}
	return n
}

func readFloatInput(prompt string, def float64) float64 {
	s := readInput(prompt)
	if s == "" {
		return def
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		log.Log.Warn(fmt.Sprintf("输入无效，使用默认值 %.1f", def))
		return def
	}
	return n
}

func readPosInput(prompt string) [3]int32 {
	for {
		s := readRequiredInput(prompt)
		s = strings.ReplaceAll(s, ",", " ")
		parts := strings.Fields(s)
		if len(parts) != 3 {
			log.Log.Error("坐标必须为3个值（x y z 或 x,y,z）")
			continue
		}
		var pos [3]int32
		ok := true
		for i := 0; i < 3; i++ {
			n, err := strconv.ParseInt(parts[i], 10, 32)
			if err != nil {
				log.Log.Error(fmt.Sprintf("坐标 %s 不是有效整数", parts[i]))
				ok = false
				break
			}
			pos[i] = int32(n)
		}
		if ok {
			return pos
		}
	}
}

func readMediaType() string {
	for {
		s := readRequiredInput("请选择媒体类型（1=图片 | 2=视频）：")
		switch s {
		case "1", "图片":
			return "image"
		case "2", "视频":
			return "video"
		default:
			log.Log.Error("请输入 1 或 2")
		}
	}
}

func readOverlayMode() OverlayMode {
	log.Log.Info("覆盖模式说明：")
	log.Log.Info("  1. 直接覆盖模式：仅更新内容，保留边缘原有内容")
	log.Log.Info("  2. 清空覆盖模式：首次清空边缘（设为透明），后续直接覆盖内容")
	for {
		s := readRequiredInput("请选择覆盖模式（1=直接覆盖 | 2=清空覆盖）：")
		switch s {
		case "1", "直接覆盖":
			return ModeDirect
		case "2", "清空覆盖":
			return ModeClear
		default:
			log.Log.Error("请输入 1 或 2")
		}
	}
}

func readScaleMode() ScaleMode {
	log.Log.Info("比例模式说明：")
	log.Log.Info("  1. 拉伸填充：强制适配地图尺寸（可能改变比例）")
	log.Log.Info("  2. 原比例填充：保持媒体原有比例，居中显示（黑边填充）")
	for {
		s := readRequiredInput("请选择比例模式（1=拉伸填充 | 2=原比例填充）：")
		switch s {
		case "1", "拉伸填充":
			return ScaleStretch
		case "2", "原比例填充":
			return ScaleKeep
		default:
			log.Log.Error("请输入 1 或 2")
		}
	}
}

func readStartFrame() int {
	s := readInput("请输入视频起始帧（默认从0开始）：")
	if s == "" {
		return 0
	}
	frame, err := strconv.Atoi(s)
	if err != nil || frame < 0 {
		log.Log.Warn("输入无效，使用默认值 0")
		return 0
	}
	return frame
}

func readMediaPath(mediaType string) string {
	dir := strings.TrimSpace(MediaDir)
	if dir == "" {
		dir = "."
	}
	exts := map[string][]string{
		"图片": {".png", ".jpg", ".jpeg"},
		"视频": {".mp4", ".mov", ".avi", ".flv", ".mkv"},
	}[mediaType]
	extLabel := map[string]string{"图片": "png/jpg/jpeg", "视频": "mp4/mov/avi/flv/mkv"}[mediaType]

	for {
		files := listMediaFiles(dir, exts)
		if len(files) == 0 {
			log.Log.Warn(fmt.Sprintf("%s 目录下未找到任何 %s 文件", dir, extLabel))
			s := readRequiredInput(fmt.Sprintf("请输入%s文件名（位于 %s 目录下）：", mediaType, dir))
			s = strings.ReplaceAll(s, "\\", "/")
			full := s
			if !strings.Contains(s, "/") {
				full = filepath.Join(dir, s)
			}
			if _, err := os.Stat(full); os.IsNotExist(err) {
				log.Log.Error("文件不存在，请重新输入")
				continue
			}
			return full
		}

		log.Log.Info("文件列表:")
		for i, f := range files {
			fmt.Printf("  [%d] %s\n", i+1, f)
		}
		fmt.Printf("  [0] 手动输入文件名（支持%s）\n", extLabel)
		fmt.Printf("  [r] 刷新列表\n")
		choice := readRequiredInput(fmt.Sprintf("请选择%s编号或输入文件名：", mediaType))
		if strings.EqualFold(choice, "r") {
			continue
		}
		if n, err := strconv.Atoi(choice); err == nil {
			if n == 0 {
				s := readRequiredInput(fmt.Sprintf("请输入%s文件名（位于 %s 目录下）：", mediaType, dir))
				s = strings.ReplaceAll(s, "\\", "/")
				full := s
				if !strings.Contains(s, "/") {
					full = filepath.Join(dir, s)
				}
				if _, err := os.Stat(full); os.IsNotExist(err) {
					log.Log.Error("文件不存在，请重新输入")
					continue
				}
				return full
			}
			if n >= 1 && n <= len(files) {
				return filepath.Join(dir, files[n-1])
			}
			log.Log.Error("编号超出范围，请重新输入")
			continue
		}
		full := filepath.Join(dir, choice)
		if _, err := os.Stat(full); err == nil {
			return full
		}
		log.Log.Error("文件不存在，请重新输入")
	}
}

// listMediaFiles 列出 dir 下扩展名命中 exts 的文件，返回相对文件名。
func listMediaFiles(dir string, exts []string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	matched := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		for _, ext := range exts {
			if strings.HasSuffix(lower, ext) {
				matched = append(matched, name)
				break
			}
		}
	}
	sort.Strings(matched)
	return matched
}

func readBool(prompt string, def bool) bool {
	defStr := "y"
	if !def {
		defStr = "n"
	}
	for {
		s := strings.ToLower(readInput(fmt.Sprintf("%s (y/n) [默认:%s]: ", prompt, defStr)))
		if s == "" {
			return def
		}
		switch s {
		case "y", "yes":
			return true
		case "n", "no":
			return false
		default:
			log.Log.Error("请输入 y/yes 或 n/no")
		}
	}
}

func readConcurrency() int {
	log.Log.Warn("高级配置 - 并发数（不建议新手调整！）")
	log.Log.Info("  说明：并发数过大会导致服务器压力剧增、程序崩溃；过小会降低播放流畅度")
	log.Log.Info("  推荐范围：2-8，默认值：4")
	s := readInput("请输入并发处理数量（直接回车使用默认值4）：")
	if s == "" {
		return 4
	}
	concurrency, err := strconv.Atoi(s)
	if err != nil || concurrency < 1 || concurrency > 20 {
		log.Log.Warn("输入无效（必须是1-20之间的整数），使用默认值 4")
		return 4
	}
	if concurrency > 8 {
		log.Log.Warn(fmt.Sprintf("并发数设置为%d（超过推荐最大值8），可能导致程序不稳定", concurrency))
		confirm := readBool("是否确认使用该值？", false)
		if !confirm {
			log.Log.Info("使用默认值 4")
			return 4
		}
	}
	return concurrency
}

func readBatchSize() int {
	log.Log.Info("批处理数量配置")
	log.Log.Info("  说明：每批处理的地图块数量，建议为并发数的2倍")
	log.Log.Info("  推荐范围：4-16，默认值：8")
	s := readInput("请输入每批处理数量（直接回车使用默认值8）：")
	if s == "" {
		return 8
	}
	batchSize, err := strconv.Atoi(s)
	if err != nil || batchSize < 1 || batchSize > 32 {
		log.Log.Warn("输入无效（必须是1-32之间的整数），使用默认值 8")
		return 8
	}
	return batchSize
}

func parseOverlayMode(s string) OverlayMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "clear":
		return ModeClear
	default:
		return ModeDirect
	}
}

func parseScaleMode(s string) ScaleMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stretch":
		return ScaleStretch
	default:
		return ScaleKeep
	}
}

func loadFileConfig(path string) (FileConfig, error) {
	var cfg FileConfig
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("读取配置失败: %w", err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("解析配置失败: %w", err)
	}
	return cfg, nil
}

func getMediaConfig() MediaConfig {
	log.Log.Info("媒体播放配置")
	mediaType := readMediaType()
	mediaPath := readMediaPath(map[string]string{"image": "图片", "video": "视频"}[mediaType])
	overlayMode := readOverlayMode()
	scaleMode := readScaleMode()
	videoFPS := 10
	videoSpeed := 1.0
	startFrame := 0
	concurrency := 4
	batchSize := 8
	if mediaType == "video" {
		videoFPS = readIntInput("请输入播放帧率（默认 10）：", 10)
		videoSpeed = readFloatInput("请输入播放速度（默认 1.0）：", 1.0)
		startFrame = readStartFrame()
		configAdvanced := readBool("是否配置高级参数（并发数/批处理数量）？", false)
		if configAdvanced {
			concurrency = readConcurrency()
			batchSize = readBatchSize()
		}
	}
	log.Log.Info("播放配置摘要")
	log.Log.Info(fmt.Sprintf("  媒体类型：%s", map[string]string{"image": "图片", "video": "视频"}[mediaType]))
	log.Log.Info(fmt.Sprintf("  媒体路径：%s", filepath.Base(mediaPath)))
	log.Log.Info(fmt.Sprintf("  覆盖模式：%s", map[OverlayMode]string{
		ModeDirect: "直接覆盖",
		ModeClear:  "清空覆盖（首次清空）",
	}[overlayMode]))
	log.Log.Info(fmt.Sprintf("  比例模式：%s", map[ScaleMode]string{
		ScaleStretch: "拉伸填充",
		ScaleKeep:    "原比例填充",
	}[scaleMode]))
	if mediaType == "video" {
		log.Log.Info(fmt.Sprintf("  播放帧率：%d | 播放速度：%.1f | 起始帧：%d", videoFPS, videoSpeed, startFrame))
		log.Log.Info(fmt.Sprintf("  并发数：%d | 每批处理数量：%d", concurrency, batchSize))
	}
	readInput("按回车键开始播放...")
	return MediaConfig{
		MediaType:   mediaType,
		MediaPath:   mediaPath,
		VideoFPS:    videoFPS,
		VideoSpeed:  videoSpeed,
		OverlayMode: overlayMode,
		ScaleMode:   scaleMode,
		StartFrame:  startFrame,
		Concurrency: concurrency,
		BatchSize:   batchSize,
	}
}

func askContinue() (bool, bool) {
	continuePlay := readBool("是否继续播放其他媒体？", true)
	if !continuePlay {
		return false, false
	}
	keepMapConfig := readBool("是否保留当前地图画配置？（保留则无需重新选择坐标）", true)
	return true, keepMapConfig
}

func playMedia(player *VideoPlayer, mediaCfg MediaConfig) error {
	player.ResetForNewMedia(mediaCfg)
	if err := player.ClearMaps(); err != nil {
		return err
	}
	if err := player.StartDecoding(); err != nil {
		return err
	}
	return player.Play()
}

// AskMediaConfig 让调用方在登录之前先把媒体相关配置全部问完。
func AskMediaConfig() MediaConfig {
	return getMediaConfig()
}

// AskMapConfig 让调用方在登录之前把地图区域配置（坐标/锁定）问完。
// X/Z 方向自动根据起止坐标推断（end >= start → +1，end < start → -1）。
func AskMapConfig() MapConfig {
	log.Log.Info("开始配置地图区域")
	log.Log.Info("提示：在 Minecraft 中使用 F3 查看坐标，选择包含所有地图物品框的矩形区域")
	startPos := readPosInput("请输入地图区域起始坐标（x y z）：")
	endPos := readPosInput("请输入地图区域结束坐标（x y z）：")
	xDir := 1
	if endPos[0] < startPos[0] {
		xDir = -1
	}
	zDir := 1
	if endPos[2] < startPos[2] {
		zDir = -1
	}
	lockMap := readBool("是否锁定地图（锁定后不会被更新）？", true)
	return MapConfig{
		StartPos:     startPos,
		EndPos:       endPos,
		XDirection:   xDir,
		ZDirection:   zDir,
		LockMap:      lockMap,
		IsConfigured: true,
	}
}

// RunWithPreparedAll 在登录完成、机器人拿到 OP 权限之后调用。
// 使用预先收集好的地图区域配置 + 媒体配置直接扫描并开始播放。
func RunWithPreparedAll(api MapAPI, mapCfg MapConfig, mediaCfg MediaConfig) {
	if api == nil {
		log.Log.Error("MapBuilder 启动失败：未提供有效的连接接入点")
		return
	}
	player := NewVideoPlayer(api)
	player.mapCfg = mapCfg
	if err := player.prepareMaps(); err != nil {
		log.Log.Error("地图配置失败：" + err.Error())
		return
	}

	first := true
	current := mediaCfg
	for {
		if !first {
			current = getMediaConfig()
		}
		first = false
		if err := playMedia(player, current); err != nil {
			log.Log.Error("播放失败：" + err.Error())
		}
		continuePlay, keepMapConfig := askContinue()
		if !continuePlay {
			break
		}
		if !keepMapConfig {
			player.mapCfg.IsConfigured = false
			for {
				if err := player.ConfigureMap(); err != nil {
					log.Log.Error("地图配置失败：" + err.Error())
					if !readBool("是否重新配置？", true) {
						player.Stop()
						return
					}
					continue
				}
				break
			}
		}
	}

	player.Stop()
	log.Log.Success("MapBuilder 已正常退出")
}

// RunWithPreparedMedia 在登录完成、机器人拿到 OP 权限之后调用。
// 它会让玩家定坐标、扫描物品框，然后按预先收集到的媒体配置直接开始播放。
// 播放结束后询问"是否继续"，继续时再次询问媒体配置。
func RunWithPreparedMedia(api MapAPI, mediaCfg MediaConfig) {
	if api == nil {
		log.Log.Error("MapBuilder 启动失败：未提供有效的连接接入点")
		return
	}
	player := NewVideoPlayer(api)

	for {
		if err := player.ConfigureMap(); err != nil {
			log.Log.Error("地图配置失败：" + err.Error())
			if !readBool("是否重新配置地图区域？", true) {
				player.Stop()
				return
			}
			continue
		}
		break
	}

	first := true
	current := mediaCfg
	for {
		if !first {
			current = getMediaConfig()
		}
		first = false
		if err := playMedia(player, current); err != nil {
			log.Log.Error("播放失败：" + err.Error())
		}
		continuePlay, keepMapConfig := askContinue()
		if !continuePlay {
			break
		}
		if !keepMapConfig {
			player.mapCfg.IsConfigured = false
			for {
				if err := player.ConfigureMap(); err != nil {
					log.Log.Error("地图配置失败：" + err.Error())
					if !readBool("是否重新配置？", true) {
						player.Stop()
						return
					}
					continue
				}
				break
			}
		}
	}

	player.Stop()
	log.Log.Success("MapBuilder 已正常退出")
}

// RunCLI 启动 MapBuilder 的交互式 CLI，使用调用方提供的 api（主机器人接入点）。
// configPath 可选，指向一个 JSON 配置文件直接执行一次。
func RunCLI(api MapAPI, configPath string) {
	if api == nil {
		log.Log.Error("MapBuilder 启动失败：未提供有效的连接接入点")
		return
	}

	if configPath != "" {
		cfg, err := loadFileConfig(configPath)
		if err != nil {
			log.Log.Error("配置读取失败：" + err.Error())
			return
		}
		player := NewVideoPlayer(api)
		lockMap := true
		if cfg.Map.LockMap != nil {
			lockMap = *cfg.Map.LockMap
		}
		player.mapCfg = MapConfig{
			StartPos:     cfg.Map.StartPos,
			EndPos:       cfg.Map.EndPos,
			XDirection:   cfg.Map.XDirection,
			ZDirection:   cfg.Map.ZDirection,
			LockMap:      lockMap,
			IsConfigured: true,
		}
		if err := player.prepareMaps(); err != nil {
			log.Log.Error("地图配置失败：" + err.Error())
			return
		}
		mediaCfg := MediaConfig{
			MediaType:   strings.ToLower(cfg.Media.Type),
			MediaPath:   cfg.Media.Path,
			VideoFPS:    cfg.Media.FPS,
			VideoSpeed:  cfg.Media.Speed,
			OverlayMode: parseOverlayMode(cfg.Media.Overlay),
			ScaleMode:   parseScaleMode(cfg.Media.Scale),
			StartFrame:  cfg.Media.StartFrame,
			Concurrency: cfg.Media.Concurrency,
			BatchSize:   cfg.Media.BatchSize,
		}
		if mediaCfg.VideoFPS == 0 {
			mediaCfg.VideoFPS = 10
		}
		if mediaCfg.VideoSpeed == 0 {
			mediaCfg.VideoSpeed = 1.0
		}
		if mediaCfg.Concurrency == 0 {
			mediaCfg.Concurrency = 4
		}
		if mediaCfg.BatchSize == 0 {
			mediaCfg.BatchSize = 8
		}
		if err := playMedia(player, mediaCfg); err != nil {
			log.Log.Error("播放失败：" + err.Error())
		}
		player.Stop()
		return
	}

	player := NewVideoPlayer(api)

	for {
		if !player.mapCfg.IsConfigured {
			if err := player.ConfigureMap(); err != nil {
				log.Log.Error("地图配置失败：" + err.Error())
				if !readBool("是否重新配置地图区域？", true) {
					player.Stop()
					return
				}
				continue
			}
		}
		break
	}

	for {
		mediaCfg := getMediaConfig()
		if err := playMedia(player, mediaCfg); err != nil {
			log.Log.Error("播放失败：" + err.Error())
		}
		continuePlay, keepMapConfig := askContinue()
		if !continuePlay {
			break
		}
		if !keepMapConfig {
			player.mapCfg.IsConfigured = false
			for {
				if err := player.ConfigureMap(); err != nil {
					log.Log.Error("地图配置失败：" + err.Error())
					if !readBool("是否重新配置？", true) {
						player.Stop()
						return
					}
					continue
				}
				break
			}
		}
	}

	player.Stop()
	log.Log.Success("MapBuilder 已正常退出")
}

// 防止 context 包的导入被优化（保留以便日后扩展）
var _ = context.Background


