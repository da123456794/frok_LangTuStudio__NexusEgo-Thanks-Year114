package NBTAssigner

import (
	"fmt"
	GameInterface "nexus/utils/api/game_interface"
	"nexus/utils/bdump/mcstructure"
)

// 获取物品展示框的物品数据
func (f *Frame) Get_Frame_Item_Data() (ItemOrigin, error) {
	return nil, nil
	// return f.getContainerContents()
}

// 我们不再检查用户提供的物品展示框的 NBT 是否正确，
// 我们信任并且永远认为它们是正确且完整的
func (f *Frame) Decode() error {
	// bdump load "C:\Users\12198\Downloads\展示框2.bdx" 40010 100 40010
	return nil
}

// 放置一个展示框并写入展示框数据
func (f *Frame) WriteData() error {
	gameInterface := f.BlockEntity.Interface.(*GameInterface.GameInterface)

	item_raw, ishave_nbt := f.BlockEntity.Block.NBT["Item"]

	if !ishave_nbt {
		err := gameInterface.SetBlock(f.BlockEntity.AdditionalData.Position, f.BlockEntity.Block.Name, f.BlockEntity.AdditionalData.BlockStates)
		if err != nil {
			return fmt.Errorf("WriteData: %v", err)
		}
		return nil
	}
	// 把原来的展示框替换为新的展示框
	err := gameInterface.SetBlock(f.BlockEntity.AdditionalData.Position, "air", "0")
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	// 放置展示框
	err = gameInterface.SetBlock(f.BlockEntity.AdditionalData.Position, f.BlockEntity.Block.Name, f.BlockEntity.AdditionalData.BlockStates)
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	uniqueID_1, err := gameInterface.BackupStructure(GameInterface.MCStructure{
		BeginX: int(f.BlockEntity.AdditionalData.Position[0]),
		BeginY: int(f.BlockEntity.AdditionalData.Position[1]),
		BeginZ: int(f.BlockEntity.AdditionalData.Position[2]),
		SizeX:  1,
		SizeY:  1,
		SizeZ:  1,
	})
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	err = gameInterface.RevertStructure(
		uniqueID_1,
		GameInterface.BlockPos{
			int(f.BlockEntity.AdditionalData.Position[0]),
			int(f.BlockEntity.AdditionalData.Position[1]),
			int(f.BlockEntity.AdditionalData.Position[2]),
		},
	)
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	// 切换手持物品栏到快捷栏 5
	err = gameInterface.ChangeSelectedHotbarSlot(5)
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	items, is_true2 := item_raw.(map[string]interface{})
	if !is_true2 {
		return fmt.Errorf("WriteData: %v", "无数据")
	}
	// ItemOrigin
	newPackage := ItemPackage{
		Interface: f.BlockEntity.Interface,
		Item:      GeneralItem{},
		AdditionalData: ItemAdditionalData{
			HotBarSlot: 5,
			Position:   f.BlockEntity.AdditionalData.Position,
			Type:       "",
			FastMode:   f.BlockEntity.AdditionalData.FastMode,
			Others:     f.BlockEntity.AdditionalData.Others,
		},
	}
	err = newPackage.ParseItemFromNBT(items)
	if err != nil {
		return fmt.Errorf("Decode: %v", err)
	}
	is_true, err2 := f.GetNBTItem(newPackage)
	if err2 != nil {
		return fmt.Errorf("WriteData: %v", err2)
	}
	if !is_true {
		return fmt.Errorf("WriteData: %v", err)
	}

	// 点击展示框
	// 获取容器资源
	blockStatesMap, err := mcstructure.UnmarshalBlockStates(f.BlockEntity.AdditionalData.BlockStates)
	if err != nil {
		return fmt.Errorf("RenameItemByAnvil: %v", err)
	}
	err = gameInterface.ClickBlock(GameInterface.UseItemOnBlocks{
		BlockPos:     f.BlockEntity.AdditionalData.Position,
		HotbarSlotID: 5,
		BlockName:    "minecraft:" + f.BlockEntity.Block.Name,
		BlockStates:  blockStatesMap,
	})
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	uniqueID_2, err := gameInterface.BackupStructure(GameInterface.MCStructure{
		BeginX: int(f.BlockEntity.AdditionalData.Position[0]),
		BeginY: int(f.BlockEntity.AdditionalData.Position[1]),
		BeginZ: int(f.BlockEntity.AdditionalData.Position[2]),
		SizeX:  1,
		SizeY:  1,
		SizeZ:  1,
	})
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	{
		err := gameInterface.AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("PlaceCommandBlockLegacy: %v", err)
		}
	}
	{
		err := gameInterface.AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("PlaceCommandBlockLegacy: %v", err)
		}
	}
	{
		err := gameInterface.AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("PlaceCommandBlockLegacy: %v", err)
		}
	}
	err = gameInterface.RevertStructure(
		uniqueID_2,
		GameInterface.BlockPos{
			int(f.BlockEntity.AdditionalData.Position[0]),
			int(f.BlockEntity.AdditionalData.Position[1]),
			int(f.BlockEntity.AdditionalData.Position[2]),
		},
	)
	if err != nil {
		return fmt.Errorf("WriteData: %v", err)
	}
	return nil
}

// 获取 itemPackage.Item 所指代的 NBT 物品到快捷栏 5 。
// 如果 itemPackage.Item 有自定义的物品显示名称或附魔属性，
// 则还会使用铁砧进行改名并使用 enchant 命令附魔。
//
// 返回的布尔值代表以上操作是否成功
func (c *Frame) GetNBTItem(
	itemPackage ItemPackage,
) (bool, error) {
	api := c.BlockEntity.Interface.(*GameInterface.GameInterface)
	// 初始化
	err := api.SendAICommand("clear", true)
	if err != nil {
		return false, fmt.Errorf("GetNBTItem: %v", err)
	}
	// 清除物品栏
	uniqueId, err := api.BackupStructure(
		GameInterface.MCStructure{
			BeginX: int(c.BlockEntity.AdditionalData.Position[0]),
			BeginY: int(c.BlockEntity.AdditionalData.Position[1]),
			BeginZ: int(c.BlockEntity.AdditionalData.Position[2]),
			SizeX:  1,
			SizeY:  1,
			SizeZ:  1,
		},
	)
	if err != nil {
		return false, fmt.Errorf("GetNBTItem: %v", err)
	}
	defer api.RevertStructure(uniqueId, GameInterface.BlockPos{
		int(c.BlockEntity.AdditionalData.Position[0]),
		int(c.BlockEntity.AdditionalData.Position[1]),
		int(c.BlockEntity.AdditionalData.Position[2]),
	})
	// 备份容器
	method := GetGenerateItemMethod(&itemPackage)
	// 得到获取该 NBT 物品的方法
	err = method.Decode()
	if err != nil {
		return false, fmt.Errorf("GetNBTItem: %v", err)
	}
	err = method.WriteData()
	if err != nil {
		return false, fmt.Errorf("GetNBTItem: %v", err)
	}
	// 解码并取得该 NBT 物品
	err = api.AwaitChangesGeneral()
	if err != nil {
		return false, fmt.Errorf("GetNBTItem: %v", err)
	}
	// 等待更改
	return true, nil
	// 返回值
}
