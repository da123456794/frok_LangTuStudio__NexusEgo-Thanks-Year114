package mcstructure

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	types "nexus/defines"

	"github.com/LangTuStudio/Conbit/minecraft/nbt"
)

type Area struct {
	BeginX int32
	BeginY int32
	BeginZ int32
	SizeX  int32
	SizeY  int32
	SizeZ  int32
}

type AreaLocation [2]int

type BlockPos [3]int32

type Mcstructure struct {
	area                     Area
	blockPalette             []string
	blockPalette_blockStates []string
	// ^ NOTE: All TAG_BYTE values are treated as booleans
	blockPalette_blockData []int16
	foreground             []int16
	background             []int16
	blockNBT               map[int]map[string]interface{}
}

/*
用于将一个大区域按给定尺寸拆分成若干个小区域。
如果 useSpecialSplitWay 为 true，则按蛇形顺序拆分。

返回值依次为：
1. 拆分后的区域列表；
2. 区域坐标到切片下标的映射；
3. 切片下标到区域坐标的逆映射。
*/
func SplitArea(beginPos BlockPos, endPos BlockPos, splitSizeX int32, splitSizeZ int32, useSpecialSplitWay bool) ([]Area, map[AreaLocation]int, map[int]AreaLocation) {
	if splitSizeX < 0 {
		splitSizeX = -splitSizeX
	}
	if splitSizeZ < 0 {
		splitSizeZ = -splitSizeZ
	}

	// 兼容 beginPos 和 endPos 传反的情况，统一调整为从小到大。
	if endPos[0] < beginPos[0] {
		tmp := beginPos[0]
		beginPos[0] = endPos[0]
		endPos[0] = tmp
	}
	if endPos[1] < beginPos[1] {
		tmp := beginPos[1]
		beginPos[1] = endPos[1]
		endPos[1] = tmp
	}
	if endPos[2] < beginPos[2] {
		tmp := beginPos[2]
		beginPos[2] = endPos[2]
		endPos[2] = tmp
	}

	// 计算规范化后的区域尺寸。
	sizeX := endPos[0] - beginPos[0] + 1
	sizeY := endPos[1] - beginPos[1] + 1
	sizeZ := endPos[2] - beginPos[2] + 1

	// 计算 X/Z 方向需要拆分出的区域数。
	chunkXLength := int(math.Ceil(float64(sizeX) / float64(splitSizeX)))
	chunkZLength := int(math.Ceil(float64(sizeZ) / float64(splitSizeZ)))

	// 预分配拆分后的区域，以及正反向索引表。
	ret := make([]Area, chunkXLength*chunkZLength)
	areaLoctionToInt := map[AreaLocation]int{}
	intToAreaLoction := map[int]AreaLocation{}

	// facing 用于蛇形拆分，key 表示当前写入 ret 的位置。
	facing := -1
	key := -1
	for chunkX := 1; chunkX <= chunkXLength; chunkX++ {
		facing *= -1
		beginX := splitSizeX*(int32(chunkX)-1) + beginPos[0]
		xLength := splitSizeX
		if beginX+xLength-1 > endPos[0] {
			xLength = endPos[0] - beginX + 1
		}
		for chunkZ := 1; chunkZ <= chunkZLength; chunkZ++ {
			key++
			currentChunkZ := chunkZ
			if useSpecialSplitWay && facing == -1 {
				currentChunkZ = chunkZLength - currentChunkZ + 1
			}
			beginZ := splitSizeZ*(int32(currentChunkZ)-1) + beginPos[2]
			zLength := splitSizeZ
			if beginZ+zLength-1 > endPos[2] {
				zLength = endPos[2] - beginZ + 1
			}
			ret[key] = Area{
				BeginX: beginX,
				BeginY: beginPos[1],
				BeginZ: beginZ,
				SizeX:  xLength,
				SizeY:  sizeY,
				SizeZ:  zLength,
			}
			areaLoctionToInt[AreaLocation{chunkX - 1, currentChunkZ - 1}] = key
			intToAreaLoction[key] = AreaLocation{chunkX - 1, currentChunkZ - 1}
		}
	}
	return ret, areaLoctionToInt, intToAreaLoction
}

