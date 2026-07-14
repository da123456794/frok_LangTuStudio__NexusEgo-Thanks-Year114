package structure

import (
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	bwo_define "github.com/TriM-Organization/bedrock-world-operator/define"
	"github.com/TriM-Organization/bedrock-world-operator/world"
	"github.com/Yeah114/WaterStructure/define"
	"github.com/Yeah114/WaterStructure/utils/nbt"
	"github.com/Yeah114/blocks"
)

type Schematic struct {
	BaseReader
	file                *os.File
	size                *define.Size // [x, y, z] 当前尺寸（原始尺寸+偏移扩展部分）
	originalSize        *define.Size // [x, y, z] 原始建筑尺寸
	Origin              *define.Origin
	Offset              *define.Offset
	offsetPos           define.Offset // 建筑在新尺寸中的偏移量（相对于原始位置）
	Materials           string
	EntityNBT           []map[string]any
	BlockNBT            []map[string]any
	BlocksTagGzipOffset int64
	DataTagGzipOffset   int64
	BlocksLength        int // Blocks标签的数据长度（字节数）
	DataLength          int // Data标签的数据长度（字节数）
	blockIndex          []byte // 完整的方块ID数组
	blockData           []byte // 完整的方块数据数组
}

func (s *Schematic) ID() uint8 {
	return IDSchematic
}

func (s *Schematic) Name() string {
	return NameSchematic
}

