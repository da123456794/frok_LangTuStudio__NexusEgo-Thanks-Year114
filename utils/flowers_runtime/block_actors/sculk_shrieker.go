package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type SculkShrieker struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*SculkShrieker) ID() string {
	return IDSculkShrieker
}
