package function

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	types "nexus/defines"
	ResourcesControl "nexus/utils/api/resources_control"
	"nexus/utils/chunk_fill"
	clientType "nexus/utils/client"
	"nexus/utils/log"
	mirrorchunk "nexus/utils/mirror/chunk"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	bwo_define "github.com/TriM-Organization/bedrock-world-operator/define"
	wsdefine "github.com/Yeah114/WaterStructure/define"
	wsstructure "github.com/Yeah114/WaterStructure/structure"
	wsutils "github.com/Yeah114/WaterStructure/utils"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

const (
	verifyChunkLevelNone = iota + 1
	verifyChunkLevelAir
	verifyChunkLevelSimilar
	verifyChunkLevelComplete
)

const (
	defaultVerifyAfterChunk = 1
	defaultVerifyLevel      = verifyChunkLevelAir
	minMatchingDegree       = 80.0
	maxFixDepth             = 2
	tickingAreaServerLimit  = 10
	tickingAreaListRetry    = 5 * time.Second
)

var errTickingAreaListTimeout = errors.New("tickingarea list timeout")

type buildCommandAction struct {
	hasMove bool
	moveTo  types.Position
	command string
}

type buildChunkGroup struct {
	groupIndex int
	groupPos   wsdefine.ChunkPos

	chunks         map[wsdefine.ChunkPos]*chunk.Chunk
	chunkPositions []wsdefine.ChunkPos
	chunksEmpty    bool

	blockBuildChunks         map[wsdefine.ChunkPos]*chunk.Chunk
	blockBuildChunkPositions []wsdefine.ChunkPos
	nbtLists                 map[wsdefine.ChunkPos]map[wsdefine.BlockPos]map[string]any

	minChunkPosX int32
	maxChunkPosX int32
	minChunkPosZ int32
	maxChunkPosZ int32

	worldChunkX int32
	worldChunkY int32
	worldChunkZ int32

	realChunkPos wsdefine.ChunkPos

	blockActions      []buildCommandAction
	blockCommandCount int64
}

type buildGroupFuture struct {
	ready chan struct{}
	group *buildChunkGroup
	err   error
}

type buildGroupRaw struct {
	index       int
	groupPos    wsdefine.ChunkPos
	chunks      map[wsdefine.ChunkPos]*chunk.Chunk
	worldChunkX int32
	worldChunkY int32
	worldChunkZ int32
	err         error
}

func newBuildGroupFuture() *buildGroupFuture {
	return &buildGroupFuture{ready: make(chan struct{})}
}

func (f *buildGroupFuture) resolve(group *buildChunkGroup, err error) {
	f.group = group
	f.err = err
	close(f.ready)
}

