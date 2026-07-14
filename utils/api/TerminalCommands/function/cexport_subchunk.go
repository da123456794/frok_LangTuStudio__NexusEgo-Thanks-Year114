package function

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	ResourcesControl "nexus/utils/api/resources_control"
	clientType "nexus/utils/client"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/LangTuStudio/Conbit/minecraft/nbt"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	wsnbt "github.com/Yeah114/WaterStructure/utils/nbt"

	bwchunk "github.com/TriM-Organization/bedrock-world-operator/chunk"
	bwdefine "github.com/TriM-Organization/bedrock-world-operator/define"
	"github.com/TriM-Organization/bedrock-world-operator/world"
	"github.com/mitchellh/mapstructure"
	"github.com/schollz/progressbar/v3"
	ssblockactors "nexus/utils/flowers_runtime/block_actors"
	ssprotocol "nexus/utils/flowers_runtime/protocol"
)

type OptimizedExportConfig struct {
	ChunkRadius       int32
	SubChunkTimeout   time.Duration
	WaitChunkLoad     bool
	SkipOperatorProbe bool
	Context           context.Context
}

func DefaultOptimizedExportConfig() *OptimizedExportConfig {
	return &OptimizedExportConfig{
		ChunkRadius:       32,
		SubChunkTimeout:   5 * time.Second,
		WaitChunkLoad:     false,
		SkipOperatorProbe: false,
		Context:           context.Background(),
	}
}

func isSensitiveFile(filename string) bool {
	sensitiveFiles := []string{
		"config.json",
		"build_interrupt.json",
		".env",
		"credentials.json",
		"secrets.json",
	}
	lowerName := strings.ToLower(filename)
	for _, sensitive := range sensitiveFiles {
		if lowerName == sensitive {
			return true
		}
	}
	if strings.HasPrefix(filename, ".") && filename != "." && filename != ".." {
		return true
	}
	return false
}

type exportArea struct {
	Start protocol.BlockPos
	End   protocol.BlockPos
}

type exportProgress struct {
	mu   sync.Mutex
	bar  *progressbar.ProgressBar
	done map[bwdefine.SubChunkPos]struct{}
}

type exportPaths struct {
	worldName  string
	worldDir   string
	outputPath string
}

type awaitChangesCapable interface {
	AwaitChangesGeneral() error
}

func mergeExportMaps(maps ...map[string]any) map[string]any {
	merged := make(map[string]any)
	for _, current := range maps {
		for key, value := range current {
			merged[key] = value
		}
	}
	return merged
}

func newExportProgress(total int) *exportProgress {
	if total <= 0 {
		return &exportProgress{done: make(map[bwdefine.SubChunkPos]struct{})}
	}
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetDescription("Export Progress"),
		progressbar.OptionShowCount(),
		progressbar.OptionFullWidth(),
		progressbar.OptionThrottle(65*time.Millisecond),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetRenderBlankState(true),
	)
	return &exportProgress{
		bar:  bar,
		done: make(map[bwdefine.SubChunkPos]struct{}, total),
	}
}

func (p *exportProgress) Mark(pos bwdefine.SubChunkPos) {
	if p == nil || p.bar == nil {
		return
	}
	p.mu.Lock()
	if _, ok := p.done[pos]; ok {
		p.mu.Unlock()
		return
	}
	p.done[pos] = struct{}{}
	p.mu.Unlock()
	_ = p.bar.Add(1)
}

func (p *exportProgress) MarkMissing(list []bwdefine.SubChunkPos) {
	if p == nil {
		return
	}
	for _, pos := range list {
		p.Mark(pos)
	}
}

