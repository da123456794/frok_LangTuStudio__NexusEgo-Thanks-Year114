package block_actors

import general "nexus/utils/flowers_runtime/block_actors/general_actors"

// 宝库 (不祥宝库)
type Vault struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*Vault) ID() string {
	return IDVault
}
