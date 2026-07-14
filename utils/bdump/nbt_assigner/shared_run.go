package NBTAssigner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nexus/defines"
	GameInterface "nexus/utils/api/game_interface"
	"nexus/utils/client"
	"nexus/utils/log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

var interfaceLock sync.Mutex

const visibleNBTProcessingLatency = time.Millisecond
const visibleNBTProcessingLogInterval = 300 * time.Millisecond
const flowersReadyCacheTTL = 2 * time.Second

var lastVisibleNBTProcessingLog atomic.Int64
var flowersReadyCacheUntil atomic.Int64
var flowersReadyCachePort atomic.Int64
var flowersReadyCheckLock sync.Mutex
var flowersHealthHTTPClient = http.Client{Timeout: 3 * time.Second}

type flowersHealthResponse struct {
	Alive bool `json:"alive"`
}

type PreparedBlockPlacement struct {
	BlockName     string
	BlockStates   string
	Position      [3]int32
	StructureName string
	CanFast       bool
	UseFacing     bool
	Facing        uint8
	CommandBlock  bool
	CommandData   types.CommandBlockData
}

var (
	GetFlowersPort                 func() int
	GetFlowersReady                func() bool
	PlaceNBTBlockAtPositionViaHTTP func(port int, blockName string, blockStates string, blockNBT map[string]interface{}, dimensionID uint8, x, y, z int) (map[string]interface{}, error)
	SendCommand                    func(string)
	DefaultDimensionID             uint8
)

func isFlowersServiceReady() (bool, bool) {
	if GetFlowersPort == nil {
		return false, false
	}

	port := GetFlowersPort()
	if port <= 0 {
		return false, false
	}

	url := fmt.Sprintf("http://localhost:%d/check_alive", port)

	start := time.Now()
	resp, err := flowersHealthHTTPClient.Get(url)
	visible := time.Since(start) >= visibleNBTProcessingLatency
	if err != nil {
		return false, visible
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return false, visible
	}

	if resp.StatusCode != http.StatusOK {
		return false, visible
	}
	var health flowersHealthResponse
	if err := json.Unmarshal(body, &health); err == nil {
		return health.Alive, visible
	}
	bodyStr := strings.TrimSpace(string(body))
	return bodyStr == "true" || strings.EqualFold(bodyStr, "ok"), visible
}

func PlaceBlockWithNBTData(
	intf client.GameInterface,
	blockInfo *types.Module,
	additionalData *BlockAdditionalData,
) error {
	if isCommandBlockModule(blockInfo) {
		return placeCommandBlockWithGameInterface(intf, blockInfo, additionalData)
	}
	prepared, err := PrepareBlockWithNBTData(blockInfo, additionalData)
	if err != nil {
		return err
	}
	return ApplyPreparedBlockWithNBTData(intf, prepared, additionalData)
}

