package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

// 高炉
type BlastFurnace struct {
	general.FurnaceBlockActor `mapstructure:",squash"`
}

// ID ...
func (*BlastFurnace) ID() string {
	return IDBlastFurnace
}
