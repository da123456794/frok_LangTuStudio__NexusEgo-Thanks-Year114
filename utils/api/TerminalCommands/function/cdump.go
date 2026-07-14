package function

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"nexus/constants"
	types "nexus/defines"
	"nexus/utils/api/commands_generator"
	ResourcesControl "nexus/utils/api/resources_control"
	NBTAssigner "nexus/utils/bdump/nbt_assigner"
	clientType "nexus/utils/client"
	"nexus/utils/dimension"
	"nexus/utils/file"
	"nexus/utils/log"
	"nexus/utils/webclient"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/Yeah114/blocks"

	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	bwo_define "github.com/TriM-Organization/bedrock-world-operator/define"
	"github.com/TriM-Organization/bedrock-world-operator/world"

	"github.com/pterm/pterm"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

const (
	defaultImportCommandSpeed  = constants.DefaultImportSpeed
	commandBlockBatchSize      = 200
	inGameProgressPushInterval = importGameProgressPushInterval
	importRetryLimit           = 5
	chunkProbeTimeout          = 500 * time.Millisecond
	chunkProbeRetryDelay       = 100 * time.Millisecond
	importKeepAliveInterval    = 3 * time.Second
)

// ChunkRegionManager manages buffered attachment regions during import.
type ChunkRegionManager struct {
	RegionSize          int
	ChunkSize           int
	Regions             map[[2]int]*RegionData
	RegionOrder         [][2]int
	WaitChunkLoad       bool
	CommandLimiter      *rate.Limiter
	BufferedRefs        int
	ProcessedRegions    int
	ProcessedChunks     int
	TotalChunks         int
	ProgressCb          func(processed int, total int)
	GameProgress        *ImportGameProgress
	PostVerify          func(chunks [][2]int) error
	PostVerifyErr       error
	RepairMode          bool
	ClearDrops          bool
	RegionBaseChunkX    int
	RegionBaseChunkZ    int
	MinChunkX           int
	MaxChunkX           int
	MinChunkZ           int
	MaxChunkZ           int
	BuildBounds         mcworldBounds
	ProtectionTask      types.Task
	ProtectionLayerY    int32
	AggressiveFlush     bool
	FlushTimeout        time.Duration
	activeTickingRegion [2]int
	activeTickingAreas  []string
	NBTPrefetchOn       bool
	NBTPrefetchDimID    uint8
	nbtPrepareQueue     chan *preparedNBTTask
	nbtPrepareTasks     sync.Map
	nbtPrepareWG        sync.WaitGroup
	nbtRegionSource     *nbtRegionPrefetchSource
	nbtRegionQueue      chan *preparedNBTRegionTask
	nbtRegionTasks      sync.Map
	nbtRegionWG         sync.WaitGroup
}

type RegionData struct {
	Blocks          map[int][]*types.Module
	CommandBlocks   []*types.Module
	NBTBlocks       []*types.Module
	Chests          []*types.Module
	SpecialBlocks   []*types.Module
	MinY            int
	MaxY            int
	Processed       bool
	BlocksDone      bool
	AttachmentsDone bool
	SeenChunks      map[[2]int]bool
	FirstSeen       int64
	BufferedRefs    int
}

type nbtTaskKey struct {
	X int
	Y int
	Z int
}

type preparedNBTTask struct {
	key            nbtTaskKey
	module         *types.Module
	additionalData *NBTAssigner.BlockAdditionalData
	done           chan struct{}
	prepared       *NBTAssigner.PreparedBlockPlacement
	err            error
	queueOnce      sync.Once
}

type nbtRegionPrefetchSource struct {
	bw                 *world.BedrockWorld
	bounds             mcworldBounds
	sourceDimID        int32
	offsetX            int
	offsetY            int
	offsetZ            int
	importNBT          bool
	importCommandBlock bool
	defaultSignWax     bool
	skipChunk          func(chunkX, chunkZ int32) bool
}

type importAttachmentCounts struct {
	commandBlocks int
	nbtBlocks     int
}

type preparedNBTRegionTask struct {
	key          [2]int
	targetBounds mcworldBounds
	done         chan struct{}
	blocks       []*types.Module
	err          error
	queueOnce    sync.Once
	attached     uint32
}

var traversedChunkProgress sync.Map // map[*ChunkRegionManager]map[[2]int]int

type connState interface {
	Closed() bool
	CloseError() error
}

type importAbortCancelKey struct{}

func withImportAbort(ctx context.Context, cancel context.CancelFunc) context.Context {
	if ctx == nil || cancel == nil {
		return ctx
	}
	return context.WithValue(ctx, importAbortCancelKey{}, cancel)
}

func importContextDone(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func isFatalImportConnError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	if text == "" {
		return false
	}
	if strings.Contains(text, "conn dead") ||
		strings.Contains(text, "connection closed") ||
		strings.Contains(text, "use of closed network connection") ||
		strings.Contains(text, "closed network connection") {
		return true
	}
	return strings.Contains(text, "permission removed") ||
		strings.Contains(text, "permission revoked") ||
		strings.Contains(text, "/deop") ||
		strings.Contains(text, " deop ") ||
		(strings.Contains(text, "op") && strings.Contains(text, "permission"))
}

func cancelImportOnFatalError(ctx context.Context, client *clientType.Client, err error) bool {
	if !isFatalImportConnError(err) {
		return false
	}
	if client != nil && strings.TrimSpace(client.LastImportError) == "" {
		setLastImportError(client, err)
	}
	if ctx != nil {
		if cancel, ok := ctx.Value(importAbortCancelKey{}).(context.CancelFunc); ok && cancel != nil {
			cancel()
		}
	}
	return true
}

func abortRegionOnDeadConnection(ctx context.Context, client *clientType.Client) bool {
	err := clientConnError(client)
	if err == nil {
		return false
	}
	return cancelImportOnFatalError(ctx, client, err)
}

func getTraversedChunkMap(m *ChunkRegionManager) map[[2]int]int {
	if m == nil {
		return nil
	}
	if value, ok := traversedChunkProgress.Load(m); ok {
		return value.(map[[2]int]int)
	}
	progress := make(map[[2]int]int)
	actual, _ := traversedChunkProgress.LoadOrStore(m, progress)
	return actual.(map[[2]int]int)
}

func connDeadError(reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "connection closed"
	}
	if strings.HasPrefix(strings.ToLower(reason), "conn dead") {
		return errors.New(reason)
	}
	return fmt.Errorf("conn dead: %s", reason)
}

func clientConnError(client *clientType.Client) error {
	if client == nil {
		return connDeadError("client unavailable")
	}
	if client.Conn == nil {
		return connDeadError(client.LastImportError)
	}
	if state, ok := client.Conn.(connState); ok && state.Closed() {
		if err := state.CloseError(); err != nil {
			return connDeadError(err.Error())
		}
		return connDeadError(client.LastImportError)
	}
	if client.GameInterface == nil {
		return connDeadError("game interface unavailable")
	}
	return nil
}

func sendAICommandDim(client *clientType.Client, cmd string, wait bool) error {
	if client == nil || client.GameInterface == nil {
		return nil
	}
	if err := clientConnError(client); err != nil {
		return err
	}
	return client.GameInterface.SendAICommand(client.WrapCommandInDimension(cmd), wait)
}

func sendSettingsCommandDim(client *clientType.Client, cmd string) error {
	if client == nil || client.GameInterface == nil {
		return nil
	}
	if err := clientConnError(client); err != nil {
		return err
	}
	return client.GameInterface.SendSettingsCommand(client.WrapCommandInDimension(cmd), false)
}

func sendAICommandWithResponseDim(client *clientType.Client, cmd string, opts ResourcesControl.CommandRequestOptions) ResourcesControl.CommandRespond {
	if client == nil || client.GameInterface == nil {
		return ResourcesControl.CommandRespond{}
	}
	if err := clientConnError(client); err != nil {
		return ResourcesControl.CommandRespond{
			Error:     err,
			ErrorType: ResourcesControl.ErrCommandRequestOthers,
		}
	}
	return client.GameInterface.SendAICommandWithResponse(client.WrapCommandInDimension(cmd), opts)
}

func sendWSCommandWithResponseDim(client *clientType.Client, cmd string, opts ResourcesControl.CommandRequestOptions) ResourcesControl.CommandRespond {
	if client == nil || client.GameInterface == nil {
		return ResourcesControl.CommandRespond{}
	}
	if err := clientConnError(client); err != nil {
		return ResourcesControl.CommandRespond{
			Error:     err,
			ErrorType: ResourcesControl.ErrCommandRequestOthers,
		}
	}
	return client.GameInterface.SendWSCommandWithResponse(client.WrapCommandInDimension(cmd), opts)
}

func sendWSCommandWithResponseRaw(client *clientType.Client, cmd string, opts ResourcesControl.CommandRequestOptions) ResourcesControl.CommandRespond {
	if client == nil || client.GameInterface == nil {
		return ResourcesControl.CommandRespond{}
	}
	if err := clientConnError(client); err != nil {
		return ResourcesControl.CommandRespond{
			Error:     err,
			ErrorType: ResourcesControl.ErrCommandRequestOthers,
		}
	}
	return client.GameInterface.SendWSCommandWithResponse(cmd, opts)
}

func isClientConnDead(client *clientType.Client) bool {
	return clientConnError(client) != nil
}

func sendAICommandWithTimeoutDim(client *clientType.Client, cmd string, timeout time.Duration) (resp ResourcesControl.CommandRespond, isTimeout bool, err error) {
	resp = sendAICommandWithResponseDim(client, cmd, ResourcesControl.CommandRequestOptions{TimeOut: timeout})
	if resp.Error != nil {
		if resp.ErrorType == ResourcesControl.ErrCommandRequestTimeOut {
			return resp, true, nil
		}
		return resp, false, resp.Error
	}
	return resp, false, nil
}

func sendWSCommandWithTimeoutDim(client *clientType.Client, cmd string, timeout time.Duration) (resp ResourcesControl.CommandRespond, isTimeout bool, err error) {
	resp = sendWSCommandWithResponseDim(client, cmd, ResourcesControl.CommandRequestOptions{TimeOut: timeout})
	if resp.Error != nil {
		if resp.ErrorType == ResourcesControl.ErrCommandRequestTimeOut {
			return resp, true, nil
		}
		return resp, false, resp.Error
	}
	return resp, false, nil
}

func sendWSCommandWithTimeoutRaw(client *clientType.Client, cmd string, timeout time.Duration) (resp ResourcesControl.CommandRespond, isTimeout bool, err error) {
	resp = sendWSCommandWithResponseRaw(client, cmd, ResourcesControl.CommandRequestOptions{TimeOut: timeout})
	if resp.Error != nil {
		if resp.ErrorType == ResourcesControl.ErrCommandRequestTimeOut {
			return resp, true, nil
		}
		return resp, false, resp.Error
	}
	return resp, false, nil
}

func commandFailedMessage(resp ResourcesControl.CommandRespond) string {
	if resp.Respond == nil || len(resp.Respond.OutputMessages) == 0 {
		return ""
	}
	for _, msg := range resp.Respond.OutputMessages {
		if !msg.Success && msg.Message != "" {
			return msg.Message
		}
	}
	return ""
}

func commandRespondError(resp ResourcesControl.CommandRespond, fallback string) error {
	if resp.Error != nil {
		return resp.Error
	}
	if resp.AICommand != nil {
		if resp.AICommand.PreCheckError != nil && strings.TrimSpace(resp.AICommand.PreCheckError.Reason) != "" {
			return errors.New(resp.AICommand.PreCheckError.Reason)
		}
		if resp.AICommand.Result != nil && !resp.AICommand.Result.Success {
			return errors.New(fallback)
		}
	}
	if msg := commandFailedMessage(resp); msg != "" {
		return errors.New(msg)
	}
	return nil
}

func commandRespondSuccessCount(resp ResourcesControl.CommandRespond) bool {
	if resp.Respond != nil {
		return resp.Respond.SuccessCount > 0
	}
	return resp.AICommand != nil && resp.AICommand.Result != nil && resp.AICommand.Result.Success
}

func ensureMainBotCreativeForCommandBlocks(client *clientType.Client) error {
	opts := ResourcesControl.CommandRequestOptions{TimeOut: 5 * time.Second}
	if err := sendSettingsCommandDim(client, "gamemode 1 @s"); err != nil {
		return fmt.Errorf("set creative mode before command block write: %w", err)
	}

	confirmCommands := []string{
		"testfor @s[m=creative]",
		"testfor @s[m=1]",
	}
	var lastErr error
	for _, cmd := range confirmCommands {
		resp := sendWSCommandWithResponseDim(client, cmd, opts)
		if err := commandRespondError(resp, "creative mode confirmation command failed"); err != nil {
			lastErr = err
			continue
		}
		if commandRespondSuccessCount(resp) {
			return nil
		}
		lastErr = fmt.Errorf("creative mode confirmation returned no matching target")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("creative mode confirmation failed")
	}
	return lastErr
}

func (m *ChunkRegionManager) markCommandBlocksFailed(client *clientType.Client, count int) {
	if m == nil || m.GameProgress == nil || count <= 0 {
		return
	}
	for i := 0; i < count; i++ {
		m.GameProgress.MarkCommandFailed()
	}
	m.GameProgress.SendToClient(client)
}

func setLastImportError(client *clientType.Client, err error) {
	if client == nil {
		return
	}
	if err == nil {
		client.LastImportError = ""
		return
	}
	client.LastImportError = err.Error()
}

func setLastImportErrorMessage(client *clientType.Client, msg string) {
	if client == nil {
		return
	}
	client.LastImportError = strings.TrimSpace(msg)
}

func importFail(client *clientType.Client, message string, err error) bool {
	if err != nil {
		setLastImportError(client, err)
		pterm.Println(pterm.Red(fmt.Sprintf("%s: %v", message, err)))
		return false
	}
	setLastImportErrorMessage(client, message)
	pterm.Println(pterm.Red(message))
	return false
}

func failImportVerify(client *clientType.Client, err error) bool {
	if err == nil {
		return false
	}
	if isFatalImportConnError(err) {
		setLastImportError(client, err)
		return false
	}
	return importFail(client, "import chunk verification failed", err)
}

func removeNamedTickingArea(client *clientType.Client, name string) {
	if client == nil || client.GameInterface == nil || strings.TrimSpace(name) == "" {
		return
	}
	_ = sendWSCommandWithResponseRaw(client, fmt.Sprintf("tickingarea remove %s", name), ResourcesControl.CommandRequestOptions{WithNoResponse: true})
}

func removeAllTickingAreas(client *clientType.Client) {
	if client == nil || client.GameInterface == nil {
		return
	}
	_ = sendWSCommandWithResponseRaw(client, "tickingarea remove_all", ResourcesControl.CommandRequestOptions{WithNoResponse: true})
}

func addNamedTickingAreaWithRetry(client *clientType.Client, minX, minY, minZ, maxX, maxY, maxZ int, name string) error {
	var lastErr error
	for attempt := 1; attempt <= importRetryLimit; attempt++ {
		removeNamedTickingArea(client, name)
		resp, isTimeout, err := sendWSCommandWithTimeoutRaw(
			client,
			fmt.Sprintf("tickingarea add %d %d %d %d %d %d %s", minX, minY, minZ, maxX, maxY, maxZ, name),
			3*time.Second,
		)
		if isTimeout {
			lastErr = fmt.Errorf("timeout")
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}
		if msg := commandFailedMessage(resp); msg != "" {
			if strings.Contains(strings.ToLower(msg), "conflictingname") {
				lastErr = errors.New(msg)
				continue
			}
			lastErr = errors.New(msg)
			continue
		}
		if resp.Respond == nil || resp.Respond.SuccessCount > 0 || len(resp.Respond.OutputMessages) == 0 || resp.Respond.OutputMessages[0].Success {
			return nil
		}
		lastErr = fmt.Errorf("unknown tickingarea add failure")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unknown tickingarea add failure")
	}
	return fmt.Errorf("tickingarea add failed after %d attempts: %w", importRetryLimit, lastErr)
}

func preloadTickingAreaWithRetry(client *clientType.Client, x, y, z int) error {
	var lastErr error
	for attempt := 1; attempt <= importRetryLimit; attempt++ {
		resp, isTimeout, err := sendWSCommandWithTimeoutRaw(
			client,
			fmt.Sprintf("tickingarea preload %d %d %d true", x, y, z),
			3*time.Second,
		)
		if isTimeout {
			lastErr = fmt.Errorf("timeout")
			continue
		}
		if err != nil {
			lastErr = err
			continue
		}
		if msg := commandFailedMessage(resp); msg != "" {
			lastErr = errors.New(msg)
			continue
		}
		if resp.Respond == nil || resp.Respond.SuccessCount > 0 || len(resp.Respond.OutputMessages) == 0 || resp.Respond.OutputMessages[0].Success {
			return nil
		}
		lastErr = fmt.Errorf("unknown tickingarea preload failure")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unknown tickingarea preload failure")
	}
	return fmt.Errorf("tickingarea preload failed after %d attempts: %w", importRetryLimit, lastErr)
}

func downgradeTickingArea(err error, extra map[string]any) {
	args := map[string]any{
		"error": err.Error(),
	}
	for k, v := range extra {
		args[k] = v
	}
	log.Log.Warn("tickingarea unavailable; fallback to no ticking area import", log.Log.ArgsFromMap(args))
}

func resolveImportCommandSpeed(client *clientType.Client) int {
	if client != nil && client.Cdump_Setting != nil && client.Cdump_Setting.Speed > 0 {
		return client.Cdump_Setting.Speed
	}
	return defaultImportCommandSpeed
}

func newCommandRateLimiter(commandsPerSecond int) *rate.Limiter {
	if commandsPerSecond <= 0 {
		commandsPerSecond = defaultImportCommandSpeed
	}
	return rate.NewLimiter(rate.Limit(commandsPerSecond), commandsPerSecond)
}

func moduleBounds(blocks []*types.Module) (minX, maxX, minY, maxY, minZ, maxZ int, ok bool) {
	if len(blocks) == 0 {
		return 0, 0, 0, 0, 0, 0, false
	}

	minX, maxX = blocks[0].Point.X, blocks[0].Point.X
	minY, maxY = blocks[0].Point.Y, blocks[0].Point.Y
	minZ, maxZ = blocks[0].Point.Z, blocks[0].Point.Z
	for _, block := range blocks {
		if block.Point.X < minX {
			minX = block.Point.X
		}
		if block.Point.X > maxX {
			maxX = block.Point.X
		}
		if block.Point.Y < minY {
			minY = block.Point.Y
		}
		if block.Point.Y > maxY {
			maxY = block.Point.Y
		}
		if block.Point.Z < minZ {
			minZ = block.Point.Z
		}
		if block.Point.Z > maxZ {
			maxZ = block.Point.Z
		}
	}
	return minX, maxX, minY, maxY, minZ, maxZ, true
}