func PrepareBlockWithNBTData(
	blockInfo *types.Module,
	additionalData *BlockAdditionalData,
) (*PreparedBlockPlacement, error) {
	if blockInfo == nil || blockInfo.Block == nil || blockInfo.Block.Name == nil {
		return nil, fmt.Errorf("PlaceBlockWithNBTData: invalid block payload")
	}
	blockName := *blockInfo.Block.Name
	blockStatesStr := resolveBlockStates(blockInfo, additionalData)
	position := [3]int32{int32(blockInfo.Point.X), int32(blockInfo.Point.Y), int32(blockInfo.Point.Z)}
	if isCommandBlockModule(blockInfo) {
		commandData := buildCommandBlockData(blockInfo, normalizeBlockName(blockName))
		if upgraded, _, err := UpgradeExecuteCommand(commandData.Command); err == nil {
			commandData.Command = upgraded
		}
		if commandData.Mode == 0 {
			commandData.Mode = modeFromBlockName(normalizeBlockName(blockName))
		}
		releasePreparedSourceBlock(blockInfo)
		return &PreparedBlockPlacement{
			BlockName:    blockName,
			BlockStates:  blockStatesStr,
			Position:     position,
			CommandBlock: true,
			CommandData:  commandData,
		}, nil
	}
	if len(blockInfo.NBTMap) == 0 {
		return &PreparedBlockPlacement{
			BlockName:   blockName,
			BlockStates: blockStatesStr,
			Position:    position,
			CanFast:     true,
		}, nil
	}
	if emptyShulker, facing := emptyShulkerPlacement(blockName, blockInfo.NBTMap); emptyShulker {
		prepared := &PreparedBlockPlacement{
			BlockName:   blockName,
			BlockStates: blockStatesStr,
			Position:    position,
		}
		if facing == 1 {
			prepared.CanFast = true
		} else {
			prepared.UseFacing = true
			prepared.Facing = facing
		}
		releasePreparedSourceBlock(blockInfo)
		return prepared, nil
	}

	checkAliveVisible, err := waitFlowersReady()
	if err != nil {
		return nil, err
	}
	shouldLogProcessing := checkAliveVisible
	var resp map[string]interface{}
	retryCount := 0
	const maxRetries = 3

	for retryCount < maxRetries {
		dimensionID := DefaultDimensionID
		if additionalData != nil && additionalData.DimensionID != 0 {
			dimensionID = additionalData.DimensionID
		}
		requestStart := time.Now()
		resp, err = PlaceNBTBlockAtPositionViaHTTP(GetFlowersPort(), blockName, blockStatesStr, blockInfo.NBTMap, dimensionID, blockInfo.Point.X, blockInfo.Point.Y, blockInfo.Point.Z)
		if time.Since(requestStart) >= visibleNBTProcessingLatency {
			shouldLogProcessing = true
		}
		if err != nil {
			flowersReadyCacheUntil.Store(0)
			retryCount++
			if retryCount >= maxRetries {
				return nil, fmt.Errorf("PlaceBlockWithNBTData: Failed to place the entity block named %v via HTTP at (%d,%d,%d) after %d attempts, error: %v", blockName, blockInfo.Point.X, blockInfo.Point.Y, blockInfo.Point.Z, maxRetries, err)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if success, ok := resp["success"].(bool); !ok || !success {
			flowersReadyCacheUntil.Store(0)
			retryCount++
			if retryCount >= maxRetries {
				return nil, fmt.Errorf("PlaceBlockWithNBTData: Failed to place the entity block named %v via HTTP at (%d,%d,%d), response: %v", blockName, blockInfo.Point.X, blockInfo.Point.Y, blockInfo.Point.Z, resp["error_info"])
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}

	if shouldLogProcessing && shouldEmitVisibleNBTLog() {
		log.Log.Info("正在处理NBT方块")
	}

	prepared := &PreparedBlockPlacement{
		BlockName:   blockName,
		BlockStates: blockStatesStr,
		Position:    position,
	}
	if canFast, ok := resp["can_fast"].(bool); ok && canFast {
		prepared.CanFast = true
		releasePreparedSourceBlock(blockInfo)
		return prepared, nil
	}

	structureName, ok := resp["structure_name"].(string)
	if !ok {
		return nil, fmt.Errorf("PlaceBlockWithNBTData: structure name not found in response")
	}
	prepared.StructureName = structureName
	releasePreparedSourceBlock(blockInfo)
	return prepared, nil
}

func ApplyPreparedBlockWithNBTData(
	intf client.GameInterface,
	prepared *PreparedBlockPlacement,
	additionalData *BlockAdditionalData,
) error {
	if prepared == nil || prepared.BlockName == "" {
		return fmt.Errorf("PlaceBlockWithNBTData: invalid prepared block payload")
	}
	if prepared.CommandBlock {
		return placePreparedCommandBlockWithGameInterface(intf, prepared)
	}
	if prepared.UseFacing {
		return placePreparedBlockWithFacing(intf, prepared)
	}
	if prepared.CanFast {
		return sendPlacementCommand(intf, buildSetBlockCommand(prepared.BlockName, prepared.BlockStates, prepared.Position), "PlaceBlockWithNBTData: Failed to send setblock command via AI")
	}

	loadStructureCmd := fmt.Sprintf("structure load \"%s\" %d %d %d", prepared.StructureName, prepared.Position[0], prepared.Position[1], prepared.Position[2])
	return sendPlacementCommand(intf, loadStructureCmd, "PlaceBlockWithNBTData: Failed to send structure load command via AI")
}

func waitFlowersReady() (bool, error) {
	if PlaceNBTBlockAtPositionViaHTTP == nil || GetFlowersPort == nil {
		return false, fmt.Errorf("PlaceBlockWithNBTData: RaaBel not initialized")
	}

	port := GetFlowersPort()
	if port <= 0 {
		return false, fmt.Errorf("PlaceBlockWithNBTData: invalid flowers port")
	}
	now := time.Now().UnixNano()
	if flowersReadyCachePort.Load() == int64(port) && now < flowersReadyCacheUntil.Load() {
		return false, nil
	}

	flowersReadyCheckLock.Lock()
	defer flowersReadyCheckLock.Unlock()

	now = time.Now().UnixNano()
	if flowersReadyCachePort.Load() == int64(port) && now < flowersReadyCacheUntil.Load() {
		return false, nil
	}

	deadline := time.Now().Add(30 * time.Second)
	visible := false

	for {
		ready, currentVisible := isFlowersServiceReady()
		if currentVisible {
			visible = true
		}
		if ready {
			flowersReadyCachePort.Store(int64(port))
			flowersReadyCacheUntil.Store(time.Now().Add(flowersReadyCacheTTL).UnixNano())
			return visible, nil
		}
		if time.Now().After(deadline) {
			return visible, fmt.Errorf("PlaceBlockWithNBTData: flowers service not ready within 30s")
		}
		sleepInterval := 100 * time.Millisecond
		time.Sleep(sleepInterval)
	}
}

func shouldEmitVisibleNBTLog() bool {
	now := time.Now().UnixNano()
	last := lastVisibleNBTProcessingLog.Load()
	if now-last < visibleNBTProcessingLogInterval.Nanoseconds() {
		return false
	}
	return lastVisibleNBTProcessingLog.CompareAndSwap(last, now)
}

func resolveBlockStates(blockInfo *types.Module, additionalData *BlockAdditionalData) string {
	if blockInfo == nil || blockInfo.Block == nil {
		return "[]"
	}
	blockStates := blockInfo.Block.BlockStates
	if blockStates == "" && additionalData != nil {
		blockStates = additionalData.BlockStates
	}
	if blockStates == "" {
		return "[]"
	}
	return blockStates
}

func buildSetBlockCommand(blockName, blockStates string, position [3]int32) string {
	if blockStates == "" || blockStates == "[]" {
		return fmt.Sprintf("setblock %d %d %d %s", position[0], position[1], position[2], blockName)
	}
	return fmt.Sprintf("setblock %d %d %d %s %s", position[0], position[1], position[2], blockName, blockStates)
}

func sendPlacementCommand(intf client.GameInterface, cmd string, errPrefix string) error {
	if gameInterface, ok := intf.(*GameInterface.GameInterface); ok {
		if err := gameInterface.SendAICommand(cmd, false); err != nil {
			return fmt.Errorf("%s: %v", errPrefix, err)
		}
		return nil
	}
	if SendCommand != nil {
		SendCommand(cmd)
	}
	return nil
}

func emptyShulkerPlacement(blockName string, nbtMap map[string]interface{}) (bool, uint8) {
	if !isShulkerBoxName(blockName) || !isEmptyContainerNBT(nbtMap) {
		return false, 1
	}
	facing, ok := readUint8(nbtMap["facing"])
	if !ok || facing > 5 {
		facing = 1
	}
	return true, facing
}

func isShulkerBoxName(blockName string) bool {
	return strings.Contains(normalizeBlockName(blockName), "shulker_box")
}

func isEmptyContainerNBT(nbtMap map[string]interface{}) bool {
	if len(nbtMap) == 0 {
		return true
	}
	if items, ok := nbtMap["Items"]; ok && !isEmptyNBTItemList(items) {
		return false
	}
	if customName, ok := nbtMap["CustomName"]; ok {
		name, valid := customName.(string)
		if !valid || name != "" {
			return false
		}
	}
	return true
}

func isEmptyNBTItemList(items interface{}) bool {
	if items == nil {
		return true
	}
	switch value := items.(type) {
	case []interface{}:
		return len(value) == 0
	case []map[string]interface{}:
		return len(value) == 0
	default:
		return false
	}
}

func placePreparedBlockWithFacing(intf client.GameInterface, prepared *PreparedBlockPlacement) error {
	gameInterface, ok := intf.(*GameInterface.GameInterface)
	if !ok {
		return fmt.Errorf("PlaceBlockWithNBTData: game interface unavailable for facing block")
	}
	if err := gameInterface.ReplaceItemInInventory(
		GameInterface.TargetMySelf,
		GameInterface.ItemGenerateLocation{
			Path: "slot.hotbar",
			Slot: 5,
		},
		types.ChestSlot{
			Name:   prepared.BlockName,
			Count:  1,
			Damage: 0,
		},
		"",
		true,
	); err != nil {
		return fmt.Errorf("PlaceBlockWithNBTData: Failed to prepare facing block item: %v", err)
	}
	if err := gameInterface.ChangeSelectedHotbarSlot(5); err != nil {
		return fmt.Errorf("PlaceBlockWithNBTData: Failed to select facing block item: %v", err)
	}
	if err := gameInterface.PlaceBlockWithFacing(prepared.Position, 5, prepared.Facing); err != nil {
		return fmt.Errorf("PlaceBlockWithNBTData: Failed to place facing block: %v", err)
	}
	return nil
}

func placePreparedCommandBlockWithGameInterface(intf client.GameInterface, prepared *PreparedBlockPlacement) error {
	gameInterface, ok := intf.(*GameInterface.GameInterface)
	if !ok {
		return fmt.Errorf("PlaceBlockWithNBTData: game interface unavailable for command block")
	}
	return gameInterface.WritePacket(&packet.CommandBlockUpdate{
		Block: true,
		Position: protocol.BlockPos{
			prepared.Position[0],
			prepared.Position[1],
			prepared.Position[2],
		},
		Mode:               prepared.CommandData.Mode,
		NeedsRedstone:      prepared.CommandData.NeedsRedstone,
		Conditional:        prepared.CommandData.Conditional,
		Command:            prepared.CommandData.Command,
		LastOutput:         prepared.CommandData.LastOutput,
		Name:               prepared.CommandData.CustomName,
		ShouldTrackOutput:  prepared.CommandData.TrackOutput,
		TickDelay:          prepared.CommandData.TickDelay,
		ExecuteOnFirstTick: prepared.CommandData.ExecuteOnFirstTick,
	})
}

func releasePreparedSourceBlock(blockInfo *types.Module) {
	if blockInfo == nil {
		return
	}
	blockInfo.NBTMap = nil
	blockInfo.NBTData = nil
	blockInfo.DebugNBTData = nil
}

func GenerateItemWithNBTData(
	intf client.GameInterface,
	singleItem ItemOrigin,
	additionalData *ItemAdditionalData,
) error {
	defer interfaceLock.Unlock()
	interfaceLock.Lock()
	newRequest := ItemPackage{
		Interface:      intf,
		Item:           GeneralItem{},
		AdditionalData: *additionalData,
	}
	err := newRequest.ParseItemFromNBT(singleItem)
	if err != nil {
		return fmt.Errorf("GenerateItemWithNBTData: Failed to generate the NBT item in hotbar %d, and the error log is %v", additionalData.HotBarSlot, err)
	}
	generateNBTItemMethod := GetGenerateItemMethod(&newRequest)
	if !additionalData.Decoded {
		err = generateNBTItemMethod.Decode()
		if err != nil {
			return fmt.Errorf("GenerateItemWithNBTData: Failed to generate the NBT item in hotbar %d, and the error log is %v", additionalData.HotBarSlot, err)
		}
	}
	err = generateNBTItemMethod.WriteData()
	if err != nil {
		return fmt.Errorf("GenerateItemWithNBTData: Failed to generate the NBT item in hotbar %d, and the error log is %v", additionalData.HotBarSlot, err)
	}
	return nil
}

func placeCommandBlockWithGameInterface(
	intf client.GameInterface,
	blockInfo *types.Module,
	additionalData *BlockAdditionalData,
) error {
	gameInterface, ok := intf.(*GameInterface.GameInterface)
	if !ok {
		return fmt.Errorf("PlaceBlockWithNBTData: game interface unavailable for command block")
	}
	if blockInfo == nil || blockInfo.Block == nil || blockInfo.Block.Name == nil {
		return fmt.Errorf("PlaceBlockWithNBTData: invalid command block payload")
	}

	pos := [3]int32{int32(blockInfo.Point.X), int32(blockInfo.Point.Y), int32(blockInfo.Point.Z)}
	blockName := normalizeBlockName(*blockInfo.Block.Name)
	blockStates := blockInfo.Block.BlockStates
	if blockStates == "" && additionalData != nil {
		blockStates = additionalData.BlockStates
	}
	if blockStates == "" {
		blockStates = "[]"
	}

	data := buildCommandBlockData(blockInfo, blockName)
	if upgraded, _, err := UpgradeExecuteCommand(data.Command); err == nil {
		data.Command = upgraded
	}
	if data.Mode == 0 {
		data.Mode = modeFromBlockName(blockName)
	}

	return gameInterface.WritePacket(&packet.CommandBlockUpdate{
		Block: true,
		Position: protocol.BlockPos{
			pos[0],
			pos[1],
			pos[2],
		},
		Mode:               data.Mode,
		NeedsRedstone:      data.NeedsRedstone,
		Conditional:        data.Conditional,
		Command:            data.Command,
		LastOutput:         data.LastOutput,
		Name:               data.CustomName,
		ShouldTrackOutput:  data.TrackOutput,
		TickDelay:          data.TickDelay,
		ExecuteOnFirstTick: data.ExecuteOnFirstTick,
	})
}

func buildCommandBlockData(blockInfo *types.Module, blockName string) types.CommandBlockData {
	if blockInfo != nil && blockInfo.CommandBlockData != nil {
		return *blockInfo.CommandBlockData
	}

	data := types.CommandBlockData{
		Mode:               modeFromBlockName(blockName),
		Command:            "",
		CustomName:         "",
		LastOutput:         "",
		TickDelay:          0,
		ExecuteOnFirstTick: true,
		TrackOutput:        true,
		Conditional:        false,
		NeedsRedstone:      false,
	}
	if blockInfo == nil || blockInfo.NBTMap == nil {
		return data
	}

	n := blockInfo.NBTMap
	if cmd, ok := n["Command"].(string); ok {
		data.Command = cmd
	}
	if name, ok := n["CustomName"].(string); ok {
		data.CustomName = name
	}
	if output, ok := n["LastOutput"].(string); ok {
		data.LastOutput = output
	}
	if tick, ok := readInt32(n["TickDelay"]); ok {
		data.TickDelay = tick
	}
	if v, ok := readBool(n["ExecuteOnFirstTick"]); ok {
		data.ExecuteOnFirstTick = v
	}
	if v, ok := readBool(n["TrackOutput"]); ok {
		data.TrackOutput = v
	}
	if v, ok := readBool(n["conditionalMode"]); ok {
		data.Conditional = v
	}
	if v, ok := readBool(n["auto"]); ok {
		data.NeedsRedstone = !v
	}
	return data
}

func readInt32(v interface{}) (int32, bool) {
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

func readBool(v interface{}) (bool, bool) {
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

func readUint8(v interface{}) (uint8, bool) {
	switch val := v.(type) {
	case uint8:
		return val, true
	case int8:
		if val < 0 {
			return 0, false
		}
		return uint8(val), true
	case int16:
		if val < 0 || val > 255 {
			return 0, false
		}
		return uint8(val), true
	case uint16:
		if val > 255 {
			return 0, false
		}
		return uint8(val), true
	case int32:
		if val < 0 || val > 255 {
			return 0, false
		}
		return uint8(val), true
	case uint32:
		if val > 255 {
			return 0, false
		}
		return uint8(val), true
	case int:
		if val < 0 || val > 255 {
			return 0, false
		}
		return uint8(val), true
	case int64:
		if val < 0 || val > 255 {
			return 0, false
		}
		return uint8(val), true
	case uint64:
		if val > 255 {
			return 0, false
		}
		return uint8(val), true
	case float64:
		if val < 0 || val > 255 || val != float64(uint8(val)) {
			return 0, false
		}
		return uint8(val), true
	default:
		return 0, false
	}
}

func modeFromBlockName(name string) uint32 {
	switch name {
	case "chain_command_block":
		return packet.CommandBlockChain
	case "repeating_command_block":
		return packet.CommandBlockRepeating
	default:
		return packet.CommandBlockImpulse
	}
}

func isCommandBlockModule(blockInfo *types.Module) bool {
	if blockInfo == nil || blockInfo.Block == nil || blockInfo.Block.Name == nil {
		return false
	}
	name := normalizeBlockName(*blockInfo.Block.Name)
	return name == "command_block" || name == "chain_command_block" || name == "repeating_command_block"
}

func normalizeBlockName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimPrefix(name, "minecraft:")
	return name
}