func (s *Schematic) FromFile(file *os.File) error {
	s.file = file
	s.size = &define.Size{}
	s.originalSize = &define.Size{}
	s.Origin = &define.Origin{}
	s.Offset = &define.Offset{}
	s.Materials = "Alpha"

	gzipReader, err := gzip.NewReader(s.file)
	if err != nil {
		return fmt.Errorf("创建 gzip 读取器失败: %w", err)
	}
	defer gzipReader.Close()

	tagReader := nbt.NewTagReader(nbt.BigEndian)
	offsetReader := nbt.NewOffsetReader(gzipReader)

	rootTagType, rootTagName, err := tagReader.ReadTag(offsetReader)
	if err != nil {
		return fmt.Errorf("读取根标签失败: %w", err)
	}

	if rootTagType != nbt.TagStruct {
		return ErrInvalidRootTagType
	}

	if rootTagName != "Schematic" {
		return ErrInvalidRootTagName
	}

	for {
		tagType, tagName, err := tagReader.ReadTag(offsetReader)
		if err != nil {
			return fmt.Errorf("读取标签失败: %w", err)
		}

		if tagType == nbt.TagEnd {
			break
		}

		switch tagName {
		case "Width":
			if tagType != nbt.TagInt16 {
				return fmt.Errorf("期望 Width 为 TAG_Short, 实际为 %s", tagType)
			}
			width, err := tagReader.ReadTagInt16(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 Width 失败: %w", err)
			}
			s.size.Width = int(width)
			s.originalSize.Width = int(width)

		case "Height":
			if tagType != nbt.TagInt16 {
				return fmt.Errorf("期望 Height 为 TAG_Short, 实际为 %s", tagType)
			}
			height, err := tagReader.ReadTagInt16(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 Height 失败: %w", err)
			}
			s.size.Height = int(height)
			s.originalSize.Height = int(height)

		case "Length":
			if tagType != nbt.TagInt16 {
				return fmt.Errorf("期望 Length 为 TAG_Short, 实际为 %s", tagType)
			}
			length, err := tagReader.ReadTagInt16(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 Length 失败: %w", err)
			}
			s.size.Length = int(length)
			s.originalSize.Length = int(length)

		case "WEOriginX":
			if tagType != nbt.TagInt32 {
				return fmt.Errorf("期望 WEOriginX 为 TAG_Int, 实际为 %s", tagType)
			}
			x, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 WEOriginX 失败: %w", err)
			}
			s.Origin[0] = x

		case "WEOriginY":
			if tagType != nbt.TagInt32 {
				return fmt.Errorf("期望 WEOriginY 为 TAG_Int, 实际为 %s", tagType)
			}
			y, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 WEOriginY 失败: %w", err)
			}
			s.Origin[1] = y

		case "WEOriginZ":
			if tagType != nbt.TagInt32 {
				return fmt.Errorf("期望 WEOriginZ 为 TAG_Int, 实际为 %s", tagType)
			}
			z, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 WEOriginZ 失败: %w", err)
			}
			s.Origin[2] = z

		case "WEOffsetX":
			if tagType != nbt.TagInt32 {
				return fmt.Errorf("期望 WEOffsetX 为 TAG_Int, 实际为 %s", tagType)
			}
			x, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 WEOffsetX 失败: %w", err)
			}
			s.Offset[0] = x

		case "WEOffsetY":
			if tagType != nbt.TagInt32 {
				return fmt.Errorf("期望 WEOffsetY 为 TAG_Int, 实际为 %s", tagType)
			}
			y, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 WEOffsetY 失败: %w", err)
			}
			s.Offset[1] = y

		case "WEOffsetZ":
			if tagType != nbt.TagInt32 {
				return fmt.Errorf("期望 WEOffsetZ 为 TAG_Int, 实际为 %s", tagType)
			}
			z, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 WEOffsetZ 失败: %w", err)
			}
			s.Offset[2] = z

		case "Materials":
			if tagType != nbt.TagString {
				return fmt.Errorf("期望 Materials 为 TAG_String, 实际为 %s", tagType)
			}
			materials, err := tagReader.ReadTagString(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 Materials 失败: %w", err)
			}
			s.Materials = materials

		case "Entities":
			if tagType != nbt.TagSlice {
				return fmt.Errorf("期望 Entities 为 TAG_List, 实际为 %s", tagType)
			}
			entityNBT, err := tagReader.ReadTagList(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 Entities 失败: %w", err)
			}
			entities := make([]map[string]any, len(entityNBT))
			for i, entity := range entityNBT {
				if entityMap, ok := entity.(map[string]any); ok {
					entities[i] = entityMap
				} else {
					return fmt.Errorf("期望 entity 为 map[string]any, 实际为 %T", entity)
				}
			}
			s.EntityNBT = entities

		case "TileEntities":
			if tagType != nbt.TagSlice {
				return fmt.Errorf("期望 TileEntities 为 TAG_List, 实际为 %s", tagType)
			}
			blockNBT, err := tagReader.ReadTagList(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 TileEntities 失败: %w", err)
			}
			blocks := make([]map[string]any, len(blockNBT))
			for i, block := range blockNBT {
				if blockMap, ok := block.(map[string]any); ok {
					blocks[i] = blockMap
				} else {
					return fmt.Errorf("期望 block 为 map[string]any, 实际为 %T", block)
				}
			}
			s.BlockNBT = blocks

		case "Blocks":
			s.BlocksTagGzipOffset = offsetReader.GetOffset()
			// 读取Blocks标签的字节数组长度
			if tagType != nbt.TagByteArray {
				return fmt.Errorf("期望 Blocks 为 TAG_Byte_Array, 实际为 %s", tagType)
			}
			length, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 Blocks 长度失败: %w", err)
			}
			s.BlocksLength = int(length)
			// 读取实际数据
			s.blockIndex = make([]byte, length)
			if _, err := io.ReadFull(offsetReader, s.blockIndex); err != nil {
				return fmt.Errorf("读取 Blocks 数据失败: %w", err)
			}

		case "Data":
			s.DataTagGzipOffset = offsetReader.GetOffset()
			// 读取Data标签的字节数组长度
			if tagType != nbt.TagByteArray {
				return fmt.Errorf("期望 Data 为 TAG_Byte_Array, 实际为 %s", tagType)
			}
			length, err := tagReader.ReadTagInt32(offsetReader)
			if err != nil {
				return fmt.Errorf("读取 Data 长度失败: %w", err)
			}
			s.DataLength = int(length)
			// 读取实际数据
			s.blockData = make([]byte, length)
			if _, err := io.ReadFull(offsetReader, s.blockData); err != nil {
				return fmt.Errorf("读取 Data 数据失败: %w", err)
			}

		default:
			err = tagReader.SkipTagValue(offsetReader, tagType)
			if err != nil {
				return fmt.Errorf("跳过标签 %s 失败: %w", tagName, err)
			}
		}
	}

	// 验证数据完整性
	volume := s.originalSize.Width * s.originalSize.Height * s.originalSize.Length
	if len(s.blockIndex) != volume {
		return fmt.Errorf("Blocks数据长度不匹配: 期望 %d, 实际 %d", volume, len(s.blockIndex))
	}
	if len(s.blockData) != volume {
		return fmt.Errorf("Data数据长度不匹配: 期望 %d, 实际 %d", volume, len(s.blockData))
	}

	return nil
}

