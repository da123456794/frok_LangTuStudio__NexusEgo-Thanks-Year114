package nbt_block

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	nbt_assigner_interface "github.com/LangTuStudio/RaaBel/nbt_assigner/interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_assigner_utils "github.com/LangTuStudio/RaaBel/nbt_assigner/utils"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
	nbt_hash "github.com/LangTuStudio/RaaBel/nbt_parser/hash"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
	"github.com/LangTuStudio/RaaBel/utils"
)

// 纹饰陶罐
type DecoratedPot struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.DecoratedPot
}

func (DecoratedPot) Offset() protocol.BlockPos {
	return protocol.BlockPos{0, 0, 0}
}

// processComplex 处理复杂的物品
func (d *DecoratedPot) processComplex(item nbt_parser_interface.Item) (canUseCommand bool, resultSlot resources_control.SlotID, err error) {
	api := d.console.API()
	underlying := item.UnderlyingItem()
	defaultItem := underlying.(*nbt_parser_item.DefaultItem)

	// 子方块
	if defaultItem.Block.SubBlock != nil {
		if !defaultItem.Block.SubBlock.NeedSpecialHandle() {
			return true, 0, nil
		}
		_, _, _, err = nbt_assigner_interface.PlaceNBTBlock(d.console, d.cache, defaultItem.Block.SubBlock)
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}

		_, hit, partHit, err := d.cache.NBTBlockCache().LoadCache(nbt_hash.CompletelyHashNumber{
			HashNumber:    nbt_hash.NBTBlockFullHash(defaultItem.Block.SubBlock),
			SetHashNumber: nbt_hash.ContainerSetHash(defaultItem.Block.SubBlock),
		})
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !hit || partHit {
			panic("processComplex: Should never happened")
		}

		_, err = d.console.API().Commands().SendWSCommandWithResp("clear")
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		d.console.CleanInventory()

		success, currentSlot, err := api.BotClick().PickBlock(d.console.Center(), true)
		if err != nil || !success {
			_ = d.console.ChangeAndUpdateHotbarSlotID(nbt_console.DefaultHotbarSlot)
		}
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !success {
			return false, 0, fmt.Errorf("processComplex: Failed to pick block due to unknown reason")
		}
		d.console.UpdateHotbarSlotID(currentSlot)
		d.console.UseInventorySlot(nbt_console.RequesterUser, currentSlot, true)

		return false, currentSlot, nil
	}

	// 复杂NBT物品制作
	methods := nbt_assigner_interface.MakeNBTItemMethod(d.console, d.cache, item)
	if len(methods) != 1 {
		panic("Make: Should never happened")
	}
	resultSlotMapping, err := methods[0].Make()
	if err != nil {
		return false, 0, fmt.Errorf("processComplex: %v", err)
	}
	if len(resultSlotMapping) != 1 {
		panic("Make: Should never happened")
	}

	// 将复杂 NBT 物品移动到快捷栏
	for _, slotID := range resultSlotMapping {
		resultSlot = slotID
	}
	if resultSlot > 8 {
		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     "minecraft:air",
				Count:    1,
				MetaData: 0,
				Slot:     d.console.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		d.console.UseInventorySlot(nbt_console.RequesterUser, d.console.HotbarSlotID(), false)

		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !success {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			MoveBetweenInventory(resultSlot, d.console.HotbarSlotID(), 1).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return false, 0, fmt.Errorf("processComplex: The server rejected the stack request action")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return false, 0, fmt.Errorf("processComplex: %v", err)
		}

		resultSlot = d.console.HotbarSlotID()
	}

	return false, resultSlot, nil
}

func (d *DecoratedPot) setYawBeforePlace() error {
	direction, ok := d.data.BlockStates()["direction"].(int32)
	if !ok {
		return nil
	}

	yaw := utils.DirectionToYaw(direction)
	inputData := protocol.NewBitset(packet.PlayerAuthInputBitsetSize)
	inputData.Set(packet.InputFlagStartFlying)
	err := d.console.API().Resources().WritePacket(&packet.PlayerAuthInput{
		Yaw:       yaw,
		HeadYaw:   yaw,
		InputData: inputData,
		Position:  d.console.Position(),
	})
	if err != nil {
		return fmt.Errorf("setYawBeforePlace: %v", err)
	}

	return nil
}

