package function

import (
	"context"
	"fmt"
	"os"

	"nexus/constants"
	types "nexus/defines"
	NBTAssigner "nexus/utils/bdump/nbt_assigner"
	"nexus/utils/client"
	"nexus/utils/file"

	wsdefine "github.com/Yeah114/WaterStructure/define"
	wsstructure "github.com/Yeah114/WaterStructure/structure"
	"golang.org/x/time/rate"
)

// ClearChunksForRepair 在修补前先清理指定区块所覆盖的修补范围。
func ClearChunksForRepair(cl *client.Client, repairCtx *client.RepairContext, chunks [][2]int) error {
	if repairCtx == nil || !repairCtx.Enabled {
		return fmt.Errorf("repair mode is not enabled")
	}
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks to clear")
	}

	bounds := repairCtx.Bounds
	limiter := newCommandRateLimiter(resolveImportCommandSpeed(cl))
	ctx := context.Background()

	for _, c := range chunks {
		chunkMinX := c[0] << 4
		chunkMaxX := chunkMinX + 15
		chunkMinZ := c[1] << 4
		chunkMaxZ := chunkMinZ + 15
		minX, maxX := chunkMinX, chunkMaxX
		minZ, maxZ := chunkMinZ, chunkMaxZ
		if minX < bounds.MinX {
			minX = bounds.MinX
		}
		if maxX > bounds.MaxX {
			maxX = bounds.MaxX
		}
		if minZ < bounds.MinZ {
			minZ = bounds.MinZ
		}
		if maxZ > bounds.MaxZ {
			maxZ = bounds.MaxZ
		}
		if minX > maxX || minZ > maxZ {
			continue
		}
		clearChunkBoxForRepair(cl, minX, bounds.MinY, minZ, maxX, bounds.MaxY, maxZ, limiter, ctx)
	}
	return nil
}

func clearChunkBoxForRepair(cl *client.Client, minX, minY, minZ, maxX, maxY, maxZ int, limiter *rate.Limiter, ctx context.Context) {
	centerX := (minX + maxX) / 2
	centerZ := (minZ + maxZ) / 2
	teleportSafe(cl, centerX, maxY+2, centerZ, limiter, ctx)
	runClearCommandsLayered(cl, minX, minY, minZ, maxX, maxY, maxZ, limiter, ctx, nil)
}

// ReimportChunkAt reimports a single chunk while repair mode is active.
func ReimportChunkAt(cl *client.Client, repairCtx *client.RepairContext, chunkX, chunkZ int, returnPos types.Position, gameProgress *ImportGameProgress) error {
	return ReimportChunkSet(cl, repairCtx, [][2]int{{chunkX, chunkZ}}, returnPos, gameProgress)
}

// ReimportChunkSet reimports a set of target chunks using the same mcworld pipeline as normal import,
// so repair mode and full import share identical NBT handling.
func ReimportChunkSet(cl *client.Client, repairCtx *client.RepairContext, chunks [][2]int, returnPos types.Position, gameProgress *ImportGameProgress) error {
	return reimportChunkSet(cl, repairCtx, chunks, returnPos, gameProgress, false)
}

func reimportChunkSetQuiet(cl *client.Client, repairCtx *client.RepairContext, chunks [][2]int, returnPos types.Position, gameProgress *ImportGameProgress) error {
	return reimportChunkSet(cl, repairCtx, chunks, returnPos, gameProgress, true)
}

