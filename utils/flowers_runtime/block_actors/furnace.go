package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

// 熔炉
type Furnace struct {
	general.FurnaceBlockActor `mapstructure:",squash"`
}

// ID ...
func (*Furnace) ID() string {
	return IDFurnace
}
