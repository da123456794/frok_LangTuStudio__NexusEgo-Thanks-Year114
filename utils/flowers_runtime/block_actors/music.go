package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

type Music struct {
	general.BlockActor `mapstructure:",squash"`
	Note               byte `mapstructure:"note"` // TAG_Byte(1) = 0
}

// ID ...
func (*Music) ID() string {
	return IDMusic
}

func (n *Music) Marshal(io protocol.IO) {
	protocol.Single(io, &n.BlockActor)
	protocol.NBTInt(&n.Note, io.Varuint32)
}