// makeSpecialSherdPot 通过合成器制作纹饰陶罐。
func (d *DecoratedPot) makeSpecialSherdPot() (resultSlot resources_control.SlotID, err error) {
	api := d.console.API()
	targetSherds := d.data.NBT.Sherds

	// 合成器输入顺序：[上, 左, 右, 下]
	// 目标 NBT 顺序：[后, 左, 右, 前]。
	// 在当前实现中，二者可以直接一一对应
	craftingSherds := [4]string{
		targetSherds[0],
		targetSherds[1],
		targetSherds[2],
		targetSherds[3],
	}
	crafterSlots := [4]resources_control.SlotID{1, 3, 5, 7}

	// 准备 4 个合成物品
	usedSlots := make([]resources_control.SlotID, 0, 5)
	sherdSlots := make([]resources_control.SlotID, 0, 4)
	for index := range 4 {
		slot := d.console.FindInventorySlot(usedSlots)
		usedSlots = append(usedSlots, slot)
		sherdSlots = append(sherdSlots, slot)

		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathInventory,
			game_interface.ReplaceitemInfo{
				Name:     craftingSherds[index],
				Count:    1,
				MetaData: 0,
				Slot:     slot,
			},
			"",
			false,
		)
		if err != nil {
			return 0, fmt.Errorf("makeSpecialSherdPot: %v", err)
		}
		d.console.UseInventorySlot(nbt_console.RequesterUser, slot, true)
	}

	err = api.Commands().AwaitChangesGeneral()
	if err != nil {
		return 0, fmt.Errorf("makeSpecialSherdPot: %v", err)
	}

	index, err := d.console.FindOrGenerateNewCrafter()
	if err != nil {
		return 0, fmt.Errorf("makeSpecialSherdPot: %v", err)
	}

	resultSlot = d.console.FindInventorySlot(usedSlots)
	err = api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathInventory,
		game_interface.ReplaceitemInfo{
			Name:     "minecraft:air",
			Count:    1,
			MetaData: 0,
			Slot:     resultSlot,
		},
		"",
		true,
	)
	if err != nil {
		return 0, fmt.Errorf("makeSpecialSherdPot: %v", err)
	}
	d.console.UseInventorySlot(nbt_console.RequesterUser, resultSlot, false)

	inventoryToCrafterSlotMapping := make(map[resources_control.SlotID]resources_control.SlotID)
	for idx := range 4 {
		inventoryToCrafterSlotMapping[sherdSlots[idx]] = crafterSlots[idx]
	}

	err = d.console.CraftByCrafter(index, inventoryToCrafterSlotMapping, resultSlot)
	if err != nil {
		return 0, fmt.Errorf("makeSpecialSherdPot: %v", err)
	}

	return resultSlot, nil
}

// makePotItemToHotbar 获取纹饰陶罐物品并返回其所在快捷栏槽位。
// 如果是特殊陶片组合，将通过工作台动态合成
func (d *DecoratedPot) makePotItemToHotbar() (resultSlot resources_control.SlotID, err error) {
	api := d.console.API()

	if d.data.NBT.HaveSpecialSherd {
		resultSlot, err = d.makeSpecialSherdPot()
		if err != nil {
			return 0, fmt.Errorf("makePotItemToHotbar: %v", err)
		}
	} else {
		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     d.data.BlockName(),
				Count:    1,
				MetaData: 0,
				Slot:     d.console.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return 0, fmt.Errorf("makePotItemToHotbar: %v", err)
		}
		d.console.UseInventorySlot(nbt_console.RequesterUser, d.console.HotbarSlotID(), true)
		return d.console.HotbarSlotID(), nil
	}

	if resultSlot > 8 {
		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     "minecraft:air",
				Count:    1,
				MetaData: 0,
				Slot:     d.console.HotbarSlotID(),
			},
			"",
			true,
		)
		if err != nil {
			return 0, fmt.Errorf("makePotItemToHotbar: %v", err)
		}
		d.console.UseInventorySlot(nbt_console.RequesterUser, d.console.HotbarSlotID(), false)

		success, err := api.ContainerOpenAndClose().OpenInventory()
		if err != nil {
			return 0, fmt.Errorf("makePotItemToHotbar: %v", err)
		}
		if !success {
			return 0, fmt.Errorf("makePotItemToHotbar: Failed to open the inventory")
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			MoveBetweenInventory(resultSlot, d.console.HotbarSlotID(), 1).
			Commit()
		if err != nil {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return 0, fmt.Errorf("makePotItemToHotbar: %v", err)
		}
		if !success {
			_ = api.ContainerOpenAndClose().CloseContainer()
			return 0, fmt.Errorf("makePotItemToHotbar: The server rejected the stack request action")
		}

		err = api.ContainerOpenAndClose().CloseContainer()
		if err != nil {
			return 0, fmt.Errorf("makePotItemToHotbar: %v", err)
		}

		d.console.UseInventorySlot(nbt_console.RequesterUser, resultSlot, false)
		d.console.UseInventorySlot(nbt_console.RequesterUser, d.console.HotbarSlotID(), true)

		resultSlot = d.console.HotbarSlotID()
	}

	return resultSlot, nil
}

