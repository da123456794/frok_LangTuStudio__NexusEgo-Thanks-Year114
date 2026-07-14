package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

type CraftingTable struct {
	general.BlockActor `mapstructure:",squash"`
	PrivateUniqueID    string `mapstructure:"_uuid"` // TAG_String(8) = ""
}

// ID ...
func (*CraftingTable) ID() string {
	return IDCraftingTable
}

func (c *CraftingTable) Marshal(io protocol.IO) {
	protocol.Single(io, &c.BlockActor)
	io.String(&c.PrivateUniqueID)
}
