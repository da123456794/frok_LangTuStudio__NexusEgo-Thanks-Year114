package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type Dropper struct {
	general.DispenserBlockActor `mapstructure:",squash"`
}

// ID ...
func (*Dropper) ID() string {
	return IDDropper
}
