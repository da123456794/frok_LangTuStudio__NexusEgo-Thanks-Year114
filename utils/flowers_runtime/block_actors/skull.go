package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

// 头颅
type Skull struct {
	general.BlockActor `mapstructure:",squash"`
	DoingAnimation     byte    `mapstructure:"DoingAnimation"` // * TAG_Byte(1) = 0
	MouthTickCount     int32   `mapstructure:"MouthTickCount"` // TAG_Int(4) = 0
	Rotation           float32 `mapstructure:"Rotation"`       // TAG_Float(6) = 0
	SkullType          byte    `mapstructure:"SkullType"`      // TAG_Byte(1) = 0
}

// ID ...
func (*Skull) ID() string {
	return IDSkull
}

func (s *Skull) Marshal(io protocol.IO) {
	protocol.Single(io, &s.BlockActor)
	protocol.NBTInt(&s.SkullType, io.Varuint16)
	io.Float32(&s.Rotation)
	io.Uint8(&s.DoingAnimation)
	io.Varint32(&s.MouthTickCount)
}