func teleportNearModules(client *clientType.Client, blocks []*types.Module, limiter *rate.Limiter, ctx context.Context) bool {
	minX, maxX, _, maxY, minZ, maxZ, ok := moduleBounds(blocks)
	if !ok {
		return false
	}
	teleportSafe(client, (minX+maxX)/2, maxY+2, (minZ+maxZ)/2, limiter, ctx)
	return true
}

func writeCommandBlockUpdate(client *clientType.Client, block *types.Module) error {
	if err := clientConnError(client); err != nil {
		return err
	}
	if block == nil || block.CommandBlockData == nil {
		return nil
	}
	return client.Conn.WritePacket(&packet.CommandBlockUpdate{
		Block: true,
		Position: protocol.BlockPos{
			int32(block.Point.X),
			int32(block.Point.Y),
			int32(block.Point.Z),
		},
		Mode:               block.CommandBlockData.Mode,
		NeedsRedstone:      block.CommandBlockData.NeedsRedstone,
		Conditional:        block.CommandBlockData.Conditional,
		Command:            block.CommandBlockData.Command,
		LastOutput:         block.CommandBlockData.LastOutput,
		Name:               block.CommandBlockData.CustomName,
		ShouldTrackOutput:  block.CommandBlockData.TrackOutput,
		TickDelay:          block.CommandBlockData.TickDelay,
		ExecuteOnFirstTick: block.CommandBlockData.ExecuteOnFirstTick,
	})
}

func sendImportKeepAlive(client *clientType.Client, last *time.Time) {
	if client == nil || client.GameInterface == nil || last == nil {
		return
	}
	if !last.IsZero() && time.Since(*last) < importKeepAliveInterval {
		return
	}
	*last = time.Now()
	resp := client.GameInterface.SendWSCommandWithResponse("", ResourcesControl.CommandRequestOptions{TimeOut: time.Second})
	if resp.Error != nil {
		log.Log.Warn("import keepalive failed", log.Log.ArgsFromMap(map[string]any{
			"error": resp.Error.Error(),
		}))
	}
}

func NewChunkRegionManager(regionSize int) *ChunkRegionManager {
	return &ChunkRegionManager{
		RegionSize:  regionSize,
		ChunkSize:   16,
		Regions:     make(map[[2]int]*RegionData),
		RegionOrder: [][2]int{},
	}
}

func (m *ChunkRegionManager) EnableNBTPrefetch(dimensionID uint8) {
	if m == nil || m.NBTPrefetchOn {
		return
	}
	m.NBTPrefetchOn = true
	m.NBTPrefetchDimID = dimensionID
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 4 {
		workerCount = 4
	}
	if workerCount > 12 {
		workerCount = 12
	}
	if m.RegionSize >= 8 && workerCount < 8 {
		workerCount = 8
	}
	if m.RegionSize >= 10 && workerCount < 12 {
		workerCount = 12
	}
	queueSize := workerCount * 1024
	if queueSize < 4096 {
		queueSize = 4096
	}
	if m.RegionSize >= 10 && queueSize < 8192 {
		queueSize = 8192
	}
	m.nbtPrepareQueue = make(chan *preparedNBTTask, queueSize)
	m.nbtPrepareWG.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func(workerID int) {
			defer m.nbtPrepareWG.Done()
			for task := range m.nbtPrepareQueue {
				if task == nil {
					continue
				}
				task.prepared, task.err = NBTAssigner.PrepareBlockWithNBTData(task.module, task.additionalData)
				if task.err != nil {
					log.Log.Warn("NBT prepare failed", log.Log.ArgsFromMap(map[string]any{"workerID": workerID, "error": task.err}))
				}
				task.module = nil
				close(task.done)
			}
		}(i)
	}
}

func (m *ChunkRegionManager) StopNBTPrefetch() {
	if m == nil || !m.NBTPrefetchOn {
		return
	}
	if m.nbtRegionQueue != nil {
		close(m.nbtRegionQueue)
		m.nbtRegionQueue = nil
	}
	m.nbtRegionWG.Wait()
	if m.nbtPrepareQueue != nil {
		close(m.nbtPrepareQueue)
		m.nbtPrepareQueue = nil
	}
	m.nbtPrepareWG.Wait()
	m.NBTPrefetchOn = false
	m.nbtPrepareTasks = sync.Map{}
	m.nbtRegionTasks = sync.Map{}
	m.nbtRegionSource = nil
}

func (m *ChunkRegionManager) ConfigureNBTRegionPrefetch(source *nbtRegionPrefetchSource) {
	if m == nil || !m.NBTPrefetchOn || source == nil || source.bw == nil || m.nbtRegionQueue != nil {
		m.nbtRegionSource = source
		return
	}
	m.nbtRegionSource = source
	m.nbtRegionQueue = make(chan *preparedNBTRegionTask, 128)
	m.nbtRegionWG.Add(1)
	go func() {
		defer m.nbtRegionWG.Done()
		for task := range m.nbtRegionQueue {
			if task == nil {
				continue
			}
			task.blocks, task.err = m.readNBTRegionBlocks(task)
			close(task.done)
		}
	}()
}

func (m *ChunkRegionManager) ensurePreparedNBTRegionTask(regionKey [2]int, targetBounds mcworldBounds) *preparedNBTRegionTask {
	if m == nil {
		return nil
	}
	if value, ok := m.nbtRegionTasks.Load(regionKey); ok {
		task, _ := value.(*preparedNBTRegionTask)
		if task != nil && targetBounds.valid() {
			task.targetBounds = targetBounds
		}
		return task
	}
	task := &preparedNBTRegionTask{
		key:          regionKey,
		targetBounds: targetBounds,
		done:         make(chan struct{}),
	}
	actual, _ := m.nbtRegionTasks.LoadOrStore(regionKey, task)
	realTask, _ := actual.(*preparedNBTRegionTask)
	if realTask != nil && targetBounds.valid() {
		realTask.targetBounds = targetBounds
	}
	return realTask
}

func (m *ChunkRegionManager) queueCompletedRegionNBTReadTasks() {
	if m == nil || !m.NBTPrefetchOn || m.nbtRegionQueue == nil || m.nbtRegionSource == nil {
		return
	}
	for _, regionKey := range m.GetSortedRegions() {
		region, ok := m.Regions[regionKey]
		if !ok || region == nil || region.Processed || !m.IsRegionComplete(regionKey) {
			continue
		}
		targetBounds, ok := m.regionSeenChunkBounds(region)
		if !ok {
			continue
		}
		task := m.ensurePreparedNBTRegionTask(regionKey, targetBounds)
		if task == nil {
			continue
		}
		task.queueOnce.Do(func() {
			m.nbtRegionQueue <- task
		})
	}
}

func (m *ChunkRegionManager) attachRegionNBTBlocks(regionKey [2]int, region *RegionData) (bool, error) {
	if m == nil || !m.NBTPrefetchOn || region == nil || m.nbtRegionSource == nil {
		return true, nil
	}
	targetBounds, ok := m.regionSeenChunkBounds(region)
	if !ok {
		return true, nil
	}
	task := m.ensurePreparedNBTRegionTask(regionKey, targetBounds)
	if task == nil {
		return true, nil
	}
	task.queueOnce.Do(func() {
		if m.nbtRegionQueue != nil {
			m.nbtRegionQueue <- task
		}
	})
	select {
	case <-task.done:
	default:
		return false, nil
	}
	if task.err != nil {
		return true, task.err
	}
	if atomic.CompareAndSwapUint32(&task.attached, 0, 1) && len(task.blocks) > 0 {
		for _, block := range task.blocks {
			if block == nil {
				continue
			}
			if isCommandBlockPrefetchExcluded(block) {
				if block.CommandBlockData == nil {
					name := ""
					if block.Block != nil && block.Block.Name != nil {
						name = strings.TrimPrefix(*block.Block.Name, "minecraft:")
					}
					block.CommandBlockData = &types.CommandBlockData{
						Mode:               commandBlockModeFromName(name),
						Command:            readStringFromNBTMap(block.NBTMap, "Command"),
						CustomName:         readStringFromNBTMap(block.NBTMap, "CustomName"),
						LastOutput:         readStringFromNBTMap(block.NBTMap, "LastOutput"),
						TickDelay:          readInt32FromNBTMap(block.NBTMap, "TickDelay"),
						ExecuteOnFirstTick: readBoolFromNBTMap(block.NBTMap, "ExecuteOnFirstTick", true),
						TrackOutput:        readBoolFromNBTMap(block.NBTMap, "TrackOutput", true),
						Conditional:        readBoolFromNBTMap(block.NBTMap, "conditionalMode", false),
						NeedsRedstone:      !readBoolFromNBTMap(block.NBTMap, "auto", true),
					}
				}
				region.CommandBlocks = append(region.CommandBlocks, block)
				continue
			}
			region.NBTBlocks = append(region.NBTBlocks, block)
		}
	}
	return true, nil
}

func commandBlockModeFromName(name string) uint32 {
	switch strings.TrimPrefix(strings.ToLower(name), "minecraft:") {
	case "chain_command_block":
		return packet.CommandBlockChain
	case "repeating_command_block":
		return packet.CommandBlockRepeating
	default:
		return packet.CommandBlockImpulse
	}
}

func readStringFromNBTMap(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	if got, ok := values[key].(string); ok {
		return got
	}
	return ""
}

func readInt32FromNBTMap(values map[string]interface{}, key string) int32 {
	if values == nil {
		return 0
	}
	if got, ok := values[key]; ok {
		if value, ok := nbtMapInt32(got); ok {
			return value
		}
	}
	return 0
}

func readBoolFromNBTMap(values map[string]interface{}, key string, fallback bool) bool {
	if values == nil {
		return fallback
	}
	if got, ok := values[key]; ok {
		if value, ok := nbtMapBool(got); ok {
			return value
		}
	}
	return fallback
}

func cloneNBTMap(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]interface{}, len(values))
	for k, v := range values {
		cloned[k] = v
	}
	return cloned
}

func buildAttachmentImportModule(name string, state string, pos types.Position, nbtMap map[string]interface{}, importNBT bool, importCommandBlock bool, defaultSignWax bool) (*types.Module, string) {
	if len(nbtMap) == 0 {
		return nil, ""
	}

	normalizedName := strings.TrimPrefix(strings.ToLower(name), "minecraft:")
	blockName := strings.TrimPrefix(name, "minecraft:")
	module := &types.Module{
		Block: &types.Block{
			Name:        &blockName,
			BlockStates: state,
		},
		Point: pos,
	}

	if isCommandBlockName(normalizedName) {
		if !importCommandBlock {
			return nil, ""
		}
		module.CommandBlockData = &types.CommandBlockData{
			Mode:               commandBlockModeFromName(normalizedName),
			Command:            readStringFromNBTMap(nbtMap, "Command"),
			CustomName:         readStringFromNBTMap(nbtMap, "CustomName"),
			LastOutput:         readStringFromNBTMap(nbtMap, "LastOutput"),
			TickDelay:          readInt32FromNBTMap(nbtMap, "TickDelay"),
			ExecuteOnFirstTick: readBoolFromNBTMap(nbtMap, "ExecuteOnFirstTick", true),
			TrackOutput:        readBoolFromNBTMap(nbtMap, "TrackOutput", true),
			Conditional:        readBoolFromNBTMap(nbtMap, "conditionalMode", false),
			NeedsRedstone:      !readBoolFromNBTMap(nbtMap, "auto", true),
		}
		return module, "command"
	}

	if !importNBT {
		return nil, ""
	}
	module.NBTMap = cloneNBTMap(nbtMap)
	if defaultSignWax && isSignBlockName(normalizedName) {
		module.NBTMap = applyDefaultSignWax(module.NBTMap)
	}
	return module, "nbt"
}

func nbtMapInt32(v interface{}) (int32, bool) {
	switch val := v.(type) {
	case int32:
		return val, true
	case int16:
		return int32(val), true
	case int8:
		return int32(val), true
	case byte:
		return int32(val), true
	case int:
		return int32(val), true
	case int64:
		return int32(val), true
	case float64:
		return int32(val), true
	default:
		return 0, false
	}
}

func nbtMapBool(v interface{}) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case byte:
		return val != 0, true
	case int:
		return val != 0, true
	case int32:
		return val != 0, true
	case int64:
		return val != 0, true
	case float64:
		return val != 0, true
	default:
		return false, false
	}
}

func (m *ChunkRegionManager) readNBTRegionBlocks(task *preparedNBTRegionTask) ([]*types.Module, error) {
	if m == nil || task == nil || !task.targetBounds.valid() || m.nbtRegionSource == nil || m.nbtRegionSource.bw == nil {
		return nil, nil
	}
	source := m.nbtRegionSource
	sourceBounds := mcworldBounds{
		minX: task.targetBounds.minX - int32(source.offsetX),
		minY: task.targetBounds.minY - int32(source.offsetY),
		minZ: task.targetBounds.minZ - int32(source.offsetZ),
		maxX: task.targetBounds.maxX - int32(source.offsetX),
		maxY: task.targetBounds.maxY - int32(source.offsetY),
		maxZ: task.targetBounds.maxZ - int32(source.offsetZ),
	}
	if sourceBounds.minX < source.bounds.minX {
		sourceBounds.minX = source.bounds.minX
	}
	if sourceBounds.minY < source.bounds.minY {
		sourceBounds.minY = source.bounds.minY
	}
	if sourceBounds.minZ < source.bounds.minZ {
		sourceBounds.minZ = source.bounds.minZ
	}
	if sourceBounds.maxX > source.bounds.maxX {
		sourceBounds.maxX = source.bounds.maxX
	}
	if sourceBounds.maxY > source.bounds.maxY {
		sourceBounds.maxY = source.bounds.maxY
	}
	if sourceBounds.maxZ > source.bounds.maxZ {
		sourceBounds.maxZ = source.bounds.maxZ
	}
	if !sourceBounds.valid() {
		return nil, nil
	}

	minChunkX := int32(floorDiv(int(sourceBounds.minX), 16))
	maxChunkX := int32(floorDiv(int(sourceBounds.maxX), 16))
	minChunkZ := int32(floorDiv(int(sourceBounds.minZ), 16))
	maxChunkZ := int32(floorDiv(int(sourceBounds.maxZ), 16))
	dimension := bwo_define.Dimension(source.sourceDimID)
	blocks := make([]*types.Module, 0)

	for chunkX := minChunkX; chunkX <= maxChunkX; chunkX++ {
		for chunkZ := minChunkZ; chunkZ <= maxChunkZ; chunkZ++ {
			if source.skipChunk != nil && source.skipChunk(chunkX, chunkZ) {
				continue
			}
			pos := bwo_define.ChunkPos{chunkX, chunkZ}
			entries, err := source.bw.LoadNBT(dimension, pos)
			if err != nil {
				return nil, err
			}
			if len(entries) == 0 {
				continue
			}

			chunkData, exists, err := source.bw.LoadChunk(dimension, pos)
			if err != nil {
				return nil, err
			}
			if !exists || chunkData == nil {
				continue
			}

			chunkStartX := chunkX * 16
			chunkStartZ := chunkZ * 16
			for _, entry := range entries {
				xVal, okX := toInt32(entry["x"])
				yVal, okY := toInt32(entry["y"])
				zVal, okZ := toInt32(entry["z"])
				if !okX || !okY || !okZ {
					continue
				}
				if xVal < sourceBounds.minX || xVal > sourceBounds.maxX || yVal < sourceBounds.minY || yVal > sourceBounds.maxY || zVal < sourceBounds.minZ || zVal > sourceBounds.maxZ {
					continue
				}

				runtimeID := pickRuntimeID(chunkData, uint8(xVal-chunkStartX), int16(yVal), uint8(zVal-chunkStartZ))
				if runtimeID == block.AirRuntimeID {
					continue
				}
				name, state, found := convertRuntimeToNameState(runtimeID)
				if !found {
					continue
				}
				name, ok := normalizeBlockName(name)
				if !ok || name == "air" || name == "minecraft:air" {
					continue
				}
				name = strings.TrimPrefix(name, "minecraft:")
				isCommandBlock := isCommandBlockName(name)
				if isCommandBlock {
					if !source.importCommandBlock {
						continue
					}
				} else if !source.importNBT {
					continue
				}

				targetX := int(xVal) + source.offsetX
				targetY := int(yVal) + source.offsetY
				targetZ := int(zVal) + source.offsetZ
				if targetX < int(task.targetBounds.minX) || targetX > int(task.targetBounds.maxX) || targetY < int(task.targetBounds.minY) || targetY > int(task.targetBounds.maxY) || targetZ < int(task.targetBounds.minZ) || targetZ > int(task.targetBounds.maxZ) {
					continue
				}

				nbtData := entry
				if source.defaultSignWax && !isCommandBlock && isSignBlockName(name) {
					nbtData = applyDefaultSignWax(nbtData)
				}
				module := &types.Module{
					Block: &types.Block{
						Name:        &name,
						BlockStates: state,
					},
					Point:  types.Position{X: targetX, Y: targetY, Z: targetZ},
					NBTMap: nbtData,
				}
				blocks = append(blocks, module)
			}
		}
	}
	return blocks, nil
}

