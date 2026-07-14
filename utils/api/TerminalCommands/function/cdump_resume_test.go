package function

import (
	"context"
	"errors"
	"testing"

	types "nexus/defines"
	clientType "nexus/utils/client"

	newlogin "github.com/LangTuStudio/Conbit/minecraft/protocol/login"
	oldpacket "github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	newgamedata "github.com/LangTuStudio/Conbit/minecraft_neo/game_data"
)

type closedImportTestConn struct{}

func (closedImportTestConn) GameData() newgamedata.GameData        { return newgamedata.GameData{} }
func (closedImportTestConn) IdentityData() newlogin.IdentityData   { return newlogin.IdentityData{} }
func (closedImportTestConn) WritePacket(oldpacket.Packet) error    { return nil }
func (closedImportTestConn) ReadPacket() (oldpacket.Packet, error) { return nil, nil }
func (closedImportTestConn) Close() error                          { return nil }
func (closedImportTestConn) Closed() bool                          { return true }
func (closedImportTestConn) CloseError() error                     { return errors.New("connection closed") }

func TestBuildImportChunkPlanUsesExactResumeProgress(t *testing.T) {
	bounds := mcworldBounds{
		minX: 0,
		maxX: 63,
		minY: 0,
		maxY: 0,
		minZ: 0,
		maxZ: 63,
	}
	plan, err := buildImportChunkPlan(bounds, 2, 50, 7, 16)
	if err != nil {
		t.Fatalf("buildImportChunkPlan returned error: %v", err)
	}
	if plan.totalChunks != 16 {
		t.Fatalf("totalChunks = %d, want 16", plan.totalChunks)
	}
	if plan.skipRegions != 1 {
		t.Fatalf("skipRegions = %d, want 1", plan.skipRegions)
	}
	if plan.skipChunks != 4 {
		t.Fatalf("skipChunks = %d, want 4", plan.skipChunks)
	}
}

func TestProcessRegionAbortsWhenConnectionAlreadyClosed(t *testing.T) {
	regionKey := [2]int{4, 3}
	manager := NewChunkRegionManager(5)
	manager.Regions[regionKey] = &RegionData{
		Blocks:     map[int][]*types.Module{},
		MinY:       0,
		MaxY:       65,
		SeenChunks: map[[2]int]bool{{4, 3}: true},
	}
	message := "机器人无响应倒计时触发，剩余 53.15 秒，主动中断当前导入以等待接入点重启后断点续导"
	client := &clientType.Client{
		Conn:            closedImportTestConn{},
		LastImportError: message,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = withImportAbort(ctx, cancel)

	if manager.ProcessRegion(client, regionKey, nil, ctx, nil) {
		t.Fatal("ProcessRegion returned success for a closed import connection")
	}
	if !importContextDone(ctx) {
		t.Fatal("ProcessRegion did not cancel import context for a closed connection")
	}
	if manager.Regions[regionKey].Processed {
		t.Fatal("ProcessRegion marked region as processed after connection closed")
	}
	if client.LastImportError != message {
		t.Fatalf("LastImportError was overwritten: %q", client.LastImportError)
	}
}
