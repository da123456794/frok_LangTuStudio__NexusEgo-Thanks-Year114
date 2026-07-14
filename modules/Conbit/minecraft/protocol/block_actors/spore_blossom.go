package block_actors

import general "github.com/LangTuStudio/Conbit/minecraft/protocol/block_actors/general_actors"

// 孢子花
type SporeBlossom struct {
	general.BlockActor `mapstructure:",squash"`
}

// ID ...
func (*SporeBlossom) ID() string {
	return IDSporeBlossom
}
