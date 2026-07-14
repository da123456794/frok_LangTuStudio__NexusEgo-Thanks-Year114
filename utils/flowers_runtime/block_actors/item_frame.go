package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type ItemFrame struct {
	general.ItemFrameBlockActor `mapstructure:",squash"`
}

// ID ...
func (*ItemFrame) ID() string {
	return IDItemFrame
}