func reimportChunkSet(cl *client.Client, repairCtx *client.RepairContext, chunks [][2]int, returnPos types.Position, gameProgress *ImportGameProgress, quietProgress bool) error {
	if repairCtx == nil || !repairCtx.Enabled {
		return fmt.Errorf("repair mode is not enabled")
	}
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks to repair")
	}
	if quietProgress {
		gameProgress = nil
	}
	if gameProgress == nil && !quietProgress {
		gameProgress = RepairGameProgress(cl)
	}
	if !file.Is_File(repairCtx.FilePath) {
		return fmt.Errorf("repair source file missing: %s", repairCtx.FilePath)
	}

	snapshot := repairCtx.SettingsSnapshot
	if snapshot.Speed <= 0 {
		snapshot.Speed = constants.DefaultImportSpeed
	}
	var restore func()
	snapshotCopy := snapshot
	if cl.Cdump_Setting != nil {
		original := *cl.Cdump_Setting
		cl.Cdump_Setting = &snapshotCopy
		restore = func() { *cl.Cdump_Setting = original }
	} else {
		cl.Cdump_Setting = &snapshotCopy
		restore = func() { cl.Cdump_Setting = nil }
	}
	defer restore()

	limiter, _ := newImportLimiters(cl, types.Task{CommandDataSpeed: repairCtx.CommandDataSpeed})
	ctx := context.Background()

	prevNBTDim := NBTAssigner.DefaultDimensionID
	NBTAssigner.DefaultDimensionID = uint8(cl.DimensionID)
	defer func() {
		NBTAssigner.DefaultDimensionID = prevNBTDim
	}()

	fileHandle, err := os.Open(repairCtx.FilePath)
	if err != nil {
		return fmt.Errorf("open repair structure: %w", err)
	}
	defer fileHandle.Close()
	reader, err := wsstructure.StructureFromFile(fileHandle)
	if err != nil {
		return fmt.Errorf("open repair structure: %w", err)
	}
	defer reader.Close()

	origin := repairCtx.Origin
	realStartChunkX := int32(origin.X - floorMod(origin.X, 16))
	realStartChunkY := int32(origin.Y - floorMod(origin.Y, 16))
	realStartChunkZ := int32(origin.Z - floorMod(origin.Z, 16))
	reader.SetOffsetPos(wsdefine.Offset{int32(origin.X) - realStartChunkX, int32(origin.Y) - realStartChunkY, int32(origin.Z) - realStartChunkZ})

	if gameProgress != nil {
		gameProgress.ResetImportCounters(0, len(chunks))
		gameProgress.SetPhase("§c§l修补模式 §e正在修补")
		gameProgress.SetBuilderStatus("正在修补")
		if !snapshot.No_NBT || repairCtx.ImportCommandBlock {
			gameProgress.SetNBTStatus("在线待命")
		} else {
			gameProgress.SetNBTStatus("未启用")
		}
		gameProgress.SendToClientNow(cl)
	}

	builder := &importBuilder{
		client:             cl,
		reader:             reader,
		task:               types.Task{ClearArea: snapshot.Clear_Building, ClearDrops: snapshot.Clear_Drops, AutoPlaceDenyBlock: repairCtx.AutoPlaceDenyBlock, AutoPlaceBorder: repairCtx.AutoPlaceBorder, ImportNBT: !snapshot.No_NBT, ImportCommandBlock: repairCtx.ImportCommandBlock, DefaultSignWax: snapshot.Close_Sign},
		ctx:                ctx,
		limiter:            limiter,
		gameProgress:       gameProgress,
		buildStartPos:      origin,
		originalStartPos:   repairCtx.OuterOrigin,
		chunkGroupSide:     1,
		verifyAfterChunk:   defaultVerifyAfterChunk,
		verifyChunkLevel:   verifyChunkLevelNone,
		tickingAreaRecords: make(map[wsdefine.ChunkPos][]string),
	}

	for _, target := range chunks {
		chunkX, chunkZ := target[0], target[1]
		sourceChunkX := int32(chunkX) - realStartChunkX/16
		sourceChunkZ := int32(chunkZ) - realStartChunkZ/16
		chunkMap, err := reader.GetChunks([]wsdefine.ChunkPos{{sourceChunkX, sourceChunkZ}})
		if err != nil {
			return fmt.Errorf("read repair chunk: %w", err)
		}
		c := chunkMap[wsdefine.ChunkPos{sourceChunkX, sourceChunkZ}]
		if c == nil {
			continue
		}
		if err := builder.buildChunk(c, int32(chunkX*16), realStartChunkY, int32(chunkZ*16)); err != nil {
			return fmt.Errorf("repair chunk rebuild: %w", err)
		}
		if gameProgress != nil {
			gameProgress.SetChunkProgress(gameProgressSnapshotChunkCurrent(gameProgress)+1, len(chunks))
			gameProgress.SendToClient(cl)
		}
	}

	teleportSafe(cl, returnPos.X, returnPos.Y, returnPos.Z, limiter, ctx)
	return nil
}

func gameProgressSnapshotChunkCurrent(progress *ImportGameProgress) int {
	if progress == nil {
		return 0
	}
	progress.mu.Lock()
	defer progress.mu.Unlock()
	return progress.chunkCurrent
}
