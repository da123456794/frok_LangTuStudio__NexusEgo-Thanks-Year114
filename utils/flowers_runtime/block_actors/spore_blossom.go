package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type SporeBlossom struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*SporeBlossom) ID() string {
	return IDSporeBlossom
}
