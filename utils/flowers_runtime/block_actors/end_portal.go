package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type EndPortal struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*EndPortal) ID() string {
	return IDEndPortal
}
