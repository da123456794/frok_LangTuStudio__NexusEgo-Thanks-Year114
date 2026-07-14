package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type DayLightDetector struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*DayLightDetector) ID() string {
	return IDDayLightDetector
}
