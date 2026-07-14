package mainfunction

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	types "nexus/defines"
	"nexus/utils/api/TerminalCommands/function"
	gameapi "nexus/utils/api/game_interface"
	ResourcesControl "nexus/utils/api/resources_control"
	"nexus/utils/client"
	"nexus/utils/log"
)

func broadcastRepairModeTips(env *client.Client) {
	if env == nil || env.GameInterface == nil {
		return
	}
	msg := "当前处于修补模式\n" +
		"请移动到缺失的区块范围内，发送'修补'\n" +
		"如需先清空再重导，发送'清理修补'\n" +
		"随后 NexusEgo[fixer] 将为您修复\n" +
		"!! 注意 !! 任何人都有该权限\n" +
		"发送完成' 或exit' 以退出修补模式"
	env.GameInterface.SendWSCommandWithResponse(
		fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(msg)),
		ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
	)
}

func handleRepairChat(env *client.Client, text *packet.Text) {
	if env == nil || env.RepairCtx == nil || !env.RepairCtx.Enabled || !env.RepairCtx.ChatEnabled {
		return
	}
	playerName := normalizePlayerName(env.ResolvePlayerName(text.XUID, text.PlatformChatID, text.SourceName))
	trimmed := strings.TrimSpace(text.Message)
	if trimmed == "" {
		return
	}
	if strings.EqualFold(trimmed, "exit") || trimmed == "完成" || trimmed == "完成。" {
		go cleanupRepairAndExit(env, "exit", playerName)
		return
	}

	normalized := strings.TrimSuffix(strings.TrimSuffix(trimmed, "。"), ".")
	lowerNormalized := strings.ToLower(normalized)
	if strings.Contains(lowerNormalized, "xingbai") {
		go func() {
			if env == nil || env.RepairCtx == nil || !env.RepairCtx.Enabled {
				return
			}
			chatEnabled := env.RepairCtx.ChatEnabled
			if !env.RepairCtx.TryLockRepair() {
				env.GameInterface.SendWSCommandWithResponse(
					fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote("已有修补任务正在执行")),
					ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
				)
				return
			}
			defer env.RepairCtx.UnlockRepair()

			origin := env.RepairCtx.OuterOrigin
			snapshot := env.RepairCtx.SettingsSnapshot
			task := types.Task{
				ImportNBT:          !snapshot.No_NBT,
				ImportCommandBlock: env.RepairCtx.ImportCommandBlock,
				UseFill:            env.RepairCtx.UseFill,
				RegionSize:         env.RepairCtx.RegionSize,
				AutoPlaceDenyBlock: env.RepairCtx.AutoPlaceDenyBlock,
				AutoPlaceBorder:    env.RepairCtx.AutoPlaceBorder,
			}

			var restore func()
			snapshotCopy := snapshot
			if env.Cdump_Setting != nil {
				original := *env.Cdump_Setting
				env.Cdump_Setting = &snapshotCopy
				restore = func() { *env.Cdump_Setting = original }
			} else {
				env.Cdump_Setting = &snapshotCopy
				restore = func() { env.Cdump_Setting = nil }
			}
			defer restore()

			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote("检测到 xingbai，开始重新导入")),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)

			progress := function.RepairGameProgress(env)
			if progress != nil {
				progress.ResetImportCounters(0, 0)
				progress.SetPhase("§c§l修补模式 §e重新导入")
				progress.SetBuilderStatus("整图重导")
				progress.SendToClientNow(env)
			}
			ok := function.Cdump_import(env, env.RepairCtx.FilePath, origin.X, origin.Y, origin.Z, false, 0, nil, "", task, nil, progress)
			if env != nil && env.RepairCtx != nil && env.RepairCtx.Enabled {
				env.RepairCtx.ChatEnabled = chatEnabled
			}
			if ok {
				broadcastRepairModeTips(env)
				function.ShowRepairModeIdle(env, progress, task.ImportNBT)
			}
		}()
		return
	}

	sizeFlag := 1
	clearBefore := false
	if normalized == "修补" {
		sizeFlag = 0
	} else if normalized == "清理修补" {
		sizeFlag = 0
		clearBefore = true
	} else if strings.HasPrefix(normalized, "清理修补") {
		clearBefore = true
		tail := strings.TrimSpace(strings.TrimPrefix(normalized, "清理修补"))
		switch tail {
		case "3":
			sizeFlag = 1
		case "5":
			sizeFlag = 2
		case "":
			sizeFlag = 0
		default:
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote("清理修补指令只支持 清理修补、清理修补3、清理修补5")),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)
			return
		}
	} else if strings.HasPrefix(normalized, "修补") {
		tail := strings.TrimSpace(strings.TrimPrefix(normalized, "修补"))
		switch tail {
		case "3":
			sizeFlag = 1
		case "5":
			sizeFlag = 2
		case "":
			sizeFlag = 0
		default:
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote("修补指令只支持 修补、修补3、修补5")),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)
			return
		}
	} else {
		return
	}
	if playerName == "" {
		return
	}

	go func() {
		gameAPI, ok := env.GameInterface.(*gameapi.GameInterface)
		if !ok {
			log.Log.Info("修补模式: 无法获取游戏接口实现", log.Log.ArgsFromMap(map[string]any{
				"player": playerName,
			}))
			return
		}

		resp := env.GameInterface.SendWSCommandWithResponse(
			fmt.Sprintf(`querytarget "%s"`, playerName),
			ResourcesControl.CommandRequestOptions{TimeOut: 5 * time.Second},
		)
		var pos [3]float32
		var parsed bool
		if resp.Error == nil && resp.Respond != nil {
			if targets, err := gameAPI.ParseTargetQueryingInfo(*resp.Respond); err == nil && len(targets) > 0 {
				pos = targets[0].Position
				parsed = true
			}
		}

		teleported := false
		if !parsed {
			tpResp := env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tp @s "%s"`, playerName),
				ResourcesControl.CommandRequestOptions{TimeOut: 5 * time.Second},
			)
			if tpResp.Error != nil || tpResp.Respond == nil {
				log.Log.Info("修补模式: 查询玩家坐标失败", log.Log.ArgsFromMap(map[string]any{
					"player": playerName,
					"error":  tpResp.Error,
				}))
				return
			}
			coord, err := gameAPI.ParseTeleportCoordinates(*tpResp.Respond)
			if err != nil {
				log.Log.Info("修补模式: 解析玩家坐标失败", log.Log.ArgsFromMap(map[string]any{
					"player": playerName,
					"error":  err,
				}))
				return
			}
			pos = coord
			teleported = true
		}

		playerPos := types.Position{
			X: int(math.Floor(float64(pos[0]))),
			Y: int(math.Floor(float64(pos[1]))),
			Z: int(math.Floor(float64(pos[2]))),
		}

		bounds := env.RepairCtx.Bounds
		if playerPos.X < bounds.MinX || playerPos.X > bounds.MaxX || playerPos.Z < bounds.MinZ || playerPos.Z > bounds.MaxZ {
			msg := fmt.Sprintf("玩家 %s 不在修补范围内", playerName)
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(msg)),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)
			log.Log.Info("修补模式: 玩家不在修补范围内", log.Log.ArgsFromMap(map[string]any{
				"player": playerName,
				"pos":    fmt.Sprintf("%d,%d,%d", playerPos.X, playerPos.Y, playerPos.Z),
			}))
			return
		}

		if !teleported {
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tp @s "%s"`, playerName),
				ResourcesControl.CommandRequestOptions{TimeOut: 3 * time.Second},
			)
		}

		chunkX := playerPos.X >> 4
		chunkZ := playerPos.Z >> 4
		env.RepairCtx.StartRepairWorkerOnce(func(jobs <-chan client.RepairJob) {
			repairQueueWorker(env, jobs)
		})
		queue := env.RepairCtx.EnsureRepairQueue()
		pending := len(queue)
		radius := sizeFlag
		added := 0
		chunkList := [][2]int{}
		for dx := -radius; dx <= radius; dx++ {
			for dz := -radius; dz <= radius; dz++ {
				targetChunkX := chunkX + dx
				targetChunkZ := chunkZ + dz
				chunkMinX := targetChunkX << 4
				chunkMaxX := chunkMinX + 15
				chunkMinZ := targetChunkZ << 4
				chunkMaxZ := chunkMinZ + 15
				bounds := env.RepairCtx.Bounds
				if chunkMaxX < bounds.MinX || chunkMinX > bounds.MaxX || chunkMaxZ < bounds.MinZ || chunkMinZ > bounds.MaxZ {
					continue
				}
				chunkList = append(chunkList, [2]int{targetChunkX, targetChunkZ})
				added++
			}
		}
		if added == 0 {
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote("修补范围超出已导入区域，未添加任何任务")),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)
			return
		}

		if !env.RepairCtx.TryBeginRepairJob(chunkList) {
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote("已有相同或进行中的修补任务，请等待当前修补完成")),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)
			return
		}

		queue <- client.RepairJob{
			ChunkX:              chunkX,
			ChunkZ:              chunkZ,
			Chunks:              chunkList,
			PlayerName:          playerName,
			PlayerPos:           playerPos,
			ClearBeforeReimport: clearBefore,
		}
		origin := env.RepairCtx.OuterOrigin
		chunkIndexX := int(math.Floor(float64(chunkX*16-origin.X)/16.0)) + 1
		chunkIndexZ := int(math.Floor(float64(chunkZ*16-origin.Z)/16.0)) + 1
		var queuedMsg string
		if pending > 0 {
			queuedMsg = fmt.Sprintf("区块 (%d, %d) 修补请求已排队，前方还有 %d 个，本次共加入 %d 个区块", chunkIndexX, chunkIndexZ, pending, added)
		} else {
			queuedMsg = fmt.Sprintf("区块 (%d, %d) 修补请求已排队，本次共加入 %d 个区块", chunkIndexX, chunkIndexZ, added)
		}
		env.GameInterface.SendWSCommandWithResponse(
			fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(queuedMsg)),
			ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
		)
	}()
}