func walkMCWorldAttachmentBlocks(
	bw *world.BedrockWorld,
	bounds mcworldBounds,
	sourceDimID int32,
	chunkPlan importChunkPlan,
	offsetX int,
	offsetY int,
	offsetZ int,
	task types.Task,
	visit func(module *types.Module, attachmentType string) error,
) (importAttachmentCounts, error) {
	var counts importAttachmentCounts
	if bw == nil || !bounds.valid() || (!task.ImportNBT && !task.ImportCommandBlock) {
		return counts, nil
	}

	dimension := bwo_define.Dimension(sourceDimID)
	for chunkX := chunkPlan.minChunkX; chunkX <= chunkPlan.maxChunkX; chunkX++ {
		for chunkZ := chunkPlan.minChunkZ; chunkZ <= chunkPlan.maxChunkZ; chunkZ++ {
			sourceChunkX := int32(chunkX)
			sourceChunkZ := int32(chunkZ)
			if chunkPlan.skipChunk != nil && chunkPlan.skipChunk(sourceChunkX, sourceChunkZ) {
				continue
			}

			pos := bwo_define.ChunkPos{sourceChunkX, sourceChunkZ}
			entries, err := bw.LoadNBT(dimension, pos)
			if err != nil {
				return counts, err
			}
			if len(entries) == 0 {
				continue
			}

			chunkData, exists, err := bw.LoadChunk(dimension, pos)
			if err != nil {
				return counts, err
			}
			if !exists || chunkData == nil {
				continue
			}

			chunkStartX := sourceChunkX * 16
			chunkStartZ := sourceChunkZ * 16
			for _, entry := range entries {
				xVal, okX := toInt32(entry["x"])
				yVal, okY := toInt32(entry["y"])
				zVal, okZ := toInt32(entry["z"])
				if !okX || !okY || !okZ {
					continue
				}
				if xVal < bounds.minX || xVal > bounds.maxX || yVal < bounds.minY || yVal > bounds.maxY || zVal < bounds.minZ || zVal > bounds.maxZ {
					continue
				}
				if xVal < chunkStartX || xVal > chunkStartX+15 || zVal < chunkStartZ || zVal > chunkStartZ+15 {
					continue
				}

				runtimeID := pickRuntimeID(chunkData, uint8(xVal-chunkStartX), int16(yVal), uint8(zVal-chunkStartZ))
				if runtimeID == block.AirRuntimeID {
					continue
				}
				name, state, found := convertRuntimeToNameState(runtimeID)
				if !found {
					continue
				}
				name, ok := normalizeBlockName(name)
				if !ok || name == "air" || name == "minecraft:air" {
					continue
				}

				module, attachmentType := buildAttachmentImportModule(
					name,
					state,
					types.Position{X: int(xVal) + offsetX, Y: int(yVal) + offsetY, Z: int(zVal) + offsetZ},
					entry,
					task.ImportNBT,
					task.ImportCommandBlock,
					task.DefaultSignWax,
				)
				if module == nil || attachmentType == "" {
					continue
				}
				switch attachmentType {
				case "command":
					counts.commandBlocks++
				case "nbt":
					counts.nbtBlocks++
				}
				if visit != nil {
					if err := visit(module, attachmentType); err != nil {
						return counts, err
					}
				}
			}
		}
	}
	return counts, nil
}

func nbtTaskKeyForBlock(block *types.Module) (nbtTaskKey, bool) {
	if block == nil {
		return nbtTaskKey{}, false
	}
	return nbtTaskKey{X: block.Point.X, Y: block.Point.Y, Z: block.Point.Z}, true
}

func (m *ChunkRegionManager) ensurePreparedNBTTask(block *types.Module) *preparedNBTTask {
	if block == nil {
		return nil
	}
	key, ok := nbtTaskKeyForBlock(block)
	if !ok {
		return nil
	}
	if task, ok := m.nbtPrepareTasks.Load(key); ok {
		return task.(*preparedNBTTask)
	}

	blockStates := ""
	if block.Block != nil {
		blockStates = block.Block.BlockStates
	}
	additionalData := &NBTAssigner.BlockAdditionalData{
		BlockStates: blockStates,
		Position:    [3]int32{int32(block.Point.X), int32(block.Point.Y), int32(block.Point.Z)},
		DimensionID: m.NBTPrefetchDimID,
		FastMode:    false,
		Others:      nil,
	}
	task := &preparedNBTTask{
		key:            key,
		module:         cloneModuleForNBTPrefetch(block),
		additionalData: additionalData,
		done:           make(chan struct{}),
	}
	actual, _ := m.nbtPrepareTasks.LoadOrStore(key, task)
	return actual.(*preparedNBTTask)
}

func cloneModuleForNBTPrefetch(block *types.Module) *types.Module {
	if block == nil {
		return nil
	}

	cloned := &types.Module{
		Point: block.Point,
	}
	if block.Block != nil && block.Block.Name != nil {
		name := *block.Block.Name
		cloned.Block = &types.Block{
			Name:        &name,
			BlockStates: block.Block.BlockStates,
		}
	}
	if block.NBTMap != nil {
		cloned.NBTMap = make(map[string]interface{}, len(block.NBTMap))
		for k, v := range block.NBTMap {
			cloned.NBTMap[k] = v
		}
	}
	return cloned
}

func cloneRegionForFill(region *RegionData) *RegionData {
	if region == nil {
		return nil
	}
	cloned := &RegionData{
		Blocks: make(map[int][]*types.Module, len(region.Blocks)),
		MinY:   region.MinY,
		MaxY:   region.MaxY,
	}
	for y, blocks := range region.Blocks {
		if len(blocks) == 0 {
			continue
		}
		copied := make([]*types.Module, len(blocks))
		copy(copied, blocks)
		cloned.Blocks[y] = copied
	}
	return cloned
}

func (m *ChunkRegionManager) queueNBTPreparation(block *types.Module) {
	if m == nil || !m.NBTPrefetchOn || m.nbtPrepareQueue == nil || block == nil {
		return
	}
	if isCommandBlockPrefetchExcluded(block) {
		return
	}
	task := m.ensurePreparedNBTTask(block)
	if task == nil {
		return
	}
	if !isCommandBlockPrefetchExcluded(block) {
		block.NBTMap = nil
		block.NBTData = nil
		block.DebugNBTData = nil
	}
	task.queueOnce.Do(func() {
		m.nbtPrepareQueue <- task
	})
}

func (m *ChunkRegionManager) waitPreparedNBT(block *types.Module, dimensionID uint8) (*preparedNBTTask, error) {
	if block == nil {
		return nil, fmt.Errorf("nil nbt block")
	}
	if isCommandBlockPrefetchExcluded(block) {
		return nil, fmt.Errorf("command block should not use nbt prefetch")
	}
	if !m.NBTPrefetchOn {
		m.EnableNBTPrefetch(dimensionID)
	}
	for attempt := 0; attempt < 2; attempt++ {
		task := m.ensurePreparedNBTTask(block)
		if task == nil {
			return nil, fmt.Errorf("failed to create nbt task")
		}
		if task.additionalData != nil {
			task.additionalData.DimensionID = dimensionID
		}
		m.queueNBTPreparation(block)
		<-task.done
		if task.err == nil || attempt > 0 {
			return task, task.err
		}
		m.nbtPrepareTasks.Delete(task.key)
	}
	return nil, fmt.Errorf("failed to prepare nbt task")
}

func (m *ChunkRegionManager) waitRegionPreparedNBTTasks(blocks []*types.Module) {
	if m == nil || !m.NBTPrefetchOn || len(blocks) == 0 {
		return
	}
	for _, block := range blocks {
		if block == nil || isCommandBlockPrefetchExcluded(block) {
			continue
		}
		key, ok := nbtTaskKeyForBlock(block)
		if !ok {
			continue
		}
		value, ok := m.nbtPrepareTasks.Load(key)
		if !ok {
			continue
		}
		task, ok := value.(*preparedNBTTask)
		if !ok || task == nil {
			continue
		}
		<-task.done
	}
}

func (m *ChunkRegionManager) deletePreparedNBTTask(block *types.Module) {
	key, ok := nbtTaskKeyForBlock(block)
	if !ok {
		return
	}
	m.nbtPrepareTasks.Delete(key)
}

// premergeCrossRegionBlocks is retained for the attachment-region pipeline; normal block import is handled by build.go.
func (m *ChunkRegionManager) premergeCrossRegionBlocks(regionKey [2]int) {
	return
}

func isCommandBlockPrefetchExcluded(block *types.Module) bool {
	if block == nil || block.Block == nil || block.Block.Name == nil {
		return false
	}
	return isCommandBlockName(strings.TrimPrefix(*block.Block.Name, "minecraft:"))
}

func (m *ChunkRegionManager) ensureRegion(regionKey [2]int, minY, maxY int) *RegionData {
	if region, exists := m.Regions[regionKey]; exists {
		if minY < region.MinY {
			region.MinY = minY
		}
		if maxY > region.MaxY {
			region.MaxY = maxY
		}
		return region
	}
	region := &RegionData{
		Blocks: make(map[int][]*types.Module),
		MinY:   minY,
		MaxY:   maxY,
	}
	m.Regions[regionKey] = region
	m.RegionOrder = append(m.RegionOrder, regionKey)
	return region
}

func (m *ChunkRegionManager) markChunkSeen(region *RegionData, chunkX, chunkZ int) {
	if region == nil {
		return
	}
	if region.SeenChunks == nil {
		region.SeenChunks = make(map[[2]int]bool)
	}
	region.SeenChunks[[2]int{chunkX, chunkZ}] = true
	if region.FirstSeen == 0 {
		region.FirstSeen = time.Now().UnixNano()
	}
}

func (m *ChunkRegionManager) TouchChunk(chunkStartX, chunkStartZ int32, minY, maxY int) {
	region := m.ensureRegion(m.regionKeyForBlockCoords(int(chunkStartX), int(chunkStartZ)), minY, maxY)
	m.markChunkSeen(region, int(chunkStartX>>4), int(chunkStartZ>>4))
}

func (m *ChunkRegionManager) RecordTraversedChunk(chunkStartX, chunkStartZ int32) {
	progress := getTraversedChunkMap(m)
	if progress == nil {
		return
	}
	regionKey := m.regionKeyForBlockCoords(int(chunkStartX), int(chunkStartZ))
	minY, maxY := 0, 0
	if m.BuildBounds.valid() {
		minY = int(m.BuildBounds.minY)
		maxY = int(m.BuildBounds.maxY)
	}
	region := m.ensureRegion(regionKey, minY, maxY)
	m.markChunkSeen(region, int(chunkStartX>>4), int(chunkStartZ>>4))
	progress[regionKey]++
}

func (m *ChunkRegionManager) traversedChunkCount(regionKey [2]int) int {
	progress := getTraversedChunkMap(m)
	if progress == nil {
		return 0
	}
	return progress[regionKey]
}

func (m *ChunkRegionManager) clearTraversedChunkCount(regionKey [2]int) {
	progress := getTraversedChunkMap(m)
	if progress == nil {
		return
	}
	delete(progress, regionKey)
}

func (m *ChunkRegionManager) advanceChunkProgress(chunkCount int, bar *progressbar.ProgressBar) {
	if chunkCount <= 0 {
		return
	}
	m.ProcessedChunks += chunkCount
	if bar != nil {
		bar.Add(chunkCount)
	}
	if m.GameProgress != nil && m.TotalChunks > 0 {
		m.GameProgress.SetChunkProgress(m.ProcessedChunks, m.TotalChunks)
	}
	if m.ProgressCb != nil {
		m.ProgressCb(m.ProcessedChunks, m.TotalChunks)
	}
}

func (m *ChunkRegionManager) regionKeyForBlockCoords(x, z int) [2]int {
	regionSize := m.RegionSize
	if regionSize < 1 {
		regionSize = 1
	}
	chunkSize := m.ChunkSize
	if chunkSize < 1 {
		chunkSize = 16
	}
	chunkX := floorDiv(x, chunkSize)
	chunkZ := floorDiv(z, chunkSize)
	return [2]int{
		floorDiv(chunkX-m.RegionBaseChunkX, regionSize),
		floorDiv(chunkZ-m.RegionBaseChunkZ, regionSize),
	}
}

func (m *ChunkRegionManager) GetRegionKey(x, z int) [2]int {
	return m.regionKeyForBlockCoords(x, z)
}

func (m *ChunkRegionManager) AddBlock(block *types.Module, blockType string) {
	regionKey := m.regionKeyForBlockCoords(block.Point.X, block.Point.Z)
	region := m.ensureRegion(regionKey, block.Point.Y, block.Point.Y)
	m.markChunkSeen(region, block.Point.X>>4, block.Point.Z>>4)

	if block.Point.Y < region.MinY {
		region.MinY = block.Point.Y
	}
	if block.Point.Y > region.MaxY {
		region.MaxY = block.Point.Y
	}

	addedRefs := 0
	switch blockType {
	case "normal":
		if _, ok := region.Blocks[block.Point.Y]; !ok {
			region.Blocks[block.Point.Y] = []*types.Module{}
		}
		region.Blocks[block.Point.Y] = append(region.Blocks[block.Point.Y], block)
		addedRefs++
	case "chest":
		region.Chests = append(region.Chests, block)
		addedRefs++
	case "special":
		region.SpecialBlocks = append(region.SpecialBlocks, block)
		addedRefs++
	}
	region.BufferedRefs += addedRefs
	m.BufferedRefs += addedRefs
}

func (m *ChunkRegionManager) AddAttachmentBlock(block *types.Module, attachmentType string) {
	if m == nil || block == nil {
		return
	}
	regionKey := m.regionKeyForBlockCoords(block.Point.X, block.Point.Z)
	region := m.ensureRegion(regionKey, block.Point.Y, block.Point.Y)
	m.markChunkSeen(region, block.Point.X>>4, block.Point.Z>>4)

	if block.Point.Y < region.MinY {
		region.MinY = block.Point.Y
	}
	if block.Point.Y > region.MaxY {
		region.MaxY = block.Point.Y
	}

	addedRefs := 0
	switch attachmentType {
	case "command":
		region.CommandBlocks = append(region.CommandBlocks, block)
		addedRefs++
	case "nbt":
		region.NBTBlocks = append(region.NBTBlocks, block)
		addedRefs++
	}
	region.BufferedRefs += addedRefs
	m.BufferedRefs += addedRefs
}

// IsRegionComplete reports whether every expected chunk in this region has been seen.
func (m *ChunkRegionManager) IsRegionComplete(regionKey [2]int) bool {
	region, ok := m.Regions[regionKey]
	if !ok || region == nil {
		return false
	}
	expected := m.expectedChunksInRegion(regionKey)
	if expected <= 0 {
		return false
	}
	return len(region.SeenChunks) >= expected
}

func (m *ChunkRegionManager) expectedChunksInRegion(regionKey [2]int) int {
	minChunkX := maxInt(m.MinChunkX, m.RegionBaseChunkX+regionKey[0]*m.RegionSize)
	maxChunkX := minInt(m.MaxChunkX, m.RegionBaseChunkX+regionKey[0]*m.RegionSize+m.RegionSize-1)
	minChunkZ := maxInt(m.MinChunkZ, m.RegionBaseChunkZ+regionKey[1]*m.RegionSize)
	maxChunkZ := minInt(m.MaxChunkZ, m.RegionBaseChunkZ+regionKey[1]*m.RegionSize+m.RegionSize-1)
	if maxChunkX < minChunkX || maxChunkZ < minChunkZ {
		return 0
	}
	return (maxChunkX - minChunkX + 1) * (maxChunkZ - minChunkZ + 1)
}

func (m *ChunkRegionManager) FindFlushableRegion(timeout time.Duration) (int, [2]int, bool) {
	for i, key := range m.RegionOrder {
		region, ok := m.Regions[key]
		if !ok || region == nil {
			continue
		}
		if m.IsRegionComplete(key) {
			return i, key, true
		}
		if region.FirstSeen != 0 && time.Since(time.Unix(0, region.FirstSeen)) > timeout {
			return i, key, true
		}
	}
	return -1, [2]int{}, false
}

func (m *ChunkRegionManager) GetRegionCenter(regionKey [2]int) (int, int) {
	centerChunkX := m.RegionBaseChunkX + regionKey[0]*m.RegionSize + m.RegionSize/2
	centerChunkZ := m.RegionBaseChunkZ + regionKey[1]*m.RegionSize + m.RegionSize/2
	return centerChunkX * m.ChunkSize, centerChunkZ * m.ChunkSize
}

func (m *ChunkRegionManager) regionBounds(regionKey [2]int) (int, int, int, int) {
	minChunkX := m.RegionBaseChunkX + regionKey[0]*m.RegionSize
	minChunkZ := m.RegionBaseChunkZ + regionKey[1]*m.RegionSize
	maxChunkX := minChunkX + m.RegionSize - 1
	maxChunkZ := minChunkZ + m.RegionSize - 1
	minX := minChunkX * m.ChunkSize
	minZ := minChunkZ * m.ChunkSize
	maxX := maxChunkX*m.ChunkSize + (m.ChunkSize - 1)
	maxZ := maxChunkZ*m.ChunkSize + (m.ChunkSize - 1)
	return minX, minZ, maxX, maxZ
}

func (m *ChunkRegionManager) regionChunkBounds(regionKey [2]int, region *RegionData) (int, int, int, int) {
	minChunkX := m.RegionBaseChunkX + regionKey[0]*m.RegionSize
	minChunkZ := m.RegionBaseChunkZ + regionKey[1]*m.RegionSize
	maxChunkX := minChunkX + m.RegionSize - 1
	maxChunkZ := minChunkZ + m.RegionSize - 1
	if region != nil {
		if bounds, ok := m.regionSeenChunkBounds(region); ok {
			minChunkX = floorDiv(int(bounds.minX), m.ChunkSize)
			minChunkZ = floorDiv(int(bounds.minZ), m.ChunkSize)
			maxChunkX = floorDiv(int(bounds.maxX), m.ChunkSize)
			maxChunkZ = floorDiv(int(bounds.maxZ), m.ChunkSize)
		}
	}
	return minChunkX, minChunkZ, maxChunkX, maxChunkZ
}

func (m *ChunkRegionManager) shouldUseTickingArea() bool {
	return m.RegionSize >= 6
}

func (m *ChunkRegionManager) addRegionTickingArea(client *clientType.Client, regionKey [2]int, region *RegionData, minY, maxY int) []string {
	if client == nil || client.GameInterface == nil {
		return nil
	}
	if m != nil && m.BuildBounds.valid() {
		minY = int(m.BuildBounds.minY)
		maxY = int(m.BuildBounds.maxY)
	}
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	if minY < -64 {
		minY = -64
	}
	if maxY > 319 {
		maxY = 319
	}

	const maxChunks = 10
	minChunkX, minChunkZ, maxChunkX, maxChunkZ := m.regionChunkBounds(regionKey, region)
	names := []string{}
	idx := 1
	for chunkX := minChunkX; chunkX <= maxChunkX; chunkX += maxChunks {
		endChunkX := minInt(chunkX+maxChunks-1, maxChunkX)
		for chunkZ := minChunkZ; chunkZ <= maxChunkZ; chunkZ += maxChunks {
			endChunkZ := minInt(chunkZ+maxChunks-1, maxChunkZ)
			minX := chunkX * m.ChunkSize
			minZ := chunkZ * m.ChunkSize
			maxX := endChunkX*m.ChunkSize + (m.ChunkSize - 1)
			maxZ := endChunkZ*m.ChunkSize + (m.ChunkSize - 1)
			name := fmt.Sprintf("Nexus_%d", idx)
			if err := addNamedTickingAreaWithRetry(client, minX, minY, minZ, maxX, maxY, maxZ, name); err != nil {
				downgradeTickingArea(err, map[string]any{"name": name, "x": minX, "y": minY, "z": minZ, "maxX": maxX, "maxY": maxY, "maxZ": maxZ})
				idx++
				continue
			}
			if err := preloadTickingAreaWithRetry(client, minX, minY, minZ); err != nil {
				downgradeTickingArea(err, map[string]any{"name": name, "x": minX, "y": minY, "z": minZ})
				removeNamedTickingArea(client, name)
				idx++
				continue
			}
			names = append(names, name)
			idx++
		}
	}
	return names
}