func (s *Schematic) GetOffsetPos() define.Offset {
	return s.offsetPos
}

// SetOffsetPos 调整偏移并扩展尺寸, 使原始建筑偏移后保留, 周围填充空气
// 偏移量会使尺寸扩大, 以包含原始建筑和偏移产生的空气区域
func (s *Schematic) SetOffsetPos(offset define.Offset) {
	// 保存新的偏移位置
	s.offsetPos = offset

	// 计算需要扩展的尺寸: 原始尺寸 + 偏移量的绝对值（确保包含所有区域）
	// 例如: 原始宽16, 偏移X=16 → 新宽=16+16=32
	s.size.Width = s.originalSize.Width + int(math.Abs(float64(offset.X())))
	s.size.Length = s.originalSize.Length + int(math.Abs(float64(offset.Z())))
	s.size.Height = s.originalSize.Height + int(math.Abs(float64(offset.Y())))
}

// GetSize 返回当前尺寸（原始尺寸+偏移扩展部分）
func (s *Schematic) GetSize() define.Size {
	return *s.size
}

func (s *Schematic) GetChunks(posList []define.ChunkPos) (map[define.ChunkPos]*chunk.Chunk, error) {
	chunks := make(map[define.ChunkPos]*chunk.Chunk)
	// 初始化所有请求的区块为空气
	for _, pos := range posList {
		chunks[pos] = chunk.NewChunk(block.AirRuntimeID, MCWorldOverworldRange)
	}

	// 原始建筑的尺寸 (注意：schematic中顺序是 Width(x), Height(y), Length(z))
	origWidth := s.originalSize.Width   // x
	origHeight := s.originalSize.Height // y
	origLength := s.originalSize.Length // z

	// 偏移量（建筑在新尺寸中的位置）
	offsetX := int(s.offsetPos.X())
	offsetY := int(s.offsetPos.Y())
	offsetZ := int(s.offsetPos.Z())

	// 遍历所有方块位置
	for y := 0; y < origHeight; y++ {
		newY := y + offsetY
		if newY < 0 || newY >= s.size.Height {
			continue
		}

		for z := 0; z < origLength; z++ {
			newZ := z + offsetZ
			if newZ < 0 || newZ >= s.size.Length {
				continue
			}

			for x := 0; x < origWidth; x++ {
				newX := x + offsetX
				if newX < 0 || newX >= s.size.Width {
					continue
				}

				// 计算原始建筑中的索引 - 按照 yzx 顺序 (x变化最频繁)
				index := (y*origLength+z)*origWidth + x
				
				// 获取方块ID和数据
				blockID := s.blockIndex[index]
				blockData := s.blockData[index]

				// 跳过空气
				if blockID == 0 {
					continue
				}

				// 计算区块位置
				chunkX := int32(newX / 16)
				chunkZ := int32(newZ / 16)
				localX := uint8(newX % 16)
				localZ := uint8(newZ % 16)
				localY := int16(newY)

				// 获取区块
				c, ok := chunks[define.ChunkPos{chunkX, chunkZ}]
				if !ok {
					continue
				}

				// 转换方块ID - 只取data的低4位
				runtimeID := blocks.SchematicToRuntimeID(blockID, blockData&0x0F)
				baseName, properties, found := blocks.RuntimeIDToState(runtimeID)
				blockRuntimeID := block.AirRuntimeID
				if found {
					if rtid, found := block.StateToRuntimeID("minecraft:"+baseName, properties); found {
						blockRuntimeID = rtid
					}
				}

				// 设置方块
				c.SetBlock(localX, localY-64, localZ, 0, blockRuntimeID)
			}
		}
	}

	return chunks, nil
}