func (f *buildGroupFuture) Wait(ctx context.Context) (*buildChunkGroup, error) {
	if f == nil {
		return nil, nil
	}
	select {
	case <-f.ready:
		return f.group, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type chunkGroupManager struct {
	reader            wsstructure.Structure
	chunkPosGenerator func(*chunkGroupManager, int) wsdefine.ChunkPos
	progress          int
	max               int
	chunkGroupSide    int
}

func getGroupIndex(pos int32, groupSide int) int {
	return int(math.Floor(float64(pos) / float64(groupSide)))
}

func getChunkPosInGroup(groupIdx, offset, groupSide int) int32 {
	return int32(groupIdx*groupSide + offset)
}

func getGroupPosByChunkPos(chunkPos wsdefine.ChunkPos, groupSide int) wsdefine.ChunkPos {
	groupCX := getGroupIndex(chunkPos.X(), groupSide)
	groupCZ := getGroupIndex(chunkPos.Z(), groupSide)
	return wsdefine.ChunkPos{int32(groupCX), int32(groupCZ)}
}

func snakeGroupPosByIndex(size wsdefine.Size, groupSide int, n int) wsdefine.ChunkPos {
	xChunkNum := size.GetChunkXCount()
	if groupSide <= 0 {
		groupSide = 1
	}
	xGroupNum := int(math.Ceil(float64(xChunkNum) / float64(groupSide)))
	groupCZ := n / xGroupNum
	groupRowOffset := floorMod(n, xGroupNum)
	groupCX := groupRowOffset
	if groupCZ%2 != 0 {
		groupCX = xGroupNum - 1 - groupRowOffset
	}
	return wsdefine.ChunkPos{int32(groupCX), int32(groupCZ)}
}

func snakeChunkPosGenerator(cm *chunkGroupManager, n int) wsdefine.ChunkPos {
	return snakeGroupPosByIndex(cm.reader.GetSize(), cm.chunkGroupSide, n)
}

func newChunkGroupManager(reader wsstructure.Structure, progress, chunkGroupSide int) *chunkGroupManager {
	size := reader.GetSize()
	if chunkGroupSide <= 0 {
		chunkGroupSide = 1
	}
	chunkXGroups := (size.GetChunkXCount() + chunkGroupSide - 1) / chunkGroupSide
	chunkZGroups := (size.GetChunkZCount() + chunkGroupSide - 1) / chunkGroupSide
	return &chunkGroupManager{
		reader:            reader,
		chunkPosGenerator: snakeChunkPosGenerator,
		progress:          progress,
		max:               chunkXGroups * chunkZGroups,
		chunkGroupSide:    chunkGroupSide,
	}
}

func (c *chunkGroupManager) parseChunkGroup() (map[wsdefine.ChunkPos]*chunk.Chunk, error) {
	if c.progress >= c.max {
		return nil, nil
	}
	groupPos := c.chunkPosGenerator(c, c.progress)
	c.progress++

	allChunkPositions := make([]wsdefine.ChunkPos, 0)
	groupSide := c.chunkGroupSide
	size := c.reader.GetSize()
	maxCX := int32(size.GetChunkXCount())
	maxCZ := int32(size.GetChunkZCount())
	for zOffset := 0; zOffset < groupSide; zOffset++ {
		for xOffset := 0; xOffset < groupSide; xOffset++ {
			cx := getChunkPosInGroup(int(groupPos.X()), xOffset, groupSide)
			cz := getChunkPosInGroup(int(groupPos.Z()), zOffset, groupSide)
			if cx >= 0 && cx < maxCX && cz >= 0 && cz < maxCZ {
				allChunkPositions = append(allChunkPositions, wsdefine.ChunkPos{cx, cz})
			}
		}
	}
	if len(allChunkPositions) == 0 {
		return nil, nil
	}
	return c.reader.GetChunks(allChunkPositions)
}

func (c *chunkGroupManager) GetChunks() (wsdefine.ChunkPos, map[wsdefine.ChunkPos]*chunk.Chunk, error) {
	chunks, err := c.parseChunkGroup()
	var pos wsdefine.ChunkPos
	for p := range chunks {
		pos = p
		break
	}
	return getGroupPosByChunkPos(pos, c.chunkGroupSide), chunks, err
}

type importBuilder struct {
	client            *clientType.Client
	reader            wsstructure.Structure
	task              types.Task
	ctx               context.Context
	limiter           *rate.Limiter
	attachmentLimiter *rate.Limiter
	bar               *progressbar.ProgressBar
	gameProgress      *ImportGameProgress
	progressCb        func(processed int, total int)

	chunkManager      *chunkGroupManager
	readerGetChunksMu sync.Mutex

	buildStartPos    types.Position
	originalStartPos types.Position
	chunkGroupSide   int
	totalGroups      int
	processedGroups  int
	verifyAfterChunk int
	verifyChunkLevel int

	tickingAreaRecords map[wsdefine.ChunkPos][]string
	tickingAreaOwned   map[string]struct{}
	tickingAreaCreated int
	tickingAreaOff     bool
	preloadOff         bool
	tickingAreaMu      sync.Mutex
}

func (b *importBuilder) shouldWaitChunkLoad() bool {
	if b == nil {
		return false
	}
	return b.task.UseFill || b.task.RegionSize > 0
}

func (b *importBuilder) shouldWaitPreloadBeforeNBT() bool {
	if b == nil {
		return false
	}
	return b.task.ImportNBT || b.task.ImportCommandBlock
}

func (b *importBuilder) prepareTickingAreaRuntimeStrategy() error {
	if b.shouldWaitChunkLoad() {
		if err := b.waitForControl(); err != nil {
			return err
		}
		b.tickingAreaMu.Lock()
		defer b.tickingAreaMu.Unlock()
		snapshot, err := b.waitTickingAreaSnapshotLocked()
		if err != nil {
			return err
		}
		if snapshot.available == 0 && b.tickingAreaCreated == 0 && snapshot.owned == 0 {
			b.disableTickingAreaRuntimeLocked()
		}
	}
	return nil
}

func (b *importBuilder) waitForControl() error {
	if importContextDone(b.ctx) {
		return b.ctx.Err()
	}
	return clientConnError(b.client)
}

func (b *importBuilder) moveBot(pos types.Position) error {
	if err := b.waitForControl(); err != nil {
		return err
	}
	return sendSettingsCommandDim(b.client, fmt.Sprintf("tp @s %d %d %d", pos.X, pos.Y, pos.Z))
}

func (b *importBuilder) sendSettingsCommand(cmd string, dimensional bool) error {
	if err := b.waitForControl(); err != nil {
		return err
	}
	if b.limiter != nil {
		if err := b.limiter.Wait(b.ctx); err != nil {
			return err
		}
	}
	if strings.Contains(cmd, `"upper_block_bit"=false`) {
		return nil
	}
	if strings.Contains(cmd, `"upper_block_bit"=true`) {
		cmd = fixDoorCommand(cmd)
	}
	if dimensional {
		return b.client.GameInterface.SendSettingsCommand(b.client.WrapCommandInDimension(cmd), true)
	}
	return sendSettingsCommandDim(b.client, cmd)
}

func (b *importBuilder) effectiveChunkGroupSide() int {
	if b.chunkGroupSide <= 0 {
		return 1
	}
	return b.chunkGroupSide
}

func (b *importBuilder) getChunkLoadBounds(chunkPos wsdefine.ChunkPos) (startX, y, startZ, endX, endZ int32) {
	startX = chunkPos.X() * 16
	y = int32(b.buildStartPos.Y - floorMod(b.buildStartPos.Y, 16))
	y = clampProbeYForDimension(y, b.client)
	startZ = chunkPos.Z() * 16
	endX = startX + 15
	endZ = startZ + 15
	if side := b.effectiveChunkGroupSide(); side != 1 {
		width := int32(side * 16)
		endX = startX + width - 1
		endZ = startZ + width - 1
	}
	return startX, y, startZ, endX, endZ
}

func clampProbeYForDimension(y int32, client *clientType.Client) int32 {
	minY, maxY := int32(-63), int32(319)
	if client != nil {
		switch client.DimensionID {
		case 1:
			minY, maxY = 1, 127
		case 2, 3:
			minY, maxY = 1, 255
		}
	}
	if y < minY {
		return minY
	}
	if y > maxY {
		return maxY
	}
	return y
}

type buildTickingAreaSnapshot struct {
	total     int
	owned     int
	available int
}

func (b *importBuilder) listTickingAreasLocked() (buildTickingAreaSnapshot, error) {
	resp, isTimeout, err := sendWSCommandWithTimeoutDim(b.client, "tickingarea list", 3*time.Second)
	if err != nil {
		return buildTickingAreaSnapshot{}, err
	}
	if isTimeout {
		return buildTickingAreaSnapshot{}, errTickingAreaListTimeout
	}
	nameSet := make(map[string]struct{})
	lineRegexp := regexp.MustCompile(`(?m)^-\s*([^:\r\n]+):`)
	if resp.Respond != nil {
		for _, msg := range resp.Respond.OutputMessages {
			for _, match := range lineRegexp.FindAllStringSubmatch(msg.Message, -1) {
				if len(match) < 2 {
					continue
				}
				name := strings.TrimSpace(match[1])
				if name != "" {
					nameSet[name] = struct{}{}
				}
			}
		}
	}
	owned := 0
	for name := range nameSet {
		if _, ok := b.tickingAreaOwned[name]; ok {
			owned++
		}
	}
	available := tickingAreaServerLimit - len(nameSet)
	if available < 0 {
		available = 0
	}
	return buildTickingAreaSnapshot{total: len(nameSet), owned: owned, available: available}, nil
}

func (b *importBuilder) waitTickingAreaSnapshotLocked() (buildTickingAreaSnapshot, error) {
	for {
		snapshot, err := b.listTickingAreasLocked()
		if !errors.Is(err, errTickingAreaListTimeout) {
			return snapshot, err
		}
		log.Log.Warn("tickingarea list timeout; retry after 5 seconds")
		select {
		case <-time.After(tickingAreaListRetry):
		case <-b.ctx.Done():
			return buildTickingAreaSnapshot{}, b.ctx.Err()
		}
		if err := b.waitForControl(); err != nil {
			return buildTickingAreaSnapshot{}, err
		}
	}
}

func (b *importBuilder) disableTickingAreaRuntimeLocked() {
	b.tickingAreaOff = true
	b.preloadOff = true
	b.chunkGroupSide = 1
}

func (b *importBuilder) waitTickingAreaCapacityLocked(required int) error {
	if required <= 0 {
		required = 1
	}
	for {
		if err := b.waitForControl(); err != nil {
			return err
		}
		snapshot, err := b.waitTickingAreaSnapshotLocked()
		if err != nil {
			return err
		}
		if snapshot.available >= required {
			return nil
		}
		if b.tickingAreaCreated == 0 && snapshot.owned == 0 {
			b.disableTickingAreaRuntimeLocked()
			return nil
		}
		select {
		case <-time.After(500 * time.Millisecond):
		case <-b.ctx.Done():
			return b.ctx.Err()
		}
	}
}

func (b *importBuilder) prepareChunkLoadTickingArea(chunkPos wsdefine.ChunkPos) error {
	if err := b.waitForControl(); err != nil {
		return err
	}
	b.tickingAreaMu.Lock()
	defer b.tickingAreaMu.Unlock()
	if len(b.tickingAreaRecords[chunkPos]) > 0 {
		return nil
	}
	if err := b.waitTickingAreaCapacityLocked(1); err != nil {
		return err
	}
	if b.tickingAreaOff {
		return nil
	}
	startX, y, startZ, endX, endZ := b.getChunkLoadBounds(chunkPos)
	name := ""
	for {
		name = fmt.Sprintf("%s%06d", constantsName(), rand.Intn(1000000))
		resp, isTimeout, err := sendWSCommandWithTimeoutDim(
			b.client,
			fmt.Sprintf("tickingarea add %d %d %d %d %d %d \"%s\" true", startX, y, startZ, endX, y, endZ, name),
			500*time.Millisecond,
		)
		if isTimeout {
			if b.tickingAreaOwned == nil {
				b.tickingAreaOwned = make(map[string]struct{})
			}
			b.tickingAreaOwned[name] = struct{}{}
			b.tickingAreaCreated++
			go b.releaseTickingAreaName(name)
			if err := b.waitTickingAreaCapacityLocked(1); err != nil {
				return err
			}
			if b.tickingAreaOff {
				return nil
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to add tickingarea: %w", err)
		}
		msg := commandFailedMessage(resp)
		if msg != "" {
			switch msg {
			case "commands.tickingarea-add.failure":
				if err := b.waitTickingAreaCapacityLocked(1); err != nil {
					return err
				}
				if b.tickingAreaOff {
					return nil
				}
				continue
			case "commands.tickingarea-add.conflictingname":
				continue
			default:
				return fmt.Errorf("tickingarea add failed: %s", msg)
			}
		}
		break
	}
	b.tickingAreaRecords[chunkPos] = append(b.tickingAreaRecords[chunkPos], name)
	if b.tickingAreaOwned == nil {
		b.tickingAreaOwned = make(map[string]struct{})
	}
	b.tickingAreaOwned[name] = struct{}{}
	b.tickingAreaCreated++
	return nil
}

func constantsName() string {
	return "NexusEgo"
}

func (b *importBuilder) releaseTickingAreaName(name string) {
	removeNamedTickingArea(b.client, name)
	b.tickingAreaMu.Lock()
	delete(b.tickingAreaOwned, name)
	b.tickingAreaMu.Unlock()
}

func (b *importBuilder) waitChunkAccessible(chunkPos wsdefine.ChunkPos) error {
	startX, y, startZ, endX, endZ := b.getChunkLoadBounds(chunkPos)
	lastNotice := time.Now()
	for {
		if err := b.moveBot(types.Position{X: int(startX), Y: int(y), Z: int(startZ)}); err != nil {
			return err
		}
		resp, isTimeout, err := sendWSCommandWithTimeoutDim(
			b.client,
			fmt.Sprintf("fill %d %d %d %d %d %d air keep", startX, y, startZ, endX, y, endZ),
			500*time.Millisecond,
		)
		if !isTimeout && err != nil {
			return err
		}
		if !isTimeout && resp.Respond != nil && len(resp.Respond.OutputMessages) == 1 &&
			resp.Respond.OutputMessages[0].Message != "commands.fill.outOfWorld" {
			return nil
		}
		if time.Since(lastNotice) >= 5*time.Second {
			msg := ""
			status := "timeout"
			if resp.Respond != nil && len(resp.Respond.OutputMessages) > 0 {
				msg = resp.Respond.OutputMessages[0].Message
			}
			if !isTimeout {
				status = "response"
			}
			log.Log.Warn("区块探测仍未成功，继续等待", log.Log.ArgsFromMap(map[string]any{
				"chunk":   chunkPos,
				"status":  status,
				"timeout": isTimeout,
				"message": msg,
				"x":       startX,
				"y":       y,
				"z":       startZ,
				"endX":    endX,
				"endZ":    endZ,
			}))
			lastNotice = time.Now()
		}
		if importContextDone(b.ctx) {
			return b.ctx.Err()
		}
	}
}

func (b *importBuilder) waitChunkLoad(chunkPos wsdefine.ChunkPos) error {
	if b == nil {
		return nil
	}
	if b.shouldWaitChunkLoad() && !b.tickingAreaOff {
		if err := b.prepareChunkLoadTickingArea(chunkPos); err != nil {
			return err
		}
	}
	return b.waitChunkAccessible(chunkPos)
}

func (b *importBuilder) preWaitChunkLoad(chunkPos wsdefine.ChunkPos) error {
	err := b.waitChunkLoad(chunkPos)
	if err == nil {
		return nil
	}
	if !b.tickingAreaOff {
		b.clearChunkLoadTickingArea(chunkPos)
	}
	return err
}

func (b *importBuilder) clearChunkLoadTickingArea(chunkPos wsdefine.ChunkPos) {
	b.tickingAreaMu.Lock()
	names := append([]string(nil), b.tickingAreaRecords[chunkPos]...)
	delete(b.tickingAreaRecords, chunkPos)
	b.tickingAreaMu.Unlock()
	for _, name := range names {
		b.releaseTickingAreaName(name)
	}
}

func summarizeChunkGroup(chunks map[wsdefine.ChunkPos]*chunk.Chunk) ([]wsdefine.ChunkPos, int32, int32, int32, int32, bool) {
	if len(chunks) == 0 {
		return nil, 0, 0, 0, 0, true
	}
	minChunkPosX := int32(math.MaxInt32)
	maxChunkPosX := int32(math.MinInt32)
	minChunkPosZ := int32(math.MaxInt32)
	maxChunkPosZ := int32(math.MinInt32)
	chunkPositions := make([]wsdefine.ChunkPos, 0, len(chunks))
	chunksEmpty := true
	for chunkPos, c := range chunks {
		chunkPositions = append(chunkPositions, chunkPos)
		if chunkPos.X() < minChunkPosX {
			minChunkPosX = chunkPos.X()
		}
		if chunkPos.X() > maxChunkPosX {
			maxChunkPosX = chunkPos.X()
		}
		if chunkPos.Z() < minChunkPosZ {
			minChunkPosZ = chunkPos.Z()
		}
		if chunkPos.Z() > maxChunkPosZ {
			maxChunkPosZ = chunkPos.Z()
		}
		if c == nil || !chunksEmpty {
			continue
		}
		for _, subChunk := range c.Sub() {
			if subChunk != nil && !subChunk.Empty() {
				chunksEmpty = false
				break
			}
		}
	}
	return chunkPositions, minChunkPosX, maxChunkPosX, minChunkPosZ, maxChunkPosZ, chunksEmpty
}

func (b *importBuilder) buildChunkGroupActions(group *buildChunkGroup) []buildCommandAction {
	if group == nil || len(group.blockBuildChunks) == 0 {
		return nil
	}
	if !b.task.UseFill {
		actions := make([]buildCommandAction, 0)
		for _, chunkPos := range group.blockBuildChunkPositions {
			c := group.blockBuildChunks[chunkPos]
			if c == nil {
				continue
			}
			chunkWorldPosX := group.worldChunkX + (chunkPos.X()-group.minChunkPosX)*16
			chunkWorldPosZ := group.worldChunkZ + (chunkPos.Z()-group.minChunkPosZ)*16
			for subChunkY, subChunk := range c.Sub() {
				if subChunk == nil || subChunk.Empty() {
					continue
				}
				layerBaseY := group.worldChunkY + int32(subChunkY)*16
				actions = append(actions, buildCommandAction{
					hasMove: true,
					moveTo: types.Position{
						X: int(chunkWorldPosX),
						Y: int(layerBaseY),
						Z: int(chunkWorldPosZ),
					},
				})
				layer := subChunk.Layer(0)
				for x := byte(0); x < 16; x++ {
					for y := byte(0); y < 16; y++ {
						for z := byte(0); z < 16; z++ {
							runtimeID := layer.At(x, y, z)
							if runtimeID == block.AirRuntimeID {
								continue
							}
							name, properties, _ := block.RuntimeIDToState(runtimeID)
							stateStr := wsutils.PropertiesToStateStr(properties)
							actions = append(actions, buildCommandAction{
								command: fmt.Sprintf(
									"setblock %d %d %d %s %s",
									chunkWorldPosX+int32(x),
									layerBaseY+int32(y),
									chunkWorldPosZ+int32(z),
									name,
									stateStr,
								),
							})
						}
					}
				}
			}
		}
		return actions
	}
	commands := collectProgressCommands(chunk_fill.GenerateChunksCommand(
		toBWOChunks(group.blockBuildChunks),
		types.Position{X: int(group.worldChunkX), Y: int(group.worldChunkY), Z: int(group.worldChunkZ)},
	))
	actions := make([]buildCommandAction, 0, len(commands))
	for _, command := range commands {
		actions = append(actions, buildCommandAction{command: command})
	}
	return actions
}

func toBWOChunks(in map[wsdefine.ChunkPos]*chunk.Chunk) map[bwo_define.ChunkPos]*chunk.Chunk {
	out := make(map[bwo_define.ChunkPos]*chunk.Chunk, len(in))
	for pos, c := range in {
		out[bwo_define.ChunkPos{pos.X(), pos.Z()}] = c
	}
	return out
}

func countBuildCommandActions(actions []buildCommandAction) int64 {
	total := int64(0)
	for _, action := range actions {
		if action.command != "" {
			total++
		}
	}
	return total
}

func (b *importBuilder) prepareChunkGroupBuildPlan(group *buildChunkGroup) error {
	if group == nil {
		return nil
	}
	start := time.Now()
	group.blockBuildChunks = group.chunks
	group.blockBuildChunkPositions = append(group.blockBuildChunkPositions[:0], group.chunkPositions...)
	if b.task.ImportNBT || b.task.ImportCommandBlock {
		chunksNBTs, err := b.reader.GetChunksNBT(group.chunkPositions)
		if err != nil {
			return err
		}
		group.nbtLists = chunksNBTs
	}
	group.blockActions = b.buildChunkGroupActions(group)
	group.blockCommandCount = countBuildCommandActions(group.blockActions)
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		log.Log.Warn("区块组命令预生成耗时较长", log.Log.ArgsFromMap(map[string]any{
			"group":    group.groupIndex,
			"chunk":    group.realChunkPos,
			"commands": group.blockCommandCount,
			"elapsed":  elapsed.String(),
		}))
	}
	return nil
}

func (b *importBuilder) makeChunkGroupRaw(groupIndex int, groupPos wsdefine.ChunkPos, chunks map[wsdefine.ChunkPos]*chunk.Chunk, worldChunkX, worldChunkY, worldChunkZ int32) *buildChunkGroup {
	chunkPositions, minChunkPosX, maxChunkPosX, minChunkPosZ, maxChunkPosZ, chunksEmpty := summarizeChunkGroup(chunks)
	return &buildChunkGroup{
		groupIndex:     groupIndex,
		groupPos:       groupPos,
		chunks:         chunks,
		chunkPositions: chunkPositions,
		chunksEmpty:    chunksEmpty,
		minChunkPosX:   minChunkPosX,
		maxChunkPosX:   maxChunkPosX,
		minChunkPosZ:   minChunkPosZ,
		maxChunkPosZ:   maxChunkPosZ,
		worldChunkX:    worldChunkX,
		worldChunkY:    worldChunkY,
		worldChunkZ:    worldChunkZ,
		realChunkPos:   wsdefine.ChunkPos{worldChunkX / 16, worldChunkZ / 16},
	}
}

func (b *importBuilder) makeChunkGroup(groupIndex int, groupPos wsdefine.ChunkPos, chunks map[wsdefine.ChunkPos]*chunk.Chunk, worldChunkX, worldChunkY, worldChunkZ int32) (*buildChunkGroup, error) {
	group := b.makeChunkGroupRaw(groupIndex, groupPos, chunks, worldChunkX, worldChunkY, worldChunkZ)
	if err := b.prepareChunkGroupBuildPlan(group); err != nil {
		return nil, err
	}
	return group, nil
}

func (b *importBuilder) readNextChunkGroupRaw(groupIndex int, realStartChunkX, realStartChunkY, realStartChunkZ int32) buildGroupRaw {
	if b.chunkManager == nil {
		return buildGroupRaw{index: groupIndex}
	}
	b.readerGetChunksMu.Lock()
	groupPos, chunks, err := b.chunkManager.GetChunks()
	b.readerGetChunksMu.Unlock()
	if err != nil || chunks == nil {
		return buildGroupRaw{index: groupIndex, err: err}
	}
	side := b.effectiveChunkGroupSide()
	worldChunkX := realStartChunkX + groupPos.X()*int32(side)*16
	worldChunkZ := realStartChunkZ + groupPos.Z()*int32(side)*16
	return buildGroupRaw{
		index:       groupIndex,
		groupPos:    groupPos,
		chunks:      chunks,
		worldChunkX: worldChunkX,
		worldChunkY: realStartChunkY,
		worldChunkZ: worldChunkZ,
	}
}

func (b *importBuilder) loadNextChunkGroup(groupIndex int, realStartChunkX, realStartChunkY, realStartChunkZ int32) (*buildChunkGroup, error) {
	raw := b.readNextChunkGroupRaw(groupIndex, realStartChunkX, realStartChunkY, realStartChunkZ)
	if raw.err != nil || raw.chunks == nil {
		return nil, raw.err
	}
	return b.makeChunkGroup(raw.index, raw.groupPos, raw.chunks, raw.worldChunkX, raw.worldChunkY, raw.worldChunkZ)
}

func (b *importBuilder) startPreHandleNextChunkGroup(ctx context.Context, groupIndex int, realStartChunkX, realStartChunkY, realStartChunkZ int32) *buildGroupFuture {
	future := newBuildGroupFuture()
	go func() {
		group, err := b.loadNextChunkGroup(groupIndex, realStartChunkX, realStartChunkY, realStartChunkZ)
		select {
		case <-ctx.Done():
			future.resolve(nil, ctx.Err())
		default:
			future.resolve(group, err)
		}
	}()
	return future
}

func (b *importBuilder) startPreWaitNextChunkLoad(chunkPos wsdefine.ChunkPos) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if b.preloadOff {
			return
		}
		_ = b.preWaitChunkLoad(chunkPos)
	}()
	return done
}

