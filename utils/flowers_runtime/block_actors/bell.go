package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

type Bell struct {
	general.BlockActor `mapstructure:",squash"`
	Direction          int32 `mapstructure:"Direction"` // TAG_Int(4) = 255
	Ringing            byte  `mapstructure:"Ringing"`   // TAG_Byte(1) = 0
	Ticks              int32 `mapstructure:"Ticks"`     // TAG_Int(4) = 18
}

// ID ...
func (*Bell) ID() string {
	return IDBell
}

func (b *Bell) Marshal(io protocol.IO) {
	protocol.Single(io, &b.BlockActor)
	io.Uint8(&b.Ringing)
	io.Varint32(&b.Ticks)
	io.Varint32(&b.Direction)
}
