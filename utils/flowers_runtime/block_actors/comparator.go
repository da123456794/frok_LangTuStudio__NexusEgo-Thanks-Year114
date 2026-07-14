package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

type Comparator struct {
	general.BlockActor `mapstructure:",squash"`
	OutputSignal       int32 `mapstructure:"OutputSignal"` // TAG_Int(4) = 0
}

// ID ...
func (*Comparator) ID() string {
	return IDComparator
}

func (c *Comparator) Marshal(io protocol.IO) {
	protocol.Single(io, &c.BlockActor)
	io.Varint32(&c.OutputSignal)
}
