package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

type Crafter struct {
	general.DispenserBlockActor `mapstructure:",squash"`
	DisabledSlots               string `mapstructure:"disabled_slots"` // Not used; TAG_Short(3) = 0
}

// ID ...
func (*Crafter) ID() string {
	return IDCrafter
}
