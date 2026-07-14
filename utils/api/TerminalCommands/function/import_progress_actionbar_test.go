package function

import (
	"strings"
	"testing"
	"time"

	resources_control "nexus/utils/api/resources_control"
	clientType "nexus/utils/client"
)

type titleRecorder struct {
	count int
}

func (r *titleRecorder) SendAICommand(string, bool) error { return nil }
func (r *titleRecorder) SendAICommandWithResponse(string, resources_control.CommandRequestOptions) resources_control.CommandRespond {
	return resources_control.CommandRespond{}
}
func (r *titleRecorder) SendSettingsCommand(string, bool) error { return nil }
func (r *titleRecorder) SendCommand(string) error               { return nil }
func (r *titleRecorder) SendWSCommand(string) error             { return nil }
func (r *titleRecorder) SendCommandWithResponse(string, resources_control.CommandRequestOptions) resources_control.CommandRespond {
	return resources_control.CommandRespond{}
}
func (r *titleRecorder) SendWSCommandWithResponse(string, resources_control.CommandRequestOptions) resources_control.CommandRespond {
	return resources_control.CommandRespond{}
}
func (r *titleRecorder) SendCommandWithOrigin(string, uint32, resources_control.CommandRequestOptions) resources_control.CommandRespond {
	return resources_control.CommandRespond{}
}
func (r *titleRecorder) SetBlock([3]int32, string, string) error      { return nil }
func (r *titleRecorder) SetBlockAsync([3]int32, string, string) error { return nil }
func (r *titleRecorder) SendChat(string) error                        { return nil }
func (r *titleRecorder) Output(string) error                          { return nil }
func (r *titleRecorder) Title(string) error {
	r.count++
	return nil
}

var _ clientType.GameInterface = (*titleRecorder)(nil)

func TestShowRepairModeIdleKeepsImportStatsAndGlobalSpeed(t *testing.T) {
	progress := NewImportGameProgress("导入完成")
	progress.ResetImportCounters(100, 100)
	progress.SetCommandTotal(2)
	progress.MarkCommandDone()
	progress.MarkCommandDone()
	progress.SetNBTTotal(1)
	progress.MarkNBTDone()
	progress.AddBlockProgress(600)

	startedAt := time.Now().Add(-10 * time.Minute)
	progress.mu.Lock()
	progress.startedAt = startedAt
	progress.finishedAt = startedAt.Add(5 * time.Minute)
	progress.mu.Unlock()

	ShowRepairModeIdle(nil, progress, true)

	progress.mu.RLock()
	if progress.chunkCurrent != 100 || progress.chunkTotal != 100 {
		t.Fatalf("chunk progress = %d/%d, want 100/100", progress.chunkCurrent, progress.chunkTotal)
	}
	if progress.commandCurrent != 2 || progress.commandTotal != 2 {
		t.Fatalf("command progress = %d/%d, want 2/2", progress.commandCurrent, progress.commandTotal)
	}
	if progress.nbtCurrent != 1 || progress.nbtTotal != 1 {
		t.Fatalf("nbt progress = %d/%d, want 1/1", progress.nbtCurrent, progress.nbtTotal)
	}
	if !progress.startedAt.Equal(startedAt) {
		t.Fatalf("startedAt = %v, want %v", progress.startedAt, startedAt)
	}
	if !progress.finishedAt.After(startedAt.Add(5 * time.Minute)) {
		t.Fatalf("finishedAt = %v, want updated repair-mode entry time", progress.finishedAt)
	}
	progress.mu.RUnlock()

	message := progress.Render()
	if !strings.Contains(message, "修补模式 §f在线待命") {
		t.Fatalf("message does not show repair idle phase: %q", message)
	}
	if !strings.Contains(message, "当前机器人行为") || !strings.Contains(message, "进入修补模式") {
		t.Fatalf("message does not show repair-mode bot behavior: %q", message)
	}
	if strings.Contains(message, "等待聊天指令") {
		t.Fatalf("message still shows old chat-command behavior: %q", message)
	}
	if !strings.Contains(message, "区块 100/100") || !strings.Contains(message, "命令 2/2") || !strings.Contains(message, "NBT 1/1") {
		t.Fatalf("message does not keep import statistics: %q", message)
	}
	if strings.Contains(message, "平均方块速度 §a0/s") {
		t.Fatalf("message reset global average block speed: %q", message)
	}
	if !strings.Contains(message, "方块 §a0/s") || !strings.Contains(message, "区块 0/s") {
		t.Fatalf("message should reset instant block and chunk speed in repair idle mode: %q", message)
	}
}

func TestImportGameProgressTitleMute(t *testing.T) {
	recorder := &titleRecorder{}
	client := &clientType.Client{GameInterface: recorder}
	progress := NewImportGameProgress("建筑导入中")

	progress.SetTitleMuted(true)
	progress.SendToClientNow(client)
	if recorder.count != 0 {
		t.Fatalf("muted progress sent %d titles, want 0", recorder.count)
	}

	progress.SetTitleMuted(false)
	progress.SendToClientNow(client)
	if recorder.count != 1 {
		t.Fatalf("unmuted progress sent %d titles, want 1", recorder.count)
	}
}

func TestImportGameProgressKnownTotalsDoNotGrow(t *testing.T) {
	progress := NewImportGameProgress("导入中")
	progress.ResetImportCounters(1, 10)
	progress.SetCommandTotal(1)
	progress.SetNBTTotal(1)

	progress.AddCommandTotal(5)
	progress.AddNBTTotal(5)
	progress.MarkCommandDone()
	progress.MarkCommandDone()
	progress.MarkNBTDone()
	progress.MarkNBTDone()

	progress.mu.RLock()
	if progress.commandTotal != 1 {
		t.Fatalf("commandTotal = %d, want fixed known total 1", progress.commandTotal)
	}
	if progress.nbtTotal != 1 {
		t.Fatalf("nbtTotal = %d, want fixed known total 1", progress.nbtTotal)
	}
	progress.mu.RUnlock()
}

func TestImportGameProgressRendersChunkGroupProgress(t *testing.T) {
	progress := NewImportGameProgress("导入普通方块")
	progress.ResetImportCounters(0, 10)
	progress.SetChunkGroupProgress(3, 8)

	message := progress.Render()
	if !strings.Contains(message, "当前区块组 3/8") {
		t.Fatalf("message does not show chunk group progress: %q", message)
	}
	if !strings.Contains(message, "§d") {
		t.Fatalf("message does not use distinct chunk group color: %q", message)
	}
}
