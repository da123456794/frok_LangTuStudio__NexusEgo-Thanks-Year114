package game_interface

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
)

const MaxRetryChangeFlyingStates = 30

var StartFlyingInputData1 = []int{
	packet.InputFlagJumpDown,
	packet.InputFlagJumping,
	packet.InputFlagWantUp,
	packet.InputFlagStartFlying,
	packet.InputFlagJumpPressedRaw,
	packet.InputFlagJumpCurrentRaw,
}

var StartFlyingInputData2 = []int{
	packet.InputFlagJumpDown,
	packet.InputFlagJumping,
	packet.InputFlagWantUp,
	packet.InputFlagJumpCurrentRaw,
}

var StartFlyingInputData3 = []int{
	packet.InputFlagJumpReleasedRaw,
}

var StopFlyingInputData1 = []int{
	packet.InputFlagJumpDown,
	packet.InputFlagJumping,
	packet.InputFlagWantUp,
	packet.InputFlagStopFlying,
	packet.InputFlagJumpPressedRaw,
	packet.InputFlagJumpCurrentRaw,
}

var StopFlyingInputData2 = []int{
	packet.InputFlagJumpDown,
	packet.InputFlagJumping,
	packet.InputFlagWantUp,
	packet.InputFlagJumpCurrentRaw,
}

var StopFlyingInputData3 = []int{
	packet.InputFlagJumpReleasedRaw,
}

type Movement struct {
	api         *ResourcesWrapper
	querytarget *Querytarget
}

func NewMovement(api *ResourcesWrapper, querytarget *Querytarget) *Movement {
	return &Movement{api: api, querytarget: querytarget}
}

func (m *Movement) getBotPos() (pos [3]float32, err error) {
	info, err := m.querytarget.DoQuerytarget("@s")
	if err != nil {
		return [3]float32{}, fmt.Errorf("getBotPos: %v", err)
	}
	if len(info) == 0 {
		return [3]float32{}, fmt.Errorf("getBotPos: Failed to query the bot position")
	}
	return [3]float32{
		info[0].Position.X,
		info[0].Position.Y,
		info[0].Position.Z,
	}, nil
}

func (m *Movement) sendPlayerAuthInput(pos [3]float32, flags []int) error {
	inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	for _, flag := range flags {
		inputData.Set(flag)
	}
	err := m.api.WritePacket(&packet.PlayerAuthInput{
		Position:  pos,
		InputData: inputData,
	})
	if err != nil {
		return fmt.Errorf("sendPlayerAuthInput: %v", err)
	}
	time.Sleep(time.Second / 20)
	return nil
}

func (m *Movement) StartFlying() error {
	isFlying := new(atomic.Bool)
	isFlying.Store(false)

	uniqueID, err := m.api.PacketListener().ListenPacket(
		[]uint32{packet.IDUpdateAbilities},
		func(p packet.Packet, connCloseErr error) {
			if connCloseErr != nil {
				return
			}
			pk := p.(*packet.UpdateAbilities)
			if pk.AbilityData.EntityUniqueID == m.api.BotInfo.EntityUniqueID {
				flying := pk.AbilityData.Layers[0].Values&protocol.AbilityFlying != 0
				isFlying.Store(flying)
			}
		},
	)
	if err != nil {
		return fmt.Errorf("StartFlying: %v", err)
	}
	defer m.api.PacketListener().DestroyListener(uniqueID)

	pos, err := m.getBotPos()
	if err != nil {
		return fmt.Errorf("StartFlying: %v", err)
	}

	for i := 0; i < MaxRetryChangeFlyingStates; i++ {
		if err = m.sendPlayerAuthInput(pos, StartFlyingInputData1); err != nil {
			return fmt.Errorf("StartFlying: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StartFlyingInputData2); err != nil {
			return fmt.Errorf("StartFlying: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StartFlyingInputData3); err != nil {
			return fmt.Errorf("StartFlying: %v", err)
		}
		if isFlying.Load() {
			break
		}
	}
	if !isFlying.Load() {
		return fmt.Errorf("StartFlying: Failed to switch the flying state")
	}

	for i := 0; i < 5; i++ {
		if err = m.sendPlayerAuthInput(pos, nil); err != nil {
			return fmt.Errorf("StartFlying: %v", err)
		}
	}
	return nil
}

func (m *Movement) StopFlying() error {
	isFlying := new(atomic.Bool)
	isFlying.Store(true)

	uniqueID, err := m.api.PacketListener().ListenPacket(
		[]uint32{packet.IDUpdateAbilities},
		func(p packet.Packet, connCloseErr error) {
			if connCloseErr != nil {
				return
			}
			pk := p.(*packet.UpdateAbilities)
			if pk.AbilityData.EntityUniqueID == m.api.BotInfo.EntityUniqueID {
				flying := pk.AbilityData.Layers[0].Values&protocol.AbilityFlying != 0
				isFlying.Store(flying)
			}
		},
	)
	if err != nil {
		return fmt.Errorf("StopFlying: %v", err)
	}
	defer m.api.PacketListener().DestroyListener(uniqueID)

	pos, err := m.getBotPos()
	if err != nil {
		return fmt.Errorf("StopFlying: %v", err)
	}

	for i := 0; i < MaxRetryChangeFlyingStates; i++ {
		if err = m.sendPlayerAuthInput(pos, StopFlyingInputData1); err != nil {
			return fmt.Errorf("StopFlying: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StopFlyingInputData2); err != nil {
			return fmt.Errorf("StopFlying: %v", err)
		}
		if err = m.sendPlayerAuthInput(pos, StopFlyingInputData3); err != nil {
			return fmt.Errorf("StopFlying: %v", err)
		}
		if !isFlying.Load() {
			break
		}
	}
	if isFlying.Load() {
		return fmt.Errorf("StopFlying: Failed to switch the flying state")
	}

	for i := 0; i < 5; i++ {
		if err = m.sendPlayerAuthInput(pos, nil); err != nil {
			return fmt.Errorf("StopFlying: %v", err)
		}
	}
	return nil
}
