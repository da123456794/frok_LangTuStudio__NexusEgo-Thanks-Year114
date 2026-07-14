package block_actors

import (
	"nexus/utils/flowers_runtime/block_actors/fields"
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

// 化合物创建器
type ChemistryTable struct {
	general.BlockActor         `mapstructure:",squash"`
	*fields.ChemistryTableItem `mapstructure:",omitempty"`
}

// ID ...
func (*ChemistryTable) ID() string {
	return IDChemistryTable
}

func (c *ChemistryTable) Marshal(io protocol.IO) {
	f := func() *fields.ChemistryTableItem {
		if c.ChemistryTableItem == nil {
			c.ChemistryTableItem = new(fields.ChemistryTableItem)
		}
		return c.ChemistryTableItem
	}

	protocol.Single(io, &c.BlockActor)
	protocol.NBTOptionalMarshaler(io, c.ChemistryTableItem, f, true)
}
