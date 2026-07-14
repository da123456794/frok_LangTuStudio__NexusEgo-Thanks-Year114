package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

type EndGateway struct {
	general.BlockActor `mapstructure:",squash"`
	Age                int32   `mapstructure:"Age"`        // TAG_Int(4) = 0
	ExitPortal         []int32 `mapstructure:"ExitPortal"` // TAG_List[TAG_Int] (9[4])
}

// ID ...
func (*EndGateway) ID() string {
	return IDEndGateway
}

func (b *EndGateway) Marshal(io protocol.IO) {
	protocol.Single(io, &b.BlockActor)
	io.Varint32(&b.Age)
	protocol.FuncSliceOfLen(io, 3, &b.ExitPortal, io.Varint32)
}