func repairQueueWorker(env *client.Client, jobs <-chan client.RepairJob) {
	for job := range jobs {
		processRepairJob(env, job)
	}
}

func processRepairJob(env *client.Client, job client.RepairJob) {
	defer env.RepairCtx.FinishRepairJob(job.Chunks)
	if !env.RepairCtx.TryLockRepair() {
		log.Log.Warn("修补模式: 修补任务启动时发现已有运行中的任务", log.Log.ArgsFromMap(map[string]any{
			"player": job.PlayerName,
			"chunk":  fmt.Sprintf("%d,%d", job.ChunkX, job.ChunkZ),
		}))
		return
	}
	defer env.RepairCtx.UnlockRepair()
	origin := env.RepairCtx.OuterOrigin
	chunkIndexX := int(math.Floor(float64(job.ChunkX*16-origin.X)/16.0)) + 1
	chunkIndexZ := int(math.Floor(float64(job.ChunkZ*16-origin.Z)/16.0)) + 1
	totalChunks := len(job.Chunks)
	if totalChunks == 0 {
		totalChunks = 1
	}
	repairProgress, stopRepairProgress := beginRepairJobProgress(env, job, totalChunks)
	defer stopRepairProgress()

	log.Log.Info(fmt.Sprintf("修补模式: 开始修补区块 (%d, %d) 玩家: %s 共 %d 个", chunkIndexX, chunkIndexZ, job.PlayerName, totalChunks))
	startMsg := fmt.Sprintf("开始区块 (%d, %d) 玩家: %s (共 %d 个区块)", chunkIndexX, chunkIndexZ, job.PlayerName, totalChunks)
	if job.ClearBeforeReimport {
		startMsg = fmt.Sprintf("开始清理并修补区块 (%d, %d) 玩家: %s (共 %d 个区块)", chunkIndexX, chunkIndexZ, job.PlayerName, totalChunks)
	}
	env.GameInterface.SendWSCommandWithResponse(
		fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(startMsg)),
		ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
	)

	if job.ClearBeforeReimport && len(job.Chunks) > 0 {
		if repairProgress != nil {
			repairProgress.SetPhase("§c§l修补模式 §e清理区块")
			repairProgress.SendToClientNow(env)
		}
		if err := function.ClearChunksForRepair(env, env.RepairCtx, job.Chunks); err != nil {
			log.Log.Info("修补模式: 清理修补区块失败", log.Log.ArgsFromMap(map[string]any{
				"player": job.PlayerName,
				"chunk":  fmt.Sprintf("%d,%d", job.ChunkX, job.ChunkZ),
				"error":  err.Error(),
			}))
			errMsg := fmt.Sprintf("区块 (%d, %d) 清理失败: %s", chunkIndexX, chunkIndexZ, err.Error())
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(errMsg)),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)
			showRepairFailureProgress(env, repairProgress, "清理失败")
			return
		}
	}

	if len(job.Chunks) > 0 {
		if err := function.ReimportChunkSet(env, env.RepairCtx, job.Chunks, job.PlayerPos, repairProgress); err != nil {
			log.Log.Info("修补模式: 重新导入区块失败", log.Log.ArgsFromMap(map[string]any{
				"player": job.PlayerName,
				"chunk":  fmt.Sprintf("%d,%d", job.ChunkX, job.ChunkZ),
				"error":  err.Error(),
			}))
			errMsg := fmt.Sprintf("区块 (%d, %d) 修补失败: %s", chunkIndexX, chunkIndexZ, err.Error())
			env.GameInterface.SendWSCommandWithResponse(
				fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(errMsg)),
				ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
			)
			showRepairFailureProgress(env, repairProgress, "修补失败")
			return
		}
	} else if err := function.ReimportChunkAt(env, env.RepairCtx, job.ChunkX, job.ChunkZ, job.PlayerPos, repairProgress); err != nil {
		log.Log.Info("修补模式: 重新导入区块失败", log.Log.ArgsFromMap(map[string]any{
			"player": job.PlayerName,
			"chunk":  fmt.Sprintf("%d,%d", job.ChunkX, job.ChunkZ),
			"error":  err.Error(),
		}))
		errMsg := fmt.Sprintf("区块 (%d, %d) 修补失败: %s", chunkIndexX, chunkIndexZ, err.Error())
		env.GameInterface.SendWSCommandWithResponse(
			fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(errMsg)),
			ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
		)
		showRepairFailureProgress(env, repairProgress, "修补失败")
		return
	}

	log.Log.Info(fmt.Sprintf("修补模式: 结束修补区块 (%d, %d) 玩家: %s 共 %d 个", chunkIndexX, chunkIndexZ, job.PlayerName, totalChunks))
	finishMsg := fmt.Sprintf("区块 (%d, %d) 修补完成 (共 %d 个区块)", chunkIndexX, chunkIndexZ, totalChunks)
	env.GameInterface.SendWSCommandWithResponse(
		fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(finishMsg)),
		ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
	)
	function.ShowRepairModeIdle(env, repairProgress, repairModeNeedsNBT(env))
}

