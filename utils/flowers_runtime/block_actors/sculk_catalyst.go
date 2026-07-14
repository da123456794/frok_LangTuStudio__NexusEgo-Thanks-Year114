package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type SculkCatalyst struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*SculkCatalyst) ID() string {
	return IDSculkCatalyst
}