func GetMCStructureData(area Area, structure map[string]interface{}) (Mcstructure, error) {
	blockPalette := []string{}
	blockPaletteBlockStates := []string{}
	blockPaletteBlockData := []int16{}
	blockNBT := map[int]map[string]interface{}{}
	foreground := []int16{}
	background := []int16{}

	_, ok := structure["structure"]
	if !ok {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"]; structure = %#v", structure)
	}
	valueStructure, normal := structure["structure"].(map[string]interface{})
	if !normal {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"]; structure = %#v", structure)
	}

	_, ok = valueStructure["palette"]
	if !ok {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"]; structure = %#v", valueStructure)
	}
	valuePalette, normal := valueStructure["palette"].(map[string]interface{})
	if !normal {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"]; structure = %#v", valueStructure)
	}

	_, ok = valuePalette["default"]
	if !ok {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"]; palette = %#v", valuePalette)
	}
	valueDefault, normal := valuePalette["default"].(map[string]interface{})
	if !normal {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"]; palette = %#v", valuePalette)
	}

	_, ok = valueDefault["block_palette"]
	if !ok {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"]; default = %#v", valueDefault)
	}
	valueBlockPalette, normal := valueDefault["block_palette"].([]interface{})
	if !normal {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"]; default = %#v", valueDefault)
	}

	for key, value := range valueBlockPalette {
		got, normal := value.(map[string]interface{})
		if !normal {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"name\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}

		// 读取方块名称。
		_, ok = got["name"]
		if !ok {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"name\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}
		valueName, normal := got["name"].(string)
		if !normal {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"name\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}
		blockPalette = append(blockPalette, valueName)

		// 读取方块状态。
		// 这里的名称仍带有 minecraft: 命名空间，后续使用时再去掉。
		_, ok = got["states"]
		if !ok {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"states\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}
		valueStates, normal := got["states"].(map[string]interface{})
		if !normal {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"states\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}
		blockStates, err := MarshalBlockStates(valueStates)
		if err != nil {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"states\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}
		blockPaletteBlockStates = append(blockPaletteBlockStates, blockStates)

		// 读取附加数据值。
		_, ok = got["val"]
		if !ok {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"val\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}
		val, normal := got["val"].(int16)
		if !normal {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"palette\"][\"default\"][\"block_palette\"][%v][\"val\"]; block_palette[%v] = %#v", key, key, valueBlockPalette[key])
		}
		blockPaletteBlockData = append(blockPaletteBlockData, val)
	}

	// block_position_data 保存了与方块位置关联的 NBT 数据。
	_, ok = valueDefault["block_position_data"]
	if ok {
		valueBlockPositionData, normal := valueDefault["block_position_data"].(map[string]interface{})
		if !normal {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"default\"][\"block_position_data\"]; default = %#v", valueDefault)
		}
		for key, value := range valueBlockPositionData {
			blockPositionData, ok := value.(map[string]interface{})
			if !ok {
				return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"default\"][\"block_position_data\"][%v]; block_position_data[%v] = %#v", key, key, blockPositionData[key])
			}
			locationOfBlockPositionData, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"default\"][\"block_position_data\"][%v]; block_position_data[%v] = %#v", key, key, blockPositionData[key])
			}
			if blockNBT[int(locationOfBlockPositionData)] == nil {
				blockNBT[int(locationOfBlockPositionData)] = make(map[string]interface{})
			}
			blockNBT[int(locationOfBlockPositionData)] = map[string]interface{}{"block_position_data": blockPositionData}
		}
	}

	// 方块实体数据并不一定存在；找到后写入 blockNBT 映射。
	_, ok = valueStructure["block_indices"]
	if !ok {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"block_indices\"]; structure = %#v", structure)
	}
	valueBlockIndices, normal := valueStructure["block_indices"].([]interface{})
	if !normal {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"block_indices\"]; structure = %#v", structure)
	}
	if len(valueBlockIndices) != 2 {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"block_indices\"]; structure = %#v", structure)
	}

	// block_indices 固定包含前景层和背景层两个索引表。
	valueBlockIndices0, normal := valueBlockIndices[0].([]interface{})
	if !normal {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"block_indices\"][0]; block_indices = %#v", valueBlockIndices)
	}
	for blockLocationKey, blockLocation := range valueBlockIndices0 {
		got, normal := blockLocation.(int32)
		if !normal {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"block_indices\"][0][%v]; block_indices[0] = %#v", blockLocationKey, valueBlockIndices[0])
		}
		foreground = append(foreground, int16(got))
	}

	// 读取背景层方块索引。
	valueBlockIndices1, normal := valueBlockIndices[1].([]interface{})
	if !normal {
		return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"block_indices\"][1]; block_indices = %#v", valueBlockIndices)
	}
	for blockLocationKey, blockLocation := range valueBlockIndices1 {
		got, normal := blockLocation.(int32)
		if !normal {
			return Mcstructure{}, fmt.Errorf("GetMCStructureData: Failed on structure[\"structure\"][\"block_indices\"][1][%v]; block_indices[1] = %#v", blockLocationKey, valueBlockIndices[1])
		}
		background = append(background, int16(got))
	}

	// 组装并返回解析结果。
	return Mcstructure{
		area:                     area,
		blockPalette:             blockPalette,
		blockPalette_blockStates: blockPaletteBlockStates,
		blockPalette_blockData:   blockPaletteBlockData,
		foreground:               foreground,
		background:               background,
		blockNBT:                 blockNBT,
	}, nil
}

