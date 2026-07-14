package NBTAssigner

import (
	"testing"
	"time"

	types "nexus/defines"
)

func TestPrepareBlockWithNBTDataEmptyShulkerFacingSkipsHTTP(t *testing.T) {
	oldGetFlowersPort := GetFlowersPort
	oldPlaceNBTBlockAtPositionViaHTTP := PlaceNBTBlockAtPositionViaHTTP
	GetFlowersPort = nil
	PlaceNBTBlockAtPositionViaHTTP = nil
	defer func() {
		GetFlowersPort = oldGetFlowersPort
		PlaceNBTBlockAtPositionViaHTTP = oldPlaceNBTBlockAtPositionViaHTTP
	}()

	module := testBlockModule("green_shulker_box", map[string]interface{}{
		"Items":      []interface{}{},
		"CustomName": "",
		"facing":     byte(0),
	})

	prepared, err := PrepareBlockWithNBTData(module, nil)
	if err != nil {
		t.Fatalf("PrepareBlockWithNBTData() error = %v", err)
	}
	if !prepared.UseFacing || prepared.Facing != 0 {
		t.Fatalf("prepared facing placement = (%v, %d), want (true, 0)", prepared.UseFacing, prepared.Facing)
	}
	if prepared.CanFast {
		t.Fatalf("prepared.CanFast = true, want false")
	}
	if module.NBTMap != nil {
		t.Fatalf("module.NBTMap was not released")
	}
}

func TestPrepareBlockWithNBTDataDefaultFacingEmptyShulkerCanFast(t *testing.T) {
	module := testBlockModule("minecraft:white_shulker_box", map[string]interface{}{
		"Items":      []interface{}{},
		"CustomName": "",
		"facing":     byte(1),
	})

	prepared, err := PrepareBlockWithNBTData(module, nil)
	if err != nil {
		t.Fatalf("PrepareBlockWithNBTData() error = %v", err)
	}
	if !prepared.CanFast {
		t.Fatalf("prepared.CanFast = false, want true")
	}
	if prepared.UseFacing {
		t.Fatalf("prepared.UseFacing = true, want false")
	}
}

func TestPrepareBlockWithNBTDataEmptyShulkerIgnoresUnsupportedNBTFields(t *testing.T) {
	oldGetFlowersPort := GetFlowersPort
	oldPlaceNBTBlockAtPositionViaHTTP := PlaceNBTBlockAtPositionViaHTTP
	GetFlowersPort = nil
	PlaceNBTBlockAtPositionViaHTTP = nil
	defer func() {
		GetFlowersPort = oldGetFlowersPort
		PlaceNBTBlockAtPositionViaHTTP = oldPlaceNBTBlockAtPositionViaHTTP
	}()

	module := testBlockModule("light_gray_shulker_box", map[string]interface{}{
		"Findable":      byte(0),
		"Items":         []interface{}{},
		"LootTable":     "chests/simple_dungeon",
		"LootTableSeed": int32(0),
		"facing":        byte(0),
		"id":            "ShulkerBox",
		"isMovable":     byte(1),
		"x":             int32(176),
		"y":             int32(17),
		"z":             int32(486),
	})

	prepared, err := PrepareBlockWithNBTData(module, nil)
	if err != nil {
		t.Fatalf("PrepareBlockWithNBTData() error = %v", err)
	}
	if !prepared.UseFacing || prepared.Facing != 0 {
		t.Fatalf("prepared facing placement = (%v, %d), want (true, 0)", prepared.UseFacing, prepared.Facing)
	}
}

func TestPrepareBlockWithNBTDataShulkerWithItemsUsesHTTP(t *testing.T) {
	oldGetFlowersPort := GetFlowersPort
	oldPlaceNBTBlockAtPositionViaHTTP := PlaceNBTBlockAtPositionViaHTTP
	oldCachePort := flowersReadyCachePort.Load()
	oldCacheUntil := flowersReadyCacheUntil.Load()
	defer func() {
		GetFlowersPort = oldGetFlowersPort
		PlaceNBTBlockAtPositionViaHTTP = oldPlaceNBTBlockAtPositionViaHTTP
		flowersReadyCachePort.Store(oldCachePort)
		flowersReadyCacheUntil.Store(oldCacheUntil)
	}()

	const port = 19132
	calls := 0
	GetFlowersPort = func() int { return port }
	flowersReadyCachePort.Store(port)
	flowersReadyCacheUntil.Store(time.Now().Add(time.Minute).UnixNano())
	PlaceNBTBlockAtPositionViaHTTP = func(_ int, _ string, _ string, _ map[string]interface{}, _ uint8, _, _, _ int) (map[string]interface{}, error) {
		calls++
		return map[string]interface{}{"success": true, "can_fast": true}, nil
	}

	module := testBlockModule("green_shulker_box", map[string]interface{}{
		"Items": []interface{}{map[string]interface{}{"Slot": byte(0)}},
	})

	prepared, err := PrepareBlockWithNBTData(module, nil)
	if err != nil {
		t.Fatalf("PrepareBlockWithNBTData() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("HTTP placement calls = %d, want 1", calls)
	}
	if !prepared.CanFast {
		t.Fatalf("prepared.CanFast = false, want true")
	}
}

func TestPrepareBlockWithNBTDataShulkerWithCustomNameUsesHTTP(t *testing.T) {
	oldGetFlowersPort := GetFlowersPort
	oldPlaceNBTBlockAtPositionViaHTTP := PlaceNBTBlockAtPositionViaHTTP
	oldCachePort := flowersReadyCachePort.Load()
	oldCacheUntil := flowersReadyCacheUntil.Load()
	defer func() {
		GetFlowersPort = oldGetFlowersPort
		PlaceNBTBlockAtPositionViaHTTP = oldPlaceNBTBlockAtPositionViaHTTP
		flowersReadyCachePort.Store(oldCachePort)
		flowersReadyCacheUntil.Store(oldCacheUntil)
	}()

	const port = 19133
	calls := 0
	GetFlowersPort = func() int { return port }
	flowersReadyCachePort.Store(port)
	flowersReadyCacheUntil.Store(time.Now().Add(time.Minute).UnixNano())
	PlaceNBTBlockAtPositionViaHTTP = func(_ int, _ string, _ string, _ map[string]interface{}, _ uint8, _, _, _ int) (map[string]interface{}, error) {
		calls++
		return map[string]interface{}{"success": true, "can_fast": true}, nil
	}

	module := testBlockModule("green_shulker_box", map[string]interface{}{
		"Items":      []interface{}{},
		"CustomName": "Storage",
	})

	_, err := PrepareBlockWithNBTData(module, nil)
	if err != nil {
		t.Fatalf("PrepareBlockWithNBTData() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("HTTP placement calls = %d, want 1", calls)
	}
}

func testBlockModule(name string, nbtMap map[string]interface{}) *types.Module {
	return &types.Module{
		Block: &types.Block{
			Name:        &name,
			BlockStates: "[]",
		},
		NBTMap: nbtMap,
		Point: types.Position{
			X: 1,
			Y: 2,
			Z: 3,
		},
	}
}