func (m *ChunkRegionManager) preloadRegionChunks(client *clientType.Client, regionKey [2]int, region *RegionData) {
	if client == nil || client.GameInterface == nil {
		return
	}
	minChunkX, minChunkZ, maxChunkX, maxChunkZ := m.regionChunkBounds(regionKey, region)
	preloadY := 0
	if m != nil && m.BuildBounds.valid() {
		preloadY = clampChunkY(int((m.BuildBounds.minY + m.BuildBounds.maxY) / 2))
	}
	for chunkX := minChunkX; chunkX <= maxChunkX; chunkX++ {
		for chunkZ := minChunkZ; chunkZ <= maxChunkZ; chunkZ++ {
			_ = sendWSCommandWithResponseRaw(
				client,
				fmt.Sprintf("tickingarea preload %d %d %d true", chunkX*m.ChunkSize, preloadY, chunkZ*m.ChunkSize),
				ResourcesControl.CommandRequestOptions{WithNoResponse: true},
			)
		}
	}
}

func (m *ChunkRegionManager) removeRegionTickingArea(client *clientType.Client, names []string) {
	if client == nil || client.GameInterface == nil {
		return
	}
	for _, name := range names {
		if name != "" {
			removeNamedTickingArea(client, name)
		}
	}
}

// CleanupNexusTickingAreas removes Nexus ticking areas used during import.
func CleanupNexusTickingAreas(client *clientType.Client, silent ...bool) {
	if client == nil || client.GameInterface == nil {
		return
	}
	quiet := len(silent) > 0 && silent[0]
	if !quiet {
		log.Log.Info("正在清理导入常加载区块...")
	}
	removeAllTickingAreas(client)
	if !quiet {
		log.Log.Info("导入常加载区块清理完成")
	}
}

func (m *ChunkRegionManager) GetSortedRegions() [][2]int {
	if len(m.RegionOrder) == 0 {
		return [][2]int{}
	}
	xGroups := make(map[int][][2]int)
	minX, maxX := m.RegionOrder[0][0], m.RegionOrder[0][0]
	for _, key := range m.RegionOrder {
		x := key[0]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		xGroups[x] = append(xGroups[x], key)
	}
	for x := range xGroups {
		sort.Slice(xGroups[x], func(i, j int) bool { return xGroups[x][i][1] < xGroups[x][j][1] })
	}
	sorted := make([][2]int, 0, len(m.RegionOrder))
	for x := minX; x <= maxX; x++ {
		regions, ok := xGroups[x]
		if !ok {
			continue
		}
		if (x-minX)%2 == 0 {
			sorted = append(sorted, regions...)
			continue
		}
		for i := len(regions) - 1; i >= 0; i-- {
			sorted = append(sorted, regions[i])
		}
	}
	return sorted
}

func (m *ChunkRegionManager) ProcessRegion(client *clientType.Client, regionKey [2]int, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) bool {
	if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
		return false
	}
	region, exists := m.Regions[regionKey]
	if !exists || region.Processed {
		return true
	}
	regionChunkCount := m.regionChunkCount(regionKey, region)

	if !region.BlocksDone {
		log.Log.Info("处理导入分区附件", log.Log.ArgsFromMap(map[string]any{
			"region":         fmt.Sprintf("(%d,%d)", regionKey[0], regionKey[1]),
			"command_blocks": len(region.CommandBlocks),
			"nbt_blocks":     len(region.NBTBlocks),
			"chests":         len(region.Chests),
			"special_blocks": len(region.SpecialBlocks),
			"min_y":          region.MinY,
			"max_y":          region.MaxY,
		}))
		cleanup := m.prepareRegionImport(client, regionKey, region, limiter, ctx)
		defer cleanup()
		if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
			return false
		}
		region.BlocksDone = true
	}

	if !region.AttachmentsDone {
		attachmentsReady, err := m.attachRegionNBTBlocks(regionKey, region)
		if err != nil {
			log.Log.Warn(fmt.Sprintf("分区 NBT 附件准备失败: %v", err))
		}
		if !attachmentsReady {
			return false
		}
		m.processRegionAttachments(client, region, limiter, ctx)
		if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
			return false
		}
		region.AttachmentsDone = true
	}

	if err := m.placeProtectionForRegion(client, region, limiter, ctx); err != nil {
		if cancelImportOnFatalError(ctx, client, err) {
			return false
		}
		log.Log.Warn(fmt.Sprintf("分区保护方块放置失败: %v", err))
	}
	if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
		return false
	}
	m.stabilizeRegionAfterImport(client, regionKey, region, limiter, ctx)
	m.cleanRegionDrops(client, region, limiter, ctx)
	if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
		return false
	}
	if err := m.finishRegion(regionKey, region, regionChunkCount, bar); err != nil {
		m.PostVerifyErr = err
		cancelImportOnFatalError(ctx, client, err)
		log.Log.Warn("分区导入后验证失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		return false
	}
	return true
}
func (m *ChunkRegionManager) regionChunkCount(regionKey [2]int, region *RegionData) int {
	if traversed := m.traversedChunkCount(regionKey); traversed > 0 {
		return traversed
	}
	if region == nil {
		return 0
	}
	regionChunkCount := len(region.SeenChunks)
	if regionChunkCount <= 0 {
		return m.expectedChunksInRegion(regionKey)
	}
	return regionChunkCount
}

func (m *ChunkRegionManager) prepareRegionImport(client *clientType.Client, regionKey [2]int, region *RegionData, limiter *rate.Limiter, ctx context.Context) func() {
	cleanup := func() {}
	if m.shouldUseTickingArea() {
		if len(m.activeTickingAreas) > 0 && m.activeTickingRegion != regionKey {
			m.removeRegionTickingArea(client, m.activeTickingAreas)
			m.activeTickingAreas = nil
		}
		tickingAreas := m.addRegionTickingArea(client, regionKey, region, region.MinY, region.MaxY)
		m.activeTickingRegion = regionKey
		m.activeTickingAreas = tickingAreas
		m.preloadRegionChunks(client, regionKey, region)
	}

	if m.WaitChunkLoad {
		minChunkX, minChunkZ, _, _ := m.regionChunkBounds(regionKey, region)
		waitX := minChunkX * m.ChunkSize
		waitZ := minChunkZ * m.ChunkSize
		waitY := clampChunkY(region.MinY)
		if err := waitChunkAreaLoaded(client, waitX, waitY, waitZ, limiter, ctx); err != nil {
			if cancelImportOnFatalError(ctx, client, err) {
				return cleanup
			}
			log.Log.Warn(fmt.Sprintf("wait chunk area failed (%d, %d): %v", regionKey[0], regionKey[1], err))
		}
		if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
			return cleanup
		}
		if !waitChunkLoaded(client, waitX, waitY, waitZ, limiter, ctx, regionLoadWaitTimeout(region)) {
			if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
				return cleanup
			}
			log.Log.Warn(fmt.Sprintf("wait chunk area timeout (%d, %d)", regionKey[0], regionKey[1]))
		}
	}

	if m.AggressiveFlush {
		return cleanup
	}

	centerX, centerZ := m.GetRegionCenter(regionKey)
	if bounds, ok := m.regionSeenChunkBounds(region); ok {
		centerX = int((bounds.minX + bounds.maxX) / 2)
		centerZ = int((bounds.minZ + bounds.maxZ) / 2)
	}

	if limiter != nil {
		_ = limiter.Wait(ctx)
	}
	_ = sendSettingsCommandDim(client, fmt.Sprintf("tp @s %d %d %d", centerX, 320, centerZ))

	if client.Cdump_Setting != nil && client.Cdump_Setting.Clear_Building {
		if len(region.Blocks) == 0 {
			if bounds, ok := m.regionSeenChunkBounds(region); ok {
				clearSeenChunkBounds(client, bounds, limiter, ctx)
			}
		} else {
			clearRegionBox(client, region, limiter, ctx)
		}
	}

	return cleanup
}

func (m *ChunkRegionManager) processRegionAttachments(client *clientType.Client, region *RegionData, limiter *rate.Limiter, ctx context.Context) {
	if len(region.CommandBlocks) > 0 {
		cmdLimiter := limiter
		if m.CommandLimiter != nil {
			cmdLimiter = m.CommandLimiter
		}
		m.processCommandBlocksStable(client, region.CommandBlocks, cmdLimiter, ctx, nil)
		if importContextDone(ctx) {
			return
		}
	}

	if len(region.Chests) > 0 {
		m.processChests(client, region.Chests, limiter, ctx, nil)
		if importContextDone(ctx) {
			return
		}
	}

	if len(region.SpecialBlocks) > 0 {
		m.processSpecialBlocks(client, region.SpecialBlocks, limiter, ctx, nil)
		if importContextDone(ctx) {
			return
		}
	}

	if len(region.NBTBlocks) > 0 {
		m.processNBTBlocks(client, region.NBTBlocks, limiter, ctx, nil)
	}
}

func (m *ChunkRegionManager) stabilizeRegionAfterImport(client *clientType.Client, regionKey [2]int, region *RegionData, limiter *rate.Limiter, ctx context.Context) {
	if m == nil || region == nil || !m.WaitChunkLoad {
		return
	}
	if client == nil || client.Conn == nil {
		return
	}
	centerX, centerZ := m.GetRegionCenter(regionKey)
	testY := clampChunkY((region.MinY + region.MaxY) / 2)
	if bounds, ok := m.regionSeenChunkBounds(region); ok {
		centerX = int((bounds.minX + bounds.maxX) / 2)
		centerZ = int((bounds.minZ + bounds.maxZ) / 2)
	}
	_ = waitChunkLoaded(client, centerX, testY, centerZ, limiter, ctx, regionLoadWaitTimeout(region))
}

func (m *ChunkRegionManager) cleanRegionDrops(client *clientType.Client, region *RegionData, limiter *rate.Limiter, ctx context.Context) {
	if m == nil || region == nil || !m.ClearDrops {
		return
	}
	bounds, ok := m.regionSeenChunkBounds(region)
	if !ok {
		return
	}
	cleanDropsInBounds(client, int(bounds.minX), -64, int(bounds.minZ), int(bounds.maxX), 320, int(bounds.maxZ), limiter, ctx)
}

func regionLoadWaitTimeout(region *RegionData) time.Duration {
	timeout := 2500 * time.Millisecond
	if region != nil && region.BufferedRefs >= 16000 && timeout < 5*time.Second {
		timeout = 5 * time.Second
	}
	return timeout
}

func (m *ChunkRegionManager) finishRegion(regionKey [2]int, region *RegionData, regionChunkCount int, bar *progressbar.ProgressBar) error {
	completedChunks := regionCompletedChunkKeys(region)
	m.advanceChunkProgress(regionChunkCount, bar)
	if m.PostVerify != nil {
		if err := m.PostVerify(completedChunks); err != nil {
			return err
		}
	}
	m.clearTraversedChunkCount(regionKey)
	region.Processed = true
	m.releaseRegion(region)
	m.ProcessedRegions++
	return nil
}

func regionCompletedChunkKeys(region *RegionData) [][2]int {
	if region == nil || len(region.SeenChunks) == 0 {
		return nil
	}
	chunks := make([][2]int, 0, len(region.SeenChunks))
	for key := range region.SeenChunks {
		chunks = append(chunks, key)
	}
	sortChunkKeys(chunks)
	return chunks
}

func (m *ChunkRegionManager) AdvanceRemainingTraversedChunks(bar *progressbar.ProgressBar) {
	progress := getTraversedChunkMap(m)
	if len(progress) == 0 {
		return
	}
	for regionKey, traversed := range progress {
		m.advanceChunkProgress(traversed, bar)
		delete(progress, regionKey)
	}
}

func (m *ChunkRegionManager) regionSeenChunkBounds(region *RegionData) (mcworldBounds, bool) {
	if region == nil || len(region.SeenChunks) == 0 {
		return mcworldBounds{}, false
	}

	minChunkX := math.MaxInt32
	maxChunkX := math.MinInt32
	minChunkZ := math.MaxInt32
	maxChunkZ := math.MinInt32
	for chunkPos := range region.SeenChunks {
		if chunkPos[0] < minChunkX {
			minChunkX = chunkPos[0]
		}
		if chunkPos[0] > maxChunkX {
			maxChunkX = chunkPos[0]
		}
		if chunkPos[1] < minChunkZ {
			minChunkZ = chunkPos[1]
		}
		if chunkPos[1] > maxChunkZ {
			maxChunkZ = chunkPos[1]
		}
	}
	if minChunkX > maxChunkX || minChunkZ > maxChunkZ {
		return mcworldBounds{}, false
	}

	return mcworldBounds{
		minX: int32(minChunkX * 16),
		minY: int32(region.MinY),
		minZ: int32(minChunkZ * 16),
		maxX: int32(maxChunkX*16 + 15),
		maxY: int32(region.MaxY),
		maxZ: int32(maxChunkZ*16 + 15),
	}, true
}

func (m *ChunkRegionManager) placeProtectionForRegion(client *clientType.Client, region *RegionData, limiter *rate.Limiter, ctx context.Context) error {
	if region == nil {
		return nil
	}
	if !m.ProtectionTask.AutoPlaceDenyBlock && !m.ProtectionTask.AutoPlaceBorder {
		return nil
	}
	if !m.BuildBounds.valid() {
		return nil
	}

	areaBounds, ok := m.regionSeenChunkBounds(region)
	if !ok {
		return nil
	}
	if err := m.placeProtectionForBounds(client, limiter, ctx, areaBounds); err != nil {
		return err
	}
	return nil
}

func (m *ChunkRegionManager) placeProtectionForBounds(client *clientType.Client, limiter *rate.Limiter, ctx context.Context, areaBounds mcworldBounds) error {
	if err := placeBorderRingNearArea(client, limiter, ctx, m.ProtectionTask, m.BuildBounds, m.ProtectionLayerY, areaBounds.minX, areaBounds.maxX, areaBounds.minZ, areaBounds.maxZ); err != nil {
		return err
	}
	if err := placeDenyLayerForArea(client, limiter, ctx, m.ProtectionTask, m.BuildBounds, m.ProtectionLayerY, areaBounds.minX, areaBounds.maxX, areaBounds.minZ, areaBounds.maxZ); err != nil {
		return err
	}
	return nil
}

func (m *ChunkRegionManager) sortBlocksSnake(blocks []*types.Module) {
	if len(blocks) <= 1 {
		return
	}
	xGroups := make(map[int][]*types.Module)
	minX, maxX := blocks[0].Point.X, blocks[0].Point.X
	for _, block := range blocks {
		if block == nil {
			continue
		}
		x := block.Point.X
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		xGroups[x] = append(xGroups[x], block)
	}
	for x := range xGroups {
		sort.Slice(xGroups[x], func(i, j int) bool { return xGroups[x][i].Point.Z < xGroups[x][j].Point.Z })
	}
	sorted := make([]*types.Module, 0, len(blocks))
	for x := minX; x <= maxX; x++ {
		group, ok := xGroups[x]
		if !ok {
			continue
		}
		if (x-minX)%2 == 0 {
			sorted = append(sorted, group...)
			continue
		}
		for i := len(group) - 1; i >= 0; i-- {
			sorted = append(sorted, group[i])
		}
	}
	copy(blocks, sorted)
}

// processCommandBlocks writes command-block runtime data after the backing blocks are already in place.
func (m *ChunkRegionManager) processCommandBlocks(client *clientType.Client, blocks []*types.Module, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) {
	m.processCommandBlocksStable(client, blocks, limiter, ctx, bar)
}

