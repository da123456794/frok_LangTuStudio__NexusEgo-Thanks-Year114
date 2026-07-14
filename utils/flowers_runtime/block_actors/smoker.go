package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type Smoker struct {
	general.FurnaceBlockActor `mapstructure:",squash"`
}

// ID ...
func (*Smoker) ID() string {
	return IDSmoker
}
