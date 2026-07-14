package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_assigner_utils "github.com/LangTuStudio/RaaBel/nbt_assigner/utils"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 蜂巢
type Beehive struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.Beehive
}

func (Beehive) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (b *Beehive) Make() error {
	api := b.console.API()

	// 生成蜂巢
	err := nbt_assigner_utils.SpawnNewEmptyBlock(
		b.console,
		b.cache,
		nbt_assigner_utils.EmptyBlockData{
			Name:               b.data.BlockName(),
			States:             b.data.BlockStates(),
			IsCanOpenConatiner: false,
			BlockCustomName:    "Yeah",
		},
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 前往操作台中心处
	err = b.console.CanReachOrMove(b.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 生成蜂巢里的蜜蜂
	centerPos := b.console.Center()
	for range len(b.data.NBT.Occupants) {
		_, err = api.Commands().SendWSCommandWithResp(fmt.Sprintf(
			"summon bee %d %d %d ~~ find_hive_event",
			centerPos.X(),
			centerPos.Y(),
			centerPos.Z(),
		))
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		// 等待蜜蜂进入蜂巢
		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	return nil
}
