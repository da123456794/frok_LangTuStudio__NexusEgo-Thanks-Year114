package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	nbt_assigner_interface "github.com/LangTuStudio/RaaBel/nbt_assigner/interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_assigner_utils "github.com/LangTuStudio/RaaBel/nbt_assigner/utils"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 讲台
type Lectern struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.Lectern
}

func (Lectern) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

func (l *Lectern) Make() error {
	var targetSlot resources_control.SlotID
	api := l.console.API()

	// 生成讲台
	err := nbt_assigner_utils.SpawnNewEmptyBlock(
		l.console,
		l.cache,
		nbt_assigner_utils.EmptyBlockData{
			Name:               l.data.BlockName(),
			States:             l.data.BlockStates(),
			IsCanOpenConatiner: false,
			BlockCustomName:    l.data.NBT.CustomName,
		},
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 如果讲台上没有书，则应当直接返回值
	if !l.data.NBT.HaveBook {
		return nil
	}

	// 如果书可以直接使用命令放置
	if !l.data.NBT.Book.IsComplex() && !l.data.NBT.Book.NeedEnchRenameDyeOrLore() {
		underlying := l.data.NBT.Book.UnderlyingItem()
		defaultItem := underlying.(*nbt_parser_item.DefaultItem)

		err = api.Replaceitem().ReplaceitemInContainerAsync(
			l.console.Center(),
			game_interface.ReplaceitemInfo{
				Name:     l.data.NBT.Book.ItemName(),
				Count:    l.data.NBT.Book.ItemCount(),
				MetaData: l.data.NBT.Book.ItemMetadata(),
				Slot:     0,
			},
			utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		err = api.Commands().AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}

		return nil
	}

	// 制作成书
	methods := nbt_assigner_interface.MakeNBTItemMethod(l.console, l.cache, l.data.NBT.Book)
	if len(methods) != 1 {
		panic("Make: Should nerver happened")
	}
	resultSlot, err := methods[0].Make()
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	if len(resultSlot) != 1 {
		panic("Make: Should nerver happened")
	}
	for _, slotID := range resultSlot {
		targetSlot = slotID
	}

	if l.data.NBT.Book.NeedEnchRenameDyeOrLore() {
		err = nbt_assigner_interface.EnchRenameDyeOrLoreSingle(l.console, l.data.NBT.Book, targetSlot)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	// 切换手持物品栏
	if targetSlot != l.console.HotbarSlotID() {
		err = l.console.ChangeAndUpdateHotbarSlotID(targetSlot)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	// 传送到操作台中心
	err = l.console.CanReachOrMove(l.console.Center())
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 放置成书
	err = api.BotClick().ClickBlock(game_interface.UseItemOnBlocks{
		HotbarSlotID: l.console.HotbarSlotID(),
		BotPos:       l.console.Position(),
		BlockPos:     l.console.Center(),
		BlockName:    l.data.BlockName(),
		BlockStates:  l.data.BlockStates(),
	})
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