func ExportMCWorldOptimized(client *clientType.Client, filePath string, x1, y1, z1, x2, y2, z2 int, config *OptimizedExportConfig) (string, error) {
	if config == nil {
		config = DefaultOptimizedExportConfig()
	}

	if client == nil || client.Conn == nil {
		return "", errors.New("client not connected")
	}
	res, ok := client.Resources.(*ResourcesControl.Resources)
	if !ok || res == nil {
		return "", errors.New("missing resources")
	}

	if config.Context != nil {
		select {
		case <-config.Context.Done():
			return "", config.Context.Err()
		default:
		}
	}

	minX, maxX, minY, maxY, minZ, maxZ := normalizeExportBounds(x1, y1, z1, x2, y2, z2)

	dimID := client.DimensionID
	if dimID < 0 {
		dimID = 0
	}
	requestDimension := bwdefine.Dimension(dimID)
	worldDimension := bwdefine.Dimension(bwdefine.DimensionIDOverworld)

	paths, err := prepareExportPaths(filePath, minX, minY, minZ, maxX, maxY, maxZ)
	if err != nil {
		return "", err
	}

	_ = os.RemoveAll(paths.worldDir)
	_ = os.RemoveAll(paths.outputPath)

	bw, err := world.Open(paths.worldDir, nil)
	if err != nil {
		return "", fmt.Errorf("open world failed: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = bw.CloseWorld()
		}
	}()

	if err := prepareExportEnvironmentOptimized(client, config, protocol.BlockPos{int32(minX), int32(minY), int32(minZ)}); err != nil {
		return "", err
	}

	grantedChunkRadius, err := setChunkRadiusOptimized(client, res, config.ChunkRadius)
	if err != nil {
		return "", err
	}
	effectiveChunkRadius := resolveExportChunkRadiusOptimized(config.ChunkRadius, grantedChunkRadius)

	areas := splitExportAreaOptimized(protocol.BlockPos{int32(minX), int32(minY), int32(minZ)}, protocol.BlockPos{int32(maxX), int32(maxY), int32(maxZ)}, effectiveChunkRadius)
	totalSubChunks := calculateTotalSubChunks(areas)
	progress := newExportProgress(totalSubChunks)
	exportSucceeded := false
	if progress.bar != nil {
		defer func() {
			if exportSucceeded {
				_ = progress.bar.Finish()
			}
			fmt.Println()
		}()
	}

	worldChunkY := int32(minY >> 4)

	for _, area := range areas {
		if err := processExportAreaOptimized(client, res, bw, requestDimension, worldDimension, worldChunkY, area, config, progress); err != nil {
			return "", err
		}
	}

	bw.LevelDat().LevelName = paths.worldName
	if err := bw.UpdateLevelDat(); err != nil {
		return "", err
	}
	if err := bw.CloseWorld(); err != nil {
		return "", err
	}
	closed = true

	if err := archiveWorldAsMCWorldOptimized(paths.worldDir, paths.outputPath); err != nil {
		return "", err
	}
	_ = os.RemoveAll(paths.worldDir)
	exportSucceeded = true
	return paths.outputPath, nil
}

func calculateTotalSubChunks(areas []exportArea) int {
	total := 0
	for _, area := range areas {
		width := area.End.X() - area.Start.X() + 1
		length := area.End.Z() - area.Start.Z() + 1
		height := area.End.Y() - area.Start.Y() + 1
		if width <= 0 || length <= 0 || height <= 0 {
			continue
		}
		yCount := (height + 15) / 16
		chunkXCount := (width + 15) / 16
		chunkZCount := (length + 15) / 16
		total += int(chunkXCount * chunkZCount * yCount)
	}
	return total
}

func normalizeExportBounds(x1, y1, z1, x2, y2, z2 int) (minX, maxX, minY, maxY, minZ, maxZ int) {
	minX, maxX = x1, x2
	if x1 > x2 {
		minX, maxX = x2, x1
	}
	minY, maxY = y1, y2
	if y1 > y2 {
		minY, maxY = y2, y1
	}
	minZ, maxZ = z1, z2
	if z1 > z2 {
		minZ, maxZ = z2, z1
	}
	return
}

func prepareExportPaths(filePath string, minX, minY, minZ, maxX, maxY, maxZ int) (exportPaths, error) {
	if filepath.Ext(filePath) == "" {
		filePath += ".mcworld"
	} else if strings.ToLower(filepath.Ext(filePath)) != ".mcworld" {
		filePath = strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".mcworld"
	}
	baseDir := filepath.Dir(filePath)
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	if baseName == "" {
		return exportPaths{}, errors.New("invalid output path")
	}

	worldName := fmt.Sprintf("%s@[%d,%d,%d]~[%d,%d,%d]", baseName, minX, minY, minZ, maxX, maxY, maxZ)
	return exportPaths{
		worldName:  worldName,
		worldDir:   filepath.Join(baseDir, worldName),
		outputPath: filepath.Join(baseDir, worldName+".mcworld"),
	}, nil
}