func beginRepairJobProgress(env *client.Client, job client.RepairJob, totalChunks int) (*function.ImportGameProgress, func()) {
	progress := function.RepairGameProgress(env)
	stop := func() {}
	if progress == nil {
		progress = function.NewImportGameProgress("§c§l修补模式")
		stop = progress.Start(func() *client.Client { return env })
	}

	progress.ResetImportCounters(0, totalChunks)
	phase := "§c§l修补模式 §e正在修补"
	if job.ClearBeforeReimport {
		phase = "§c§l修补模式 §e清理并修补"
	}
	progress.SetPhase(phase)
	progress.SetBuilderStatus(fmt.Sprintf("玩家 %s", job.PlayerName))
	if repairModeNeedsNBT(env) {
		progress.SetNBTStatus("在线待命")
	} else {
		progress.SetNBTStatus("未启用")
	}
	progress.SendToClientNow(env)
	return progress, stop
}

func showRepairFailureProgress(env *client.Client, progress *function.ImportGameProgress, phase string) {
	if progress == nil {
		return
	}
	progress.SetPhase("§c§l修补模式 §c" + phase)
	progress.MarkFinished()
	progress.SendToClientNow(env)
}

func repairModeNeedsNBT(env *client.Client) bool {
	if env == nil || env.RepairCtx == nil {
		return false
	}
	return !env.RepairCtx.SettingsSnapshot.No_NBT
}

func cleanupRepairAndExit(env *client.Client, reason string, player string) {
	if env != nil && env.GameInterface != nil {
		env.GameInterface.SendWSCommandWithResponse(
			`tp @s ~~~`,
			ResourcesControl.CommandRequestOptions{TimeOut: 2 * time.Second},
		)
	}

	_ = os.Remove("task")
	log.Log.Info("修补模式: 已退出", log.Log.ArgsFromMap(map[string]any{
		"reason": reason,
		"player": player,
	}))
	os.Exit(0)
}

func normalizePlayerName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}

	var cleaned []rune
	skip := false
	for _, r := range raw {
		if skip {
			skip = false
			continue
		}
		if r == '§' {
			skip = true
			continue
		}
		cleaned = append(cleaned, r)
	}
	str := strings.TrimSpace(string(cleaned))

	if right := strings.LastIndex(str, ">"); right >= 0 && right+1 < len(str) {
		name := strings.TrimSpace(str[right+1:])
		if name != "" {
			return name
		}
	}
	left := strings.LastIndex(str, "<")
	right := strings.LastIndex(str, ">")
	if left >= 0 && right > left+1 {
		name := strings.TrimSpace(str[left+1 : right])
		if name != "" {
			return name
		}
	}
	return str
}

