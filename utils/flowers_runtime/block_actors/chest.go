package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

// 箱子
type Chest struct {
	general.ChestBlockActor `mapstructure:",squash"`
}

// ID ...
func (c *Chest) ID() string {
	return IDChest
}