func processExportAreaOptimized(client *clientType.Client, res *ResourcesControl.Resources, bw *world.BedrockWorld, requestDimension, worldDimension bwdefine.Dimension, worldChunkY int32, area exportArea, config *OptimizedExportConfig, progress *exportProgress) error {
	if config.Context != nil {
		select {
		case <-config.Context.Done():
			return config.Context.Err()
		default:
		}
	}

	width := area.End.X() - area.Start.X() + 1
	length := area.End.Z() - area.Start.Z() + 1
	height := area.End.Y() - area.Start.Y() + 1
	if width <= 0 || length <= 0 || height <= 0 {
		return nil
	}

	centerX := (area.Start.X() + area.End.X()) / 2
	centerZ := (area.Start.Z() + area.End.Z()) / 2
	centerPos := protocol.BlockPos{centerX, exportCenterY, centerZ}
	probePos := protocol.BlockPos{centerX, (area.Start.Y() + area.End.Y()) / 2, centerZ}
	if err := moveBotForExportOptimized(client, centerPos, probePos, config); err != nil {
		return err
	}

	yCount := (height + 15) / 16
	chunkX := area.Start.X() >> 4
	chunkZ := area.Start.Z() >> 4
	chunkXCount := (width + 15) / 16
	chunkZCount := (length + 15) / 16
	subChunkPosList := buildAreaSubChunkRequestList(chunkX, chunkZ, worldChunkY, chunkXCount, chunkZCount, yCount)
	if len(subChunkPosList) == 0 {
		return nil
	}

	return fetchAreaSubChunksOptimized(client, res, bw, requestDimension, worldDimension, centerPos, probePos, subChunkPosList, config, progress)
}

func buildAreaSubChunkRequestList(chunkX, chunkZ, worldChunkY, chunkXCount, chunkZCount, yCount int32) []protocol.SubChunkPos {
	if chunkXCount <= 0 || chunkZCount <= 0 || yCount <= 0 {
		return nil
	}
	subChunkPosList := make([]protocol.SubChunkPos, 0, chunkXCount*chunkZCount*yCount)
	for x := int32(0); x < chunkXCount; x++ {
		for z := int32(0); z < chunkZCount; z++ {
			for y := int32(0); y < yCount; y++ {
				subChunkPosList = append(subChunkPosList, protocol.SubChunkPos{
					chunkX + x,
					worldChunkY + y,
					chunkZ + z,
				})
			}
		}
	}
	return subChunkPosList
}

func saveNBTByChunk(bw *world.BedrockWorld, dimension bwdefine.Dimension, nbtsByChunk map[bwdefine.ChunkPos][]map[string]any) error {
	for chunkPos, nbtData := range nbtsByChunk {
		if len(nbtData) == 0 {
			continue
		}
		if err := bw.SaveNBT(dimension, chunkPos, nbtData); err != nil {
			return err
		}
	}
	return nil
}

func fetchAreaSubChunksOptimized(client *clientType.Client, res *ResourcesControl.Resources, bw *world.BedrockWorld, requestDimension, worldDimension bwdefine.Dimension, centerPos, probePos protocol.BlockPos, requested []protocol.SubChunkPos, config *OptimizedExportConfig, progress *exportProgress) error {
	if len(requested) == 0 {
		return nil
	}

	resp, err := fetchSubChunksOptimized(client, res, requestDimension, requested, centerPos, probePos, config)
	if err != nil {
		return err
	}

	nbtsByChunk := map[bwdefine.ChunkPos][]map[string]any{}
	processResponse := makeSubChunkResponseProcessorOptimized(worldDimension, bw, nbtsByChunk, progress)
	unfetched := processResponse(resp)
	if len(unfetched) > 0 {
		unfetched = retryMissingSubChunksOptimized(client, res, requestDimension, unfetched, centerPos, probePos, config, processResponse)
	}
	if len(unfetched) > 0 {
		progress.MarkMissing(unfetched)
	}

	return saveNBTByChunk(bw, worldDimension, nbtsByChunk)
}

func prepareExportEnvironmentOptimized(client *clientType.Client, config *OptimizedExportConfig, startPos protocol.BlockPos) error {
	if config == nil || !config.SkipOperatorProbe {
		if err := waitForExportOperatorOptimized(client, config.Context); err != nil {
			return err
		}
	}
	if err := client.GameInterface.SendAICommand("gamemode 1", true); err != nil {
		return err
	}
	return moveBotForExportOptimized(client, startPos, startPos, config)
}

