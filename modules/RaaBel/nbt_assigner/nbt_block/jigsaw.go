package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 拼图方块
type Jigsaw struct {
	console *nbt_console.Console
	data    nbt_parser_block.Jigsaw
}

func (Jigsaw) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (j *Jigsaw) Make() error {
	api := j.console.API()

	err := api.SetBlock().SetBlock(j.console.Center(), j.data.BlockName(), j.data.BlockStatesString())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	j.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: true,
		Name:        j.data.BlockName(),
		States:      j.data.BlockStates(),
	})

	err = j.console.CanReachOrMove(j.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	position := j.console.Center()
	err = api.Resources().WritePacket(&packet.BlockActorData{
		Position: position,
		NBTData: map[string]any{
			"id":                 "JigsawBlock",
			"isMovable":          int32(1),
			"x":                  position.X(),
			"y":                  int32(position.Y()),
			"z":                  position.Z(),
			"final_state":        j.data.NBT.FinalState,
			"joint":              j.data.NBT.Joint,
			"name":               j.data.NBT.Name,
			"placement_priority": j.data.NBT.PlacementPriority,
			"selection_priority": j.data.NBT.SelectionPriority,
			"target":             j.data.NBT.Target,
			"target_pool":        j.data.NBT.TargetPool,
		},
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	err = api.Commands().AwaitChangesGeneral()
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
