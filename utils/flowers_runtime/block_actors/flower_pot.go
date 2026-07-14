package block_actors

import (
	general "nexus/utils/flowers_runtime/block_actors/general_actors"
	"nexus/utils/flowers_runtime/protocol"
)

// 花盆
type FlowerPot struct {
	general.BlockActor `mapstructure:",squash"`
	PlantBlock         map[string]any `mapstructure:"PlantBlock"` // TAG_Compound(10)
}

// ID ...
func (*FlowerPot) ID() string {
	return IDFlowerPot
}

func (f *FlowerPot) Marshal(io protocol.IO) {
	protocol.Single(io, &f.BlockActor)
	io.NBTWithLength(&f.PlantBlock)
}
