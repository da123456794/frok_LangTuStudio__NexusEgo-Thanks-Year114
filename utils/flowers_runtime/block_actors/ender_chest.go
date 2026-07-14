package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type EnderChest struct {
	general.ChestBlockActor `mapstructure:",squash"`
}

// ID ...
func (*EnderChest) ID() string {
	return IDEnderChest
}
