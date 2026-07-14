package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type Sign struct {
	general.SignBlockActor `mapstructure:",squash"`
}

// ID ...
func (*Sign) ID() string {
	return IDSign
}
