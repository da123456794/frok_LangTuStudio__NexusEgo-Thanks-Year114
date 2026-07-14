package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

type Bed struct {
	general.BlockActor `mapstructure:",squash"`
	Color              byte `mapstructure:"color"` // TAG_Byte(1) = 0
}

// ID ...
func (*Bed) ID() string {
	return IDBed
}

func (b *Bed) Marshal(io protocol.IO) {
	protocol.Single(io, &b.BlockActor)
	protocol.NBTInt(&b.Color, io.Varuint32)
}