func (b *importBuilder) startPreWaitNextChunkLoadFromFuture(ctx context.Context, future *buildGroupFuture) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if b.preloadOff {
			return
		}
		group, err := future.Wait(ctx)
		if err != nil || group == nil || group.chunksEmpty {
			return
		}
		_ = b.preWaitChunkLoad(group.realChunkPos)
	}()
	return done
}

func (b *importBuilder) waitPreWaitNextChunkLoad(done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-b.ctx.Done():
		return b.ctx.Err()
	}
}

func (b *importBuilder) buildChunkGroupBlocks(group *buildChunkGroup) error {
	if err := b.waitForControl(); err != nil {
		return err
	}
	if group == nil || len(group.blockBuildChunks) == 0 {
		return nil
	}
	groupCommandCurrent := 0
	if b.gameProgress != nil {
		b.gameProgress.SetChunkGroupProgress(0, int(group.blockCommandCount))
	}
	if b.task.ClearArea {
		widthX := int((group.maxChunkPosX - group.minChunkPosX + 1) * 16)
		widthZ := int((group.maxChunkPosZ - group.minChunkPosZ + 1) * 16)
		height := b.reader.GetSize().Height
		endY := int(group.worldChunkY) + height - floorMod(height, 16) + 16
		for _, cmd := range buildClearCommandsLayered(int(group.worldChunkX), int(group.worldChunkY), int(group.worldChunkZ), int(group.worldChunkX)+widthX-1, endY, int(group.worldChunkZ)+widthZ-1) {
			if err := b.sendSettingsCommand(cmd.cmd, false); err != nil {
				return fmt.Errorf("failed to clean chunk: %w", err)
			}
		}
	}
	if b.task.AutoPlaceBorder {
		if err := b.placeBorderRingForChunks(group.blockBuildChunks, group.worldChunkX, group.worldChunkZ); err != nil {
			return err
		}
	}
	if b.task.AutoPlaceDenyBlock {
		for _, chunkPos := range group.blockBuildChunkPositions {
			chunkWorldPosX := group.worldChunkX + (chunkPos.X()-group.minChunkPosX)*16
			chunkWorldPosZ := group.worldChunkZ + (chunkPos.Z()-group.minChunkPosZ)*16
			if err := b.placeDenyLayerForChunk(chunkWorldPosX, chunkWorldPosZ); err != nil {
				return err
			}
		}
	}
	for _, action := range group.blockActions {
		if action.hasMove {
			if err := b.moveBot(action.moveTo); err != nil {
				return err
			}
			continue
		}
		if err := b.sendSettingsCommand(action.command, false); err != nil {
			return fmt.Errorf("failed to send command: %w", err)
		}
		if b.gameProgress != nil {
			groupCommandCurrent++
			b.gameProgress.AddBlockProgress(1)
			b.gameProgress.SetChunkGroupProgress(groupCommandCurrent, int(group.blockCommandCount))
		}
	}
	return nil
}

