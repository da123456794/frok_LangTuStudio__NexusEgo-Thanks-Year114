package function

import (
	"fmt"
	"strings"
	"sync"
	"time"

	clientType "nexus/utils/client"
)

const (
	importGameProgressPushInterval = 2 * time.Second
	importGameProgressHoldDelay    = 900 * time.Millisecond
)

type ImportGameProgress struct {
	mu sync.RWMutex

	phase          string
	builderStatus  string
	builder2Status string
	nbtStatus      string

	chunkCurrent   int
	chunkTotal     int
	commandCurrent int
	commandTotal   int
	commandKnown   bool
	nbtCurrent     int
	nbtTotal       int
	nbtKnown       bool
	blockCurrent   int
	groupCurrent   int
	groupTotal     int
	commandFailed  int
	nbtFailed      int

	startedAt    time.Time
	finishedAt   time.Time
	frameIndex   int
	lastSentAt   time.Time
	sendVersion  uint64
	titleMuted   bool
	activityHook func()

	speedSampleBlocks int
	speedSampleAt     time.Time
	currentBlockSpeed float64
}

var repairGameProgressByClient sync.Map // map[*clientType.Client]*ImportGameProgress

func NewImportGameProgress(phase string) *ImportGameProgress {
	if strings.TrimSpace(phase) == "" {
		phase = "等待开始"
	}
	return &ImportGameProgress{
		phase:          phase,
		builderStatus:  "准备登录",
		builder2Status: "未启用",
		nbtStatus:      "未启用",
		startedAt:      time.Now(),
	}
}

