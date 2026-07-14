package conbit

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"nexus/utils/client"

	coreConbit "github.com/LangTuStudio/Conbit/Conbit"
	accesshelper "github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/access_helper"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	newlogin "github.com/LangTuStudio/Conbit/minecraft/protocol/login"
	newpacket "github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	newgamedata "github.com/LangTuStudio/Conbit/minecraft_neo/game_data"
	corenodes "github.com/LangTuStudio/Conbit/nodes"
	coredefines "github.com/LangTuStudio/Conbit/nodes/defines"
)

var ErrTransferHandledInternally = errors.New("current access point handles transfer internally")

type AccessPoint struct {
	omega    coreConbit.MicroOmega
	node     coredefines.Node
	shieldID int32

	packetQueue chan newpacket.Packet
	closed      chan error
	closeOnce   sync.Once
	closeMu     sync.RWMutex
	closeErr    error
	isClosed    bool
}

func NewAccessPoint(serverCode, serverPassword, token, authServer string) (*AccessPoint, error) {
	impactOption := accesshelper.DefaultImpactOption()
	impactOption.ServerCode = serverCode
	impactOption.ServerPassword = serverPassword
	impactOption.UserToken = token
	impactOption.AuthServer = authServer

	options := accesshelper.DefaultOptions()
	options.ImpactOption = impactOption
	options.MakeBotCreative = false
	options.DisableCommandBlock = false
	options.ReasonWithPrivilegeStuff = true

	ctx := context.Background()
	node := corenodes.NewGroup("Conbit", corenodes.NewLocalNode(ctx), false)
	omega, err := accesshelper.ImpactServer(ctx, node, options)
	if err != nil {
		return nil, err
	}

	ap := &AccessPoint{
		omega:       omega,
		node:        node,
		packetQueue: make(chan newpacket.Packet, 8192),
		closed:      make(chan error, 1),
	}
	node.ListenMessage("packets", func(msg coredefines.Values) {
		if len(msg) < 2 {
			return
		}
		raw := append([]byte(nil), msg[1]...)
		pk, err := client.DecodeNewPacketToOld(raw, ap.shieldID)
		if err != nil {
			return
		}
		select {
		case ap.packetQueue <- pk:
		default:
		}
	}, false)
	go func() {
		err := <-omega.WaitClosed()
		ap.close(err)
	}()
	resp, err := node.CallWithResponse("get-shield-id", coredefines.Empty).BlockGetResult()
	if err == nil {
		if shieldID, convErr := resp.ToInt32(); convErr == nil {
			ap.shieldID = shieldID
		}
	}
	return ap, nil
}

func (a *AccessPoint) close(err error) {
	a.closeOnce.Do(func() {
		a.closeMu.Lock()
		a.closeErr = err
		a.isClosed = true
		a.closeMu.Unlock()
		a.closed <- err
		close(a.closed)
	})
}

func (a *AccessPoint) TransferData(content string) (string, error) {
	return "", ErrTransferHandledInternally
}

func (a *AccessPoint) TransferCheckNum(data string) (string, error) {
	return "", ErrTransferHandledInternally
}

func (a *AccessPoint) GetBotName() string {
	return a.IdentityData().DisplayName
}

func (a *AccessPoint) CommandSender() coreConbit.CmdSender {
	if a == nil || a.omega == nil {
		return nil
	}
	return a.omega.GetGameControl()
}

func (a *AccessPoint) GameData() newgamedata.GameData {
	botInfo := a.omega.GetMicroUQHolder().GetBotBasicInfo()
	ext := a.omega.GetMicroUQHolder().GetExtendInfo()
	position, _ := ext.GetBotPosition()
	currentTick, _ := ext.GetCurrentTick()

	gameRulesMap, _ := ext.GetGameRules()
	gameRules := make([]protocol.GameRule, 0, len(gameRulesMap))
	for name, rule := range gameRulesMap {
		if rule == nil {
			continue
		}
		gameRules = append(gameRules, protocol.GameRule{
			Name:                  name,
			CanBeModifiedByPlayer: rule.CanBeModifiedByPlayer,
			Value:                 rule.Value,
		})
	}

	dimension := int32(0)
	if dim, found := ext.GetBotDimension(); found {
		dimension = int32(dim)
	}

	return newgamedata.GameData{
		EntityUniqueID:  botInfo.GetBotUniqueID(),
		EntityRuntimeID: botInfo.GetBotRuntimeID(),
		PlayerPosition:  position,
		Dimension:       dimension,
		Time:            currentTick,
		GameRules:       gameRules,
	}
}

func (a *AccessPoint) IdentityData() newlogin.IdentityData {
	botInfo := a.omega.GetMicroUQHolder().GetBotBasicInfo()
	return newlogin.IdentityData{
		Identity:    botInfo.GetBotIdentity(),
		DisplayName: botInfo.GetBotName(),
		Uid:         botInfo.GetBotUID(),
	}
}

func (a *AccessPoint) WritePacket(pk newpacket.Packet) error {
	if a == nil {
		return fmt.Errorf("connection unavailable")
	}
	if a.Closed() {
		if err := a.CloseError(); err != nil {
			return err
		}
		return fmt.Errorf("connection closed")
	}
	raw, err := client.EncodeOldPacketToNewRaw(pk, a.shieldID)
	if err != nil {
		return err
	}
	_, err = a.node.CallWithResponse("send-packet-bytes", coredefines.FromBytes(raw)).BlockGetResult()
	return err
}

func (a *AccessPoint) ReadPacket() (newpacket.Packet, error) {
	if a == nil {
		return nil, fmt.Errorf("connection unavailable")
	}
	select {
	case pk := <-a.packetQueue:
		return pk, nil
	case err := <-a.closed:
		if err == nil {
			err = fmt.Errorf("connection closed")
		}
		return nil, err
	}
}

func (a *AccessPoint) Close() error {
	if a.omega != nil {
		a.omega.Close()
	}
	a.close(nil)
	return nil
}

// Omega 暴露底层 Conbit MicroOmega，给需要走结构请求/区域请求等高级 API 的模块使用。
func (a *AccessPoint) Omega() coreConbit.MicroOmega {
	if a == nil {
		return nil
	}
	return a.omega
}

var _ client.Conn = (*AccessPoint)(nil)

func (a *AccessPoint) Closed() bool {
	if a == nil {
		return true
	}
	a.closeMu.RLock()
	defer a.closeMu.RUnlock()
	return a.isClosed
}

func (a *AccessPoint) CloseError() error {
	if a == nil {
		return fmt.Errorf("connection unavailable")
	}
	a.closeMu.RLock()
	defer a.closeMu.RUnlock()
	return a.closeErr
}