func (b *importBuilder) buildChunkGroupNBT(group *buildChunkGroup) error {
	if group == nil || len(group.chunks) == 0 || (!b.task.ImportNBT && !b.task.ImportCommandBlock) {
		return nil
	}
	for _, chunkPos := range group.chunkPositions {
		c := group.chunks[chunkPos]
		if c == nil {
			continue
		}
		nbtList := group.nbtLists[chunkPos]
		for nbtBlockPos, nbt := range nbtList {
			runtimeID := c.Block(uint8(nbtBlockPos.X()), int16(nbtBlockPos.Y()), uint8(nbtBlockPos.Z()), 0)
			module, attachmentType := buildRuntimeAttachmentImportModule(
				runtimeID,
				types.Position{
					X: int(group.worldChunkX + (chunkPos.X()-group.minChunkPosX)*16 + nbtBlockPos.X()),
					Y: int(group.worldChunkY + nbtBlockPos.Y() + 64),
					Z: int(group.worldChunkZ + (chunkPos.Z()-group.minChunkPosZ)*16 + nbtBlockPos.Z()),
				},
				nbt,
				b.task,
			)
			if module == nil || attachmentType == "" {
				continue
			}
			limiter := b.attachmentLimiter
			if limiter == nil {
				limiter = b.limiter
			}
			if err := importAttachmentModule(b.client, module, attachmentType, limiter, b.ctx, b.gameProgress, b.task); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildRuntimeAttachmentImportModule(runtimeID uint32, pos types.Position, nbt map[string]any, task types.Task) (*types.Module, string) {
	if runtimeID == block.AirRuntimeID {
		return nil, ""
	}
	name, state, found := convertRuntimeToNameState(runtimeID)
	if !found {
		return nil, ""
	}
	name = strings.TrimPrefix(name, "minecraft:")
	return buildAttachmentImportModule(
		name,
		state,
		pos,
		nbt,
		task.ImportNBT,
		task.ImportCommandBlock,
		task.DefaultSignWax,
	)
}

func countChunkGroupAttachments(group *buildChunkGroup, task types.Task) importAttachmentCounts {
	var counts importAttachmentCounts
	if group == nil || len(group.chunks) == 0 || (!task.ImportNBT && !task.ImportCommandBlock) {
		return counts
	}
	for _, chunkPos := range group.chunkPositions {
		c := group.chunks[chunkPos]
		if c == nil {
			continue
		}
		nbtList := group.nbtLists[chunkPos]
		for nbtBlockPos, nbt := range nbtList {
			runtimeID := c.Block(uint8(nbtBlockPos.X()), int16(nbtBlockPos.Y()), uint8(nbtBlockPos.Z()), 0)
			module, attachmentType := buildRuntimeAttachmentImportModule(runtimeID, types.Position{}, nbt, task)
			if module == nil || attachmentType == "" {
				continue
			}
			switch attachmentType {
			case "command":
				counts.commandBlocks++
			case "nbt":
				counts.nbtBlocks++
			}
		}
	}
	return counts
}

func (b *importBuilder) makeChunkGroupForAttachmentCount(groupIndex int, groupPos wsdefine.ChunkPos, chunks map[wsdefine.ChunkPos]*chunk.Chunk, worldChunkX, worldChunkY, worldChunkZ int32) (*buildChunkGroup, error) {
	group := b.makeChunkGroupRaw(groupIndex, groupPos, chunks, worldChunkX, worldChunkY, worldChunkZ)
	if b.task.ImportNBT || b.task.ImportCommandBlock {
		chunksNBTs, err := b.reader.GetChunksNBT(group.chunkPositions)
		if err != nil {
			return nil, err
		}
		group.nbtLists = chunksNBTs
	}
	return group, nil
}

func (b *importBuilder) loadNextChunkGroupForAttachmentCount(groupIndex int, realStartChunkX, realStartChunkY, realStartChunkZ int32) (*buildChunkGroup, error) {
	raw := b.readNextChunkGroupRaw(groupIndex, realStartChunkX, realStartChunkY, realStartChunkZ)
	if raw.err != nil || raw.chunks == nil {
		return nil, raw.err
	}
	return b.makeChunkGroupForAttachmentCount(raw.index, raw.groupPos, raw.chunks, raw.worldChunkX, raw.worldChunkY, raw.worldChunkZ)
}

func (b *importBuilder) prepareAttachmentTotals(startGroupIndex, totalGroups int, realStartChunkX, realStartChunkY, realStartChunkZ int32) error {
	if b == nil || b.gameProgress == nil || (!b.task.ImportNBT && !b.task.ImportCommandBlock) {
		return nil
	}
	b.gameProgress.SetPhase("统计导入数量")
	b.gameProgress.SendToClientNow(b.client)

	savedManager := b.chunkManager
	b.chunkManager = newChunkGroupManager(b.reader, startGroupIndex, b.effectiveChunkGroupSide())
	defer func() {
		b.chunkManager = savedManager
	}()

	var totals importAttachmentCounts
	for i := startGroupIndex; i < totalGroups; i++ {
		group, err := b.loadNextChunkGroupForAttachmentCount(i, realStartChunkX, realStartChunkY, realStartChunkZ)
		if err != nil {
			b.gameProgress.SetPhase("建筑导入中")
			b.gameProgress.SendToClientNow(b.client)
			return err
		}
		counts := countChunkGroupAttachments(group, b.task)
		totals.commandBlocks += counts.commandBlocks
		totals.nbtBlocks += counts.nbtBlocks
	}
	if b.task.ImportCommandBlock {
		b.gameProgress.SetCommandTotal(totals.commandBlocks)
	}
	if b.task.ImportNBT {
		b.gameProgress.SetNBTTotal(totals.nbtBlocks)
	}
	b.gameProgress.SetPhase("建筑导入中")
	b.gameProgress.SendToClientNow(b.client)
	return nil
}

func importAttachmentModule(client *clientType.Client, module *types.Module, attachmentType string, limiter *rate.Limiter, ctx context.Context, gameProgress *ImportGameProgress, task types.Task) error {
	manager := NewChunkRegionManager(1)
	manager.GameProgress = gameProgress
	manager.CommandLimiter = limiter
	manager.AddAttachmentBlock(module, attachmentType)
	return manager.processRegionAttachmentsDirect(client, []*types.Module{module}, attachmentType, limiter, ctx)
}

func (m *ChunkRegionManager) processRegionAttachmentsDirect(client *clientType.Client, modules []*types.Module, attachmentType string, limiter *rate.Limiter, ctx context.Context) error {
	for _, module := range modules {
		if module == nil {
			continue
		}
		switch attachmentType {
		case "command":
			m.processCommandBlocks(client, []*types.Module{module}, limiter, ctx, nil)
		case "nbt", "chest", "special":
			m.processNBTBlocks(client, []*types.Module{module}, limiter, ctx, nil)
		}
	}
	return nil
}

func (b *importBuilder) buildChunk(c *chunk.Chunk, worldChunkX, worldChunkY, worldChunkZ int32) error {
	if c == nil {
		return nil
	}
	if b.task.ClearArea {
		for subChunkY, subChunk := range c.Sub() {
			if subChunk == nil {
				continue
			}
			if err := b.sendSettingsCommand(fmt.Sprintf(
				"fill %d %d %d %d %d %d air",
				worldChunkX,
				worldChunkY+int32(subChunkY)*16,
				worldChunkZ,
				worldChunkX+15,
				worldChunkY+int32(subChunkY)*16+15,
				worldChunkZ+15,
			), false); err != nil {
				return err
			}
		}
	}
	if b.task.AutoPlaceBorder {
		if err := b.placeBorderRingNearChunk(worldChunkX, worldChunkZ); err != nil {
			return err
		}
	}
	if b.task.AutoPlaceDenyBlock {
		if err := b.placeDenyLayerForChunk(worldChunkX, worldChunkZ); err != nil {
			return err
		}
	}
	if b.task.UseFill {
		for command := range chunk_fill.GenerateChunkCommand(c, types.Position{X: int(worldChunkX), Y: int(worldChunkY), Z: int(worldChunkZ)}) {
			if err := b.sendSettingsCommand(command, false); err != nil {
				return err
			}
		}
		return nil
	}
	for subChunkY, subChunk := range c.Sub() {
		if subChunk == nil || subChunk.Empty() {
			continue
		}
		layerBaseY := worldChunkY + int32(subChunkY)*16
		if err := b.moveBot(types.Position{X: int(worldChunkX), Y: int(layerBaseY), Z: int(worldChunkZ)}); err != nil {
			return fmt.Errorf("failed to move bot: %w", err)
		}
		layer := subChunk.Layer(0)
		for x := byte(0); x < 16; x++ {
			for y := byte(0); y < 16; y++ {
				for z := byte(0); z < 16; z++ {
					runtimeID := layer.At(x, y, z)
					if runtimeID == block.AirRuntimeID {
						continue
					}
					name, properties, _ := block.RuntimeIDToState(runtimeID)
					stateStr := wsutils.PropertiesToStateStr(properties)
					if err := b.sendSettingsCommand(fmt.Sprintf(
						"setblock %d %d %d %s %s",
						worldChunkX+int32(x),
						layerBaseY+int32(y),
						worldChunkZ+int32(z),
						name,
						stateStr,
					), false); err != nil {
						return fmt.Errorf("failed to build block at [%d,%d,%d]: %w", x, y, z, err)
					}
				}
			}
		}
	}
	return nil
}

func (b *importBuilder) structureBounds() (minX, maxX, minY, maxY, minZ, maxZ int32, ok bool) {
	if b.reader == nil {
		return 0, 0, 0, 0, 0, 0, false
	}
	size := b.reader.GetSize()
	if size.Width <= 0 || size.Length <= 0 {
		return 0, 0, 0, 0, 0, 0, false
	}
	offset := b.reader.GetOffsetPos()
	minX = int32(b.buildStartPos.X)
	minY = int32(b.buildStartPos.Y)
	minZ = int32(b.buildStartPos.Z)
	maxX = minX + int32(size.Width) - offset.X() - 1
	maxY = minY + int32(size.Height) - offset.Y() - 1
	maxZ = minZ + int32(size.Length) - offset.Z() - 1
	return minX, maxX, minY, maxY, minZ, maxZ, true
}

func (b *importBuilder) placeDenyLayerForChunk(chunkWorldPosX, chunkWorldPosZ int32) error {
	if !b.task.AutoPlaceDenyBlock || b.reader == nil {
		return nil
	}
	minX, maxX, _, _, minZ, maxZ, ok := b.structureBounds()
	if !ok {
		return nil
	}
	startX := chunkWorldPosX
	endX := chunkWorldPosX + 15
	startZ := chunkWorldPosZ
	endZ := chunkWorldPosZ + 15
	if endX < minX || startX > maxX || endZ < minZ || startZ > maxZ {
		return nil
	}
	startX = maxInt32(startX, minX)
	endX = minInt32(endX, maxX)
	startZ = maxInt32(startZ, minZ)
	endZ = minInt32(endZ, maxZ)
	if startX > endX || startZ > endZ {
		return nil
	}
	y := int32(b.originalStartPos.Y)
	return b.sendSettingsCommand(fmt.Sprintf("fill %d %d %d %d %d %d minecraft:deny", startX, y, startZ, endX, y, endZ), false)
}

func (b *importBuilder) placeBorderRingForChunk(chunkWorldPosX, chunkWorldPosZ int32) error {
	if !b.task.AutoPlaceBorder || b.reader == nil {
		return nil
	}
	minX, maxX, _, _, minZ, maxZ, ok := b.structureBounds()
	if !ok {
		return nil
	}
	outerMinX := minX - 1
	outerMaxX := maxX + 1
	outerMinZ := minZ - 1
	outerMaxZ := maxZ + 1
	chunkMinX := chunkWorldPosX
	chunkMaxX := chunkWorldPosX + 15
	chunkMinZ := chunkWorldPosZ
	chunkMaxZ := chunkWorldPosZ + 15
	borderY := int32(b.originalStartPos.Y)
	if outerMinZ >= chunkMinZ && outerMinZ <= chunkMaxZ {
		x1 := maxInt32(outerMinX, chunkMinX)
		x2 := minInt32(outerMaxX, chunkMaxX)
		if x1 <= x2 {
			if err := b.sendSettingsCommand(fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", x1, borderY, outerMinZ, x2, borderY, outerMinZ), false); err != nil {
				return err
			}
		}
	}
	if outerMaxZ >= chunkMinZ && outerMaxZ <= chunkMaxZ {
		x1 := maxInt32(outerMinX, chunkMinX)
		x2 := minInt32(outerMaxX, chunkMaxX)
		if x1 <= x2 {
			if err := b.sendSettingsCommand(fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", x1, borderY, outerMaxZ, x2, borderY, outerMaxZ), false); err != nil {
				return err
			}
		}
	}
	if outerMinX >= chunkMinX && outerMinX <= chunkMaxX {
		z1 := maxInt32(outerMinZ, chunkMinZ)
		z2 := minInt32(outerMaxZ, chunkMaxZ)
		if z1 <= z2 {
			if err := b.sendSettingsCommand(fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", outerMinX, borderY, z1, outerMinX, borderY, z2), false); err != nil {
				return err
			}
		}
	}
	if outerMaxX >= chunkMinX && outerMaxX <= chunkMaxX {
		z1 := maxInt32(outerMinZ, chunkMinZ)
		z2 := minInt32(outerMaxZ, chunkMaxZ)
		if z1 <= z2 {
			if err := b.sendSettingsCommand(fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", outerMaxX, borderY, z1, outerMaxX, borderY, z2), false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *importBuilder) placeBorderRingNearChunk(chunkWorldPosX, chunkWorldPosZ int32) error {
	if !b.task.AutoPlaceBorder || b.reader == nil {
		return nil
	}
	minX, maxX, _, _, minZ, maxZ, ok := b.structureBounds()
	if !ok {
		return nil
	}
	outerMinX := minX - 1
	outerMaxX := maxX + 1
	outerMinZ := minZ - 1
	outerMaxZ := maxZ + 1
	chunkMinX := chunkWorldPosX
	chunkMaxX := chunkWorldPosX + 15
	chunkMinZ := chunkWorldPosZ
	chunkMaxZ := chunkWorldPosZ + 15
	dxList := []int32{0}
	if outerMinX == chunkMinX-1 {
		dxList = append(dxList, -16)
	}
	if outerMaxX == chunkMaxX+1 {
		dxList = append(dxList, 16)
	}
	dzList := []int32{0}
	if outerMinZ == chunkMinZ-1 {
		dzList = append(dzList, -16)
	}
	if outerMaxZ == chunkMaxZ+1 {
		dzList = append(dzList, 16)
	}
	for _, dx := range dxList {
		for _, dz := range dzList {
			if err := b.placeBorderRingForChunk(chunkWorldPosX+dx, chunkWorldPosZ+dz); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *importBuilder) placeBorderRingForChunks(chunks map[wsdefine.ChunkPos]*chunk.Chunk, worldChunkX, worldChunkZ int32) error {
	if !b.task.AutoPlaceBorder || b.reader == nil || len(chunks) == 0 {
		return nil
	}
	minChunkPosX := int32(math.MaxInt32)
	minChunkPosZ := int32(math.MaxInt32)
	for chunkPos := range chunks {
		if chunkPos.X() < minChunkPosX {
			minChunkPosX = chunkPos.X()
		}
		if chunkPos.Z() < minChunkPosZ {
			minChunkPosZ = chunkPos.Z()
		}
	}
	for chunkPos := range chunks {
		chunkWorldPosX := worldChunkX + (chunkPos.X()-minChunkPosX)*16
		chunkWorldPosZ := worldChunkZ + (chunkPos.Z()-minChunkPosZ)*16
		if err := b.placeBorderRingNearChunk(chunkWorldPosX, chunkWorldPosZ); err != nil {
			return err
		}
	}
	return nil
}

func (b *importBuilder) verifyPending(unverifyChunks map[wsdefine.ChunkPos]*chunk.Chunk, realStartChunkY int32) error {
	if b.verifyChunkLevel == verifyChunkLevelNone || len(unverifyChunks) == 0 {
		return nil
	}
	targetSubChunkPosList := make([]protocol.SubChunkPos, 0)
	for realChunkPos, unverifyChunk := range unverifyChunks {
		noAirSubChunkYList := make([]int, 0)
		for subChunkPosY, subChunk := range unverifyChunk.Sub() {
			if subChunk.Empty() {
				continue
			}
			noAirSubChunkYList = append(noAirSubChunkYList, int(unverifyChunk.SubIndex(int16(subChunkPosY*16))))
		}
		for _, subChunkPosY := range noAirSubChunkYList {
			targetSubChunkPosList = append(targetSubChunkPosList, protocol.SubChunkPos{
				realChunkPos.X(),
				realStartChunkY/16 + int32(subChunkPosY) - 4,
				realChunkPos.Z(),
			})
		}
	}
	if len(targetSubChunkPosList) == 0 {
		return nil
	}
	req := buildSubChunkRequest(int32(b.client.DimensionID), targetSubChunkPosList)
	res, ok := b.client.Resources.(*ResourcesControl.Resources)
	if !ok || res == nil {
		return fmt.Errorf("verify resources unavailable")
	}
	response, timeout, err := sendSubChunkRequestWithRespTimeoutOptimized(b.client, res, req, postImportVerifyTimeout)
	if err != nil {
		return err
	}
	if timeout {
		return errors.New("verify subchunk timeout")
	}
	chunkStatusMap := make(map[wsdefine.ChunkPos]bool)
	for _, pos := range targetSubChunkPosList {
		chunkStatusMap[wsdefine.ChunkPos{pos.X(), pos.Z()}] = true
	}
	b.verifySubChunkEntries(response, unverifyChunks, chunkStatusMap)
	chunkNeedFixMap := make(map[wsdefine.ChunkPos]bool)
	for chunkPos, verifyPassed := range chunkStatusMap {
		chunkNeedFixMap[chunkPos] = !verifyPassed
	}
	return b.handleChunkVerificationAndFix(unverifyChunks, chunkNeedFixMap)
}

func (b *importBuilder) handleChunkVerificationAndFix(unverifyChunks map[wsdefine.ChunkPos]*chunk.Chunk, initialChunkStatusMap map[wsdefine.ChunkPos]bool) error {
	fixAttempts := make(map[wsdefine.ChunkPos]int)
	chunksNeedFix := make(map[wsdefine.ChunkPos]bool)
	for chunkPos, needFix := range initialChunkStatusMap {
		if needFix {
			chunksNeedFix[chunkPos] = true
		}
	}
	for len(chunksNeedFix) > 0 {
		currentFixList := make([]wsdefine.ChunkPos, 0)
		for chunkPos := range chunksNeedFix {
			if fixAttempts[chunkPos] < maxFixDepth {
				currentFixList = append(currentFixList, chunkPos)
			}
		}
		if len(currentFixList) == 0 {
			break
		}
		for _, failedChunkPos := range currentFixList {
			if err := b.fixChunkByRebuild(failedChunkPos, unverifyChunks); err != nil {
				return err
			}
			fixAttempts[failedChunkPos]++
		}
		reVerifyResult, err := b.reVerifyChunks(currentFixList, unverifyChunks)
		if err != nil {
			return err
		}
		for _, chunkPos := range currentFixList {
			if reVerifyResult[chunkPos] || fixAttempts[chunkPos] >= maxFixDepth {
				delete(chunksNeedFix, chunkPos)
			}
		}
	}
	return nil
}

func (b *importBuilder) fixChunkByRebuild(failedChunkPos wsdefine.ChunkPos, unverifyChunks map[wsdefine.ChunkPos]*chunk.Chunk) error {
	worldChunkX := failedChunkPos.X() * 16
	worldChunkZ := failedChunkPos.Z() * 16
	worldChunkY := int32(b.buildStartPos.Y - floorMod(b.buildStartPos.Y, 16))
	if err := b.moveBot(types.Position{X: int(worldChunkX), Y: b.buildStartPos.Y, Z: int(worldChunkZ)}); err != nil {
		return err
	}
	if err := b.waitChunkLoad(failedChunkPos); err != nil {
		return err
	}
	defer b.clearChunkLoadTickingArea(failedChunkPos)
	originalChunk := unverifyChunks[failedChunkPos]
	if originalChunk == nil {
		return fmt.Errorf("original chunk data not found for chunk %v", failedChunkPos)
	}
	return b.buildChunk(originalChunk, worldChunkX, worldChunkY, worldChunkZ)
}

func (b *importBuilder) reVerifyChunks(chunkPosList []wsdefine.ChunkPos, unverifyChunks map[wsdefine.ChunkPos]*chunk.Chunk) (map[wsdefine.ChunkPos]bool, error) {
	if len(chunkPosList) == 0 {
		return make(map[wsdefine.ChunkPos]bool), nil
	}
	realStartChunkY := int32(b.buildStartPos.Y - floorMod(b.buildStartPos.Y, 16))
	targetSubChunkPosList := make([]protocol.SubChunkPos, 0)
	for _, chunkPos := range chunkPosList {
		unverifyChunk := unverifyChunks[chunkPos]
		if unverifyChunk == nil {
			continue
		}
		for subChunkPosY, subChunk := range unverifyChunk.Sub() {
			if subChunk.Empty() {
				continue
			}
			targetSubChunkPosList = append(targetSubChunkPosList, protocol.SubChunkPos{
				chunkPos.X(),
				realStartChunkY/16 + int32(unverifyChunk.SubIndex(int16(subChunkPosY*16))),
				chunkPos.Z(),
			})
		}
	}
	result := make(map[wsdefine.ChunkPos]bool)
	for _, chunkPos := range chunkPosList {
		result[chunkPos] = true
	}
	if len(targetSubChunkPosList) == 0 {
		return result, nil
	}
	req := buildSubChunkRequest(int32(b.client.DimensionID), targetSubChunkPosList)
	res, ok := b.client.Resources.(*ResourcesControl.Resources)
	if !ok || res == nil {
		return nil, fmt.Errorf("verify resources unavailable")
	}
	response, timeout, err := sendSubChunkRequestWithRespTimeoutOptimized(b.client, res, req, postImportVerifyTimeout)
	if err != nil {
		return nil, err
	}
	if timeout {
		return nil, errors.New("verify subchunk timeout")
	}
	b.verifySubChunkEntries(response, unverifyChunks, result)
	return result, nil
}

func (b *importBuilder) verifySubChunkEntries(response *packet.SubChunk, unverifyChunks map[wsdefine.ChunkPos]*chunk.Chunk, chunkStatusMap map[wsdefine.ChunkPos]bool) {
	if response == nil {
		for key := range chunkStatusMap {
			chunkStatusMap[key] = false
		}
		return
	}
	for _, entity := range response.SubChunkEntries {
		chunkPos := wsdefine.ChunkPos{
			response.Position.X() + int32(entity.Offset[0]),
			response.Position.Z() + int32(entity.Offset[2]),
		}
		unverifyChunk := unverifyChunks[chunkPos]
		if unverifyChunk == nil || !chunkStatusMap[chunkPos] {
			continue
		}
		if b.verifyChunkLevel >= verifyChunkLevelAir && entity.Result != protocol.SubChunkResultSuccess {
			chunkStatusMap[chunkPos] = false
			continue
		}
		if b.verifyChunkLevel >= verifyChunkLevelSimilar {
			subChunkIndex := int16(response.Position.Y() + int32(entity.Offset[1]))
			chunkRange := unverifyChunk.Range()
			if subChunkIndex < int16(chunkRange[0]) || subChunkIndex >= int16(chunkRange[1]) {
				chunkStatusMap[chunkPos] = false
				continue
			}
			unverifySubChunk := unverifyChunk.SubChunk(subChunkIndex)
			raw := chunk.EncodeSubChunk(
				unverifySubChunk,
				chunkRange,
				int(((response.Position.Y()+int32(entity.Offset[1]))<<4-int32(chunkRange[0]))>>4),
				chunk.NetworkEncoding,
			)
			gotPayload := entity.RawPayload
			wantPayload := raw
			if len(wantPayload) > len(gotPayload) {
				wantPayload = wantPayload[:len(gotPayload)]
			} else {
				gotPayload = gotPayload[:len(wantPayload)]
			}
			if stringSimilarityBytes(gotPayload, wantPayload) < minMatchingDegree {
				chunkStatusMap[chunkPos] = false
				continue
			}
		}
		if b.verifyChunkLevel >= verifyChunkLevelComplete {
			_, decoded, _, err := mirrorchunk.NEMCSubChunkDecode(entity.RawPayload)
			if err != nil || decoded == nil {
				chunkStatusMap[chunkPos] = false
				continue
			}
			realStartChunkY := int32(b.buildStartPos.Y - floorMod(b.buildStartPos.Y, 16))
			subChunkPosY := int16(response.Position.Y() + int32(entity.Offset[1]) - realStartChunkY/16)
			unverifySubChunk := unverifyChunk.SubChunk(subChunkPosY)
			for x := byte(0); x < 16; x++ {
				for y := byte(0); y < 16; y++ {
					for z := byte(0); z < 16; z++ {
						if decoded.Block(x, y, z, 0) != unverifySubChunk.Block(x, y, z, 0) {
							chunkStatusMap[chunkPos] = false
							break
						}
					}
					if !chunkStatusMap[chunkPos] {
						break
					}
				}
				if !chunkStatusMap[chunkPos] {
					break
				}
			}
		}
	}
}

func stringSimilarityBytes(a, b []byte) float64 {
	if bytes.Equal(a, b) {
		return 100
	}
	lenA, lenB := len(a), len(b)
	if lenA == 0 || lenB == 0 {
		return 0
	}
	prev := make([]int, lenB+1)
	curr := make([]int, lenB+1)
	for j := 0; j <= lenB; j++ {
		prev[j] = j
	}
	for i := 1; i <= lenA; i++ {
		curr[0] = i
		for j := 1; j <= lenB; j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = minInt(minInt(prev[j]+1, curr[j-1]+1), prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	maxLen := math.Max(float64(lenA), float64(lenB))
	return math.Max(0, math.Min(100, (1-float64(prev[lenB])/maxLen)*100))
}

func collectProgressCommands(commands <-chan string) []string {
	out := make([]string, 0)
	for command := range commands {
		command = strings.TrimSpace(command)
		if command != "" {
			out = append(out, command)
		}
	}
	return out
}

func groupBlockCommandCount(group *buildChunkGroup) int {
	if group == nil {
		return 0
	}
	count := 0
	for _, action := range group.blockActions {
		if !action.hasMove {
			count++
		}
	}
	return count
}

func fixDoorCommand(cmd string) string {
	newCmd1 := strings.ReplaceAll(cmd, `"upper_block_bit"=true`, `"upper_block_bit"=false`)
	ok1 := newCmd1 != cmd
	newCmd2 := strings.ReplaceAll(newCmd1, `"door_hinge_bit"=true`, `"door_hinge_bit"=false`)
	ok2 := newCmd2 != newCmd1
	currentCmd := newCmd2
	if ok2 {
		dirStr := `"direction"=`
		idxDir := strings.Index(currentCmd, dirStr)
		if idxDir != -1 {
			start := idxDir + len(dirStr)
			end := start
			for end < len(currentCmd) && currentCmd[end] >= '0' && currentCmd[end] <= '9' {
				end++
			}
			if end > start {
				num, err := strconv.Atoi(currentCmd[start:end])
				if err == nil {
					num++
					if num > 3 {
						num = 0
					}
					currentCmd = currentCmd[:start] + strconv.Itoa(num) + currentCmd[end:]
				}
			}
		}
	}
	if !ok1 {
		return currentCmd
	}
	parts := strings.SplitN(currentCmd, " ", 5)
	if len(parts) < 5 {
		return currentCmd
	}
	y, err := strconv.Atoi(parts[2])
	if err != nil {
		return currentCmd
	}
	parts[2] = strconv.Itoa(y - 1)
	return strings.Join(parts, " ")
}

func (b *importBuilder) run(startGroupIndex int, totalGroups int, realStartChunkX, realStartChunkY, realStartChunkZ int32) error {
	unverifyChunks := make(map[wsdefine.ChunkPos]*chunk.Chunk)
	currentGroup, err := b.loadNextChunkGroup(startGroupIndex, realStartChunkX, realStartChunkY, realStartChunkZ)
	if err != nil {
		return err
	}
	for i := startGroupIndex; i < totalGroups; i++ {
		if currentGroup == nil {
			break
		}
		nextIndex := i + 1
		var nextGroupFuture *buildGroupFuture
		var preWaitDone <-chan struct{}
		if nextIndex < totalGroups {
			nextGroupFuture = b.startPreHandleNextChunkGroup(b.ctx, nextIndex, realStartChunkX, realStartChunkY, realStartChunkZ)
			if b.shouldWaitChunkLoad() && !b.preloadOff {
				preWaitDone = b.startPreWaitNextChunkLoadFromFuture(b.ctx, nextGroupFuture)
			}
		}
		if !currentGroup.chunksEmpty {
			if err := b.moveBot(types.Position{X: int(currentGroup.worldChunkX), Y: b.buildStartPos.Y, Z: int(currentGroup.worldChunkZ)}); err != nil {
				return err
			}
			if b.shouldWaitChunkLoad() {
				if err := b.waitChunkLoad(currentGroup.realChunkPos); err != nil {
					return err
				}
			}
			if err := b.buildChunkGroupBlocks(currentGroup); err != nil {
				return err
			}
			if b.shouldWaitPreloadBeforeNBT() {
				if err := b.waitPreWaitNextChunkLoad(preWaitDone); err != nil {
					return err
				}
			}
			if err := b.buildChunkGroupNBT(currentGroup); err != nil {
				return err
			}
			if b.shouldWaitChunkLoad() {
				b.clearChunkLoadTickingArea(currentGroup.realChunkPos)
			}
		} else {
			if b.shouldWaitPreloadBeforeNBT() {
				if err := b.waitPreWaitNextChunkLoad(preWaitDone); err != nil {
					return err
				}
			}
			if b.shouldWaitChunkLoad() {
				b.clearChunkLoadTickingArea(currentGroup.realChunkPos)
			}
		}
		if b.verifyChunkLevel != verifyChunkLevelNone {
			for pos, c := range currentGroup.chunks {
				unverifyChunks[wsdefine.ChunkPos{realStartChunkX/16 + pos.X(), realStartChunkZ/16 + pos.Z()}] = c
			}
			if len(unverifyChunks) >= b.verifyAfterChunk {
				if err := b.verifyPending(unverifyChunks, realStartChunkY); err != nil {
					return err
				}
				unverifyChunks = make(map[wsdefine.ChunkPos]*chunk.Chunk)
			}
		}
		b.processedGroups++
		if b.bar != nil {
			_ = b.bar.Add(len(currentGroup.chunks))
		}
		if b.gameProgress != nil {
			b.gameProgress.SetChunkProgress(b.processedGroups*b.effectiveChunkGroupSide()*b.effectiveChunkGroupSide(), b.totalGroups*b.effectiveChunkGroupSide()*b.effectiveChunkGroupSide())
			b.gameProgress.SendToClient(b.client)
		}
		if b.progressCb != nil {
			b.progressCb(b.processedGroups, b.totalGroups)
		}
		if nextIndex < totalGroups {
			currentGroup, err = nextGroupFuture.Wait(b.ctx)
			if err != nil {
				return err
			}
			continue
		}
		currentGroup = nil
	}
	if len(unverifyChunks) > 0 {
		if err := b.verifyPending(unverifyChunks, realStartChunkY); err != nil {
			return err
		}
	}
	return nil
}

func importPlainBuildGroups(client *clientType.Client, filePath string, task types.Task, x, y, z int, chunkGroupSide int, skipGroups int, ctx context.Context, limiter *rate.Limiter, attachmentLimiter *rate.Limiter, bar *progressbar.ProgressBar, gameProgress *ImportGameProgress, progressCb func(processed int, total int)) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	reader, err := wsstructure.StructureFromFile(file)
	if err != nil {
		return err
	}
	defer reader.Close()
	buildX, buildY, buildZ, _ := resolveImportBuildOrigin(x, y, z, task)
	realStartChunkX := int32(buildX - floorMod(buildX, 16))
	realStartChunkY := int32(buildY - floorMod(buildY, 16))
	realStartChunkZ := int32(buildZ - floorMod(buildZ, 16))
	reader.SetOffsetPos(wsdefine.Offset{int32(buildX) - realStartChunkX, int32(buildY) - realStartChunkY, int32(buildZ) - realStartChunkZ})
	if chunkGroupSide <= 0 {
		chunkGroupSide = 1
	}
	size := reader.GetSize()
	chunkXGroups := (size.GetChunkXCount() + chunkGroupSide - 1) / chunkGroupSide
	chunkZGroups := (size.GetChunkZCount() + chunkGroupSide - 1) / chunkGroupSide
	totalGroups := chunkXGroups * chunkZGroups
	if skipGroups < 0 {
		skipGroups = 0
	}
	if skipGroups >= totalGroups {
		return nil
	}
	verifyLevel := verifyChunkLevelNone
	if task.VerifySuspicious {
		verifyLevel = defaultVerifyLevel
	}
	builder := &importBuilder{
		client:             client,
		reader:             reader,
		task:               task,
		ctx:                ctx,
		limiter:            limiter,
		attachmentLimiter:  attachmentLimiter,
		bar:                bar,
		gameProgress:       gameProgress,
		progressCb:         progressCb,
		buildStartPos:      types.Position{X: buildX, Y: buildY, Z: buildZ},
		originalStartPos:   types.Position{X: x, Y: y, Z: z},
		chunkGroupSide:     chunkGroupSide,
		totalGroups:        totalGroups,
		processedGroups:    skipGroups,
		verifyAfterChunk:   defaultVerifyAfterChunk,
		verifyChunkLevel:   verifyLevel,
		tickingAreaRecords: make(map[wsdefine.ChunkPos][]string),
		tickingAreaOwned:   make(map[string]struct{}),
	}
	if err := builder.prepareTickingAreaRuntimeStrategy(); err != nil {
		return err
	}
	chunkGroupSide = builder.effectiveChunkGroupSide()
	chunkXGroups = (size.GetChunkXCount() + chunkGroupSide - 1) / chunkGroupSide
	chunkZGroups = (size.GetChunkZCount() + chunkGroupSide - 1) / chunkGroupSide
	totalGroups = chunkXGroups * chunkZGroups
	builder.totalGroups = totalGroups
	if skipGroups >= totalGroups {
		return nil
	}
	if err := builder.prepareAttachmentTotals(skipGroups, totalGroups, realStartChunkX, realStartChunkY, realStartChunkZ); err != nil {
		return fmt.Errorf("统计命令方块/NBT 方块数量失败: %w", err)
	}
	builder.chunkManager = newChunkGroupManager(reader, skipGroups, chunkGroupSide)
	return builder.run(skipGroups, totalGroups, realStartChunkX, realStartChunkY, realStartChunkZ)
}
