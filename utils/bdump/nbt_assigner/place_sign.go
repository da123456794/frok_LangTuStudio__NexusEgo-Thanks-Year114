package NBTAssigner

import (
	"fmt"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	GameInterface "nexus/utils/api/game_interface"
	"nexus/utils/api/generics"
	ResourcesControl "nexus/utils/api/resources_control"
	"strings"
	"time"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/google/uuid"
)

const PlaceSignPath = "place_sign"

/*
从 s.BlockEntity.Block.NBT 提取告示牌的
一部分数据并保存在 s.SignData 或
s.LegacySignData 中。

如果 s.IsNotLegacySignBlock 为真，
则 s.SignData 将存放这些数据，
否则由 s.LegacySignData 存放这些数据。

对于未被解码的部分，
我们不再检查这部分 NBT 是否正确，
我们信任并且永远认为它们是正确且完整的
*/

func (s *Sign) Decode() error {
	nbt := s.BlockEntity.Block.NBT
	_, s.IsNotLegacySignBlock = nbt["IsWaxed"]
	// 初始化
	if strings.Contains(s.BlockEntity.Block.Name, "hanging") {
		s.IsHangingSignBlock = true
	}
	// 确定告示牌类型
	if !s.IsNotLegacySignBlock {
		s.LegacySignData = &LegacySignData{}
		// 初始化
		ignoreLighting, err := generics.To[byte](nbt["IgnoreLighting"], `nbt["IgnoreLighting"]`, PlaceSignPath)
		if err != nil {
			return fmt.Errorf("Decode: %v", err)
		}
		if ignoreLighting == byte(1) {
			s.LegacySignData.IgnoreLighting = true
		}
		// IgnoreLighting
		s.LegacySignData.SignTextColor, err = generics.To[int32](nbt["SignTextColor"], `nbt["SignTextColor"]`, PlaceSignPath)
		if err != nil {
			return fmt.Errorf("Decode: %v", err)
		}
		// SignTextColor
	} else {
		s.SignData = &SignData{}
		// 初始化
		isWaxed, err := generics.To[byte](nbt["IsWaxed"], `nbt["IsWaxed"]`, PlaceSignPath)
		if err != nil {
			return fmt.Errorf("Decode: %v", err)
		}
		if isWaxed == byte(1) {
			s.SignData.IsWaxed = true
		}
		// IsWaxed
		{
			text, err := generics.To[map[string]any](nbt["FrontText"], `nbt["FrontText"]`, PlaceSignPath)
			if err != nil {
				return fmt.Errorf("Decode: %v", err)
			}
			// FrontText
			ignoreLighting, err := generics.To[byte](text["IgnoreLighting"], `nbt["FrontText"]["IgnoreLighting"]`, PlaceSignPath)
			if err != nil {
				return fmt.Errorf("Decode: %v", err)
			}
			if ignoreLighting == byte(1) {
				s.SignData.FrontText.IgnoreLighting = true
			}
			// FrontText["IgnoreLighting"]
			s.SignData.FrontText.SignTextColor, err = generics.To[int32](text["SignTextColor"], `nbt["FrontText"]["SignTextColor"]`, PlaceSignPath)
			if err != nil {
				return fmt.Errorf("Decode: %v", err)
			}
			// FrontText["SignTextColor"]
		}
		// FrontText
		{
			text, err := generics.To[map[string]any](nbt["BackText"], `nbt["BackText"]`, PlaceSignPath)
			if err != nil {
				return fmt.Errorf("Decode: %v", err)
			}
			// BackText
			ignoreLighting, err := generics.To[byte](text["IgnoreLighting"], `nbt["BackText"]["IgnoreLighting"]`, PlaceSignPath)
			if err != nil {
				return fmt.Errorf("Decode: %v", err)
			}
			if ignoreLighting == byte(1) {
				s.SignData.BackText.IgnoreLighting = true
			}
			// BackText["IgnoreLighting"]
			s.SignData.BackText.SignTextColor, err = generics.To[int32](text["SignTextColor"], `nbt["BackText"]["SignTextColor"]`, PlaceSignPath)
			if err != nil {
				return fmt.Errorf("Decode: %v", err)
			}
			// BackText["SignTextColor"]
		}
		// BackText
	}
	// decode data
	return nil
	// return
}

