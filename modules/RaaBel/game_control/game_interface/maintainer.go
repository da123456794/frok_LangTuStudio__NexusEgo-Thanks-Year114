package game_interface

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
)

var DefaultMaintainer Maintainer = new(BaseMaintainer)

type Maintainer interface {
	TouchMaintainer(api *GameInterface) error
	HandlePacket(pk packet.Packet, api *GameInterface)
	PacketToListen() map[uint32]bool
}

type BaseMaintainer struct{}

func (b *BaseMaintainer) doMaintain(api *GameInterface) error {
	err := api.Commands().SendSettingsCommand("gamemode 1", true)
	if err != nil {
		return fmt.Errorf("doMaintain: %v", err)
	}
	err = api.Commands().AwaitChangesGeneral()
	if err != nil {
		return fmt.Errorf("doMaintain: %v", err)
	}
	err = api.Movement().StopFlying()
	if err != nil {
		return fmt.Errorf("doMaintain: %v", err)
	}
	err = api.Movement().StartFlying()
	if err != nil {
		return fmt.Errorf("doMaintain: %v", err)
	}
	return nil
}

func (b *BaseMaintainer) TouchMaintainer(api *GameInterface) error {
	err := b.doMaintain(api)
	if err != nil {
		return fmt.Errorf("TouchMaintainer: %v", err)
	}
	return nil
}

func (b *BaseMaintainer) HandlePacket(pk packet.Packet, api *GameInterface) {
	switch p := pk.(type) {
	case *packet.SetPlayerGameType:
		if p.GameType != packet.GameTypeCreative {
			_ = b.doMaintain(api)
		}
	case *packet.UpdatePlayerGameType:
		if p.PlayerUniqueID == api.GetBotInfo().EntityUniqueID && p.GameType != packet.GameTypeCreative {
			_ = b.doMaintain(api)
		}
	case *packet.Respawn:
		if p.State == packet.RespawnStateReadyToSpawn {
			_ = b.doMaintain(api)
		}
	}
}

func (b *BaseMaintainer) PacketToListen() map[uint32]bool {
	return map[uint32]bool{
		packet.IDSetPlayerGameType:    true,
		packet.IDUpdatePlayerGameType: true,
		packet.IDRespawn:              true,
	}
}