func waitForExportOperatorOptimized(client *clientType.Client, ctx context.Context) error {
	if client == nil || client.GameInterface == nil {
		return errors.New("client not connected")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp := client.GameInterface.SendWSCommandWithResponse("listd", ResourcesControl.CommandRequestOptions{TimeOut: 5 * time.Second})
		if commandSucceededOptimized(resp) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func commandSucceededOptimized(resp ResourcesControl.CommandRespond) bool {
	return resp.Error == nil && resp.Respond != nil && resp.Respond.SuccessCount > 0
}

func setChunkRadiusOptimized(client *clientType.Client, res *ResourcesControl.Resources, radius int32) (int32, error) {
	listenerID, packets := res.Listener.CreateNewListen([]uint32{packet.IDChunkRadiusUpdated}, 1)
	defer res.Listener.StopAndDestroy(listenerID)
	if err := client.Conn.WritePacket(&packet.RequestChunkRadius{ChunkRadius: radius, MaxChunkRadius: uint8(radius)}); err != nil {
		return 0, err
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case pk := <-packets:
			if p, ok := pk.(*packet.ChunkRadiusUpdated); ok {
				return p.ChunkRadius, nil
			}
		case <-timer.C:
			return 0, errors.New("chunk radius updated timeout")
		}
	}
}

func resolveExportChunkRadiusOptimized(requested, granted int32) int32 {
	radius := requested
	if granted > 0 && (radius <= 0 || granted < radius) {
		radius = granted
	}
	if radius <= 0 {
		return 1
	}
	return radius
}

func awaitChangesOptimized(client *clientType.Client) error {
	if client == nil || client.GameInterface == nil {
		return nil
	}
	if gameInterface, ok := client.GameInterface.(awaitChangesCapable); ok {
		return gameInterface.AwaitChangesGeneral()
	}
	return nil
}

func moveBotForExportOptimized(client *clientType.Client, pos, probePos protocol.BlockPos, config *OptimizedExportConfig) error {
	if client == nil || client.GameInterface == nil {
		return errors.New("client not connected")
	}
	resp := sendWSCommandWithResponseDim(client, fmt.Sprintf("tp @s %d %d %d", pos.X(), pos.Y(), pos.Z()), ResourcesControl.CommandRequestOptions{TimeOut: 5 * time.Second})
	if !commandSucceededOptimized(resp) {
		if resp.Error != nil {
			return resp.Error
		}
		if resp.Respond != nil {
			return fmt.Errorf("tp failed: %v", resp.Respond.OutputMessages)
		}
		return errors.New("tp failed")
	}
	if err := awaitChangesOptimized(client); err != nil {
		return err
	}
	if config != nil && config.WaitChunkLoad {
		timeout := 8 * time.Second
		if config.SubChunkTimeout > 0 {
			timeout = maxDuration(timeout, config.SubChunkTimeout*2)
		}
		if !waitChunkLoaded(client, int(probePos.X()), int(probePos.Y()), int(probePos.Z()), nil, config.Context, timeout) {
			return fmt.Errorf("chunk load verification failed at %d %d %d", probePos.X(), probePos.Y(), probePos.Z())
		}
	}
	return nil
}

func fetchSubChunksOptimized(client *clientType.Client, res *ResourcesControl.Resources, requestDimension bwdefine.Dimension, list []protocol.SubChunkPos, centerPos, probePos protocol.BlockPos, config *OptimizedExportConfig) (*packet.SubChunk, error) {
	timeout := subChunkRequestTimeoutOptimized(config, len(list))
	for {
		if config != nil && config.Context != nil {
			select {
			case <-config.Context.Done():
				return nil, config.Context.Err()
			default:
			}
		}
		if err := moveBotForExportOptimized(client, centerPos, probePos, config); err != nil {
			return nil, err
		}
		request := buildSubChunkRequest(int32(requestDimension), list)
		resp, isTimeout, err := sendSubChunkRequestWithRespTimeoutOptimized(client, res, request, timeout)
		if err != nil {
			return nil, err
		}
		if !isTimeout {
			return resp, nil
		}
		fmt.Printf(
			"SubChunkRequest was timeout at %d %d %d (%d requested), retry...\n",
			request.Position.X(),
			request.Position.Y(),
			request.Position.Z(),
			len(list),
		)
	}
}

func sendSubChunkRequestWithRespTimeoutOptimized(client *clientType.Client, res *ResourcesControl.Resources, request *packet.SubChunkRequest, timeout time.Duration) (*packet.SubChunk, bool, error) {
	listenerID, packets := res.Listener.CreateNewListen([]uint32{packet.IDSubChunk}, 8)
	defer res.Listener.StopAndDestroy(listenerID)
	if err := client.Conn.WritePacket(request); err != nil {
		return nil, false, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case pk := <-packets:
			sub, ok := pk.(*packet.SubChunk)
			if !ok {
				continue
			}
			if sub.Dimension != request.Dimension {
				continue
			}
			if sub.Position != request.Position {
				continue
			}
			return sub, false, nil
		case <-timer.C:
			return nil, true, nil
		}
	}
}