// 根据结构尺寸和绝对坐标，计算该方块在 mcstructure 中的一维索引。
func SearchForBlock(structureInfo Area, pos BlockPos) (int, error) {
	pos[0] -= structureInfo.BeginX
	pos[1] -= structureInfo.BeginY
	pos[2] -= structureInfo.BeginZ

	// 将绝对坐标转换成相对坐标后，计算总方块数与目标索引。
	blockCount := structureInfo.SizeX * structureInfo.SizeY * structureInfo.SizeZ
	angleMark := structureInfo.SizeY*structureInfo.SizeZ*pos[0] + structureInfo.SizeZ*pos[1] + pos[2]
	if angleMark > blockCount-1 {
		return -1, fmt.Errorf("SearchForBlock: Index out of the list, occurred in input[%v]", angleMark)
	}
	return int(angleMark), nil
}

/*
DumpBlocks 会将导出区域重排成可直接写出的方块列表，并携带对应的 NBT 数据。

它会先把 currentExport 按 16x16 蛇形拆分，再根据 reversedMap 找到每个子区块所属的
mcstructure，最后展开为 Module 列表。
*/
func DumpBlocks(
	allAreas []Mcstructure,
	reversedMap map[AreaLocation]int,
	currentExport Area,
) ([]*types.Module, error) {
	ans := make([]*types.Module, 0)
	allChunks, _, chunkPosIndicator := SplitArea(
		BlockPos{currentExport.BeginX, currentExport.BeginY, currentExport.BeginZ},
		BlockPos{
			currentExport.BeginX + currentExport.SizeX - 1,
			currentExport.BeginY + currentExport.SizeY - 1,
			currentExport.BeginZ + currentExport.SizeZ - 1,
		},
		16, 16, true,
	)

	// 将导出区域按 16x16 拆分后，按得到的顺序逐块重排。
	for key, value := range allChunks {
		chunkPos := chunkPosIndicator[key]
		chunkPos[0] = int(math.Floor(float64(chunkPos[0]) / 4))
		chunkPos[1] = int(math.Floor(float64(chunkPos[1]) / 4))

		// 将当前 16x16 子块映射回所属的 mcstructure 大区块。
		targetAreaPos := reversedMap[chunkPos]
		targetArea := allAreas[targetAreaPos]

		// 枚举当前子块中的所有方块坐标。
		i, _, _ := SplitArea(
			BlockPos{value.BeginX, value.BeginY, value.BeginZ},
			BlockPos{
				value.BeginX + value.SizeX - 1,
				value.BeginY + value.SizeY - 1,
				value.BeginZ + value.SizeZ - 1,
			},
			1, 1, true,
		)

		allBlocksInCurrentChunk := make([]int32, 0)
		for _, val := range i {
			got, err := SearchForBlock(targetArea.area, BlockPos{
				val.BeginX,
				val.BeginY,
				val.BeginZ,
			})
			if err != nil {
				return []*types.Module{}, fmt.Errorf("DumpBlocks: %v", err)
			}
			allBlocksInCurrentChunk = append(allBlocksInCurrentChunk, int32(got))
		}

		// 遍历当前子块中的每个方块，并按 Y 轴逐层展开。
		for key, val := range allBlocksInCurrentChunk {
			// 先回退一个 SizeZ，便于在循环里统一递增到当前层。
			val -= int32(targetArea.area.SizeZ)
			for j := int32(0); j < targetArea.area.SizeY; j++ {
				val += int32(targetArea.area.SizeZ)

				// 初始化前景和背景方块信息。
				foregroundBlockName := "undefined"
				backgroundBlockName := "undefined"
				foregroundBlockStates := "undefined"
				backgroundBlockStates := "undefined"

				fgID := targetArea.foreground[val] // 前景层方块在调色板中的 id
				bgID := targetArea.background[val] // 背景层方块在调色板中的 id
				if fgID != -1 {
					foregroundBlockName = strings.Replace(targetArea.blockPalette[fgID], "minecraft:", "", 1) // 前景层方块名称
					foregroundBlockStates = targetArea.blockPalette_blockStates[fgID]                         // 前景层方块状态
				}
				if bgID != -1 {
					backgroundBlockName = strings.Replace(targetArea.blockPalette[bgID], "minecraft:", "", 1) // 背景层方块名称
					backgroundBlockStates = targetArea.blockPalette_blockStates[bgID]                         // 背景层方块状态
				}
				if fgID == -1 && bgID == -1 {
					foregroundBlockName = "structure_void"
					foregroundBlockStates = "[]"
				}

				// 初始化 NBT 状态；这段初始化不能挪出循环，否则不同方块之间会串数据。
				hasNBT := false
				var blockNBT []byte
				var err error

				got, ok := targetArea.blockNBT[int(val)]
				if ok {
					_, ok := got["block_position_data"]
					if !ok {
						return []*types.Module{}, fmt.Errorf("DumpBlocks: Crashed by could not found \"block_position_data\", occurred in %#v", targetArea.blockNBT[int(val)])
					}
					blockPositionData, normal := got["block_position_data"].(map[string]interface{})
					if !normal {
						return []*types.Module{}, fmt.Errorf("DumpBlocks: Crashed by invalid \"block_position_data\", occurred in %#v", got["block_position_data"])
					}

					// 只要记录了 NBT，理论上就会存在 block_position_data。
					_, ok = blockPositionData["block_entity_data"]
					// block_entity_data 可能不存在；没有时说明它不是方块实体，不应报错。
					if ok {
						blockEntityData, normal := blockPositionData["block_entity_data"].(map[string]interface{})
						if !normal {
							return []*types.Module{}, fmt.Errorf("DumpBlocks: Crashed by invalid \"block_entity_data\", occurred in %#v", blockPositionData["block_entity_data"])
						}

						if foregroundBlockName == "chest" || foregroundBlockName == "trapped_chest" {
							useOfChest := "chest"
							if foregroundBlockName == "chest" {
								useOfChest = "trapped_chest"
							}

							// 为避免箱子自动连接，先放一个相反类型的占位方块。
							ans = append(ans, &types.Module{
								Block: &types.Block{
									Name: &useOfChest,
									Data: 0,
								},
								Point: types.Position{
									X: int(i[key].BeginX - currentExport.BeginX),
									Y: int(i[key].BeginY + j - currentExport.BeginY),
									Z: int(i[key].BeginZ - currentExport.BeginZ),
								},
							})
						}

						// 标记并序列化 NBT 数据。
						hasNBT = true
						blockNBT, err = nbt.MarshalEncoding(blockEntityData, nbt.LittleEndian)
						if err != nil {
							return []*types.Module{}, fmt.Errorf("DumpBlocks: %v", err)
						}
					}
				}

				// 先放背景中的水方块。
				if foregroundBlockName != "undefined" && (backgroundBlockName == "water" || backgroundBlockName == "flowing_water") {
					ans = append(ans, &types.Module{
						Block: &types.Block{
							Name:        &backgroundBlockName,
							BlockStates: backgroundBlockStates,
						},
						Point: types.Position{
							X: int(i[key].BeginX - currentExport.BeginX),
							Y: int(i[key].BeginY + j - currentExport.BeginY),
							Z: int(i[key].BeginZ - currentExport.BeginZ),
						},
					})
				}

				// 含水方块会被拆成 water + targetBlock 两条指令。
				if foregroundBlockName != "air" && foregroundBlockName != "undefined" {
					single := &types.Module{
						Block: &types.Block{
							Name: &foregroundBlockName,
						},
						Point: types.Position{
							X: int(i[key].BeginX - currentExport.BeginX),
							Y: int(i[key].BeginY + j - currentExport.BeginY),
							Z: int(i[key].BeginZ - currentExport.BeginZ),
						},
					}
					if hasNBT {
						single.NBTData = blockNBT
					}
					single.Block.BlockStates = foregroundBlockStates
					ans = append(ans, single)
				}
			}
		}
	}
	return ans, nil
}

