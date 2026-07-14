package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type GlowItemFrame struct {
	general.ItemFrameBlockActor `mapstructure:",squash"`
}

// ID ...
func (*GlowItemFrame) ID() string {
	return IDGlowItemFrame
}
