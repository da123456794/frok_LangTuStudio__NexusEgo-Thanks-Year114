package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_assigner_utils "github.com/LangTuStudio/RaaBel/nbt_assigner/utils"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 附魔台
type EnchantingTable struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.EnchantingTable
}

func (EnchantingTable) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (e *EnchantingTable) Make() error {
	// 生成附魔台
	err := nbt_assigner_utils.SpawnNewEmptyBlock(
		e.console,
		e.cache,
		nbt_assigner_utils.EmptyBlockData{
			Name:               e.data.BlockName(),
			States:             e.data.BlockStates(),
			IsCanOpenConatiner: false,
			BlockCustomName:    e.data.NBT.CustomName,
		},
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
