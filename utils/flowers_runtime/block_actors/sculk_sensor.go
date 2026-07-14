package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type SculkSensor struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*SculkSensor) ID() string {
	return IDSculkSensor
}