const exportRetryDelayOptimized = 200 * time.Millisecond
const exportCenterY int32 = 320

func subChunkRequestTimeoutOptimized(config *OptimizedExportConfig, requestSize int) time.Duration {
	timeout := 1500 * time.Millisecond
	if config != nil && config.SubChunkTimeout > 0 {
		timeout = config.SubChunkTimeout
	}
	if requestSize <= 0 {
		return timeout
	}
	return timeout
}

func buildSubChunkRequest(dimension int32, subChunksPos []protocol.SubChunkPos) *packet.SubChunkRequest {
	if len(subChunksPos) == 0 {
		return &packet.SubChunkRequest{}
	}
	base := subChunksPos[0]
	offsets := make([]protocol.SubChunkOffset, 0, len(subChunksPos))
	for _, pos := range subChunksPos {
		offsetX := int8(pos.X() - base.X())
		offsetY := int8(pos.Y() - base.Y())
		offsetZ := int8(pos.Z() - base.Z())
		offsets = append(offsets, protocol.SubChunkOffset{offsetX, offsetY, offsetZ})
	}
	return &packet.SubChunkRequest{
		Dimension: dimension,
		Position:  base,
		Offsets:   offsets,
	}
}

func makeSubChunkResponseProcessorOptimized(dimension bwdefine.Dimension, bw *world.BedrockWorld, nbtsByChunk map[bwdefine.ChunkPos][]map[string]any, progress *exportProgress) func(resp *packet.SubChunk) []bwdefine.SubChunkPos {
	return func(resp *packet.SubChunk) []bwdefine.SubChunkPos {
		subChunkStartPos := resp.Position
		nextUnfetched := make([]bwdefine.SubChunkPos, 0)
		for _, entry := range resp.SubChunkEntries {
			subChunkPos := bwdefine.SubChunkPos{
				subChunkStartPos.X() + int32(entry.Offset[0]),
				subChunkStartPos.Y() + int32(entry.Offset[1]),
				subChunkStartPos.Z() + int32(entry.Offset[2]),
			}
			chunkPos := bwdefine.ChunkPos{subChunkPos.X(), subChunkPos.Z()}

			switch entry.Result {
			case protocol.SubChunkResultSuccess:
				subChunk, _, nbts, err := exportSubChunkDecodeOptimized(entry.RawPayload, dimension.Range())
				if err != nil {
					continue
				}
				if err := bw.SaveSubChunk(dimension, subChunkPos, subChunk); err != nil {
					continue
				}
				if progress != nil {
					progress.Mark(subChunkPos)
				}
				if _, ok := nbtsByChunk[chunkPos]; !ok {
					nbtsByChunk[chunkPos] = make([]map[string]any, 0)
				}
				nbtsByChunk[chunkPos] = append(nbtsByChunk[chunkPos], nbts...)
			case protocol.SubChunkResultSuccessAllAir:
				if progress != nil {
					progress.Mark(subChunkPos)
				}
			case protocol.SubChunkResultChunkNotFound:
				nextUnfetched = append(nextUnfetched, subChunkPos)
			}
		}
		return nextUnfetched
	}
}

func retryMissingSubChunksOptimized(client *clientType.Client, res *ResourcesControl.Resources, dimension bwdefine.Dimension, missing []bwdefine.SubChunkPos, centerPos, probePos protocol.BlockPos, config *OptimizedExportConfig, processResponse func(resp *packet.SubChunk) []bwdefine.SubChunkPos) []bwdefine.SubChunkPos {
	remaining := dedupeSubChunkPos(missing)
	previous := -1

	for len(remaining) > 0 {
		if config.Context != nil {
			select {
			case <-config.Context.Done():
				return remaining
			default:
			}
		}
		if len(remaining) == previous {
			break
		}
		previous = len(remaining)
		time.Sleep(exportRetryDelayOptimized)
		if err := moveBotForExportOptimized(client, centerPos, probePos, config); err != nil {
			return remaining
		}

		posList := make([]protocol.SubChunkPos, len(remaining))
		for i, pos := range remaining {
			posList[i] = protocol.SubChunkPos{pos.X(), pos.Y(), pos.Z()}
		}

		resp, err := fetchSubChunksOptimized(client, res, dimension, posList, centerPos, probePos, config)
		if err != nil {
			return remaining
		}

		remaining = processResponse(resp)
		remaining = dedupeSubChunkPos(remaining)
	}

	return remaining
}