// GetChunksNBT 获取指定chunk位置的NBT数据
func (s *Schematic) GetChunksNBT(posList []define.ChunkPos) (map[define.ChunkPos]map[define.BlockPos]map[string]any, error) {
	return nil, nil
}

func (s *Schematic) CountNonAirBlocks() (int, error) {
	nonAirBlocks := 0
	for _, b := range s.blockIndex {
		if b != 0 {
			nonAirBlocks++
		}
	}
	return nonAirBlocks, nil
}

func (s *Schematic) ToMCWorld(
	bedrockWorld *world.BedrockWorld,
	startSubChunkPos bwo_define.SubChunkPos,
	startCallback func(num int),
	progressCallback func(),
) error {
	width := s.originalSize.Width   // x
	height := s.originalSize.Height // y
	length := s.originalSize.Length // z
	
	chunkCount := ((width + 15) / 16) * ((length + 15) / 16)
	subChunkYNum := (height + 15) / 16
	
	if startCallback != nil {
		startCallback(subChunkYNum)
	}

	// 按子区块维度处理
	for subChunkYIndex := 0; subChunkYIndex < subChunkYNum; subChunkYIndex++ {
		subChunks := make([]*chunk.SubChunk, chunkCount)
		startY := subChunkYIndex * 16
		endY := startY + 16
		if endY > height {
			endY = height
		}

		// 遍历当前Y范围的所有方块
		for y := startY; y < endY; y++ {
			for z := 0; z < length; z++ {
				for x := 0; x < width; x++ {
					// 计算索引 - yzx顺序
					index := (y*length+z)*width + x
					blockID := s.blockIndex[index]
					
					// 跳过空气
					if blockID == 0 {
						continue
					}
					
					blockData := s.blockData[index]

					// 计算区块和子区块位置
					chunkX := x / 16
					chunkZ := z / 16
					subChunkIndex := chunkZ*((width+15)/16) + chunkX
					localX := byte(x % 16)
					localY := byte(y % 16)
					localZ := byte(z % 16)

					// 初始化子区块
					if subChunks[subChunkIndex] == nil {
						subChunks[subChunkIndex] = chunk.NewSubChunk(block.AirRuntimeID)
					}

					// 转换方块ID
					runtimeID := blocks.SchematicToRuntimeID(blockID, blockData&0x0F)
					baseName, properties, found := blocks.RuntimeIDToState(runtimeID)
					blockRuntimeID := block.AirRuntimeID
					if found {
						if rtid, found := block.StateToRuntimeID("minecraft:"+baseName, properties); found {
							blockRuntimeID = rtid
						}
					}

					// 设置方块
					subChunks[subChunkIndex].SetBlock(localX, localY, localZ, 0, blockRuntimeID)
				}
			}
		}

		// 保存子区块
		XChunkCount := (width + 15) / 16
		for index, subChunk := range subChunks {
			if subChunk == nil {
				continue
			}
			chunkX := index % XChunkCount
			chunkZ := index / XChunkCount
			subChunkPos := bwo_define.SubChunkPos{
				int32(chunkX) + startSubChunkPos.X(),
				int32(subChunkYIndex) + startSubChunkPos.Y(),
				int32(chunkZ) + startSubChunkPos.Z(),
			}
			if err := bedrockWorld.SaveSubChunk(bwo_define.DimensionIDOverworld, subChunkPos, subChunk); err != nil {
				return fmt.Errorf("保存子区块 %v 失败: %w", subChunkPos, err)
			}
		}
		
		if progressCallback != nil {
			go progressCallback()
		}
	}

	return nil
}