func (p *ImportGameProgress) SetPhase(phase string) {
	phase = strings.TrimSpace(phase)
	if p == nil || phase == "" {
		return
	}
	p.mu.Lock()
	p.phase = phase
	if isImportFinishedPhase(phase) && p.finishedAt.IsZero() {
		p.finishedAt = time.Now()
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) MarkFinished() {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.finishedAt.IsZero() {
		p.finishedAt = time.Now()
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetTitleMuted(muted bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.titleMuted != muted {
		p.titleMuted = muted
		p.sendVersion++
	}
	if !muted {
		p.lastSentAt = time.Time{}
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetActivityHook(hook func()) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.activityHook = hook
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetBuilderStatus(status string) {
	if p == nil || strings.TrimSpace(status) == "" {
		return
	}
	p.mu.Lock()
	p.builderStatus = status
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetBuilder2Status(status string) {
	if p == nil || strings.TrimSpace(status) == "" {
		return
	}
	p.mu.Lock()
	p.builder2Status = status
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetNBTStatus(status string) {
	if p == nil || strings.TrimSpace(status) == "" {
		return
	}
	p.mu.Lock()
	p.nbtStatus = status
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetChunkProgress(current, total int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.chunkCurrent = clampProgressValue(current)
	p.chunkTotal = clampProgressValue(total)
	if p.chunkTotal > 0 && p.chunkCurrent > p.chunkTotal {
		p.chunkCurrent = p.chunkTotal
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) ResetImportCounters(chunkCurrent, chunkTotal int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.chunkCurrent = clampProgressValue(chunkCurrent)
	p.chunkTotal = clampProgressValue(chunkTotal)
	if p.chunkTotal > 0 && p.chunkCurrent > p.chunkTotal {
		p.chunkCurrent = p.chunkTotal
	}
	p.commandCurrent = 0
	p.commandTotal = 0
	p.commandKnown = false
	p.nbtCurrent = 0
	p.nbtTotal = 0
	p.nbtKnown = false
	p.blockCurrent = 0
	p.commandFailed = 0
	p.nbtFailed = 0
	p.startedAt = time.Now()
	p.finishedAt = time.Time{}
	p.speedSampleBlocks = 0
	p.speedSampleAt = time.Time{}
	p.currentBlockSpeed = 0
	p.mu.Unlock()
}

func (p *ImportGameProgress) AddBlockProgress(delta int) {
	if p == nil || delta <= 0 {
		return
	}
	p.mu.Lock()
	p.blockCurrent += delta
	p.mu.Unlock()
}

func (p *ImportGameProgress) AddCommandTotal(delta int) {
	if p == nil || delta <= 0 {
		return
	}
	p.mu.Lock()
	if !p.commandKnown {
		p.commandTotal += delta
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) AddNBTTotal(delta int) {
	if p == nil || delta <= 0 {
		return
	}
	p.mu.Lock()
	if !p.nbtKnown {
		p.nbtTotal += delta
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetChunkGroupProgress(current, total int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.groupCurrent = clampProgressValue(current)
	p.groupTotal = clampProgressValue(total)
	if p.groupTotal > 0 && p.groupCurrent > p.groupTotal {
		p.groupCurrent = p.groupTotal
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetCommandTotal(total int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.commandTotal = clampProgressValue(total)
	if p.commandCurrent > p.commandTotal {
		p.commandTotal = p.commandCurrent
	}
	p.commandKnown = true
	p.mu.Unlock()
}

func (p *ImportGameProgress) SetNBTTotal(total int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.nbtTotal = clampProgressValue(total)
	if p.nbtCurrent > p.nbtTotal {
		p.nbtTotal = p.nbtCurrent
	}
	p.nbtKnown = true
	p.mu.Unlock()
}

func (p *ImportGameProgress) MarkCommandDone() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.commandCurrent++
	if !p.commandKnown && p.commandCurrent > p.commandTotal {
		p.commandTotal = p.commandCurrent
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) MarkCommandFailed() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.commandFailed++
	p.commandCurrent++
	if !p.commandKnown && p.commandCurrent > p.commandTotal {
		p.commandTotal = p.commandCurrent
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) MarkNBTDone() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.nbtCurrent++
	if !p.nbtKnown && p.nbtCurrent > p.nbtTotal {
		p.nbtTotal = p.nbtCurrent
	}
	p.mu.Unlock()
}

func BindRepairGameProgress(client *clientType.Client, progress *ImportGameProgress) {
	if client == nil {
		return
	}
	if progress == nil {
		repairGameProgressByClient.Delete(client)
		return
	}
	repairGameProgressByClient.Store(client, progress)
}

func RepairGameProgress(client *clientType.Client) *ImportGameProgress {
	if client == nil {
		return nil
	}
	value, ok := repairGameProgressByClient.Load(client)
	if !ok {
		return nil
	}
	progress, _ := value.(*ImportGameProgress)
	return progress
}

func ShowRepairModeIdle(client *clientType.Client, progress *ImportGameProgress, nbtEnabled bool) {
	if progress == nil {
		progress = RepairGameProgress(client)
	}
	if progress == nil {
		return
	}
	BindRepairGameProgress(client, progress)

	progress.mu.Lock()
	if progress.finishedAt.IsZero() || isImportFinishedPhase(progress.phase) {
		progress.finishedAt = time.Now()
	}
	progress.phase = "§c§l修补模式 §f在线待命"
	progress.builderStatus = "进入修补模式"
	progress.builder2Status = "未启用"
	if nbtEnabled {
		progress.nbtStatus = "在线待命"
	} else {
		progress.nbtStatus = "未启用"
	}
	progress.mu.Unlock()

	progress.SendToClientNow(client)
}

func (p *ImportGameProgress) MarkNBTFailed() {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.nbtFailed++
	p.nbtCurrent++
	if !p.nbtKnown && p.nbtCurrent > p.nbtTotal {
		p.nbtTotal = p.nbtCurrent
	}
	p.mu.Unlock()
}

func (p *ImportGameProgress) Start(clientProvider func() *clientType.Client) func() {
	if p == nil {
		return func() {}
	}
	stop := make(chan struct{})
	var stopOnce sync.Once
	go func() {
		ticker := time.NewTicker(importGameProgressPushInterval)
		defer ticker.Stop()
		p.SendToClientNow(resolveProgressClient(clientProvider))
		for {
			select {
			case <-stop:
				p.SendToClientNow(resolveProgressClient(clientProvider))
				return
			case <-ticker.C:
				p.SendToClient(resolveProgressClient(clientProvider))
			}
		}
	}()
	return func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}
}

func (p *ImportGameProgress) SendToClient(client *clientType.Client) {
	p.sendToClient(client, false)
}

func (p *ImportGameProgress) SendToClientNow(client *clientType.Client) {
	p.sendToClient(client, true)
}

func (p *ImportGameProgress) sendToClient(client *clientType.Client, force bool) {
	if p == nil || client == nil || client.GameInterface == nil {
		return
	}
	version, ok := p.reserveSend(force)
	if !ok {
		return
	}
	p.mu.RLock()
	activityHook := p.activityHook
	p.mu.RUnlock()
	if activityHook != nil {
		activityHook()
	}
	gameInterface := client.GameInterface
	message := p.Render()
	_ = gameInterface.Title(message)
	if !force && importGameProgressHoldDelay > 0 {
		go p.repeatLatestTitle(gameInterface, message, version)
	}
}

func (p *ImportGameProgress) reserveSend(force bool) (uint64, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	if p.titleMuted {
		return p.sendVersion, false
	}
	if !force && !p.lastSentAt.IsZero() && now.Sub(p.lastSentAt) < importGameProgressPushInterval {
		return p.sendVersion, false
	}
	p.lastSentAt = now
	p.sendVersion++
	return p.sendVersion, true
}

func (p *ImportGameProgress) repeatLatestTitle(gameInterface interface{ Title(string) error }, message string, version uint64) {
	if gameInterface == nil {
		return
	}
	time.Sleep(importGameProgressHoldDelay)
	if p.currentSendVersion() != version {
		return
	}
	_ = gameInterface.Title(message)
}

func (p *ImportGameProgress) currentSendVersion() uint64 {
	if p == nil {
		return 0
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sendVersion
}

func (p *ImportGameProgress) Render() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	frames := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
	frame := frames[p.frameIndex%len(frames)]
	p.frameIndex++

	progressCurrent, progressTotal := importOverallProgress(p.chunkCurrent, p.chunkTotal, p.commandCurrent, p.commandTotal, p.nbtCurrent, p.nbtTotal)
	percent := 0
	if progressTotal > 0 {
		percent = progressCurrent * 100 / progressTotal
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	phase := strings.TrimSpace(p.phase)
	if phase == "" {
		phase = "等待开始"
	}
	builderStatus := normalizeBuilderStatus(defaultProgressText(p.builderStatus, "准备登录"))

	barWidth := 16
	filled := percent * barWidth / 100
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}

	bar := "§a" + strings.Repeat("█", filled) + "§8" + strings.Repeat("░", barWidth-filled)
	now := time.Now()
	finished := !p.finishedAt.IsZero()
	if finished {
		now = p.finishedAt
	}
	elapsed := now.Sub(p.startedAt).Round(time.Second)
	remaining, finishAt := estimateImportFinish(now, elapsed, progressCurrent, progressTotal)
	if finished {
		remaining = "0s"
		finishAt = now.Format("15:04")
	}
	processedBlocks := p.blockCurrent + p.commandCurrent + p.nbtCurrent
	averageBlockSpeed := formatProgressSpeed(processedBlocks, elapsed)
	blockSpeed := p.formatCurrentBlockSpeed(processedBlocks, now)
	chunkSpeed := formatProgressSpeed(p.chunkCurrent, elapsed)
	if isRepairModeIdlePhase(phase) {
		blockSpeed = "0/s"
		chunkSpeed = "0/s"
	}
	line1 := fmt.Sprintf("§b%s §fNexusEgo §7| §f%s §e%d%% §7[%s§7]", frame, phase, percent, bar)
	statusParts := []string{
		fmt.Sprintf("§a当前机器人行为§7:%s", builderStatus),
	}
	line2 := fmt.Sprintf("§b%s %s", frame, strings.Join(statusParts, " "))
	line3 := fmt.Sprintf("§b%s §f区块 %d/%d §8| §f命令 %d/%d §8| §fNBT %d/%d §8| §c失败 %d/%d", frame, p.chunkCurrent, p.chunkTotal, p.commandCurrent, p.commandTotal, p.nbtCurrent, p.nbtTotal, p.commandFailed, p.nbtFailed)
	groupLine := ""
	if p.groupTotal > 0 {
		groupPercent := p.groupCurrent * 100 / p.groupTotal
		if groupPercent < 0 {
			groupPercent = 0
		}
		if groupPercent > 100 {
			groupPercent = 100
		}
		groupWidth := 16
		groupFilled := groupPercent * groupWidth / 100
		groupBar := "§d" + strings.Repeat("█", groupFilled) + "§8" + strings.Repeat("░", groupWidth-groupFilled)
		groupLine = fmt.Sprintf("§d%s §f当前区块组 %d/%d §7[%s§7]", frame, p.groupCurrent, p.groupTotal, groupBar)
	} else {
		groupLine = fmt.Sprintf("§d%s §f当前区块组 --/-- §7[§8%s§7]", frame, strings.Repeat("░", 16))
	}
	line4 := fmt.Sprintf("§b%s §f实际 %s §8| §e剩余 %s §8| §a预计 %s", frame, formatProgressDuration(elapsed), remaining, finishAt)
	line5 := fmt.Sprintf("§b%s §f平均方块速度 §a%s §8| §f方块 §a%s §8| §b区块 %s", frame, averageBlockSpeed, blockSpeed, chunkSpeed)
	return sanitizeGameProgressMessage(strings.Join([]string{line1, line2, line3, groupLine, line4, line5}, "\n"))
}

func (p *ImportGameProgress) formatCurrentBlockSpeed(processedBlocks int, now time.Time) string {
	if p.speedSampleAt.IsZero() {
		p.speedSampleAt = now
		p.speedSampleBlocks = processedBlocks
		return formatProgressSpeedValue(p.currentBlockSpeed)
	}

	elapsed := now.Sub(p.speedSampleAt)
	if elapsed < 500*time.Millisecond {
		return formatProgressSpeedValue(p.currentBlockSpeed)
	}

	delta := processedBlocks - p.speedSampleBlocks
	if delta < 0 {
		delta = 0
	}
	p.currentBlockSpeed = float64(delta) / elapsed.Seconds()
	p.speedSampleAt = now
	p.speedSampleBlocks = processedBlocks
	return formatProgressSpeedValue(p.currentBlockSpeed)
}

func sanitizeGameProgressMessage(message string) string {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	return strings.TrimSpace(message)
}

func defaultProgressText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeBuilderStatus(status string) string {
	return strings.ReplaceAll(strings.TrimSpace(status), "在线 建造", "在线")
}

func isImportFinishedPhase(phase string) bool {
	return strings.TrimSpace(phase) == "导入完成"
}

func isRepairModeIdlePhase(phase string) bool {
	phase = strings.TrimSpace(phase)
	return strings.Contains(phase, "修补模式") && strings.Contains(phase, "在线待命")
}

func importOverallProgress(chunkCurrent, chunkTotal, commandCurrent, commandTotal, nbtCurrent, nbtTotal int) (int, int) {
	current := clampProgressValue(chunkCurrent) + clampProgressValue(commandCurrent) + clampProgressValue(nbtCurrent)
	total := clampProgressValue(chunkTotal) + clampProgressValue(commandTotal) + clampProgressValue(nbtTotal)
	if total > 0 && current > total {
		current = total
	}
	return current, total
}

func estimateImportFinish(now time.Time, elapsed time.Duration, current, total int) (string, string) {
	if total <= 0 || current <= 0 {
		return "--", "--"
	}
	if current >= total {
		return "0s", now.Format("15:04")
	}
	remaining := time.Duration(float64(elapsed) * float64(total-current) / float64(current)).Round(time.Second)
	if remaining < 0 {
		remaining = 0
	}
	return formatProgressDuration(remaining), now.Add(remaining).Format("15:04")
}

func formatProgressDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return d.Round(time.Second).String()
}

func formatProgressSpeed(count int, elapsed time.Duration) string {
	if count <= 0 || elapsed <= 0 {
		return "0/s"
	}
	return formatProgressSpeedValue(float64(count) / elapsed.Seconds())
}

func formatProgressSpeedValue(speed float64) string {
	if speed <= 0 {
		return "0/s"
	}
	switch {
	case speed >= 100:
		return fmt.Sprintf("%.0f/s", speed)
	case speed >= 10:
		return fmt.Sprintf("%.1f/s", speed)
	default:
		return fmt.Sprintf("%.2f/s", speed)
	}
}

func clampProgressValue(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func resolveProgressClient(provider func() *clientType.Client) *clientType.Client {
	if provider == nil {
		return nil
	}
	return provider()
}
