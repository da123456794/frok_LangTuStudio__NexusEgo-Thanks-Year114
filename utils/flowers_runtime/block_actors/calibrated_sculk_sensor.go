package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type CalibratedSculkSensor struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*CalibratedSculkSensor) ID() string {
	return IDCalibratedSculkSensor
}
