package mainfunction

import (
	ResourcesControl "nexus/utils/api/resources_control"
	"nexus/utils/client"
	"nexus/utils/log"
	"nexus/utils/mirror/io/assembler"
	"nexus/utils/mirror/io/global"
	"nexus/utils/mirror/io/lru"
	"strings"
	"time"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

func EnterWorkerThread(conn client.Conn, env *client.Client, breaker chan struct{}) {
	env.ChunkAssembler = assembler.NewAssembler(assembler.REQUEST_AGGRESSIVE, time.Second*5)
	chunkAssembler := env.ChunkAssembler
	chunkAssembler.CreateRequestScheduler(func(pk *packet.SubChunkRequest) {
		conn.WritePacket(pk)
	})
	go func() {
		env.IsOP_loop.Lock()
		is_op := env.IsOP
		env.IsOP_loop.Unlock()
		for !is_op {
			return_data := env.GameInterface.SendWSCommandWithResponse("tellraw @a {\"rawtext\":[{\"text\":\"已给予机器人op,开始任务\"}]}", ResourcesControl.CommandRequestOptions{TimeOut: time.Second * 10})
			if return_data.Respond != nil && len(return_data.Respond.OutputMessages) != 0 {
				if return_data.Respond.OutputMessages[0].Success {
					env.IsOP_loop.Lock()
					env.IsOP = true
					env.IsOP_loop.Unlock()
					return
				}
			} else if return_data.Error == nil {
				env.IsOP_loop.Lock()
				env.IsOP = true
				env.IsOP_loop.Unlock()
				return
			}
			time.Sleep(time.Second * 1)
		}
	}()
	// 假装实现了插件
	for {
		if breaker != nil {
			select {
			case <-breaker:
				return
			default:
			}
		}

		var (
			pk    packet.Packet
			err   error
			cache <-chan packet.Packet
		)
		if env.CachedPacket != nil {
			if ch, ok := env.CachedPacket.(<-chan packet.Packet); ok {
				cache = ch
			}
		}
		if cache != nil && len(cache) > 0 {
			pk = <-cache
		} else if pk, err = conn.ReadPacket(); err != nil {
			if err != nil {
				env.LastImportError = err.Error()
			}
			if err == nil || err.Error() == "conn dead: <nil>" || err.Error() == "<nil>" {
				env.Conn = nil
				return
			}
			if strings.Contains(err.Error(), "use of closed network connection") {
				env.Conn = nil
				log.Log.Error("读取数据包失败，工作线程退出", log.Log.ArgsFromMap(map[string]any{
					"error": err.Error(),
				}))
				return
			}
			countSplit := strings.SplitN(err.Error(), " ", 1)
			switch countSplit[0] {
			case "%disconnect.kicked":
				log.Log.Error("租赁服已断开连接", log.Log.ArgsFromMap(
					map[string]any{
						"原因": "机器人被管理员踢出了游戏",
					}))
			case "%disconnect.kicked.reason":
				log.Log.Error("租赁服已断开连接", log.Log.ArgsFromMap(
					map[string]any{
						"原因":   "机器人被管理员踢出了游戏",
						"踢出原因": countSplit[1],
					}))
			case "netease.report.kick":
				log.Log.Error("租赁服已断开连接", log.Log.ArgsFromMap(
					map[string]any{
						"原因":   "你已被网易踢出服务器",
						"具体原因": countSplit[1],
					}))
			case "netease.report.kick.hint":
				log.Log.Error("租赁服已断开连接", log.Log.ArgsFromMap(
					map[string]any{
						"原因": "机器人异地登录此租赁服,机器人已退出(或机器人上次未正确退出)",
					}))
			}
			env.Conn = nil
			// 记录错误并优雅退出工作线程，避免整个程序崩溃
			log.Log.Error("读取数据包失败，工作线程退出", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			return

		}

		if _, isUnknown := pk.(*packet.Unknown); isUnknown {
			continue
		}

		env.ResourcesUpdater.(func(*packet.Packet))(&pk)
		// fmt.Println(pk.ID())
		switch p := pk.(type) {
		case *packet.PyRpc:
			// pterm.Info.Println("packet.PyRpc", p)
			onPyRpc(p, conn, env)
		case *packet.Text:
			handleRepairChat(env, p)
		case *packet.PlayerList:
			env.UpdatePlayerNameCache(p.Entries, p.ActionType == packet.PlayerListActionRemove)
		case *packet.ActorEvent:
			if p.EventType == packet.ActorEventDeath && p.EntityRuntimeID == conn.GameData().EntityRuntimeID {
				conn.WritePacket(&packet.PlayerAction{
					EntityRuntimeID: conn.GameData().EntityRuntimeID,
					ActionType:      protocol.PlayerActionRespawn,
				})
			}
		case *packet.SubChunk:
			if env.SkipSubChunkCheck || chunkAssembler == nil {
				continue
			}
			chunkData := chunkAssembler.OnNewSubChunk(p)
			if chunkData != nil {
				env.ChunkFeeder.(*global.ChunkFeeder).OnNewChunk(chunkData)
				env.LRUMemoryChunkCacher.(*lru.LRUMemoryChunkCacher).Write(chunkData)
			}
		case *packet.NetworkChunkPublisherUpdate:
			if env.SkipSubChunkCheck || chunkAssembler == nil {
				continue
			}
			chunkAssembler.CancelQueueByPublishUpdate(p)
			// pterm.Info.Println("packet.NetworkChunkPublisherUpdate", p)
			// missHash := []uint64{}
			// hitHash := []uint64{}
			// for i := uint64(0); i < 64; i++ {
			// 	missHash = append(missHash, uint64(10184224921554030005+i))
			// 	hitHash = append(hitHash, uint64(6346766690299427078-i))
			// }
			// conn.WritePacket(&packet.ClientCacheBlobStatus{
			// 	MissHashes: missHash,
			// 	HitHashes:  hitHash,
			// })
		case *packet.Respawn:
			conn.WritePacket(&packet.Respawn{
				EntityRuntimeID: conn.GameData().EntityRuntimeID,
				Position:        p.Position,
				State:           packet.RespawnStateClientReadyToSpawn,
			})
			conn.WritePacket(&packet.PlayerAction{
				EntityRuntimeID: conn.GameData().EntityRuntimeID,
				ActionType:      protocol.PlayerActionRespawn,
			})
		case *packet.AvailableCommands:
			env.AvailableCommands = p
			//fmt.Println(write)

			//fmt.Println(string(t))
			// EDotCS 私有逻辑
			// 玩家消息事件,会经过菜单插件的处理才给其他插件运行
		case *packet.LevelChunk:
			if env.SkipSubChunkCheck || chunkAssembler == nil {
				continue
			}
			if exist := chunkAssembler.AddPendingTask(p); !exist {
				requests := chunkAssembler.GenRequestFromLevelChunk(p)
				chunkAssembler.ScheduleRequest(requests)
			}

		}
	}
}

