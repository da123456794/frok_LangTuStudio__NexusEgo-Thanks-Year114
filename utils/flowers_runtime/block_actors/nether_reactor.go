package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

type NetherReactor struct {
	general.BlockActor `mapstructure:",squash"`
	HasFinished        byte  `mapstructure:"HasFinished"`   // TAG_Byte(1) = 0
	IsInitialized      byte  `mapstructure:"IsInitialized"` // TAG_Byte(1) = 0
	Progress           int16 `mapstructure:"Progress"`      // TAG_Short(3) = 0
}

// ID ...
func (*NetherReactor) ID() string {
	return IDNetherReactor
}

func (n *NetherReactor) Marshal(io protocol.IO) {
	protocol.Single(io, &n.BlockActor)
	io.Uint8(&n.IsInitialized)
	io.Int16(&n.Progress)
	io.Uint8(&n.HasFinished)
}
