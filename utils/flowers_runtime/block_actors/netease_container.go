package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

// Netease container ..
type NeteaseContainer struct {
	general.BlockActor `mapstructure:",squash"`
	Size               int32 `mapstructure:"Size"`  // TAG_Int(3)
	Items              []any `mapstructure:"Items"` // TAG_List[TAG_Compound] (9[10])
}

// ID ...
func (*NeteaseContainer) ID() string {
	return IDNeteaseContainer
}

func (n *NeteaseContainer) Marshal(io protocol.IO) {
	protocol.Single(io, &n.BlockActor)
	io.Varint32(&n.Size)
	protocol.NBTSlice(io, &n.Items, func(t *[]protocol.ItemWithSlot) { io.ItemList(t) })
}