func (s *Schematic) FromMCWorld(
	world *world.BedrockWorld,
	target *os.File,
	point1BlockPos define.BlockPos,
	point2BlockPos define.BlockPos,
	startCallback func(int),
	progressCallback func(),
) error {
	startBlockPos := define.BlockPos{
		min(point1BlockPos.X(), point2BlockPos.X()),
		min(point1BlockPos.Y(), point2BlockPos.Y()),
		min(point1BlockPos.Z(), point2BlockPos.Z()),
	}
	startBlockPosX := startBlockPos.X()
	startBlockPosY := startBlockPos.Y()
	startBlockPosZ := startBlockPos.Z()

	endBlockPos := define.BlockPos{
		max(point1BlockPos.X(), point2BlockPos.X()),
		max(point1BlockPos.Y(), point2BlockPos.Y()),
		max(point1BlockPos.Z(), point2BlockPos.Z()),
	}
	endBlockPosX := endBlockPos.X()
	endBlockPosY := endBlockPos.Y()
	endBlockPosZ := endBlockPos.Z()

	width := endBlockPosX - startBlockPosX + 1
	height := endBlockPosY - startBlockPosY + 1
	length := endBlockPosZ - startBlockPosZ + 1
	volume := width * height * length

	startSubChunkPos := define.SubChunkPos{
		(startBlockPosX - mod(startBlockPosX, 16)) / 16,
		(startBlockPosY - mod(startBlockPosY, 16)) / 16,
		(startBlockPosZ - mod(startBlockPosZ, 16)) / 16,
	}

	endSubChunkPos := define.SubChunkPos{
		(endBlockPosX + mod(endBlockPosX, 16) + 15) / 16,
		(endBlockPosY + mod(endBlockPosY, 16) + 15) / 16,
		(endBlockPosZ + mod(endBlockPosZ, 16) + 15) / 16,
	}

	startSubChunkPosX := startSubChunkPos.X()
	startSubChunkPosY := startSubChunkPos.Y()
	startSubChunkPosZ := startSubChunkPos.Z()
	subChunkXNum := endSubChunkPos.X() - startSubChunkPosX + 1
	subChunkYNum := endSubChunkPos.Y() - startSubChunkPosY + 1
	subChunkZNum := endSubChunkPos.Z() - startSubChunkPosZ + 1
	chunkCount := subChunkXNum * subChunkZNum
	if startCallback != nil {
		startCallback(int(subChunkYNum) * 2)
	}

	gzipWriter, err := gzip.NewWriterLevel(target, gzip.BestSpeed)
	if err != nil {
		return err
	}
	defer gzipWriter.Close()
	tagWriter := nbt.NewTagWriter(nbt.BigEndian)
	offsetWriter := nbt.NewOffsetWriter(gzipWriter)
	err = tagWriter.WriteTag(offsetWriter, nbt.TagStruct, "Schematic")
	if err != nil {
		return err
	}

	err = tagWriter.WriteTag(offsetWriter, nbt.TagInt16, "Length")
	if err != nil {
		return err
	}
	err = tagWriter.WriteTagInt16(offsetWriter, int16(length))
	if err != nil {
		return err
	}

	err = tagWriter.WriteTag(offsetWriter, nbt.TagInt16, "Height")
	if err != nil {
		return err
	}
	err = tagWriter.WriteTagInt16(offsetWriter, int16(height))
	if err != nil {
		return err
	}

	err = tagWriter.WriteTag(offsetWriter, nbt.TagInt16, "Width")
	if err != nil {
		return err
	}
	err = tagWriter.WriteTagInt16(offsetWriter, int16(width))
	if err != nil {
		return err
	}

	err = tagWriter.WriteTag(offsetWriter, nbt.TagString, "Materials")
	if err != nil {
		return err
	}
	err = tagWriter.WriteTagString(offsetWriter, "Alpha")
	if err != nil {
		return err
	}

	err = tagWriter.WriteTag(offsetWriter, nbt.TagByteArray, "Blocks")
	if err != nil {
		return err
	}

	err = tagWriter.WriteTagInt32(offsetWriter, volume)
	if err != nil {
		return err
	}
	for subChunkY := range subChunkYNum {
		worldSubChunkPosY := startSubChunkPosY + subChunkY
		subChunkWorldYStart := worldSubChunkPosY * 16
		subChunkWorldYEnd := subChunkWorldYStart + 15
		effectiveWorldYStart := max(subChunkWorldYStart, startBlockPosY)
		effectiveWorldYEnd := min(subChunkWorldYEnd, endBlockPosY)
		if effectiveWorldYStart > effectiveWorldYEnd {
			if progressCallback != nil {
				progressCallback()
			}
			continue
		}
		subChunks := make(map[bwo_define.SubChunkPos]*chunk.SubChunk, chunkCount)

		for localY := byte(effectiveWorldYStart - subChunkWorldYStart); localY <= byte(effectiveWorldYEnd-subChunkWorldYStart); localY++ {
			for subChunkZ := range subChunkZNum {
				worldSubChunkPosZ := startSubChunkPosZ + subChunkZ
				subChunkWorldZStart := worldSubChunkPosZ * 16
				subChunkWorldZEnd := subChunkWorldZStart + 15
				effectiveWorldZStart := max(subChunkWorldZStart, startBlockPosZ)
				effectiveWorldZEnd := min(subChunkWorldZEnd, endBlockPosZ)
				if effectiveWorldZStart > effectiveWorldZEnd {
					continue
				}
				for localZ := byte(effectiveWorldZStart - subChunkWorldZStart); localZ <= byte(effectiveWorldZEnd-subChunkWorldZStart); localZ++ {
					for subChunkX := range subChunkXNum {
						worldSubChunkPosX := startSubChunkPosX + subChunkX
						subChunkWorldXStart := worldSubChunkPosX * 16
						subChunkWorldXEnd := subChunkWorldXStart + 15
						effectiveWorldXStart := max(subChunkWorldXStart, startBlockPosX)
						effectiveWorldXEnd := min(subChunkWorldXEnd, endBlockPosX)
						if effectiveWorldXStart > effectiveWorldXEnd {
							continue
						}
						worldSubChunkPos := bwo_define.SubChunkPos{
							worldSubChunkPosX,
							worldSubChunkPosY,
							worldSubChunkPosZ,
						}
						for localX := byte(effectiveWorldXStart - subChunkWorldXStart); localX <= byte(effectiveWorldXEnd-subChunkWorldXStart); localX++ {
							subChunk, ok := subChunks[worldSubChunkPos]
							if !ok {
								subChunk = world.LoadSubChunk(bwo_define.DimensionIDOverworld, worldSubChunkPos)
								if subChunk == nil {
									subChunk = chunk.NewSubChunk(block.AirRuntimeID)
								}
								subChunks[worldSubChunkPos] = subChunk
							}
							blockRuntimeID := subChunk.Block(byte(localX), byte(localY), byte(localZ), 0)
							name, properties, _ := block.RuntimeIDToState(blockRuntimeID)
							runtimeID, _ := blocks.BlockNameAndStateToRuntimeID(name, properties)
							block, _, _ := blocks.RuntimeIDToSchematic(runtimeID)
							_, err := offsetWriter.Write([]byte{block})
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
		if progressCallback != nil {
			progressCallback()
		}
	}

	err = tagWriter.WriteTag(offsetWriter, nbt.TagByteArray, "Data")
	if err != nil {
		return err
	}

	err = tagWriter.WriteTagInt32(offsetWriter, volume)
	if err != nil {
		return err
	}

	for subChunkY := range subChunkYNum {
		worldSubChunkPosY := startSubChunkPosY + subChunkY
		subChunkWorldYStart := worldSubChunkPosY * 16
		subChunkWorldYEnd := subChunkWorldYStart + 15
		effectiveWorldYStart := max(subChunkWorldYStart, startBlockPosY)
		effectiveWorldYEnd := min(subChunkWorldYEnd, endBlockPosY)
		if effectiveWorldYStart > effectiveWorldYEnd {
			if progressCallback != nil {
				progressCallback()
			}
			continue
		}
		subChunks := make(map[bwo_define.SubChunkPos]*chunk.SubChunk, chunkCount)

		for localY := byte(effectiveWorldYStart - subChunkWorldYStart); localY <= byte(effectiveWorldYEnd-subChunkWorldYStart); localY++ {
			for subChunkZ := range subChunkZNum {
				worldSubChunkPosZ := startSubChunkPosZ + subChunkZ
				subChunkWorldZStart := worldSubChunkPosZ * 16
				subChunkWorldZEnd := subChunkWorldZStart + 15
				effectiveWorldZStart := max(subChunkWorldZStart, startBlockPosZ)
				effectiveWorldZEnd := min(subChunkWorldZEnd, endBlockPosZ)
				if effectiveWorldZStart > effectiveWorldZEnd {
					continue
				}
				for localZ := byte(effectiveWorldZStart - subChunkWorldZStart); localZ <= byte(effectiveWorldZEnd-subChunkWorldZStart); localZ++ {
					for subChunkX := range subChunkXNum {
						worldSubChunkPosX := startSubChunkPosX + subChunkX
						subChunkWorldXStart := worldSubChunkPosX * 16
						subChunkWorldXEnd := subChunkWorldXStart + 15
						effectiveWorldXStart := max(subChunkWorldXStart, startBlockPosX)
						effectiveWorldXEnd := min(subChunkWorldXEnd, endBlockPosX)
						if effectiveWorldXStart > effectiveWorldXEnd {
							continue
						}
						worldSubChunkPos := bwo_define.SubChunkPos{
							worldSubChunkPosX,
							worldSubChunkPosY,
							worldSubChunkPosZ,
						}
						for localX := byte(effectiveWorldXStart - subChunkWorldXStart); localX <= byte(effectiveWorldXEnd-subChunkWorldXStart); localX++ {
							subChunk, ok := subChunks[worldSubChunkPos]
							if !ok {
								subChunk = world.LoadSubChunk(bwo_define.DimensionIDOverworld, worldSubChunkPos)
								if subChunk == nil {
									subChunk = chunk.NewSubChunk(block.AirRuntimeID)
								}
								subChunks[worldSubChunkPos] = subChunk
							}
							blockRuntimeID := subChunk.Block(byte(localX), byte(localY), byte(localZ), 0)
							name, properties, _ := block.RuntimeIDToState(blockRuntimeID)
							runtimeID, _ := blocks.BlockNameAndStateToRuntimeID(name, properties)
							_, value, _ := blocks.RuntimeIDToSchematic(runtimeID)
							_, err := offsetWriter.Write([]byte{value})
							if err != nil {
								return err
							}
						}
					}
				}
			}
		}
		if progressCallback != nil {
			progressCallback()
		}
	}

	err = tagWriter.WriteTag(offsetWriter, nbt.TagEnd, "")
	if err != nil {
		return err
	}
	return nil
}

func (s *Schematic) Close() error {
	return nil
}