func (d *DecoratedPot) prepareStoredItem(item nbt_parser_interface.Item) (err error) {
	api := d.console.API()

	var canUseCommand bool
	var resultSlot resources_control.SlotID

	if item.IsComplex() {
		canUseCommand, resultSlot, err = d.processComplex(item)
		if err != nil {
			return fmt.Errorf("prepareStoredItem: %v", err)
		}
	} else {
		canUseCommand = true
	}

	if canUseCommand {
		underlying := item.UnderlyingItem()
		defaultItem := underlying.(*nbt_parser_item.DefaultItem)

		err = api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathHotbarOnly,
			game_interface.ReplaceitemInfo{
				Name:     item.ItemName(),
				Count:    item.ItemCount(),
				MetaData: item.ItemMetadata(),
				Slot:     d.console.HotbarSlotID(),
			},
			utils.MarshalItemComponent(defaultItem.Enhance.ItemComponent),
			true,
		)
		if err != nil {
			return fmt.Errorf("prepareStoredItem: %v", err)
		}
		d.console.UseInventorySlot(nbt_console.RequesterUser, d.console.HotbarSlotID(), true)
		resultSlot = d.console.HotbarSlotID()
	}

	if resultSlot != d.console.HotbarSlotID() {
		err = d.console.ChangeAndUpdateHotbarSlotID(resultSlot)
		if err != nil {
			return fmt.Errorf("prepareStoredItem: %v", err)
		}
	}

	if item.NeedEnchRenameDyeOrLore() {
		err = nbt_assigner_interface.EnchRenameDyeOrLoreSingle(d.console, item, resultSlot)
		if err != nil {
			return fmt.Errorf("prepareStoredItem: %v", err)
		}
	}

	return nil
}

func (d *DecoratedPot) placeSpecialSherdPot() error {
	api := d.console.API()

	resultSlot, err := d.makePotItemToHotbar()
	if err != nil {
		return fmt.Errorf("placeSpecialSherdPot: %v", err)
	}

	if resultSlot != d.console.HotbarSlotID() {
		err = d.console.ChangeAndUpdateHotbarSlotID(resultSlot)
		if err != nil {
			return fmt.Errorf("placeSpecialSherdPot: %v", err)
		}
	}

	err = d.console.CanReachOrMove(d.console.Center())
	if err != nil {
		return fmt.Errorf("placeSpecialSherdPot: %v", err)
	}

	err = d.setYawBeforePlace()
	if err != nil {
		return fmt.Errorf("placeSpecialSherdPot: %v", err)
	}

	_, offsetPos, err := api.BotClick().PlaceBlockHighLevel(
		d.console.Center(),
		d.console.Position(),
		d.console.HotbarSlotID(),
		1,
	)
	if err != nil {
		return fmt.Errorf("placeSpecialSherdPot: %v", err)
	}
	d.console.UseHelperBlock(nbt_console.RequesterUser, nbt_console.ConsoleIndexCenterBlock, block_helper.ComplexBlock{
		KnownStates: false,
		Name:        d.data.BlockName(),
	})
	*d.console.NearBlockByIndex(nbt_console.ConsoleIndexCenterBlock, offsetPos) = block_helper.NearBlock{
		Name: game_interface.BasePlaceBlock,
	}
	d.console.UseInventorySlot(nbt_console.RequesterUser, d.console.HotbarSlotID(), false)

	return nil
}

func (d *DecoratedPot) putItemInPotByClick() error {
	if !d.data.NBT.HaveItem || d.data.NBT.Item == nil {
		return nil
	}

	err := d.prepareStoredItem(d.data.NBT.Item)
	if err != nil {
		return fmt.Errorf("putItemInPotByClick: %v", err)
	}

	err = d.console.CanReachOrMove(d.console.Center())
	if err != nil {
		return fmt.Errorf("putItemInPotByClick: %v", err)
	}

	request := game_interface.UseItemOnBlocks{
		HotbarSlotID: d.console.HotbarSlotID(),
		BotPos:       d.console.Position(),
		BlockPos:     d.console.Center(),
		BlockName:    d.data.BlockName(),
		BlockStates:  d.data.BlockStates(),
	}
	for range d.data.NBT.Item.ItemCount() {
		err = d.console.API().BotClick().ClickBlock(request)
		if err != nil {
			return fmt.Errorf("putItemInPotByClick: %v", err)
		}
	}

	d.console.UseInventorySlot(nbt_console.RequesterUser, d.console.HotbarSlotID(), false)

	return nil
}

func (d *DecoratedPot) Make() error {
	if d.data.NBT.HaveSpecialSherd {
		err := d.placeSpecialSherdPot()
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	} else {
		err := nbt_assigner_utils.SpawnNewEmptyBlock(
			d.console,
			d.cache,
			nbt_assigner_utils.EmptyBlockData{
				Name:               d.data.BlockName(),
				States:             d.data.BlockStates(),
				IsCanOpenConatiner: false,
			},
		)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	err := d.putItemInPotByClick()
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	return nil
}
