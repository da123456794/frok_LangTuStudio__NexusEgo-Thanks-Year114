package game_interface

import (
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	"github.com/go-gl/mathgl/mgl32"
)

type BotAction struct {
	api *GameInterface
}

// NewBotAction 基于 api 创建并返回一个新的 BotAction
func NewBotAction(api *GameInterface) *BotAction {
	return &BotAction{api: api}
}

func (b *BotAction) MoveToPosition(position mgl32.Vec3, facing mgl32.Vec3) (err error) {
	inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	inputData.Set(packet.InputFlagPersistSneak)
	inputData.Set(packet.InputFlagPaddlingRight)
	for j := 0; j < 10; j++ {
		err = b.api.Resources().WritePacket(&packet.PlayerAuthInput{
			Pitch:            facing[0],
			Yaw:              facing[1],
			HeadYaw:          facing[2],
			InputData:        inputData,
			InputMode:        packet.InputModeTouch,
			PlayMode:         packet.PlayModeScreen,
			InteractionModel: packet.InteractionModelCrosshair,
			Position:         position,
		})
		if err != nil {
			return
		}
	}
	return
}