func dedupeSubChunkPos(list []bwdefine.SubChunkPos) []bwdefine.SubChunkPos {
	unique := make(map[bwdefine.SubChunkPos]struct{}, len(list))
	for _, pos := range list {
		unique[pos] = struct{}{}
	}
	out := make([]bwdefine.SubChunkPos, 0, len(unique))
	for pos := range unique {
		out = append(out, pos)
	}
	return out
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func exportSubChunkDecodeOptimized(data []byte, r bwdefine.Range) (*bwchunk.SubChunk, int, []map[string]interface{}, error) {
	buf := bytes.NewBuffer(data)
	subChunk, index, err := bwchunk.DecodeSubChunk(buf, r, bwchunk.NetworkEncoding)
	if err != nil {
		return nil, 0, nil, err
	}

	nbts := make([]map[string]interface{}, 0)
	if buf.Len() > 0 {
		for buf.Len() != 0 {
			blockData := make(map[string]interface{})
			if err := nbt.NewDecoderWithEncoding(buf, nbt.NetworkLittleEndian).Decode(&blockData); err != nil {
				break
			}
			id, hasID := blockData["id"].(string)
			tagNBT, hasTag := blockData["__tag"].(string)
			if hasID && hasTag {
				result, err := exportTagNBTDecodeOptimized(id, tagNBT)
				if err != nil {
					continue
				}
				delete(blockData, "__tag")
				blockData = mergeExportMaps(blockData, result)
			}
			nbts = append(nbts, blockData)
		}
	}
	return subChunk, index, nbts, nil
}

func exportTagNBTDecodeOptimized(id string, tag string) (map[string]any, error) {
	blockActor, resolvedID, ok := resolveBlockActorOptimized(id)
	if ok {
		result, err := decodeTypedBlockActorOptimized(blockActor, tag)
		if err == nil {
			return result, nil
		}
		fallback, fallbackErr := decodeGenericBlockActorNBTOptimized(tag)
		if fallbackErr == nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("decode block actor %s failed: %w; fallback failed: %v", resolvedID, err, fallbackErr)
	}

	fallback, err := decodeGenericBlockActorNBTOptimized(tag)
	if err != nil {
		return nil, fmt.Errorf("unknown block actor %s and fallback decode failed: %w", id, err)
	}
	return fallback, nil
}

func resolveBlockActorOptimized(id string) (ssblockactors.BlockActors, string, bool) {
	pool := starShuttlerBlockActorPoolOptimized()
	candidates := make([]string, 0, 3)
	appendCandidate := func(candidate string) {
		if candidate == "" {
			return
		}
		for _, existing := range candidates {
			if existing == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	appendCandidate(id)
	trimmed := strings.TrimPrefix(id, "minecraft:")
	appendCandidate(trimmed)
	appendCandidate(normalizeBlockActorIDOptimized(trimmed))

	for _, candidate := range candidates {
		if actor, ok := pool[candidate]; ok {
			return actor, candidate, true
		}
	}
	return nil, "", false
}

func starShuttlerBlockActorPoolOptimized() map[string]ssblockactors.BlockActors {
	pool := ssblockactors.NewPool()
	pool[ssblockactors.IDNeteaseContainer] = &ssblockactors.NeteaseContainer{}
	return pool
}

func normalizeBlockActorIDOptimized(id string) string {
	if id == "" {
		return ""
	}
	if !strings.Contains(id, "_") {
		runes := []rune(id)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}

	parts := strings.Split(id, "_")
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, "")
}

func decodeTypedBlockActorOptimized(blockActor ssblockactors.BlockActors, tag string) (result map[string]any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("panic decoding typed block actor: %v", r)
		}
	}()

	buffer := bytes.NewBuffer([]byte(tag))
	reader := ssprotocol.NewReader(buffer, 0, false)
	blockActor.Marshal(reader)
	if err := mapstructure.Decode(blockActor, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func decodeGenericBlockActorNBTOptimized(tag string) (map[string]any, error) {
	fallback := make(map[string]any)
	if err := wsnbt.UnmarshalEncoding([]byte(tag), &fallback, wsnbt.NetworkLittleEndian); err == nil {
		return fallback, nil
	}
	if err := nbt.UnmarshalEncoding([]byte(tag), &fallback, nbt.NetworkLittleEndian); err == nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("generic block actor fallback decode failed")
}

func splitExportAreaOptimized(start, end protocol.BlockPos, chunkRadius int32) []exportArea {
	blockRadius := (chunkRadius - 1) * 15 / 10 * 16
	if blockRadius < 16 {
		blockRadius = 16
	}
	minX, maxX := start.X(), end.X()
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := start.Y(), end.Y()
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	minZ, maxZ := start.Z(), end.Z()
	if minZ > maxZ {
		minZ, maxZ = maxZ, minZ
	}

	length := maxX - minX + 1
	width := maxZ - minZ + 1

	xSteps := length / blockRadius
	zSteps := width / blockRadius
	xRemainder := length % blockRadius
	zRemainder := width % blockRadius

	areas := make([]exportArea, 0)
	for xIdx := int32(0); xIdx < xSteps; xIdx++ {
		for zIdx := int32(0); zIdx < zSteps; zIdx++ {
			startX := minX + xIdx*blockRadius
			startZ := minZ + zIdx*blockRadius
			endX := startX + blockRadius - 1
			endZ := startZ + blockRadius - 1
			areas = append(areas, exportArea{
				Start: protocol.BlockPos{startX, minY, startZ},
				End:   protocol.BlockPos{endX, maxY, endZ},
			})
		}
	}

	if xRemainder > 0 {
		for zIdx := int32(0); zIdx < zSteps; zIdx++ {
			startX := minX + xSteps*blockRadius
			startZ := minZ + zIdx*blockRadius
			endX := startX + xRemainder - 1
			endZ := startZ + blockRadius - 1
			areas = append(areas, exportArea{
				Start: protocol.BlockPos{startX, minY, startZ},
				End:   protocol.BlockPos{endX, maxY, endZ},
			})
		}
	}

	if zRemainder > 0 {
		for xIdx := int32(0); xIdx < xSteps; xIdx++ {
			startX := minX + xIdx*blockRadius
			startZ := minZ + zSteps*blockRadius
			endX := startX + blockRadius - 1
			endZ := startZ + zRemainder - 1
			areas = append(areas, exportArea{
				Start: protocol.BlockPos{startX, minY, startZ},
				End:   protocol.BlockPos{endX, maxY, endZ},
			})
		}
	}

	if xRemainder > 0 && zRemainder > 0 {
		startX := minX + xSteps*blockRadius
		startZ := minZ + zSteps*blockRadius
		endX := startX + xRemainder - 1
		endZ := startZ + zRemainder - 1
		areas = append(areas, exportArea{
			Start: protocol.BlockPos{startX, minY, startZ},
			End:   protocol.BlockPos{endX, maxY, endZ},
		})
	}

	if len(areas) == 0 {
		areas = append(areas, exportArea{
			Start: protocol.BlockPos{minX, minY, minZ},
			End:   protocol.BlockPos{maxX, maxY, maxZ},
		})
	}
	return areas
}

func archiveWorldAsMCWorldOptimized(worldDir, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file failed: %w", err)
	}
	bufWriter := bufio.NewWriterSize(outFile, 256*1024)
	zipWriter := zip.NewWriter(bufWriter)

	walkErr := filepath.Walk(worldDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(worldDir, path)
		if err != nil {
			return err
		}

		if !info.IsDir() && isSensitiveFile(info.Name()) {
			return nil
		}

		if info.IsDir() {
			if relPath != "." {
				_, err := zipWriter.Create(relPath + "/")
				return err
			}
			return nil
		}
		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
	if walkErr != nil {
		zipWriter.Close()
		bufWriter.Flush()
		outFile.Close()
		return walkErr
	}
	if err := zipWriter.Close(); err != nil {
		bufWriter.Flush()
		outFile.Close()
		return err
	}
	if err := bufWriter.Flush(); err != nil {
		outFile.Close()
		return err
	}
	return outFile.Close()
}