// processCommandBlocksStable writes command-block data one block at a time after
// teleporting the main bot to the exact block position. This is slower than the
// batched path but avoids command-block updates being dropped when other main-bot
// attachment operations are happening nearby.
func (m *ChunkRegionManager) processCommandBlocksStable(client *clientType.Client, blocks []*types.Module, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) {
	if len(blocks) == 0 {
		return
	}
	validCount := 0
	for _, block := range blocks {
		if block != nil && block.CommandBlockData != nil {
			validCount++
		}
	}
	if m.GameProgress != nil && validCount > 0 {
		if m.RepairMode {
			m.GameProgress.SetPhase("修复命令方块数据")
		} else {
			m.GameProgress.SetPhase("导入命令方块数据")
		}
		m.GameProgress.SetBuilderStatus("正在写入命令方块数据")
		m.GameProgress.AddCommandTotal(validCount)
		m.GameProgress.SendToClient(client)
	}
	for _, c := range blocks {
		if c == nil || c.CommandBlockData == nil {
			continue
		}
		if newCmd, _, err := NBTAssigner.UpgradeExecuteCommand(c.CommandBlockData.Command); err == nil {
			c.CommandBlockData.Command = newCmd
		}
	}
	if validCount == 0 {
		return
	}
	if err := ensureMainBotCreativeForCommandBlocks(client); err != nil {
		m.markCommandBlocksFailed(client, validCount)
		log.Log.Error("command block creative mode confirmation failed", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
		return
	}
	teleportNearModules(client, blocks, limiter, ctx)
	if importContextDone(ctx) {
		return
	}
	var lastKeepAlive time.Time
	for _, c := range blocks {
		if importContextDone(ctx) {
			return
		}
		if c == nil || c.CommandBlockData == nil {
			continue
		}
		sendImportKeepAlive(client, &lastKeepAlive)
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				if importContextDone(ctx) {
					return
				}
				log.Log.Error("命令方块写入限速等待失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
				return
			}
		}
		if err := sendSettingsCommandDim(client, fmt.Sprintf("tp @s %d %d %d", c.Point.X, c.Point.Y, c.Point.Z)); err != nil {
			if m.GameProgress != nil {
				m.GameProgress.MarkCommandFailed()
				m.GameProgress.SendToClient(client)
			}
			if cancelImportOnFatalError(ctx, client, err) {
				log.Log.Error("命令方块写入前传送失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err, "x": c.Point.X, "y": c.Point.Y, "z": c.Point.Z}))
				return
			}
			log.Log.Error("命令方块写入前传送失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err, "x": c.Point.X, "y": c.Point.Y, "z": c.Point.Z}))
			continue
		}
		if err := writeCommandBlockUpdate(client, c); err != nil {
			if m.GameProgress != nil {
				m.GameProgress.MarkCommandFailed()
				m.GameProgress.SendToClient(client)
			}
			if cancelImportOnFatalError(ctx, client, err) {
				log.Log.Error("命令方块数据写入失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err, "x": c.Point.X, "y": c.Point.Y, "z": c.Point.Z}))
				return
			}
			log.Log.Error("命令方块数据写入失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err, "x": c.Point.X, "y": c.Point.Y, "z": c.Point.Z}))
			continue
		}
		if m.GameProgress != nil {
			m.GameProgress.MarkCommandDone()
			m.GameProgress.SendToClient(client)
		}
		if bar != nil {
			bar.Add(1)
		}
	}
}

// processNBTBlocks applies block-entity data one block at a time without relying on prebuilt prepared tasks.
func (m *ChunkRegionManager) processNBTBlocks(client *clientType.Client, blocks []*types.Module, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) {
	if len(blocks) == 0 {
		return
	}
	if m.GameProgress != nil {
		if m.RepairMode {
			m.GameProgress.SetPhase("修复 NBT 方块数据")
		} else {
			m.GameProgress.SetPhase("导入 NBT 方块数据")
		}
		m.GameProgress.SetBuilderStatus("正在写入 NBT 方块数据")
		m.GameProgress.AddNBTTotal(len(blocks))
		m.GameProgress.SendToClient(client)
	}
	teleportNearModules(client, blocks, limiter, ctx)
	if importContextDone(ctx) {
		return
	}
	for _, c := range blocks {
		if importContextDone(ctx) {
			return
		}
		if c == nil {
			continue
		}
		if err := clientConnError(client); err != nil {
			if cancelImportOnFatalError(ctx, client, err) {
				return
			}
		}
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				if importContextDone(ctx) || cancelImportOnFatalError(ctx, client, err) {
					return
				}
				log.Log.Error("NBT 写入限速等待失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
				return
			}
		}
		err := NBTAssigner.PlaceBlockWithNBTData(
			client.GameInterface,
			c,
			&NBTAssigner.BlockAdditionalData{
				Position:    [3]int32{int32(c.Point.X), int32(c.Point.Y), int32(c.Point.Z)},
				DimensionID: uint8(client.DimensionID),
				FastMode:    false,
				Others:      nil,
			},
		)
		if err != nil {
			if m.GameProgress != nil {
				m.GameProgress.MarkNBTFailed()
			}
			if cancelImportOnFatalError(ctx, client, err) {
				return
			}
			pterm.Warning.Printf("CreateTask: %v\n", err)
		} else if m.GameProgress != nil {
			m.GameProgress.MarkNBTDone()
		}
		if m.GameProgress != nil {
			m.GameProgress.SendToClient(client)
		}
		if bar != nil {
			bar.Add(1)
		}
	}
}

// processNBTBlocksContinuous consumes the prepared NBT queue when available and falls back to direct placement otherwise.
func (m *ChunkRegionManager) processNBTBlocksContinuous(client *clientType.Client, blocks []*types.Module, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) {
	if len(blocks) == 0 {
		return
	}
	if m.GameProgress != nil {
		if m.RepairMode {
			m.GameProgress.SetPhase("修复 NBT 方块数据")
		} else {
			m.GameProgress.SetPhase("导入 NBT 方块数据")
		}
		m.GameProgress.SetBuilderStatus("正在写入 NBT 方块数据")
		m.GameProgress.AddNBTTotal(len(blocks))
		m.GameProgress.SendToClient(client)
	}
	teleportNearModules(client, blocks, limiter, ctx)
	if importContextDone(ctx) {
		return
	}
	for _, block := range blocks {
		if importContextDone(ctx) {
			return
		}
		if block == nil {
			continue
		}
		if err := clientConnError(client); err != nil {
			if cancelImportOnFatalError(ctx, client, err) {
				return
			}
		}
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				if importContextDone(ctx) || cancelImportOnFatalError(ctx, client, err) {
					return
				}
				log.Log.Error("NBT 写入限速等待失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
				return
			}
		}
		success := true
		if isCommandBlockPrefetchExcluded(block) {
			err := NBTAssigner.PlaceBlockWithNBTData(
				client.GameInterface,
				block,
				&NBTAssigner.BlockAdditionalData{
					Position:    [3]int32{int32(block.Point.X), int32(block.Point.Y), int32(block.Point.Z)},
					DimensionID: uint8(client.DimensionID),
					FastMode:    false,
					Others:      nil,
				},
			)
			if err != nil {
				success = false
				if cancelImportOnFatalError(ctx, client, err) {
					return
				}
				pterm.Warning.Printf("CreateTask: %v\n", err)
			}
		} else {
			task, err := m.waitPreparedNBT(block, uint8(client.DimensionID))
			if err != nil {
				if importContextDone(ctx) {
					return
				}
				success = false
				pterm.Warning.Printf("CreateTask: %v\n", err)
			} else if err := NBTAssigner.ApplyPreparedBlockWithNBTData(client.GameInterface, task.prepared, task.additionalData); err != nil {
				success = false
				if cancelImportOnFatalError(ctx, client, err) {
					return
				}
				pterm.Warning.Printf("CreateTask: %v\n", err)
			}
			m.deletePreparedNBTTask(block)
		}
		if m.GameProgress != nil {
			if success {
				m.GameProgress.MarkNBTDone()
			} else {
				m.GameProgress.MarkNBTFailed()
			}
			m.GameProgress.SendToClient(client)
		}
		if bar != nil {
			bar.Add(1)
		}
	}
}
func (m *ChunkRegionManager) processChests(client *clientType.Client, blocks []*types.Module, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) {
	for _, c := range blocks {
		if importContextDone(ctx) {
			return
		}
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				if importContextDone(ctx) || cancelImportOnFatalError(ctx, client, err) {
					return
				}
				log.Log.Error("箱子物品写入限速等待失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
				return
			}
		}
		if err := sendAICommandDim(client, commands_generator.ReplaceItemInContainerRequest(c, ""), true); err != nil {
			if cancelImportOnFatalError(ctx, client, err) {
				log.Log.Error("箱子物品写入失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
				return
			}
			pterm.Warning.Printf("箱子物品写入失败: %v\n", err)
		}
		if m.GameProgress != nil {
			m.GameProgress.AddBlockProgress(1)
		}
		if bar != nil {
			bar.Add(1)
		}
	}
}

func (m *ChunkRegionManager) processSpecialBlocks(client *clientType.Client, blocks []*types.Module, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) {
	for _, c := range blocks {
		if importContextDone(ctx) {
			return
		}
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				if importContextDone(ctx) || cancelImportOnFatalError(ctx, client, err) {
					return
				}
				log.Log.Error("特殊方块物品写入限速等待失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
				return
			}
		}
		if c == nil || c.Block == nil || c.Block.Name == nil {
			continue
		}
		err := sendAICommandDim(
			client,
			fmt.Sprintf("replaceitem block %d %d %d %s %d", c.Point.X, c.Point.Y, c.Point.Z, *c.Block.Name, c.Block.Data),
			true,
		)
		if err != nil {
			if cancelImportOnFatalError(ctx, client, err) {
				log.Log.Error("特殊方块物品写入失败", pterm.DefaultLogger.ArgsFromMap(map[string]interface{}{"error": err}))
				return
			}
			pterm.Warning.Printf("特殊方块物品写入失败: %v\n", err)
		}
		if m.GameProgress != nil {
			m.GameProgress.AddBlockProgress(1)
		}
		if bar != nil {
			bar.Add(1)
		}
	}
}
func (m *ChunkRegionManager) FlushAllRegions(client *clientType.Client, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar, returnX, returnY, returnZ int) bool {
	deadline := time.Now().Add(90 * time.Second)
	for {
		if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
			return false
		}
		if m.PostVerifyErr != nil {
			return false
		}
		if m.NBTPrefetchOn {
			m.queueCompletedRegionNBTReadTasks()
		}
		progressMade := false
		pendingRegions := 0
		sortedRegions := m.GetSortedRegions()
		for _, regionKey := range sortedRegions {
			region, ok := m.Regions[regionKey]
			if !ok || region == nil || region.Processed {
				continue
			}
			pendingRegions++
			if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
				return false
			}
			if m.PostVerifyErr != nil {
				return false
			}
			if m.ProcessRegion(client, regionKey, limiter, ctx, bar) {
				progressMade = true
			}
		}
		if pendingRegions == 0 {
			break
		}
		if !progressMade {
			if importContextDone(ctx) {
				return false
			}
			if m.PostVerifyErr != nil {
				return false
			}
			if time.Now().After(deadline) {
				log.Log.Warn("分区刷新等待超时")
				return false
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
	if len(m.activeTickingAreas) > 0 {
		m.removeRegionTickingArea(client, m.activeTickingAreas)
		m.activeTickingAreas = nil
	}
	m.Regions = make(map[[2]int]*RegionData)
	m.RegionOrder = [][2]int{}
	m.BufferedRefs = 0
	m.forceMemoryCleanup()
	if client != nil && client.GameInterface != nil {
		_ = sendSettingsCommandDim(client, fmt.Sprintf("tp @s %d %d %d", returnX, 320, returnZ))
	}
	return true
}

// resolveFlushThreshold returns the threshold to start flushing regions.
func resolveFlushThreshold(client *clientType.Client, defaultThreshold int) int {
	if client.Cdump_Setting != nil && client.Cdump_Setting.StreamFlushThreshold > 0 {
		return client.Cdump_Setting.StreamFlushThreshold
	}
	return defaultThreshold
}

// flushRegionsIfNeeded flushes a region when buffered regions exceed threshold.
// If no flushable region is found and allowOldestFallback is true, it flushes the oldest.
func flushRegionsIfNeeded(regionManager *ChunkRegionManager, threshold int, client *clientType.Client, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar, allowOldestFallback bool) {
	if importContextDone(ctx) || abortRegionOnDeadConnection(ctx, client) {
		return
	}
	if regionManager.RegionSize > 10 {
		threshold = minInt(threshold, 30)
	} else if regionManager.RegionSize <= 4 {
	}

	blockThreshold := resolveBufferedRefThreshold(regionManager.RegionSize)
	if len(regionManager.Regions) <= threshold && regionManager.BufferedRefs <= blockThreshold {
		return
	}

	timeout := regionManager.FlushTimeout
	if timeout <= 0 {
		timeout = time.Second * 2
	}
	if regionManager.FlushTimeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	if idx, oldest, ok := regionManager.FindFlushableRegion(timeout); ok {
		_ = idx
		if regionManager.ProcessRegion(client, oldest, limiter, ctx, bar) {
			regionManager.discardRegion(oldest, idx)
		}
		return
	}

	if allowOldestFallback && len(regionManager.RegionOrder) > 0 {
		oldest := regionManager.RegionOrder[0]
		if regionManager.ProcessRegion(client, oldest, limiter, ctx, bar) {
			regionManager.discardRegion(oldest, 0)
		}
	}
}

func releaseModuleSlice(blocks []*types.Module) {
	for i := range blocks {
		blocks[i] = nil
	}
}

func (m *ChunkRegionManager) discardRegion(regionKey [2]int, idx int) {
	if idx >= 0 && idx < len(m.RegionOrder) {
		m.RegionOrder = append(m.RegionOrder[:idx], m.RegionOrder[idx+1:]...)
	} else {
		for i, key := range m.RegionOrder {
			if key != regionKey {
				continue
			}
			m.RegionOrder = append(m.RegionOrder[:i], m.RegionOrder[i+1:]...)
			break
		}
	}
	delete(m.Regions, regionKey)
	m.maybeMemoryCleanup()
}

func (m *ChunkRegionManager) releaseRegion(region *RegionData) {
	if region == nil {
		return
	}
	if region.Blocks != nil {
		for y, blocks := range region.Blocks {
			releaseModuleSlice(blocks)
			delete(region.Blocks, y)
		}
		clear(region.Blocks)
	}
	releaseModuleSlice(region.CommandBlocks)
	releaseModuleSlice(region.NBTBlocks)
	releaseModuleSlice(region.Chests)
	releaseModuleSlice(region.SpecialBlocks)
	if region.SeenChunks != nil {
		clear(region.SeenChunks)
	}

	m.BufferedRefs -= region.BufferedRefs
	if m.BufferedRefs < 0 {
		m.BufferedRefs = 0
	}
	region.BufferedRefs = 0
	region.Blocks = nil
	region.CommandBlocks = nil
	region.NBTBlocks = nil
	region.Chests = nil
	region.SpecialBlocks = nil
	region.SeenChunks = nil
}

func resolveBufferedRefThreshold(regionSize int) int {
	switch {
	case regionSize >= 10:
		return 8_000
	case regionSize >= 8:
		return 12_000
	case regionSize >= 6:
		return 20_000
	case regionSize >= 4:
		return 28_000
	default:
		return 40_000
	}
}

func (m *ChunkRegionManager) forceMemoryCleanup() {
	runtime.GC()
	debug.FreeOSMemory()
}

func (m *ChunkRegionManager) maybeMemoryCleanup() {
	if m.ProcessedRegions == 0 {
		return
	}

	blockThreshold := resolveBufferedRefThreshold(m.RegionSize)

	idle := m.BufferedRefs == 0
	highPressure := m.BufferedRefs >= blockThreshold*2
	if !highPressure && m.ProcessedRegions%6 != 0 && (!idle || m.ProcessedRegions%3 != 0) {
		return
	}

	runtime.GC()

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	unreleased := mem.HeapIdle - mem.HeapReleased
	heapAlloc := mem.HeapAlloc

	if highPressure && (heapAlloc >= 384<<20 || unreleased >= 192<<20) {
		debug.FreeOSMemory()
		return
	}
	if idle && m.ProcessedRegions%16 == 0 && unreleased >= 256<<20 {
		debug.FreeOSMemory()
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// startProgressNotifier runs a ticker to push progress updates to web client and actionbar.
func startProgressNotifier(bar *progressbar.ProgressBar, client *clientType.Client, web_client *webclient.Webclient, task_id, userID, execute string, progress *ImportGameProgress) func() {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(inGameProgressPushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				state := bar.State()
				if bar.GetMax64() != int64(state.CurrentNum) {
					if web_client != nil {
						web_client.Update_task_operation(task_id, userID, int(state.CurrentNum), int(state.Max), state.SecondsSince, state.SecondsLeft)
					}
				} else {
					return
				}
				if progress != nil {
					progress.SetChunkProgress(int(state.CurrentNum), int(state.Max))
					progress.SendToClient(client)
				}
			}
		}
	}()

	return func() {
		close(stop)
	}
}

func clearSeenChunkBounds(client *clientType.Client, bounds mcworldBounds, limiter *rate.Limiter, ctx context.Context) {
	if !bounds.valid() {
		return
	}
	minX := int(bounds.minX)
	minY := int(bounds.minY)
	minZ := int(bounds.minZ)
	maxX := int(bounds.maxX)
	maxY := int(bounds.maxY)
	maxZ := int(bounds.maxZ)
	centerX := (minX + maxX) / 2
	centerZ := (minZ + maxZ) / 2
	teleportSafe(client, centerX, maxY+2, centerZ, limiter, ctx)
	runClearCommandsLayered(client, minX, minY, minZ, maxX, maxY, maxZ, limiter, ctx, nil)
}

// importChunkPlan captures the chunk traversal scope and resume policy for one import.
type importChunkPlan struct {
	minChunkX       int
	maxChunkX       int
	minChunkZ       int
	maxChunkZ       int
	totalChunks     int
	regionCountX    int
	regionCountZ    int
	totalRegions    int
	resumePercent   int
	resumeProcessed int
	resumeTotal     int
	skipRegions     int
	skipChunks      int
	skipChunk       func(chunkX, chunkZ int32) bool
}

// resolveImportTargetDimension picks the target dimension from task input or falls back to the client's current one.
func resolveImportTargetDimension(client *clientType.Client, raw string) (dimension.Info, error) {
	dimInput := strings.TrimSpace(raw)
	if dimInput == "" {
		info := dimension.Info{
			Name: strings.TrimSpace(client.CommandDimension),
			ID:   client.DimensionID,
		}
		if info.Name == "" {
			info.Name = dimension.NameFromID(info.ID)
		}
		if info.Name == "" && info.ID != 0 {
			return dimension.Info{}, fmt.Errorf("invalid dimension, expected name:id")
		}
		if info.Name == "" {
			info.Name = "overworld"
		}
		return info, nil
	}
	return dimension.Parse(dimInput)
}

// applyImportDimensionContext switches dimension-dependent runtime state and returns a restore function.
func applyImportDimensionContext(client *clientType.Client, dimInfo dimension.Info) func() {
	prevDimName := client.CommandDimension
	prevDimID := client.DimensionID
	client.CommandDimension = dimInfo.Name
	client.DimensionID = dimInfo.ID

	prevNBTDim := NBTAssigner.DefaultDimensionID
	NBTAssigner.DefaultDimensionID = uint8(dimInfo.ID)

	prevSkipSubChunkCheck := client.SkipSubChunkCheck
	client.SkipSubChunkCheck = true

	var restoreChunkCache func()
	if client.ChunkAssembler != nil {
		client.ChunkAssembler.NoCache()
		restoreChunkCache = func() {
			client.ChunkAssembler.AllowCache()
		}
	}

	return func() {
		if restoreChunkCache != nil {
			restoreChunkCache()
		}
		client.SkipSubChunkCheck = prevSkipSubChunkCheck
		NBTAssigner.DefaultDimensionID = prevNBTDim
		client.CommandDimension = prevDimName
		client.DimensionID = prevDimID
	}
}

// openMCWorldForImport unpacks the mcworld, opens it, resolves bounds and returns a cleanup function.
func openMCWorldForImport(filePath string, task types.Task) (_ *world.BedrockWorld, bounds mcworldBounds, cleanup func(), err error) {
	tempDir, err := os.MkdirTemp("", "mcworld_import_*")
	if err != nil {
		return nil, mcworldBounds{}, nil, err
	}

	removeTempDir := func() {
		_ = os.RemoveAll(tempDir)
	}

	if err := unzipMCWorld(filePath, tempDir); err != nil {
		removeTempDir()
		return nil, mcworldBounds{}, nil, fmt.Errorf("unpack mcworld failed: %w", err)
	}

	bw, err := world.Open(tempDir, nil)
	if err != nil {
		removeTempDir()
		return nil, mcworldBounds{}, nil, err
	}

	bounds, err = parseMCWorldBounds(filePath, bw.LevelDat().LevelName)
	if err != nil {
		bw.CloseWorld()
		bw.Close()
		removeTempDir()
		return nil, mcworldBounds{}, nil, err
	}

	if task.CropEnabled {
		bounds = cropMCWorldBounds(bounds, task.CropMin, task.CropMax)
	}

	cleanup = func() {
		bw.CloseWorld()
		bw.Close()
		removeTempDir()
	}
	return bw, bounds, cleanup, nil
}

// resolveImportRegionSize chooses the region size used during traversal and flush scheduling.
func resolveImportRegionSize(client *clientType.Client, task types.Task) int {
	regionSize := 1
	if task.UseFill {
		regionSize = 4
	}
	if task.RegionSize > 0 {
		regionSize = task.RegionSize
	} else if client.Cdump_Setting != nil && client.Cdump_Setting.RegionSize > 0 {
		regionSize = client.Cdump_Setting.RegionSize
	}
	if regionSize < 1 {
		regionSize = 1
	}
	return regionSize
}

func clampResumePercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

// buildImportChunkPlan calculates chunk bounds, region counts and optional resume skipping.
func buildImportChunkPlan(bounds mcworldBounds, regionSize, resumePercent, resumeProcessed, resumeTotal int) (importChunkPlan, error) {
	plan := importChunkPlan{
		minChunkX:       floorDiv(int(bounds.minX), 16),
		maxChunkX:       floorDiv(int(bounds.maxX), 16),
		minChunkZ:       floorDiv(int(bounds.minZ), 16),
		maxChunkZ:       floorDiv(int(bounds.maxZ), 16),
		resumePercent:   clampResumePercent(resumePercent),
		resumeProcessed: resumeProcessed,
		resumeTotal:     resumeTotal,
	}

	plan.totalChunks = (plan.maxChunkX - plan.minChunkX + 1) * (plan.maxChunkZ - plan.minChunkZ + 1)
	if plan.totalChunks <= 0 {
		return importChunkPlan{}, fmt.Errorf("mcworld 区块范围为空")
	}

	plan.regionCountX = (plan.maxChunkX - plan.minChunkX + 1 + regionSize - 1) / regionSize
	plan.regionCountZ = (plan.maxChunkZ - plan.minChunkZ + 1 + regionSize - 1) / regionSize
	plan.totalRegions = plan.regionCountX * plan.regionCountZ
	if plan.totalRegions < 0 {
		plan.totalRegions = 0
	}

	regionIndexForChunk := func(chunkX, chunkZ int) int {
		regionX := (chunkX - plan.minChunkX) / regionSize
		regionZ := (chunkZ - plan.minChunkZ) / regionSize
		if regionX < 0 || regionX >= plan.regionCountX || regionZ < 0 || regionZ >= plan.regionCountZ {
			return -1
		}
		if regionX%2 == 0 {
			return regionX*plan.regionCountZ + regionZ
		}
		return regionX*plan.regionCountZ + (plan.regionCountZ - 1 - regionZ)
	}

	chunkCountForRegionIndex := func(idx int) int {
		if idx < 0 || idx >= plan.totalRegions {
			return 0
		}
		regionX := idx / plan.regionCountZ
		regionZOffset := idx % plan.regionCountZ
		regionZ := regionZOffset
		if regionX%2 != 0 {
			regionZ = plan.regionCountZ - 1 - regionZOffset
		}
		startX := plan.minChunkX + regionX*regionSize
		endX := minInt(startX+regionSize-1, plan.maxChunkX)
		startZ := plan.minChunkZ + regionZ*regionSize
		endZ := minInt(startZ+regionSize-1, plan.maxChunkZ)
		if endX < startX || endZ < startZ {
			return 0
		}
		return (endX - startX + 1) * (endZ - startZ + 1)
	}

	setSkipRegions := func(skipRegions int) {
		if skipRegions < 0 {
			skipRegions = 0
		}
		if skipRegions > plan.totalRegions {
			skipRegions = plan.totalRegions
		}
		plan.skipRegions = skipRegions
		plan.skipChunk = func(chunkX, chunkZ int32) bool {
			idx := regionIndexForChunk(int(chunkX), int(chunkZ))
			return idx >= 0 && idx < plan.skipRegions
		}
		plan.skipChunks = 0
		for chunkX := plan.minChunkX; chunkX <= plan.maxChunkX; chunkX++ {
			for chunkZ := plan.minChunkZ; chunkZ <= plan.maxChunkZ; chunkZ++ {
				if plan.skipChunk(int32(chunkX), int32(chunkZ)) {
					plan.skipChunks++
				}
			}
		}
	}

	if plan.totalRegions == 0 {
		return plan, nil
	}
	if resumeProcessed > 0 && resumeTotal == plan.totalChunks {
		skipRegions := 0
		skippedChunks := 0
		for idx := 0; idx < plan.totalRegions; idx++ {
			regionChunks := chunkCountForRegionIndex(idx)
			if regionChunks <= 0 || skippedChunks+regionChunks > resumeProcessed {
				break
			}
			skippedChunks += regionChunks
			skipRegions++
		}
		setSkipRegions(skipRegions)
		return plan, nil
	}
	if plan.resumePercent == 0 {
		return plan, nil
	}
	setSkipRegions(plan.totalRegions * plan.resumePercent / 100)

	return plan, nil
}

// newImportLimiters returns the shared block limiter and the optional command-data limiter.
func newImportLimiters(client *clientType.Client, task types.Task) (limiter, commandLimiter *rate.Limiter) {
	limiter = newCommandRateLimiter(resolveImportCommandSpeed(client))
	commandLimiter = limiter
	if task.CommandDataSpeed > 0 {
		commandLimiter = newCommandRateLimiter(task.CommandDataSpeed)
	}
	return limiter, commandLimiter
}

// startImportProgressBar wires the terminal bar, web progress updates and in-game actionbar notifier.
func startImportProgressBar(totalChunks, skipChunks int, client *clientType.Client, webClient *webclient.Webclient, taskID, userID, execute string, progressCb func(processed int, total int), gameProgress *ImportGameProgress) (*progressbar.ProgressBar, *ImportGameProgress, func()) {
	bar := progressbar.Default(int64(totalChunks), "导入区块")
	if gameProgress == nil {
		gameProgress = NewImportGameProgress("导入普通方块")
	}
	gameProgress.SetPhase("导入普通方块")
	gameProgress.SetBuilderStatus("正在生成并发送区块命令")
	gameProgress.ResetImportCounters(0, totalChunks)
	if skipChunks > 0 {
		_ = bar.Add(skipChunks)
		gameProgress.SetChunkProgress(skipChunks, totalChunks)
		if progressCb != nil {
			progressCb(skipChunks, totalChunks)
		}
	}
	if webClient != nil {
		webClient.Update_task_now_operation(taskID, userID, "导入区块")
	}

	stopNotify := startProgressNotifier(bar, client, webClient, taskID, userID, execute, gameProgress)
	gameProgress.SendToClientNow(client)
	return bar, gameProgress, func() {
		bar.Clear()
		stopNotify()
	}
}

func clipChunksForImport(chunks map[bwo_define.ChunkPos]*chunk.Chunk, bounds mcworldBounds, offsetX, offsetZ int, allowTargetChunk func(chunkX, chunkZ int) bool) map[bwo_define.ChunkPos]*chunk.Chunk {
	clipped := make(map[bwo_define.ChunkPos]*chunk.Chunk, len(chunks))

	for pos, src := range chunks {
		if src == nil {
			continue
		}

		chunkMinX := pos[0] * 16
		chunkMaxX := chunkMinX + 15
		chunkMinZ := pos[1] * 16
		chunkMaxZ := chunkMinZ + 15
		chunkMinY := int32(src.Range().Min())
		chunkMaxY := int32(src.Range().Max())

		fullyInside := chunkMinX >= bounds.minX &&
			chunkMaxX <= bounds.maxX &&
			chunkMinY >= bounds.minY &&
			chunkMaxY <= bounds.maxY &&
			chunkMinZ >= bounds.minZ &&
			chunkMaxZ <= bounds.maxZ
		if allowTargetChunk == nil && fullyInside {
			clipped[pos] = src
			continue
		}

		dst := chunk.NewChunk(block.AirRuntimeID, src.Range())
		for localX := 0; localX < 16; localX++ {
			worldX := chunkMinX + int32(localX)
			if worldX < bounds.minX || worldX > bounds.maxX {
				continue
			}
			for worldY := src.Range().Min(); worldY <= src.Range().Max(); worldY++ {
				if int32(worldY) < bounds.minY || int32(worldY) > bounds.maxY {
					continue
				}
				for localZ := 0; localZ < 16; localZ++ {
					worldZ := chunkMinZ + int32(localZ)
					if worldZ < bounds.minZ || worldZ > bounds.maxZ {
						continue
					}
					if allowTargetChunk != nil && !allowTargetChunk(floorDiv(int(worldX)+offsetX, 16), floorDiv(int(worldZ)+offsetZ, 16)) {
						continue
					}
					runtimeID := src.Block(uint8(localX), int16(worldY), uint8(localZ), 0)
					if runtimeID == block.AirRuntimeID {
						continue
					}
					dst.SetBlock(uint8(localX), int16(worldY), uint8(localZ), 0, runtimeID)
				}
			}
		}
		clipped[pos] = dst
	}

	return clipped
}

func needsPostChunkStreamRegionPass(client *clientType.Client, task types.Task) bool {
	return task.ImportNBT ||
		task.ImportCommandBlock ||
		task.AutoPlaceBorder ||
		task.AutoPlaceDenyBlock ||
		(client != nil && client.Cdump_Setting != nil && client.Cdump_Setting.Clear_Building && !task.UseFill)
}

func prepareChunkStreamGroup(
	client *clientType.Client,
	minChunkX int,
	minChunkZ int,
	maxChunkX int,
	maxChunkZ int,
	targetBounds mcworldBounds,
	offsetX int,
	offsetZ int,
	limiter *rate.Limiter,
	ctx context.Context,
) (func(), error) {
	if client == nil || client.GameInterface == nil {
		return func() {}, nil
	}

	x := minChunkX*16 + offsetX
	z := minChunkZ*16 + offsetZ
	y := clampChunkY(int(targetBounds.minY))
	chunkGroupSide := max(maxChunkX-minChunkX+1, maxChunkZ-minChunkZ+1)
	chunkGroupWidth := int32(chunkGroupSide * 16)

	cleanup := func() {}
	if chunkGroupSide != 1 {
		const areaName = "Nexus_1"
		maxX := x + int(chunkGroupWidth) - 1
		maxZ := z + int(chunkGroupWidth) - 1
		if err := addNamedTickingAreaWithRetry(client, x, y, z, maxX, y, maxZ, areaName); err != nil {
			downgradeTickingArea(err, map[string]any{
				"name": areaName,
				"x":    x,
				"y":    y,
				"z":    z,
				"maxX": maxX,
				"maxZ": maxZ,
			})
		} else {
			if err := preloadTickingAreaWithRetry(client, x, y, z); err != nil {
				downgradeTickingArea(err, map[string]any{
					"name": areaName,
					"x":    x,
					"y":    y,
					"z":    z,
				})
			} else {
				cleanup = func() {
					removeNamedTickingArea(client, areaName)
				}
			}
		}
	}

	if err := waitChunkAreaLoaded(client, x, y, z, limiter, ctx); err != nil {
		log.Log.Warn("等待区块加载失败", log.Log.ArgsFromMap(map[string]any{
			"error": err.Error(),
			"x":     x,
			"y":     y,
			"z":     z,
		}))
	}

	return cleanup, nil
}

func waitChunkAreaLoaded(client *clientType.Client, x, y, z int, limiter *rate.Limiter, ctx context.Context) error {
	if waitChunkLoaded(client, x, y, z, limiter, ctx, 0) {
		return nil
	}
	if err := clientConnError(client); err != nil {
		return err
	}
	if ctx != nil && importContextDone(ctx) {
		return ctx.Err()
	}
	return fmt.Errorf("等待区块加载超时")
}

func sampleVerifyBlocksInChunk(chunkData *chunk.Chunk, pos bwo_define.ChunkPos, bounds mcworldBounds, nbtMap map[[3]int32]map[string]any, fn func(wx, wy, wz int32, runtimeID uint32, nbt map[string]any) error) error {
	if chunkData == nil || fn == nil {
		return nil
	}
	chunkStartX := pos[0] * 16
	chunkStartZ := pos[1] * 16
	chunkEndX := chunkStartX + 15
	chunkEndZ := chunkStartZ + 15
	blockMinX := maxInt32(bounds.minX, chunkStartX)
	blockMaxX := minInt32(bounds.maxX, chunkEndX)
	blockMinZ := maxInt32(bounds.minZ, chunkStartZ)
	blockMaxZ := minInt32(bounds.maxZ, chunkEndZ)
	if blockMinX > blockMaxX || blockMinZ > blockMaxZ {
		return nil
	}
	for x := blockMinX; x <= blockMaxX; x++ {
		localX := uint8(x - chunkStartX)
		for z := blockMinZ; z <= blockMaxZ; z++ {
			localZ := uint8(z - chunkStartZ)
			for y := bounds.minY; y <= bounds.maxY; y++ {
				runtimeID := pickRuntimeID(chunkData, localX, int16(y), localZ)
				if runtimeID == block.AirRuntimeID {
					continue
				}
				var nbt map[string]any
				if nbtMap != nil {
					nbt = nbtMap[[3]int32{x, y, z}]
				}
				if err := fn(x, y, z, runtimeID, nbt); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func cleanDropsInBounds(client *clientType.Client, minX, minY, minZ, maxX, maxY, maxZ int, limiter *rate.Limiter, ctx context.Context) {
	if minX > maxX || minY > maxY || minZ > maxZ {
		return
	}
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			if cancelImportOnFatalError(ctx, client, err) {
				return
			}
			log.Log.Warn("清理掉落物限速等待失败", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			return
		}
	}
	cmd := fmt.Sprintf(
		"kill @e[type=Item,x=%d,y=%d,z=%d,dx=%d,dy=%d,dz=%d]",
		minX,
		minY,
		minZ,
		maxX-minX+1,
		maxY-minY+1,
		maxZ-minZ+1,
	)
	if err := sendSettingsCommandDim(client, cmd); err != nil {
		if cancelImportOnFatalError(ctx, client, err) {
			return
		}
		log.Log.Warn("清理掉落物命令发送失败", log.Log.ArgsFromMap(map[string]any{
			"error":   err.Error(),
			"command": cmd,
		}))
	}
}

func recordTargetChunksForSourceChunk(dst map[[2]int]struct{}, chunkX, chunkZ, offsetX, offsetZ int, allowTargetChunk func(chunkX, chunkZ int) bool) {
	if dst == nil {
		return
	}
	targetMinX := floorDiv(chunkX*16+offsetX, 16)
	targetMaxX := floorDiv(chunkX*16+15+offsetX, 16)
	targetMinZ := floorDiv(chunkZ*16+offsetZ, 16)
	targetMaxZ := floorDiv(chunkZ*16+15+offsetZ, 16)
	for cx := targetMinX; cx <= targetMaxX; cx++ {
		for cz := targetMinZ; cz <= targetMaxZ; cz++ {
			if allowTargetChunk != nil && !allowTargetChunk(cx, cz) {
				continue
			}
			dst[[2]int{cx, cz}] = struct{}{}
		}
	}
}

func chunkKeyMapToSortedSlice(chunkMap map[[2]int]struct{}) [][2]int {
	if len(chunkMap) == 0 {
		return nil
	}
	chunks := make([][2]int, 0, len(chunkMap))
	for key := range chunkMap {
		chunks = append(chunks, key)
	}
	sortChunkKeys(chunks)
	return chunks
}

func isPlainBlockFastImport(task types.Task) bool {
	return !task.ImportNBT &&
		!task.ImportCommandBlock &&
		!task.AutoPlaceBorder &&
		!task.AutoPlaceDenyBlock &&
		!task.ClearArea
}

type clearFillCommand struct {
	cmd    string
	volume int
}

const clearFillMaxVolume = 32768 * 70 / 100

func buildClearCommandsLayered(minX, minY, minZ, maxX, maxY, maxZ int) []clearFillCommand {
	if minX > maxX || minY > maxY || minZ > maxZ {
		return nil
	}

	width := maxX - minX + 1
	height := maxY - minY + 1
	length := maxZ - minZ + 1
	if width <= 0 || height <= 0 || length <= 0 {
		return nil
	}

	buildPhase := func(material string) []clearFillCommand {
		commands := make([]clearFillCommand, 0)
		for startX := minX; startX <= maxX; {
			currentWidth := minInt(maxX-startX+1, clearFillMaxVolume)
			if currentWidth < 1 {
				currentWidth = 1
			}
			endX := startX + currentWidth - 1
			maxLength := clearFillMaxVolume / currentWidth
			if maxLength < 1 {
				maxLength = 1
			}

			for startZ := minZ; startZ <= maxZ; {
				currentLength := minInt(maxZ-startZ+1, maxLength)
				if currentLength < 1 {
					currentLength = 1
				}
				endZ := startZ + currentLength - 1
				baseArea := currentWidth * currentLength
				maxHeight := clearFillMaxVolume / baseArea
				if maxHeight < 1 {
					maxHeight = 1
				}

				for startY := minY; startY <= maxY; startY += maxHeight {
					endY := startY + maxHeight - 1
					if endY > maxY {
						endY = maxY
					}
					commands = append(commands, clearFillCommand{
						cmd:    fmt.Sprintf("fill %d %d %d %d %d %d minecraft:%s", startX, startY, startZ, endX, endY, endZ, material),
						volume: currentWidth * (endY - startY + 1) * currentLength,
					})
				}
				startZ = endZ + 1
			}
			startX = endX + 1
		}
		return commands
	}

	commands := buildPhase("stone")
	return append(commands, buildPhase("air")...)
}

func runClearCommandsLayered(client *clientType.Client, minX, minY, minZ, maxX, maxY, maxZ int, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar) {
	for _, command := range buildClearCommandsLayered(minX, minY, minZ, maxX, maxY, maxZ) {
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				if cancelImportOnFatalError(ctx, client, err) {
					return
				}
				log.Log.Warn("清理区域限速等待失败", log.Log.ArgsFromMap(map[string]any{
					"error": err.Error(),
				}))
				return
			}
		}
		if err := sendSettingsCommandDim(client, command.cmd); err != nil {
			if cancelImportOnFatalError(ctx, client, err) {
				return
			}
			log.Log.Warn("清理区域命令发送失败", log.Log.ArgsFromMap(map[string]any{
				"error":   err.Error(),
				"command": command.cmd,
			}))
			continue
		}
		if bar != nil && command.volume > 0 {
			bar.Add(command.volume)
		}
	}
}

func sendProtectedFill(client *clientType.Client, limiter *rate.Limiter, ctx context.Context, cmd string) error {
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return err
		}
	}
	return sendAICommandDim(client, cmd, false)
}

func importedBuildBounds(bounds mcworldBounds, offsetX, offsetY, offsetZ int) mcworldBounds {
	return mcworldBounds{
		minX: bounds.minX + int32(offsetX),
		minY: bounds.minY + int32(offsetY),
		minZ: bounds.minZ + int32(offsetZ),
		maxX: bounds.maxX + int32(offsetX),
		maxY: bounds.maxY + int32(offsetY),
		maxZ: bounds.maxZ + int32(offsetZ),
	}
}

func resolveImportBuildOrigin(x, y, z int, task types.Task) (buildX, buildY, buildZ int, protectionLayerY int32) {
	buildX, buildY, buildZ = x, y, z
	if task.AutoPlaceBorder {
		buildX++
		buildZ++
	}
	if task.AutoPlaceDenyBlock {
		buildY++
	}
	return buildX, buildY, buildZ, int32(y)
}

func placeDenyLayerForArea(client *clientType.Client, limiter *rate.Limiter, ctx context.Context, task types.Task, buildBounds mcworldBounds, protectionLayerY, areaMinX, areaMaxX, areaMinZ, areaMaxZ int32) error {
	if !task.AutoPlaceDenyBlock {
		return nil
	}

	startX, endX := areaMinX, areaMaxX
	startZ, endZ := areaMinZ, areaMaxZ
	if endX < buildBounds.minX || startX > buildBounds.maxX || endZ < buildBounds.minZ || startZ > buildBounds.maxZ {
		return nil
	}
	if startX < buildBounds.minX {
		startX = buildBounds.minX
	}
	if endX > buildBounds.maxX {
		endX = buildBounds.maxX
	}
	if startZ < buildBounds.minZ {
		startZ = buildBounds.minZ
	}
	if endZ > buildBounds.maxZ {
		endZ = buildBounds.maxZ
	}
	if startX > endX || startZ > endZ {
		return nil
	}

	return sendProtectedFill(client, limiter, ctx, fmt.Sprintf(
		"fill %d %d %d %d %d %d minecraft:deny",
		startX, protectionLayerY, startZ, endX, protectionLayerY, endZ,
	))
}

func placeBorderRingNearArea(client *clientType.Client, limiter *rate.Limiter, ctx context.Context, task types.Task, buildBounds mcworldBounds, protectionLayerY, areaMinX, areaMaxX, areaMinZ, areaMaxZ int32) error {
	if !task.AutoPlaceBorder {
		return nil
	}

	outerMinX := buildBounds.minX - 1
	outerMaxX := buildBounds.maxX + 1
	outerMinZ := buildBounds.minZ - 1
	outerMaxZ := buildBounds.maxZ + 1

	clipMinX, clipMaxX := areaMinX, areaMaxX
	clipMinZ, clipMaxZ := areaMinZ, areaMaxZ
	if outerMinX == areaMinX-1 {
		clipMinX = outerMinX
	}
	if outerMaxX == areaMaxX+1 {
		clipMaxX = outerMaxX
	}
	if outerMinZ == areaMinZ-1 {
		clipMinZ = outerMinZ
	}
	if outerMaxZ == areaMaxZ+1 {
		clipMaxZ = outerMaxZ
	}

	borderY := protectionLayerY
	if outerMinZ >= clipMinZ && outerMinZ <= clipMaxZ {
		x1 := maxInt32(outerMinX, clipMinX)
		x2 := minInt32(outerMaxX, clipMaxX)
		if x1 <= x2 {
			if err := sendProtectedFill(client, limiter, ctx, fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", x1, borderY, outerMinZ, x2, borderY, outerMinZ)); err != nil {
				return err
			}
		}
	}
	if outerMaxZ >= clipMinZ && outerMaxZ <= clipMaxZ {
		x1 := maxInt32(outerMinX, clipMinX)
		x2 := minInt32(outerMaxX, clipMaxX)
		if x1 <= x2 {
			if err := sendProtectedFill(client, limiter, ctx, fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", x1, borderY, outerMaxZ, x2, borderY, outerMaxZ)); err != nil {
				return err
			}
		}
	}
	if outerMinX >= clipMinX && outerMinX <= clipMaxX {
		z1 := maxInt32(outerMinZ, clipMinZ)
		z2 := minInt32(outerMaxZ, clipMaxZ)
		if z1 <= z2 {
			if err := sendProtectedFill(client, limiter, ctx, fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", outerMinX, borderY, z1, outerMinX, borderY, z2)); err != nil {
				return err
			}
		}
	}
	if outerMaxX >= clipMinX && outerMaxX <= clipMaxX {
		z1 := maxInt32(outerMinZ, clipMinZ)
		z2 := minInt32(outerMaxZ, clipMaxZ)
		if z1 <= z2 {
			if err := sendProtectedFill(client, limiter, ctx, fmt.Sprintf("fill %d %d %d %d %d %d minecraft:border_block", outerMaxX, borderY, z1, outerMaxX, borderY, z2)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Cdump_import imports a mcworld file into the current server.
// Flow:
// 1. Validate input and switch the runtime dimension context.
// 2. Open the mcworld source and build the chunk traversal plan.
// 3. Traverse source chunks, buffer regions, and flush them into the target world.
// 4. Finalize remaining regions and report progress.
func Cdump_import(client *clientType.Client, file_path string, x int, y int, z int, No_Tip bool, jump_id int, web_client *webclient.Webclient, task_id string, task types.Task, progressCb func(processed int, total int), gameProgress *ImportGameProgress) bool {
	_ = No_Tip
	setLastImportError(client, nil)

	if !file.Is_File(file_path) {
		pterm.Println(pterm.Red("导入文件不存在"))
		return false
	}
	if strings.ToLower(path.Ext(file_path)) != ".mcworld" {
		pterm.Println(pterm.Red("导入文件必须是 .mcworld"))
		return false
	}

	dimInfo, err := resolveImportTargetDimension(client, task.Dimension)
	if err != nil {
		pterm.Println(pterm.Red(fmt.Sprintf("导入维度参数错误: %v", err)))
		return false
	}
	// The import temporarily overrides dimension-dependent runtime state on the shared client.
	restoreImportContext := applyImportDimensionContext(client, dimInfo)
	defer restoreImportContext()

	_ = sendSettingsCommandDim(client, "gamemode 1")

	// The source mcworld is always read from the local temporary extraction directory.
	_, bounds, cleanupMCWorld, err := openMCWorldForImport(file_path, task)
	if err != nil {
		pterm.Println(pterm.Red(fmt.Sprintf("打开 mcworld 失败: %v", err)))
		return false
	}
	defer cleanupMCWorld()

	sourceDimID := int32(0)
	sourceDimName := "overworld"
	if sourceDimID != dimInfo.ID {
		pterm.Warning.Printf("mcworld dimension %s (id=%d) differs from target %s (id=%d)\n", sourceDimName, sourceDimID, dimInfo.Name, dimInfo.ID)
	}

	regionSize := resolveImportRegionSize(client, task)
	chunkPlan, err := buildImportChunkPlan(bounds, regionSize, jump_id, task.ResumeProcessed, task.ResumeTotal)
	if err != nil {
		pterm.Println(pterm.Red(err.Error()))
		return false
	}
	if chunkPlan.skipRegions > 0 {
		task.ClearArea = false
		if task.ResumeProcessed > 0 && task.ResumeTotal == chunkPlan.totalChunks {
			pterm.Warning.Printf("resume import: skip %d/%d regions (%d/%d chunks, region size %d)\n", chunkPlan.skipRegions, chunkPlan.totalRegions, chunkPlan.skipChunks, chunkPlan.totalChunks, regionSize)
		} else {
			pterm.Warning.Printf("resume import: skip %d/%d regions (region size %d, %d%%)\n", chunkPlan.skipRegions, chunkPlan.totalRegions, regionSize, chunkPlan.resumePercent)
		}
		if client.Cdump_Setting != nil && client.Cdump_Setting.Clear_Building {
			originalClear := client.Cdump_Setting.Clear_Building
			client.Cdump_Setting.Clear_Building = false
			defer func() {
				client.Cdump_Setting.Clear_Building = originalClear
			}()
		}
	}

	ctx, cancelImport := context.WithCancel(context.Background())
	defer cancelImport()
	ctx = withImportAbort(ctx, cancelImport)
	limiter, commandLimiter := newImportLimiters(client, task)

	if err := sendAICommandDim(client, "/gamerule sendcommandfeedback true", false); err != nil {
		log.Log.Warn(fmt.Sprintf("failed to apply command feedback gamerule: %v", err))
	}
	disableCommandBlocksDuringImport(client)

	if client.Cdump_Setting != nil {
		if task.ImportNBT {
			client.Cdump_Setting.No_NBT = false
		} else {
			client.Cdump_Setting.No_NBT = true
		}
	}

	// Convert source coordinates into target-world coordinates before region buffering starts.
	buildX, buildY, buildZ, _ := resolveImportBuildOrigin(x, y, z, task)
	bar, gameProgress, stopProgress := startImportProgressBar(chunkPlan.totalChunks, chunkPlan.skipChunks, client, web_client, task_id, task.UserID, "mcworld", progressCb, gameProgress)
	defer stopProgress()
	if task.ImportNBT || task.ImportCommandBlock {
		gameProgress.SetNBTStatus("等待写入 NBT/命令方块数据")
	} else {
		gameProgress.SetNBTStatus("未启用 NBT/命令方块导入")
	}
	if client.RepairCtx == nil {
		client.RepairCtx = &clientType.RepairContext{}
	}
	client.RepairCtx.Setup(
		file_path,
		types.Position{X: buildX, Y: buildY, Z: buildZ},
		[3]int{int(bounds.width()), int(bounds.height()), int(bounds.length())},
		regionSize,
		task.UseFill,
		client.Cdump_Setting,
		types.Position{X: x, Y: y, Z: z},
		task.AutoPlaceDenyBlock,
		task.AutoPlaceBorder,
	)
	client.RepairCtx.ImportCommandBlock = task.ImportCommandBlock
	client.RepairCtx.CommandDataSpeed = task.CommandDataSpeed

	plainChunkGroupSide := 1
	plainSkipGroups := chunkPlan.skipChunks
	if task.UseFill {
		plainChunkGroupSide = regionSize
		plainSkipGroups = chunkPlan.skipRegions
	}
	if err := importPlainBuildGroups(client, file_path, task, x, y, z, plainChunkGroupSide, plainSkipGroups, ctx, limiter, commandLimiter, bar, gameProgress, progressCb); err != nil {
		if isFatalImportConnError(err) {
			setLastImportError(client, err)
			return false
		}
		return importFail(client, "普通方块导入失败", err)
	}

	if progressCb != nil {
		progressCb(chunkPlan.totalChunks, chunkPlan.totalChunks)
	}
	if web_client != nil {
		web_client.Finish_task_import_file(task_id, task.UserID)
	}
	gameProgress.SetPhase("导入完成")
	gameProgress.SetBuilderStatus("所有区块和附件已处理完成")
	gameProgress.SendToClientNow(client)
	return true
}

func commandBlockNameByMode(mode uint32) string {
	switch mode {
	case 1:
		return "chain_command_block"
	case 2:
		return "repeating_command_block"
	default:
		return "command_block"
	}
}

func disableCommandBlocksDuringImport(client *clientType.Client) {
	if client == nil || client.GameInterface == nil {
		return
	}
	log.Log.Info("正在临时关闭命令方块执行")
	if err := sendAICommandDim(client, "gamerule commandblocksenabled false", false); err != nil {
		log.Log.Warn(fmt.Sprintf("关闭命令方块执行失败: %v", err))
	}
}

func getGameRuleBool(client *clientType.Client, name string) (bool, bool) {
	if client == nil || client.Conn == nil {
		return false, false
	}
	for _, rule := range client.Conn.GameData().GameRules {
		if strings.EqualFold(rule.Name, name) {
			switch v := rule.Value.(type) {
			case bool:
				return v, true
			case uint32:
				return v != 0, true
			case float32:
				return v != 0, true
			}
			return false, true
		}
	}
	return false, false
}

func teleportSafe(client *clientType.Client, x, y, z int, limiter *rate.Limiter, ctx context.Context) {
	if importContextDone(ctx) {
		return
	}
	if err := clientConnError(client); err != nil {
		cancelImportOnFatalError(ctx, client, err)
		return
	}
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			if !importContextDone(ctx) {
				cancelImportOnFatalError(ctx, client, err)
			}
			return
		}
	}
	if err := sendSettingsCommandDim(client, fmt.Sprintf("tp @s %d %d %d", x, y, z)); err != nil {
		cancelImportOnFatalError(ctx, client, err)
	}
}

func clampChunkY(y int) int {
	if y < -64 {
		return -64
	}
	if y > 319 {
		return 319
	}
	return y
}

func waitChunkLoaded(client *clientType.Client, x, y, z int, limiter *rate.Limiter, ctx context.Context, timeout time.Duration) bool {
	if client == nil || client.GameInterface == nil {
		return true
	}

	testY := clampChunkY(y)
	probeTimeout := chunkProbeTimeout
	if timeout > 0 && timeout < probeTimeout {
		probeTimeout = timeout
	}
	lastMove := time.Time{}
	lastNotice := time.Now()
	attempt := 0

	for {
		if isClientConnDead(client) {
			return false
		}
		if ctx != nil {
			select {
			case <-ctx.Done():
				return false
			default:
			}
		}

		if attempt == 0 || time.Since(lastMove) >= time.Second {
			teleportSafe(client, x, testY, z, limiter, ctx)
			lastMove = time.Now()
			if importContextDone(ctx) || isClientConnDead(client) {
				return false
			}
		}

		loaded, err := probeChunkLoaded(client, x, testY, z, limiter, ctx, probeTimeout)
		if err != nil {
			return false
		}
		if loaded {
			return true
		}
		attempt++

		if timeout > 0 && time.Since(lastNotice) >= timeout {
			log.Log.Warn("等待区块加载仍未完成", log.Log.ArgsFromMap(map[string]any{
				"x": x,
				"y": testY,
				"z": z,
			}))
			lastNotice = time.Now()
		}
		if limiter == nil {
			time.Sleep(chunkProbeRetryDelay)
		}
	}
}

func probeChunkLoaded(client *clientType.Client, x, y, z int, limiter *rate.Limiter, ctx context.Context, timeout time.Duration) (bool, error) {
	if client == nil || client.GameInterface == nil {
		return true, nil
	}
	if err := clientConnError(client); err != nil {
		return false, err
	}
	if limiter != nil {
		if ctx != nil {
			if err := limiter.Wait(ctx); err != nil {
				return false, err
			}
		} else {
			_ = limiter.Wait(context.Background())
		}
	}
	resp, isTimeout, err := sendAICommandWithTimeoutDim(
		client,
		"fill ~13 ~13 ~13 ~-13 ~-13 ~-13 air keep",
		timeout,
	)
	if err != nil {
		return false, err
	}
	if isTimeout {
		return false, nil
	}
	if resp.Respond == nil || len(resp.Respond.OutputMessages) == 0 {
		return true, nil
	}
	for _, msg := range resp.Respond.OutputMessages {
		if msg.Message == "commands.setblock.outOfWorld" || msg.Message == "commands.fill.outOfWorld" {
			return false, nil
		}
	}
	return true, nil
}

func clearRegionBox(client *clientType.Client, region *RegionData, limiter *rate.Limiter, ctx context.Context) {
	minX, minY, minZ := math.MaxInt32, math.MaxInt32, math.MaxInt32
	maxX, maxY, maxZ := math.MinInt32, math.MinInt32, math.MinInt32

	updateBounds := func(x, y, z int) {
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
		if z < minZ {
			minZ = z
		}
		if z > maxZ {
			maxZ = z
		}
	}

	for y, blocks := range region.Blocks {
		for _, b := range blocks {
			updateBounds(b.Point.X, y, b.Point.Z)
		}
	}
	for _, b := range region.CommandBlocks {
		updateBounds(b.Point.X, b.Point.Y, b.Point.Z)
	}
	for _, b := range region.NBTBlocks {
		updateBounds(b.Point.X, b.Point.Y, b.Point.Z)
	}
	for _, b := range region.Chests {
		updateBounds(b.Point.X, b.Point.Y, b.Point.Z)
	}
	for _, b := range region.SpecialBlocks {
		updateBounds(b.Point.X, b.Point.Y, b.Point.Z)
	}

	if minX > maxX || minY > maxY || minZ > maxZ {
		return
	}

	teleportSafe(client, (minX+maxX)/2, maxY+2, (minZ+maxZ)/2, limiter, ctx)
	runClearCommandsLayered(client, minX, minY, minZ, maxX, maxY, maxZ, limiter, ctx, nil)
	return
}

type mcworldBounds struct {
	minX int32
	minY int32
	minZ int32
	maxX int32
	maxY int32
	maxZ int32
}

var mcworldBoundsCache sync.Map

func (b mcworldBounds) width() int {
	return int(b.maxX-b.minX) + 1
}

func (b mcworldBounds) height() int {
	return int(b.maxY-b.minY) + 1
}

func (b mcworldBounds) length() int {
	return int(b.maxZ-b.minZ) + 1
}

func (b mcworldBounds) valid() bool {
	return b.maxX >= b.minX && b.maxY >= b.minY && b.maxZ >= b.minZ
}

// clearArea clears a box before import using the layered clear strategy.
func clearArea(client *clientType.Client, minX, minY, minZ, maxX, maxY, maxZ int, limiter *rate.Limiter, ctx context.Context, bar *progressbar.ProgressBar, web_client *webclient.Webclient, task_id string, task types.Task) {
	volume := (maxX - minX + 1) * (maxY - minY + 1) * (maxZ - minZ + 1)
	if volume <= 0 {
		return
	}

	localBar := bar
	if localBar == nil {
		localBar = progressbar.Default(int64(volume*2), "清理导入区域")
		defer localBar.Clear()
	}
	if web_client != nil {
		web_client.Update_task_now_operation(task_id, task.UserID, "清理导入区域")
	}

	centerX := (minX + maxX) / 2
	centerZ := (minZ + maxZ) / 2
	teleportSafe(client, centerX, maxY+2, centerZ, limiter, ctx)
	runClearCommandsLayered(client, minX, minY, minZ, maxX, maxY, maxZ, limiter, ctx, localBar)
	return
}

func parseMCWorldBounds(filePath, levelName string) (mcworldBounds, error) {
	if v, ok := mcworldBoundsCache.Load(filePath); ok {
		if b, ok2 := v.(mcworldBounds); ok2 && b.valid() {
			return b, nil
		}
	}

	check := func(target string) (mcworldBounds, bool) {
		re := regexp.MustCompile(`@\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]~\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]`)
		matches := re.FindStringSubmatch(target)
		if len(matches) != 7 {
			return mcworldBounds{}, false
		}
		startX, _ := strconv.ParseInt(matches[1], 10, 32)
		startY, _ := strconv.ParseInt(matches[2], 10, 32)
		startZ, _ := strconv.ParseInt(matches[3], 10, 32)
		endX, _ := strconv.ParseInt(matches[4], 10, 32)
		endY, _ := strconv.ParseInt(matches[5], 10, 32)
		endZ, _ := strconv.ParseInt(matches[6], 10, 32)
		minX, maxX := minInt32(int32(startX), int32(endX)), maxInt32(int32(startX), int32(endX))
		minY, maxY := minInt32(int32(startY), int32(endY)), maxInt32(int32(startY), int32(endY))
		minZ, maxZ := minInt32(int32(startZ), int32(endZ)), maxInt32(int32(startZ), int32(endZ))
		return mcworldBounds{minX: minX, minY: minY, minZ: minZ, maxX: maxX, maxY: maxY, maxZ: maxZ}, true
	}
	if b, ok := check(filePath); ok {
		mcworldBoundsCache.Store(filePath, b)
		return b, nil
	}
	if b, ok := check(levelName); ok {
		mcworldBoundsCache.Store(filePath, b)
		return b, nil
	}

	bounds, err := promptMCWorldBounds()
	if err != nil {
		return mcworldBounds{}, fmt.Errorf("failed to read mcworld bounds: %w", err)
	}
	mcworldBoundsCache.Store(filePath, bounds)
	return bounds, nil
}

func promptMCWorldBounds() (mcworldBounds, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("请输入 mcworld 范围: x1 y1 z1 x2 y2 z2")
		fmt.Print("范围> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return mcworldBounds{}, err
		}
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 6 {
			fmt.Println("需要输入 6 个整数")
			continue
		}
		vals := make([]int, 6)
		ok := true
		for i, f := range fields {
			v, err := strconv.Atoi(f)
			if err != nil {
				ok = false
				break
			}
			vals[i] = v
		}
		if !ok {
			fmt.Println("范围坐标必须是整数")
			continue
		}
		minX, maxX := vals[0], vals[3]
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		minY, maxY := vals[1], vals[4]
		if minY > maxY {
			minY, maxY = maxY, minY
		}
		minZ, maxZ := vals[2], vals[5]
		if minZ > maxZ {
			minZ, maxZ = maxZ, minZ
		}
		return mcworldBounds{
			minX: int32(minX),
			minY: int32(minY),
			minZ: int32(minZ),
			maxX: int32(maxX),
			maxY: int32(maxY),
			maxZ: int32(maxZ),
		}, nil
	}
}

func cropMCWorldBounds(b mcworldBounds, cropMin, cropMax [3]int) mcworldBounds {
	minX := maxInt32(b.minX, int32(cropMin[0]))
	minY := maxInt32(b.minY, int32(cropMin[1]))
	minZ := maxInt32(b.minZ, int32(cropMin[2]))
	maxX := minInt32(b.maxX, int32(cropMax[0]))
	maxY := minInt32(b.maxY, int32(cropMax[1]))
	maxZ := minInt32(b.maxZ, int32(cropMax[2]))
	return mcworldBounds{minX: minX, minY: minY, minZ: minZ, maxX: maxX, maxY: maxY, maxZ: maxZ}
}

func unzipMCWorld(src, dest string) error {
	var lastErr error
	for attempt := 1; attempt <= importRetryLimit; attempt++ {
		if err := unzipMCWorldFromZip(src, dest); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(time.Duration(attempt) * 150 * time.Millisecond)
	}

	if isLikelyWorldDirectory(src) {
		if err := copyWorldDirectory(src, dest); err == nil {
			return nil
		} else {
			lastErr = fmt.Errorf("zip open failed after %d attempts, directory fallback failed: %w", importRetryLimit, err)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("unknown mcworld unpack error")
	}
	return fmt.Errorf("failed after %d attempts: %w", importRetryLimit, lastErr)
}

func unzipMCWorldFromZip(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, f := range reader.File {
		targetPath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, f.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		dstFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		srcFile, err := f.Open()
		if err != nil {
			dstFile.Close()
			return err
		}
		if _, err := io.Copy(dstFile, srcFile); err != nil {
			srcFile.Close()
			dstFile.Close()
			return err
		}
		srcFile.Close()
		dstFile.Close()
	}
	return nil
}

func isLikelyWorldDirectory(src string) bool {
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return false
	}
	if _, err := os.Stat(filepath.Join(src, "level.dat")); err == nil {
		return true
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(src, entry.Name(), "level.dat")); err == nil {
			return true
		}
	}
	return false
}

func copyWorldDirectory(src, dest string) error {
	worldRoot := src
	if _, err := os.Stat(filepath.Join(worldRoot, "level.dat")); err != nil {
		entries, readErr := os.ReadDir(src)
		if readErr != nil {
			return readErr
		}
		found := false
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidate := filepath.Join(src, entry.Name())
			if _, err := os.Stat(filepath.Join(candidate, "level.dat")); err == nil {
				worldRoot = candidate
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("level.dat not found in world directory")
		}
	}

	return filepath.Walk(worldRoot, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(worldRoot, current)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		targetPath := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		srcFile, err := os.Open(current)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		dstFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()
		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func walkRegionChunkSnake(
	bounds mcworldBounds,
	regionSize int,
	baseChunkX int32,
	baseChunkZ int32,
	skipChunk func(chunkX, chunkZ int32) bool,
	visit func(regionKey [2]int, regionChunkMinX, regionChunkMaxX, regionChunkMinZ, regionChunkMaxZ int32) error,
) error {
	minChunkX := bounds.minX >> 4
	maxChunkX := bounds.maxX >> 4
	minChunkZ := bounds.minZ >> 4
	maxChunkZ := bounds.maxZ >> 4
	if regionSize < 1 {
		regionSize = 1
	}

	if baseChunkX > minChunkX {
		baseChunkX = minChunkX
	}
	if baseChunkZ > minChunkZ {
		baseChunkZ = minChunkZ
	}
	regionCountX := (int(maxChunkX-baseChunkX) + regionSize) / regionSize
	regionCountZ := (int(maxChunkZ-baseChunkZ) + regionSize) / regionSize

	for regionX := 0; regionX < regionCountX; regionX++ {
		zStart, zEnd, zStep := 0, regionCountZ-1, 1
		if regionX%2 != 0 {
			zStart, zEnd, zStep = regionCountZ-1, 0, -1
		}
		for regionZ := zStart; ; regionZ += zStep {
			regionChunkMinX := maxInt32(minChunkX, baseChunkX+int32(regionX*regionSize))
			regionChunkMaxX := minInt32(maxChunkX, baseChunkX+int32(regionX*regionSize+regionSize-1))
			regionChunkMinZ := maxInt32(minChunkZ, baseChunkZ+int32(regionZ*regionSize))
			regionChunkMaxZ := minInt32(maxChunkZ, baseChunkZ+int32(regionZ*regionSize+regionSize-1))
			if visit != nil {
				if err := visit([2]int{regionX, regionZ}, regionChunkMinX, regionChunkMaxX, regionChunkMinZ, regionChunkMaxZ); err != nil {
					return err
				}
			}
			if regionZ == zEnd {
				break
			}
		}
	}
	return nil
}

func walkMCWorldChunks(
	bounds mcworldBounds,
	regionSize int,
	baseChunkX int32,
	baseChunkZ int32,
	skipChunk func(chunkX, chunkZ int32) bool,
	onChunk func(chunkStartX, chunkStartZ int32) error,
	onRegionDone func(regionKey [2]int, traversedChunkCount int),
) error {
	return walkRegionChunkSnake(bounds, regionSize, baseChunkX, baseChunkZ, skipChunk, func(regionKey [2]int, regionChunkMinX, regionChunkMaxX, regionChunkMinZ, regionChunkMaxZ int32) error {
		traversedChunkCount := 0
		for chunkX := regionChunkMinX; chunkX <= regionChunkMaxX; chunkX++ {
			for chunkZ := regionChunkMinZ; chunkZ <= regionChunkMaxZ; chunkZ++ {
				if skipChunk != nil && skipChunk(chunkX, chunkZ) {
					continue
				}
				if onChunk != nil {
					if err := onChunk(chunkX*16, chunkZ*16); err != nil {
						return err
					}
				}
				traversedChunkCount++
			}
		}
		if onRegionDone != nil {
			onRegionDone(regionKey, traversedChunkCount)
		}
		return nil
	})
}

func walkMCWorldBlocks(
	bw *world.BedrockWorld,
	bounds mcworldBounds,
	needNBT bool,
	dimID int32,
	regionSize int,
	baseChunkX int32,
	baseChunkZ int32,
	skipChunk func(chunkX, chunkZ int32) bool,
	onChunk func(chunkStartX, chunkStartZ int32) error,
	fn func(wx, wy, wz int32, runtimeID uint32, nbt map[string]any) error,
	onRegionDone func(regionKey [2]int, traversedChunkCount int),
) error {
	return walkRegionChunkSnake(bounds, regionSize, baseChunkX, baseChunkZ, skipChunk, func(regionKey [2]int, regionChunkMinX, regionChunkMaxX, regionChunkMinZ, regionChunkMaxZ int32) error {
		traversedChunkCount := 0
		for chunkX := regionChunkMinX; chunkX <= regionChunkMaxX; chunkX++ {
			for chunkZ := regionChunkMinZ; chunkZ <= regionChunkMaxZ; chunkZ++ {
				if skipChunk != nil && skipChunk(chunkX, chunkZ) {
					continue
				}
				if onChunk != nil {
					if err := onChunk(chunkX*16, chunkZ*16); err != nil {
						return err
					}
				}
				traversedChunkCount++
				pos := bwo_define.ChunkPos{chunkX, chunkZ}
				chunkData, exists, err := bw.LoadChunk(bwo_define.Dimension(dimID), pos)
				if err != nil {
					return err
				}
				var nbtMap map[[3]int32]map[string]any
				if needNBT {
					if entries, err := bw.LoadNBT(bwo_define.Dimension(dimID), pos); err == nil && len(entries) > 0 {
						nbtMap = buildChunkNBTMap(entries)
					}
				}
				if !exists || chunkData == nil {
					continue
				}

				chunkStartX := chunkX * 16
				chunkStartZ := chunkZ * 16
				chunkEndX := chunkStartX + 15
				chunkEndZ := chunkStartZ + 15

				blockMinX := maxInt32(bounds.minX, chunkStartX)
				blockMaxX := minInt32(bounds.maxX, chunkEndX)
				blockMinZ := maxInt32(bounds.minZ, chunkStartZ)
				blockMaxZ := minInt32(bounds.maxZ, chunkEndZ)

				for x := blockMinX; x <= blockMaxX; x++ {
					localX := uint8(x - chunkStartX)
					for z := blockMinZ; z <= blockMaxZ; z++ {
						localZ := uint8(z - chunkStartZ)
						for y := bounds.minY; y <= bounds.maxY; y++ {
							runtimeID := pickRuntimeID(chunkData, localX, int16(y), localZ)
							if runtimeID == block.AirRuntimeID {
								continue
							}
							var nbt map[string]any
							if needNBT && nbtMap != nil {
								if v, ok := nbtMap[[3]int32{x, y, z}]; ok {
									nbt = v
								}
							}
							if err := fn(x, y, z, runtimeID, nbt); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		if onRegionDone != nil {
			onRegionDone(regionKey, traversedChunkCount)
		}
		return nil
	})
}

func pickRuntimeID(c *chunk.Chunk, x uint8, y int16, z uint8) uint32 {
	if c == nil {
		return block.AirRuntimeID
	}
	if rt := c.Block(x, y, z, 0); rt != block.AirRuntimeID {
		return rt
	}
	return c.Block(x, y, z, 1)
}

func buildChunkNBTMap(entries []map[string]any) map[[3]int32]map[string]any {
	if len(entries) == 0 {
		return nil
	}
	result := make(map[[3]int32]map[string]any, len(entries))
	for _, entry := range entries {
		xVal, okX := toInt32(entry["x"])
		yVal, okY := toInt32(entry["y"])
		zVal, okZ := toInt32(entry["z"])
		if !okX || !okY || !okZ {
			continue
		}
		result[[3]int32{xVal, yVal, zVal}] = entry
	}
	return result
}

func isSignBlockName(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "sign")
}

func isCommandBlockName(name string) bool {
	name = strings.ToLower(name)
	name = strings.TrimPrefix(name, "minecraft:")
	return name == "command_block" || name == "chain_command_block" || name == "repeating_command_block"
}

func applyDefaultSignWax(nbt map[string]any) map[string]any {
	if nbt == nil {
		return nil
	}
	if _, ok := nbt["IsWaxed"]; ok {
		nbt["IsWaxed"] = byte(1)
		return nbt
	}
	if _, ok := nbt["FrontText"]; ok {
		nbt["IsWaxed"] = byte(1)
		return nbt
	}
	if _, ok := nbt["BackText"]; ok {
		nbt["IsWaxed"] = byte(1)
	}
	return nbt
}

func normalizeBlockName(name string) (string, bool) {
	switch strings.ToLower(name) {
	case "minecraft:oka_leaves", "oka_leaves":
		return "minecraft:oak_leaves", true
	default:
		return name, true
	}
}

type runtimeCacheEntry struct {
	name  string
	state string
	found bool
}

var runtimeCache sync.Map // map[uint32]runtimeCacheEntry

func convertRuntimeToNameState(runtimeID uint32) (string, string, bool) {
	if v, ok := runtimeCache.Load(runtimeID); ok {
		e := v.(runtimeCacheEntry)
		return e.name, e.state, e.found
	}
	name, state, found := convertRuntimeToNameStateUncached(runtimeID)
	runtimeCache.Store(runtimeID, runtimeCacheEntry{name, state, found})
	return name, state, found
}

func convertRuntimeToNameStateUncached(runtimeID uint32) (string, string, bool) {
	if name, props, ok := block.RuntimeIDToState(runtimeID); ok {
		return name, blockStateMapToStr(props), true
	}
	if n, s, ok := blocks.RuntimeIDToBlockNameAndStateStr(runtimeID); ok {
		return n, s, true
	}
	if n, s, ok := runtimeToStateFallback(runtimeID); ok {
		return n, s, true
	}
	return "", "", false
}

func runtimeToStateFallback(runtimeID uint32) (string, string, bool) {
	name, props, ok := block.RuntimeIDToState(runtimeID)
	if !ok {
		return "", "", false
	}
	if rtid, ok := blocks.BlockNameAndStateToRuntimeID(name, props); ok {
		if n, s, found := blocks.RuntimeIDToBlockNameAndStateStr(rtid); found {
			return n, s, true
		}
	}
	return name, blockStateMapToStr(props), true
}

func blockStateMapToStr(props map[string]any) string {
	if len(props) == 0 {
		return "[]"
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteByte('[')
	for i, k := range keys {
		if i > 0 {
			builder.WriteByte(',')
		}
		val := props[k]
		builder.WriteByte('"')
		builder.WriteString(k)
		builder.WriteString(`"=`)

		switch v := val.(type) {
		case string:
			builder.WriteByte('"')
			builder.WriteString(v)
			builder.WriteByte('"')
		case bool:
			if v {
				builder.WriteString("true")
			} else {
				builder.WriteString("false")
			}
		case byte:
			if strings.HasSuffix(k, "_bit") {
				if v == 0 {
					builder.WriteString("false")
				} else {
					builder.WriteString("true")
				}
			} else {
				builder.WriteString(strconv.Itoa(int(v)))
			}
		case int32:
			builder.WriteString(strconv.Itoa(int(v)))
		case int:
			builder.WriteString(strconv.Itoa(v))
		case uint16:
			builder.WriteString(strconv.Itoa(int(v)))
		case uint32:
			builder.WriteString(strconv.Itoa(int(v)))
		case int64:
			builder.WriteString(strconv.FormatInt(v, 10))
		case uint64:
			builder.WriteString(strconv.FormatUint(v, 10))
		default:
			builder.WriteString(fmt.Sprintf("%v", v))
		}
	}
	builder.WriteByte(']')
	return builder.String()
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

func toInt32(value any) (int32, bool) {
	switch v := value.(type) {
	case int8:
		return int32(v), true
	case int16:
		return int32(v), true
	case int32:
		return v, true
	case int64:
		return int32(v), true
	case int:
		return int32(v), true
	case uint8:
		return int32(v), true
	case uint16:
		return int32(v), true
	case uint32:
		return int32(v), true
	case uint64:
		return int32(v), true
	case uint:
		return int32(v), true
	case float32:
		return int32(v), true
	case float64:
		return int32(v), true
	case string:
		if parsed, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(parsed), true
		}
	}
	return 0, false
}

func floorDiv(value, divisor int) int {
	if divisor == 0 {
		return 0
	}
	if value >= 0 {
		return value / divisor
	}
	return -(((-value) + divisor - 1) / divisor)
}

func Println_Cdump(cmd string, tips string) {
	pterm.Println(fmt.Sprintf("%s %s %s", pterm.Green(cmd), pterm.Cyan("-"), pterm.Yellow(tips)))
}

func Cdump_Inquire(client *clientType.Client, words []string) bool {
	setting := client.Cdump_Setting.Get_Setting()
	for k, v := range setting {
		pterm.Println(pterm.Green(k), pterm.Cyan(":"), pterm.Yellow(v))
	}
	return true
}

func Cdump_Fill(client *clientType.Client, x int, y int, z int, x2 int, y2 int, z2 int, block string, query bool) ResourcesControl.CommandRespond {
	for {
		if !waitChunkLoaded(client, x, y, z, nil, nil, 2*time.Second) {
			continue
		}
		break
	}
	if !query {
		sendAICommandDim(client, fmt.Sprintf("fill %d %d %d %d %d %d %s", x, y, z, x2, y2, z2, block), true)
		return ResourcesControl.CommandRespond{}
	} else {
		for {
			a2 := sendAICommandWithResponseDim(client, fmt.Sprintf("fill %d %d %d %d %d %d %s", x, y, z, x2, y2, z2, block), ResourcesControl.CommandRequestOptions{TimeOut: time.Second * 5})
			if a2.Error == nil {
				if a2.Respond != nil {
					if a2.Respond.SuccessCount == 0 {
						if len(a2.Respond.OutputMessages) > 0 {
							if a2.Respond.OutputMessages[0].Message == "commands.fill.outOfWorld" {
								sendAICommandWithResponseDim(client, fmt.Sprintf("tp @s %d %d %d", x, y, z), ResourcesControl.CommandRequestOptions{
									TimeOut: 1 * time.Second,
								})
								if !waitChunkLoaded(client, x, y, z, nil, nil, 2*time.Second) {
									continue
								}
								continue
							}
						}
					}
				}
			}
			return a2
		}
	}
}
