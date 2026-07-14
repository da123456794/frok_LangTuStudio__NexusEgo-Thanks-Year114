package bundle

import (
	"fmt"
	"strings"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/modules/blob_hash"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"

	// "github.com/LangTuStudio/Conbit/Conbit/modules/block/placer"
	"sync"
	"time"

	"github.com/LangTuStudio/Conbit/Conbit/modules/area_request"
	"github.com/LangTuStudio/Conbit/Conbit/modules/bot_action"
	"github.com/LangTuStudio/Conbit/Conbit/modules/info_sender"
	"github.com/LangTuStudio/Conbit/Conbit/modules/player_interact"
	"github.com/LangTuStudio/Conbit/internal/termlog"
)

func init() {
	if false {
		func(omega Conbit.MicroOmega) {}(&MicroOmega{})
	}
}

type MicroOmega struct {
	Conbit.ReactCore
	Conbit.InteractCore
	Conbit.InfoSender
	Conbit.CmdSender
	Conbit.MicroUQHolder
	// Conbit.BlockPlacer
	Conbit.PlayerInteract
	Conbit.LowLevelAreaRequester
	Conbit.CommandHelper
	Conbit.BotAction
	Conbit.BotActionHighLevel
	// 添加BlobHashHolder字段
	blobHashHolder  *blob_hash.BlobHashHolder
	deferredActions []struct {
		cb   func()
		name string
	}
	mu sync.Mutex
}

func microInfof(format string, args ...any) {
	termlog.Infof("%s", strings.TrimRight(fmt.Sprintf(format, args...), "\r\n"))
}

func NewMicroOmega(
	interactCore Conbit.InteractCore,
	reactCore Conbit.UnStartedReactCore,
	microUQHolder Conbit.MicroUQHolder,
	cmdSender Conbit.CmdSender,
	node defines.Node,
	isAccessPoint bool,
	preferAIQueryTarget bool,
) Conbit.UnReadyMicroOmega {
	infoSender := info_sender.NewInfoSender(interactCore, cmdSender, microUQHolder.GetBotBasicInfo())
	playerInteract := player_interact.NewPlayerInteract(reactCore, microUQHolder.GetPlayersInfo(), microUQHolder.GetBotBasicInfo(), cmdSender, infoSender, interactCore)
	// asyncNbtBlockPlacer := placer.NewAsyncNbtBlockPlacer(reactCore, cmdSender, interactCore)
	areaRequester := area_request.NewAreaRequester(interactCore, reactCore, microUQHolder, microUQHolder)
	cmdHelper := bot_action.NewCommandHelper(cmdSender, microUQHolder)
	var botAction Conbit.BotAction
	if isAccessPoint {
		botAction = bot_action.NewAccessPointBotActionWithPersistData(microUQHolder, interactCore, reactCore, cmdSender, node)
	} else {
		botAction = bot_action.NewEndPointBotAction(node, microUQHolder, interactCore)
	}

	botActionHighLevel := bot_action.NewBotActionHighLevel(microUQHolder, interactCore, reactCore, cmdSender, cmdHelper, areaRequester, botAction, node)

	// 初始化BlobHashHolder
	var blobHashHolder *blob_hash.BlobHashHolder
	if isAccessPoint {
		// 接入点模式，作为服务端
		blobHashHolder = blob_hash.NewBlobHashHolder(2048, true, microUQHolder, interactCore, reactCore, node)
	} else {
		// 端点模式，作为客户端
		blobHashHolder = blob_hash.NewBlobHashHolder(2048, false, microUQHolder, interactCore, reactCore, node)
	}

	omega := &MicroOmega{
		reactCore,
		interactCore,
		infoSender,
		cmdSender,
		microUQHolder,
		// asyncNbtBlockPlacer,
		playerInteract,
		areaRequester,
		cmdHelper,
		botAction,
		botActionHighLevel,
		blobHashHolder, // 添加blobHashHolder字段
		make([]struct {
			cb   func()
			name string
		}, 0),
		sync.Mutex{},
	}

	if isAccessPoint {
		omega.PostponeActionsAfterChallengePassed("request tick update schedule", func() {
			go func() {
				for {
					clientTick := 0
					if tick, found := omega.GetMicroUQHolder().GetExtendInfo().GetCurrentTick(); found {
						clientTick = int(tick)
					}
					omega.GetGameControl().SendPacket(&packet.TickSync{
						ClientRequestTimestamp: int64(clientTick),
					})
					time.Sleep(time.Second * 10)
				}
			}()
		})
		omega.PostponeActionsAfterChallengePassed("auto respawn", func() {
			omega.GetGameListener().SetTypedPacketCallBack(packet.IDRespawn, func(p packet.Packet) {
				pkt := p.(*packet.Respawn)
				if pkt.State == packet.RespawnStateSearchingForSpawn {
					omega.SendPacket(&packet.Respawn{
						State:           packet.RespawnStateClientReadyToSpawn,
						EntityRuntimeID: omega.GetBotRuntimeID(),
					})
					omega.SendPacket(&packet.PlayerAction{
						EntityRuntimeID: omega.GetBotRuntimeID(),
						ActionType:      protocol.PlayerActionRespawn,
						BlockFace:       -1,
					})
				}
			}, true)
		})
		omega.PostponeActionsAfterChallengePassed("force reset dimension and pos", func() {
			e := &Conbit.PosAndDimensionInfo{}
			if bot_action.RefreshPosAndDimensionInfo(e, omega, omega.GetBotRuntimeID(), preferAIQueryTarget) == nil {
				// fmt.Println(e)
				omega.MicroUQHolder.UpdateFromPacket(&packet.ChangeDimension{
					Dimension: int32(e.Dimension),
					Position:  e.HeadPosPrecise,
				})
			}
		})
	}

	if !isAccessPoint {
		omega.PostponeActionsAfterChallengePassed("check bot command status each 10s", func() {
			go func() {
				for {
					ret, err := omega.SendWebSocketCmdNeedResponse("errcmd").SetTimeout(time.Minute).BlockGetResult()
					if err != nil || ret == nil {
						panic("for some reason, end point cannot communicate with server, reload")
					} else {
						// fmt.Println(ret)
					}
					time.Sleep(time.Second * 10)
				}
			}()
		})
	}

	reactCore.Start()
	return omega
}

func (o *MicroOmega) GetGameControl() Conbit.GameCtrl {
	return o
}

func (o *MicroOmega) GetReactCore() Conbit.ReactCore {
	return o
}

func (o *MicroOmega) GetGameListener() Conbit.PacketDispatcher {
	return o
}

func (o *MicroOmega) GetPlayerInteract() Conbit.PlayerInteract {
	return o
}

func (o *MicroOmega) GetMicroUQHolder() Conbit.MicroUQHolder {
	return o
}

func (o *MicroOmega) GetLowLevelAreaRequester() Conbit.LowLevelAreaRequester {
	return o
}

func (o *MicroOmega) GetBotAction() Conbit.BotActionComplex {
	return o
}

func (o *MicroOmega) NotifyChallengePassed() {
	for _, action := range o.deferredActions {
		microInfof(i18n.T(i18n.S_starting_post_challenge_actions), action.name)
		action.cb()
	}
}

func (o *MicroOmega) GetBlobHashHolder() interface{} {
	return o.blobHashHolder
}

func (o *MicroOmega) PostponeActionsAfterChallengePassed(name string, action func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.deferredActions = append(o.deferredActions, struct {
		cb   func()
		name string
	}{action, name})
}