// 放置一个告示牌并写入告示牌数据
func (s *Sign) WriteData() error {
	//var preBlockName string = "oak_hanging_sign"
	gameInterface := s.BlockEntity.Interface.(*GameInterface.GameInterface)

	//// 初始化
	//if s.BlockEntity.AdditionalData.FastMode {
	//	err := s.BlockEntity.Interface.SetBlockAsync(s.BlockEntity.AdditionalData.Position, s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates)
	//	if err != nil {
	//		return fmt.Errorf("WriteData: %v", err)
	//	}
	//	return nil
	//}
	//// 放置告示牌(快速导入模式下)
	//{
	//	err := gameInterface.SendAICommand(fmt.Sprintf("tp %d %d %d", s.BlockEntity.AdditionalData.Position[0], s.BlockEntity.AdditionalData.Position[1], s.BlockEntity.AdditionalData.Position[2]), true)
	//	if err != nil {
	//		return fmt.Errorf("WriteData: %v", err)
	//	}
	//	// 传送机器人到告示牌所在的位置
	//	err = gameInterface.AwaitChangesGeneral()
	//	if err != nil {
	//		return fmt.Errorf("WriteData: %v", err)
	//	}
	//	// 等待其他方块已完成方块更新，
	//	// 以确保将会放置的告示牌不会因为方块更新而掉落
	//	err = gameInterface.SetBlock(s.BlockEntity.AdditionalData.Position, "air", `[]`)
	//	if err != nil {
	//		return fmt.Errorf("WriteData: %v", err)
	//	}
	//	// 清除告示牌处的方块
	//	if !s.IsHangingSignBlock {
	//		preBlockName = "wall_sign"
	//	}
	//	useItemOnBlocks.BlockName = preBlockName
	//	// 确定 预设告示牌 的方块名称
	//	err = gameInterface.SetBlock(s.BlockEntity.AdditionalData.Position, preBlockName, `["facing_direction"=4]`)
	//	if err != nil {
	//		return fmt.Errorf("WriteData: %v", err)
	//	}
	//	// 放置预设告示牌方块
	//}
	//// 放置告示牌
	//err := gameInterface.ChangeSelectedHotbarSlot(0)
	//if err != nil {
	//	return fmt.Errorf("WriteData: %v", err)
	//}
	//// 切换手持物品栏到快捷栏 0
	//resp := gameInterface.SendWSCommandWithResponse(
	//	"replaceitem entity @s slot.hotbar 0 air",
	//	ResourcesControl.CommandRequestOptions{
	//		TimeOut: 5 * time.Second,
	//	},
	//)
	//if resp.Error != nil {
	//	return fmt.Errorf("WriteData: %v", resp.Error)
	//}
	//// 清空快捷栏 0 以防止稍后在手持蜜脾的情况下点击告示牌，
	//// 因为用 蜜脾 点击告示牌会导致告示牌被封装
	//err = gameInterface.ClickBlock(GameInterface.UseItemOnBlocks{
	//	HotbarSlotID: 0,
	//	BlockPos:     s.BlockEntity.AdditionalData.Position,
	//	BlockName:    preBlockName,
	//	BlockStates:  map[string]interface{}{"facing_direction": int32(4)},
	//})
	//if err != nil {
	//	return fmt.Errorf("WriteData: %v", err)
	//}
	//// 打开告示牌
	//signBlockNBTData := s.BlockEntity.Block.NBT
	//if !s.IsNotLegacySignBlock {
	//	signBlockNBTData = map[string]any{"FrontText": s.BlockEntity.Block.NBT}
	//}
	//err = gameInterface.WritePacket(&packet.BlockActorData{
	//	Position: s.BlockEntity.AdditionalData.Position,
	//	NBTData:  signBlockNBTData,
	//})
	//if err != nil {
	//	return fmt.Errorf("WriteData: %v", err)
	//}
	//// 写入告示牌数据
	//{
	//	var bestFrontColor [3]uint8
	//	var bestBackColor *[3]uint8
	//	playerPosition := mgl32.Vec3{
	//		float32(s.BlockEntity.AdditionalData.Position[0]),
	//		float32(s.BlockEntity.AdditionalData.Position[1]),
	//		float32(s.BlockEntity.AdditionalData.Position[2]),
	//	}
	//	// 初始化
	//	if s.IsNotLegacySignBlock {
	//		frontRGB, _ := DecodeVarRGBA(s.SignData.FrontText.SignTextColor)
	//		backRGB, _ := DecodeVarRGBA(s.SignData.BackText.SignTextColor)
	//		bestFrontColor = SearchForBestColor(frontRGB, DefaultDyeColor)
	//		bestBackColorTemp := SearchForBestColor(backRGB, DefaultDyeColor)
	//		bestBackColor = &bestBackColorTemp
	//	} else {
	//		rgb, _ := DecodeVarRGBA(s.LegacySignData.SignTextColor)
	//		bestFrontColor = SearchForBestColor(rgb, DefaultDyeColor)
	//	}
	//	// 确定告示牌各面的颜色
	//	if bestFrontColor != [3]uint8{0, 0, 0} {
	//		dyeItemName := RGBToDyeItemName[bestFrontColor]
	//		// 确定染料的物品名
	//		resp = gameInterface.SendWSCommandWithResponse(
	//			fmt.Sprintf("replaceitem entity @s slot.hotbar 0 %s", dyeItemName),
	//			ResourcesControl.CommandRequestOptions{
	//				TimeOut: ResourcesControl.CommandRequestNoDeadLine,
	//			},
	//		)
	//		if resp.Error != nil {
	//			return fmt.Errorf("WriteData: %v", resp.Error)
	//		}
	//		// 获取对应的染料到快捷栏 0
	//		err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
	//		if err != nil {
	//			return fmt.Errorf("WriteData: %v", err)
	//		}
	//		// 告示牌正面染色
	//	}
	//	if bestBackColor != nil && *bestBackColor != [3]uint8{0, 0, 0} {
	//		dyeItemName := RGBToDyeItemName[*bestBackColor]
	//		// 确定染料的物品名
	//		resp = gameInterface.SendWSCommandWithResponse(
	//			fmt.Sprintf("replaceitem entity @s slot.hotbar 0 %s", dyeItemName),
	//			ResourcesControl.CommandRequestOptions{
	//				TimeOut: ResourcesControl.CommandRequestNoDeadLine,
	//			},
	//		)
	//		if resp.Error != nil {
	//			return fmt.Errorf("WriteData: %v", resp.Error)
	//		}
	//		// 获取对应的染料到快捷栏 0
	//		playerPosition[0] = playerPosition[0] + 1
	//		err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
	//		if err != nil {
	//			return fmt.Errorf("WriteData: %v", err)
	//		}
	//		// 告示牌背面染色
	//	}
	//}
	//// 告示牌染色
	//{
	//	playerPosition := mgl32.Vec3{
	//		float32(s.BlockEntity.AdditionalData.Position[0]),
	//		float32(s.BlockEntity.AdditionalData.Position[1]),
	//		float32(s.BlockEntity.AdditionalData.Position[2]),
	//	}
	//	// 初始化
	//	{
	//		matchA := s.IsNotLegacySignBlock && (s.SignData.FrontText.IgnoreLighting || s.SignData.BackText.IgnoreLighting)
	//		matchB := !s.IsNotLegacySignBlock && s.LegacySignData.IgnoreLighting
	//		if matchA || matchB {
	//			resp = gameInterface.SendWSCommandWithResponse(
	//				"replaceitem entity @s slot.hotbar 0 glow_ink_sac",
	//				ResourcesControl.CommandRequestOptions{
	//					TimeOut: ResourcesControl.CommandRequestNoDeadLine,
	//				},
	//			)
	//			if resp.Error != nil {
	//				return fmt.Errorf("WriteData: %v", resp.Error)
	//			}
	//			// 获取一个 发光墨囊 到快捷栏 0
	//		}
	//		// 取得 发光墨囊
	//		if (s.IsNotLegacySignBlock && s.SignData.FrontText.IgnoreLighting) || matchB {
	//			err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
	//			if err != nil {
	//				return fmt.Errorf("WriteData: %v", err)
	//			}
	//		}
	//		if s.IsNotLegacySignBlock && s.SignData.BackText.IgnoreLighting {
	//			playerPosition[0] = playerPosition[0] + 1
	//			err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
	//			if err != nil {
	//				return fmt.Errorf("WriteData: %v", err)
	//			}
	//		}
	//		// 使用 发光墨囊 点击告示牌的对应面以让该面发光
	//	}
	//	// 附加发光效果
	//}
	//// 告示牌发光效果
	//if s.IsNotLegacySignBlock && s.SignData.IsWaxed {
	//	resp = gameInterface.SendWSCommandWithResponse(
	//		"replaceitem entity @s slot.hotbar 0 honeycomb",
	//		ResourcesControl.CommandRequestOptions{
	//			TimeOut: ResourcesControl.CommandRequestNoDeadLine,
	//		},
	//	)
	//	if resp.Error != nil {
	//		return fmt.Errorf("WriteData: %v", resp.Error)
	//	}
	//	// 获取一个 蜜脾 到快捷栏 0
	//	err = gameInterface.ClickBlock(useItemOnBlocks)
	//	if err != nil {
	//		return fmt.Errorf("WriteData: %v", err)
	//	}
	//	// 封装告示牌
	//}
	//// 告示牌涂蜡
	//err = gameInterface.SetBlockAsync(s.BlockEntity.AdditionalData.Position, s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates)
	//if err != nil {
	//	return fmt.Errorf("WriteData: %v", err)
	//}
	//gameInterface := s.BlockEntity.Interface.(*GameInterface.GameInterface)
	//{
	//
	//}
	//is_air := gameInterface.SendWSCommandWithResponse(
	//	fmt.Sprintf("testforblock %d %d %d %s %s", s.BlockEntity.AdditionalData.Position[0], s.BlockEntity.AdditionalData.Position[1], s.BlockEntity.AdditionalData.Position[2], s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates),
	//	ResourcesControl.CommandRequestOptions{
	//		TimeOut: time.Second * 5,
	//	},
	//)

	gameInterface = s.BlockEntity.Interface.(*GameInterface.GameInterface)
	// if is_air.Respond.SuccessCount == 0 {
	// 返回值
	var uniqueID_1 uuid.UUID
	var uniqueID_2 uuid.UUID
	// var uniqueID_3 uuid.UUID
	var facing_direction int32
	// 解析 nbt
	var err error
	if _, ok := s.BlockEntity.Block.States["facing_direction"]; ok {
		facing_direction = s.BlockEntity.Block.States["facing_direction"].(int32)
		if s.IsHangingSignBlock {
			switch facing_direction {
			case 4:
				facing_direction = 3
			case 3:
				facing_direction = 5
			case 5:
				facing_direction = 2
			case 2:
				facing_direction = 4

			}
			s.BlockEntity.Block.States["facing_direction"] = facing_direction
		}
		useItemOnBlocks := GameInterface.UseItemOnBlocks{
			HotbarSlotID: 0,
			BlockPos:     s.BlockEntity.AdditionalData.Position,
			BlockStates:  s.BlockEntity.Block.States,
		}

		// 初始化变量
		// if s.BlockEntity.AdditionalData.FastMode {
		// 	err := s.BlockEntity.Interface.SetBlockAsync(s.BlockEntity.AdditionalData.Position, s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates)
		// 	if err != nil {
		// 		return fmt.Errorf("WriteData: %v", err)
		// 	}
		// 	return nil
		// }

		// 放置告示牌
		{
			err := gameInterface.SendAICommand(fmt.Sprintf("tp %d %d %d", s.BlockEntity.AdditionalData.Position[0], s.BlockEntity.AdditionalData.Position[1], s.BlockEntity.AdditionalData.Position[2]), true)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 传送机器人到告示牌所在的位置
			err = gameInterface.SendAICommand(
				fmt.Sprintf(
					"setblock %d %d %d air",
					s.BlockEntity.AdditionalData.Position[0],
					s.BlockEntity.AdditionalData.Position[1],
					s.BlockEntity.AdditionalData.Position[2],
				),
				true,
			)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 清除当前告示牌处的方块。
			// 如果不这么做且原本该处的方块是告示牌的话，
			// 那么 NBT 数据将会注入失败
			err = gameInterface.SendAICommand(
				fmt.Sprintf("replaceitem entity @s slot.hotbar 0 %s", s.Replace_sign_name(s.BlockEntity.Block.Name)),
				true,
			)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 获取一个告示牌到快捷栏 0
			err = gameInterface.ChangeSelectedHotbarSlot(0)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 切换手持物品栏到快捷栏 0
			uniqueID_1, err = gameInterface.BackupStructure(
				GameInterface.MCStructure{
					BeginX: int(s.BlockEntity.AdditionalData.Position[0] + func() int32 {
						if facing_direction == 4 {
							return 1
						} else if facing_direction == 5 {
							return -1
						} else {
							return 0
						}
					}()),
					BeginY: int(s.BlockEntity.AdditionalData.Position[1]),
					BeginZ: int(s.BlockEntity.AdditionalData.Position[2] + func() int32 {
						if facing_direction == 2 {
							return 1
						} else if facing_direction == 3 {
							return -1
						} else {
							return 0
						}
					}()),
					SizeX: 1,
					SizeY: 1,
					SizeZ: 1,
				},
			)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			/*
				我们会在告示牌的 (~1, ~, ~) 处生成一个玻璃，
				然后点击这个玻璃并指定点击的面是 4 以将手中的告示牌放上去。

				这样，我们就可以取得反作弊的认同，
				然后我们就可以向告示牌注入 NBT 数据了。

				但在生成玻璃前，我们需要备份这个玻璃原本的方块以方便之后恢复它
			*/
			err = gameInterface.SendAICommand(
				fmt.Sprintf(
					"setblock %d %d %d %s",
					s.BlockEntity.AdditionalData.Position[0]+func() int32 {
						if facing_direction == 4 {
							return 1
						} else if facing_direction == 5 {
							return -1
						} else {
							return 0
						}
					}(),
					s.BlockEntity.AdditionalData.Position[1],
					s.BlockEntity.AdditionalData.Position[2]+func() int32 {
						if facing_direction == 2 {
							return 1
						} else if facing_direction == 3 {
							return -1
						} else {
							return 0
						}
					}(),
					GameInterface.PlaceBlockBase,
				),
				true,
			)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			err = gameInterface.AwaitChangesGeneral()
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 生成上文提到的玻璃。
			// TODO: 优化上方这段代码

			err = gameInterface.PlaceBlock(
				GameInterface.UseItemOnBlocks{
					HotbarSlotID: 0,
					BlockPos: [3]int32{
						s.BlockEntity.AdditionalData.Position[0] + func() int32 {
							if facing_direction == 4 {
								return 1
							} else if facing_direction == 5 {
								return -1
							} else {
								return 0
							}
						}(),
						s.BlockEntity.AdditionalData.Position[1],
						s.BlockEntity.AdditionalData.Position[2] + func() int32 {
							if facing_direction == 2 {
								return 1
							} else if facing_direction == 3 {
								return -1
							} else {
								return 0
							}
						}(),
					},
					BlockName:   GameInterface.PlaceBlockBase,
					BlockStates: s.BlockEntity.Block.States,
				},
				facing_direction,
			)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 在玻璃上放置手中的告示牌
			err = gameInterface.SetBlockAsync(s.BlockEntity.AdditionalData.Position, s.BlockEntity.Block.Name, s.BlockEntity.AdditionalData.BlockStates)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 现在玻璃上有了一个告示牌，这是我们刚刚放上去的，
			// 但这个告示牌的种类是 oak_sign ，且朝向固定，
			// 因此现在我们需要覆写这个告示牌的种类及朝向为正确的形式。
			// 经过测试，覆写操作不会导致 NBT 数据无法注入
			// 放置告示牌
			signBlockNBTData := s.BlockEntity.Block.NBT
			if !s.IsNotLegacySignBlock {
				signBlockNBTData = map[string]any{"FrontText": s.BlockEntity.Block.NBT, "BackText": s.BlockEntity.Block.NBT}
			}
			err = gameInterface.WritePacket(&packet.BlockActorData{
				Position: s.BlockEntity.AdditionalData.Position,
				NBTData:  signBlockNBTData,
			})
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 写入告示牌数据
			uniqueID_2, err = gameInterface.BackupStructure(GameInterface.MCStructure{
				BeginX: int(s.BlockEntity.AdditionalData.Position[0]),
				BeginY: int(s.BlockEntity.AdditionalData.Position[1]),
				BeginZ: int(s.BlockEntity.AdditionalData.Position[2]),
				SizeX:  1,
				SizeY:  1,
				SizeZ:  1,
			})
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			/*
				备份告示牌处的方块。

				稍后我们会恢复上文提到的玻璃处的方块为原本方块，
				而此方块被恢复后，游戏会按照特性刷新它附近的方块，
				也就是告示牌方块。

				但我们无法保证刷新后，我们导入的告示牌仍然可以稳定存在，
				因为它可能会因为缺少依附方块而掉落。

				因此，我们现在备份一次告示牌，然后再恢复玻璃处的方块，
				然后再强行生成一次告示牌本身。

				注：这个解法并不优雅，而且会浪费时间，
				但它可以显著提高告示牌的存活概率，
				而且用户不希望为了告示牌而再导入一次 BDX 文件。

				TODO: 在某天推迟部分方块的导入顺序，
				使得告示牌这类依附型方块在最后再被导入
			*/
			err = gameInterface.RevertStructure(
				uniqueID_1,
				GameInterface.BlockPos{
					int(s.BlockEntity.AdditionalData.Position[0] + func() int32 {
						if facing_direction == 4 {
							return 1
						} else if facing_direction == 5 {
							return -1
						} else {
							return 0
						}
					}()),
					int(s.BlockEntity.AdditionalData.Position[1]),
					int(s.BlockEntity.AdditionalData.Position[2] + func() int32 {
						if facing_direction == 2 {
							return 1
						} else if facing_direction == 3 {
							return -1
						} else {
							return 0
						}
					}()),
				},
			)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			err = gameInterface.AwaitChangesGeneral()
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 将上文提到的玻璃处的方块恢复为原本的方块
			gameInterface.RevertStructure(
				uniqueID_2,
				GameInterface.BlockPos{
					int(s.BlockEntity.AdditionalData.Position[0]),
					int(s.BlockEntity.AdditionalData.Position[1]),
					int(s.BlockEntity.AdditionalData.Position[2]),
				},
			)
			err = gameInterface.AwaitChangesGeneral()
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
		}

		// 再强行生成一次告示牌本身以抑制其可能发生的掉落
		playerPosition := mgl32.Vec3{
			float32(s.BlockEntity.AdditionalData.Position[0]),
			float32(s.BlockEntity.AdditionalData.Position[1]),
			float32(s.BlockEntity.AdditionalData.Position[2]),
		}
		switch facing_direction {
		case 2:
			playerPosition[2] += 1
		case 3:
			playerPosition[2] -= 1
		case 4:
			playerPosition[0] += 1
		case 5:
			playerPosition[0] -= 1
		}
		matchA := s.IsNotLegacySignBlock && (s.SignData.FrontText.IgnoreLighting || s.SignData.BackText.IgnoreLighting)
		matchB := !s.IsNotLegacySignBlock && s.LegacySignData.IgnoreLighting
		if matchA || matchB { // IsHangingSignBlock
			resp := gameInterface.SendWSCommandWithResponse(
				"replaceitem entity @s slot.hotbar 0 glow_ink_sac",
				ResourcesControl.CommandRequestOptions{
					TimeOut: time.Second * 5,
				},
			)
			if resp.Error != nil {
				return fmt.Errorf("WriteData: %v", resp.Error)
			}
			// 获取一个 发光墨囊 到快捷栏 0
			err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
		}
		// 写入告示牌数据
		{
			var bestFrontColor [3]uint8
			var bestBackColor *[3]uint8
			playerPosition := mgl32.Vec3{
				float32(s.BlockEntity.AdditionalData.Position[0]),
				float32(s.BlockEntity.AdditionalData.Position[1]),
				float32(s.BlockEntity.AdditionalData.Position[2]),
			}
			facing_direction2 := 0
			switch facing_direction {
			case 2:
				facing_direction2 = 3
			case 3:
				facing_direction2 = 2
			case 4:
				facing_direction2 = 5
			case 5:
				facing_direction2 = 4
			default:
				facing_direction2 = 3
			}
			useItemOnBlocks2 := GameInterface.UseItemOnBlocks{
				HotbarSlotID: 0,
				BlockPos:     s.BlockEntity.AdditionalData.Position,
				BlockStates:  s.BlockEntity.Block.States, // map[string]interface{}{"facing_direction": facing_direction2},
			}
			useItemOnBlocks2.BlockStates["facing_direction"] = facing_direction2
			// 初始化
			if s.IsNotLegacySignBlock {
				frontRGB, _ := DecodeVarRGBA(s.SignData.FrontText.SignTextColor)
				backRGB, _ := DecodeVarRGBA(s.SignData.BackText.SignTextColor)
				bestFrontColor = SearchForBestColor(frontRGB, DefaultDyeColor)
				bestBackColorTemp := SearchForBestColor(backRGB, DefaultDyeColor)
				bestBackColor = &bestBackColorTemp
			} else {
				rgb, _ := DecodeVarRGBA(s.LegacySignData.SignTextColor)
				bestFrontColor = SearchForBestColor(rgb, DefaultDyeColor)
			}
			// 确定告示牌各面的颜色
			if bestFrontColor != [3]uint8{0, 0, 0} {
				dyeItemName := RGBToDyeItemName[bestFrontColor]
				// 确定染料的物品名
				resp := gameInterface.SendWSCommandWithResponse(
					fmt.Sprintf("replaceitem entity @s slot.hotbar 0 %s", dyeItemName),
					ResourcesControl.CommandRequestOptions{
						TimeOut: ResourcesControl.CommandRequestNoDeadLine,
					},
				)
				if resp.Error != nil {
					return fmt.Errorf("WriteData: %v", resp.Error)
				}
				// 获取对应的染料到快捷栏 0
				err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
				if err != nil {
					return fmt.Errorf("WriteData: %v", err)
				}
				// 告示牌正面染色
			}
			if bestBackColor != nil && *bestBackColor != [3]uint8{0, 0, 0} {
				dyeItemName := RGBToDyeItemName[*bestBackColor]
				// 确定染料的物品名
				resp := gameInterface.SendWSCommandWithResponse(
					fmt.Sprintf("replaceitem entity @s slot.hotbar 0 %s", dyeItemName),
					ResourcesControl.CommandRequestOptions{
						TimeOut: ResourcesControl.CommandRequestNoDeadLine,
					},
				)
				if resp.Error != nil {
					return fmt.Errorf("WriteData: %v", resp.Error)
				}
				// 获取对应的染料到快捷栏 0

				err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks2, playerPosition)
				if err != nil {
					return fmt.Errorf("WriteData: %v", err)
				}
				// 告示牌背面染色
			}
		}
		// 告示牌染色
		{
			playerPosition := mgl32.Vec3{
				float32(s.BlockEntity.AdditionalData.Position[0]),
				float32(s.BlockEntity.AdditionalData.Position[1]),
				float32(s.BlockEntity.AdditionalData.Position[2]),
			}

			// 初始化
			{
				matchA := s.IsNotLegacySignBlock && (s.SignData.FrontText.IgnoreLighting || s.SignData.BackText.IgnoreLighting)
				matchB := !s.IsNotLegacySignBlock && s.LegacySignData.IgnoreLighting
				if matchA || matchB {
					resp := gameInterface.SendWSCommandWithResponse(
						"replaceitem entity @s slot.hotbar 0 glow_ink_sac",
						ResourcesControl.CommandRequestOptions{
							TimeOut: ResourcesControl.CommandRequestNoDeadLine,
						},
					)
					if resp.Error != nil {
						return fmt.Errorf("WriteData: %v", resp.Error)
					}
					// 获取一个 发光墨囊 到快捷栏 0
				}
				// 取得 发光墨囊
				if (s.IsNotLegacySignBlock && s.SignData.FrontText.IgnoreLighting) || matchB {
					err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
					if err != nil {
						return fmt.Errorf("WriteData: %v", err)
					}
				}
				if (s.IsNotLegacySignBlock && s.SignData.BackText.IgnoreLighting) || matchB {
					useItemOnBlocks2 := GameInterface.UseItemOnBlocks{
						HotbarSlotID: 0,
						BlockPos:     s.BlockEntity.AdditionalData.Position,
						BlockStates:  s.BlockEntity.Block.States, // map[string]interface{}{"facing_direction": facing_direction2},
					}
					err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks2, playerPosition)
					if err != nil {
						return fmt.Errorf("WriteData: %v", err)
					}
				}
				// 使用 发光墨囊 点击告示牌的对应面以让该面发光
			}
			// 附加发光效果
		}
		// 告示牌发光效果
		{
			resp := gameInterface.SendWSCommandWithResponse(
				"replaceitem entity @s slot.hotbar 0 honeycomb",
				ResourcesControl.CommandRequestOptions{
					TimeOut: time.Second * 5,
				},
			)
			if resp.Error != nil {
				return fmt.Errorf("WriteData: %v", resp.Error)
			}
			// 获取一个 蜜脾 到快捷栏 0
			err = gameInterface.ClickBlockWitchPlayerPosition(useItemOnBlocks, playerPosition)
			if err != nil {
				return fmt.Errorf("WriteData: %v", err)
			}
			// 封装告示牌
		}
		// 告示牌涂蜡
		// if s.IsHangingSignBlock {
		// 	err = gameInterface.RevertStructure(
		// 		uniqueID_3,
		// 		GameInterface.BlockPos{
		// 			s.BlockEntity.AdditionalData.Position[0],
		// 			s.BlockEntity.AdditionalData.Position[1] + 1,
		// 			s.BlockEntity.AdditionalData.Position[2],
		// 		},
		// 	)
		// 	if err != nil {
		// 		return fmt.Errorf("WriteData: %v", err)
		// 	}
		// }

		return nil
		// 返回值
	} else {
		return nil
	}

	return nil
}
func (s *Sign) Replace_sign_name(name string) string {
	switch name {
	case "standing_sign":
		return "oak_sign"
	case "spruce_standing_sign":
		return "spruce_sign"
	case "birch_standing_sign":
		return "birch_sign"
	case "jungle_standing_sign":
		return "jungle_sign"
	case "acacia_standing_sign":
		return "acacia_sign"
	case "darkoak_standing_sign":
		return "dark_oak_sign"
	case "mangrove_standing_sign":
		return "mangrove_sign"
	case "cherry_standing_sign":
		return "cherry_sign"
	case "pale_oak_standing_sign":
		return "pale_oak_sign"
	case "bamboo_standing_sign":
		return "bamboo_sign"
	case "crimson_standing_sign":
		return "crimson_sign"
	case "warped_standing_sign":
		return "warped_sign"
	case "wall_sign":
		return "oak_sign"
	case "spruce_wall_sign":
		return "spruce_sign"
	case "birch_wall_sign":
		return "birch_sign"
	case "jungle_wall_sign":
		return "jungle_sign"
	case "acacia_wall_sign":
		return "acacia_sign"
	case "darkoak_wall_sign":
		return "dark_oak_sign"
	case "mangrove_wall_sign":
		return "mangrove_sign"
	case "cherry_wall_sign":
		return "cherry_sign"
	case "pale_oak_wall_sign":
		return "pale_oak_sign"
	case "bamboo_wall_sign":
		return "bamboo_sign"
	case "crimson_wall_sign":
		return "crimson_sign"
	case "warped_wall_sign":
		return "warped_sign"
	case "oak_hanging_sign":
		return "oak_hanging_sign"
	case "spruce_hanging_sign":
		return "spruce_hanging_sign"
	case "birch_hanging_sign":
		return "birch_hanging_sign"
	case "jungle_hanging_sign":
		return "jungle_hanging_sign"
	case "acacia_hanging_sign":
		return "acacia_hanging_sign"
	case "dark_oak_hanging_sign":
		return "dark_oak_hanging_sign"
	case "mangrove_hanging_sign":
		return "mangrove_hanging_sign"
	case "cherry_hanging_sign":
		return "cherry_hanging_sign"
	case "pale_oak_hanging_sign":
		return "pale_oak_hanging_sign"
	case "bamboo_hanging_sign":
		return "bamboo_hanging_sign"
	case "crimson_hanging_sign":
		return "crimson_hanging_sign"
	case "warped_hanging_sign":
		return "warped_hanging_sign"
	default:
		return "oak_sign"
	}
}

