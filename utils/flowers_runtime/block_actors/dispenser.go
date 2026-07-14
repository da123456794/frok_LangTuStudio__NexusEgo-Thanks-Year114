package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type Dispenser struct {
	general.DispenserBlockActor `mapstructure:",squash"`
}

// ID ...
func (*Dispenser) ID() string {
	return IDDispenser
}
