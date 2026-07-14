package structure

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/TriM-Organization/bedrock-world-operator/block"
	"github.com/TriM-Organization/bedrock-world-operator/chunk"
	bwo_define "github.com/TriM-Organization/bedrock-world-operator/define"
	"github.com/TriM-Organization/bedrock-world-operator/world"
	"github.com/Yeah114/WaterStructure/define"
	"github.com/Yeah114/blocks"
)

type FuHongV6 struct {
	BaseReader
	file         *os.File
	size         *define.Size
	originalSize *define.Size
	offsetPos    define.Offset
	origin       define.Origin

	palette      []string
	paletteCache map[string]uint32
	blocks       []fuHongV6Block
	entities     []fuHongV6Entity

	nonAirBlocks        int
	totalBlocks         int
	blockCalculationPos bool
	buildInfo           map[string]any
}

type fuHongV6Block struct {
	LocalX    int
	LocalY    int
	LocalZ    int
	RuntimeID uint32
	Aux       int
	NBT       any
}

type fuHongV6Entity struct {
	EntityType string
	Name       string
	X, Y, Z    float64
}

type fuHongV6Root struct {
	FuHongBuild         []map[string]any `json:"FuHongBuild"`
	BlocksList          []string         `json:"BlocksList"`
	TotalBlocks         int               `json:"toobtotalBlocks"`
	BlockCalculationPos bool              `json:"BlockCalculationPos"`
	TimeUsed            string            `json:"TimeUsed"`
	BuildInfo           map[string]any    `json:"Build_Info"`
}

func (f *FuHongV6) ID() uint8 {
	return IDFuHongV6
}

func (f *FuHongV6) Name() string {
	return NameFuHongV6
}

func (f *FuHongV6) FromFile(file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("重置文件指针失败: %w", err)
	}

	var root fuHongV6Root
	if err := json.NewDecoder(file).Decode(&root); err != nil {
		return fmt.Errorf("解析 FuHong V6 的 JSON 失败: %w", err)
	}

	if len(root.BlocksList) == 0 {
		return ErrInvalidFile
	}

	f.file = file
	f.palette = root.BlocksList
	f.totalBlocks = root.TotalBlocks
	f.blockCalculationPos = root.BlockCalculationPos
	f.buildInfo = root.BuildInfo

	return f.populateFromBuild(root.FuHongBuild)
}

func (f *FuHongV6) populateFromBuild(chunks []map[string]any) error {
	f.paletteCache = make(map[string]uint32)
	f.blocks = nil
	f.entities = nil
	f.nonAirBlocks = 0

	minX, minY, minZ := math.MaxInt, math.MaxInt, math.MaxInt
	maxX, maxY, maxZ := math.MinInt, math.MinInt, math.MinInt

	accum := make(map[[3]int]*fuHongV6Block)

	for chunkIdx, chunk := range chunks {
		// 读取 startX 和 startZ 但不使用，因为 V6 中坐标是绝对坐标
		// 但保留读取以验证文件格式
		startX, err := extractIntField(chunk, "startX")
		if err != nil {
			return fmt.Errorf("区块 %d: %w", chunkIdx, err)
		}
		startZ, err := extractIntField(chunk, "startZ")
		if err != nil {
			return fmt.Errorf("区块 %d: %w", chunkIdx, err)
		}
		
		// V6 中 startX/startZ 是相对于结构原点的偏移，但方块坐标是绝对坐标
		// 根据 fuhong.py 的实现，坐标不需要加上 startX/startZ
		_ = startX
		_ = startZ

		blockEntries, ok := chunk["block"].([]any)
		if !ok {
			return fmt.Errorf("区块 %d: 方块列表缺失或无效", chunkIdx)
		}

		for entryIdx, rawEntry := range blockEntries {
			tuple, ok := rawEntry.([]any)
			if !ok || len(tuple) < 5 {
				return fmt.Errorf("区块 %d 条目 %d: 元组无效", chunkIdx, entryIdx)
			}

			// 检查是否是实体（第一个元素为字符串且包含":"）
			if str, isString := tuple[0].(string); isString && strings.Contains(str, ":") {
				entity, err := f.parseEntity(tuple)
				if err != nil {
					return fmt.Errorf("区块 %d 实体 %d: %w", chunkIdx, entryIdx, err)
				}
				
				// 实体坐标是绝对坐标
				gx := int(math.Round(entity.X))
				gy := int(math.Round(entity.Y))
				gz := int(math.Round(entity.Z))
				
				if gx < minX {
					minX = gx
				}
				if gy < minY {
					minY = gy
				}
				if gz < minZ {
					minZ = gz
				}
				if gx > maxX {
					maxX = gx
				}
				if gy > maxY {
					maxY = gy
				}
				if gz > maxZ {
					maxZ = gz
				}
				
				f.entities = append(f.entities, entity)
				continue
			}

			// 方块处理
			paletteIndex, err := toInt(tuple[0])
			if err != nil {
				return fmt.Errorf("区块 %d 条目 %d: 调色板索引: %w", chunkIdx, entryIdx, err)
			}
			if paletteIndex < 0 || paletteIndex >= len(f.palette) {
				return fmt.Errorf("区块 %d 条目 %d: 调色板索引 %d 越界", chunkIdx, entryIdx, paletteIndex)
			}

			aux, err := toInt(tuple[1])
			if err != nil {
				return fmt.Errorf("区块 %d 条目 %d: aux: %w", chunkIdx, entryIdx, err)
			}

			xs, err := toIntSlice(tuple[2])
			if err != nil {
				return fmt.Errorf("区块 %d 条目 %d: xs: %w", chunkIdx, entryIdx, err)
			}
			ys, err := toIntSlice(tuple[3])
			if err != nil {
				return fmt.Errorf("区块 %d 条目 %d: ys: %w", chunkIdx, entryIdx, err)
			}
			zs, err := toIntSlice(tuple[4])
			if err != nil {
				return fmt.Errorf("区块 %d 条目 %d: zs: %w", chunkIdx, entryIdx, err)
			}

			if len(xs) != len(ys) || len(xs) != len(zs) {
				return fmt.Errorf("区块 %d 条目 %d: 坐标数组长度不匹配", chunkIdx, entryIdx)
			}

			var extras []any
			if len(tuple) >= 6 {
				if extraSlice, ok := tuple[5].([]any); ok {
					extras = extraSlice
					if len(extras) != len(xs) {
						return fmt.Errorf("区块 %d 条目 %d: NBT 列表长度不匹配", chunkIdx, entryIdx)
					}
				}
			}

			blockName := f.palette[paletteIndex]
			runtimeID := f.runtimeIDFor(blockName, aux)

			for i := 0; i < len(xs); i++ {
				// V6 中坐标是绝对坐标，不需要加上 startX/startZ
				worldX := xs[i]
				worldY := ys[i]
				worldZ := zs[i]

				var nbt any
				if i < len(extras) {
					nbt = extras[i]
					// 空数组视为 nil
					if arr, ok := nbt.([]any); ok && len(arr) == 0 {
						nbt = nil
					}
				}

				key := [3]int{worldX, worldY, worldZ}
				if existing, ok := accum[key]; ok {
					existing.RuntimeID = runtimeID
					existing.Aux = aux
					if existing.NBT == nil && nbt != nil {
						existing.NBT = nbt
					}
				} else {
					accum[key] = &fuHongV6Block{
						LocalX:    worldX,
						LocalY:    worldY,
						LocalZ:    worldZ,
						RuntimeID: runtimeID,
						Aux:       aux,
						NBT:       nbt,
					}
				}

				if worldX < minX {
					minX = worldX
				}
				if worldY < minY {
					minY = worldY
				}
				if worldZ < minZ {
					minZ = worldZ
				}
				if worldX > maxX {
					maxX = worldX
				}
				if worldY > maxY {
					maxY = worldY
				}
				if worldZ > maxZ {
					maxZ = worldZ
				}
			}
		}
	}

	if len(accum) == 0 && len(f.entities) == 0 {
		return ErrInvalidFile
	}

	width := maxX - minX + 1
	height := maxY - minY + 1
	length := maxZ - minZ + 1
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	if length <= 0 {
		length = 1
	}

	f.origin = define.Origin{int32(minX), int32(minY), int32(minZ)}
	f.size = &define.Size{Width: width, Height: height, Length: length}
	f.originalSize = &define.Size{Width: width, Height: height, Length: length}

	keys := make([][3]int, 0, len(accum))
	for key := range accum {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][1] != keys[j][1] {
			return keys[i][1] < keys[j][1]
		}
		if keys[i][2] != keys[j][2] {
			return keys[i][2] < keys[j][2]
		}
		return keys[i][0] < keys[j][0]
	})

	f.blocks = make([]fuHongV6Block, 0, len(accum))
	f.nonAirBlocks = 0

	for _, key := range keys {
		rec := accum[key]
		blk := fuHongV6Block{
			LocalX:    rec.LocalX - int(f.origin.X()),
			LocalY:    rec.LocalY - int(f.origin.Y()),
			LocalZ:    rec.LocalZ - int(f.origin.Z()),
			RuntimeID: rec.RuntimeID,
			Aux:       rec.Aux,
			NBT:       rec.NBT,
		}
		f.blocks = append(f.blocks, blk)
		if blk.RuntimeID != block.AirRuntimeID {
			f.nonAirBlocks++
		}
	}

	if len(f.paletteCache) == 0 {
		return ErrInvalidFile
	}

	return nil
}

func (f *FuHongV6) parseEntity(tuple []any) (fuHongV6Entity, error) {
	if len(tuple) < 5 {
		return fuHongV6Entity{}, fmt.Errorf("实体条目格式无效")
	}

	entityType, ok := tuple[0].(string)
	if !ok {
		return fuHongV6Entity{}, fmt.Errorf("实体类型必须为字符串")
	}

	name := ""
	if tuple[1] != nil {
		name = fmt.Sprint(tuple[1])
	}

	x, err := toFloat64(tuple[2])
	if err != nil {
		return fuHongV6Entity{}, fmt.Errorf("X 坐标: %w", err)
	}
	y, err := toFloat64(tuple[3])
	if err != nil {
		return fuHongV6Entity{}, fmt.Errorf("Y 坐标: %w", err)
	}
	z, err := toFloat64(tuple[4])
	if err != nil {
		return fuHongV6Entity{}, fmt.Errorf("Z 坐标: %w", err)
	}

	return fuHongV6Entity{
		EntityType: entityType,
		Name:       name,
		X:          x,
		Y:          y,
		Z:          z,
	}, nil
}

func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case json.Number:
		return val.Float64()
	default:
		return 0, fmt.Errorf("无法转换为 float64: %T", v)
	}
}

func (f *FuHongV6) runtimeIDFor(name string, aux int) uint32 {
	cacheKey := fmt.Sprintf("%s|%d", name, aux)
	if runtimeID, ok := f.paletteCache[cacheKey]; ok {
		return runtimeID
	}

	runtimeID, found := blocks.LegacyBlockToRuntimeID(name, uint16(aux))
	if !found {
		runtimeID, found = blocks.BlockStrToRuntimeID(name)
	}
	if !found && !strings.Contains(name, ":") {
		prefixed := "minecraft:" + name
		runtimeID, found = blocks.LegacyBlockToRuntimeID(prefixed, uint16(aux))
		if !found {
			runtimeID, found = blocks.BlockStrToRuntimeID(prefixed)
		}
	}
	if !found {
		runtimeID = UnknownBlockRuntimeID
	}
	baseName, properties, found := blocks.RuntimeIDToState(runtimeID)
	if !found {
		runtimeID = UnknownBlockRuntimeID
	} else {
		runtimeID, found = block.StateToRuntimeID(baseName, properties)
		if !found {
			runtimeID = UnknownBlockRuntimeID
		}
	}

	f.paletteCache[cacheKey] = runtimeID
	return runtimeID
}

func (f *FuHongV6) GetOffsetPos() define.Offset {
	return f.offsetPos
}

func (f *FuHongV6) SetOffsetPos(offset define.Offset) {
	f.offsetPos = offset
	f.size.Width = f.originalSize.Width + int(math.Abs(float64(offset.X())))
	f.size.Length = f.originalSize.Length + int(math.Abs(float64(offset.Z())))
	f.size.Height = f.originalSize.Height + int(math.Abs(float64(offset.Y())))
}

func (f *FuHongV6) GetSize() define.Size {
	return *f.size
}

func (f *FuHongV6) GetChunks(posList []define.ChunkPos) (map[define.ChunkPos]*chunk.Chunk, error) {
	chunks := make(map[define.ChunkPos]*chunk.Chunk, len(posList))
	height := f.size.Height
	if height <= 0 {
		height = 1
	}
	for _, pos := range posList {
		if _, exists := chunks[pos]; !exists {
			chunks[pos] = chunk.NewChunk(block.AirRuntimeID, MCWorldOverworldRange)
		}
	}

	if len(chunks) == 0 {
		return chunks, nil
	}

	offsetX := int(f.offsetPos.X())
	offsetY := int(f.offsetPos.Y())
	offsetZ := int(f.offsetPos.Z())

	for _, blk := range f.blocks {
		newX := blk.LocalX + offsetX
		newY := blk.LocalY + offsetY
		newZ := blk.LocalZ + offsetZ

		chunkX := floorDiv(newX, 16)
		chunkZ := floorDiv(newZ, 16)
		chunkPos := define.ChunkPos{int32(chunkX), int32(chunkZ)}

		c, exists := chunks[chunkPos]
		if !exists {
			continue
		}

		localX := newX - chunkX*16
		localZ := newZ - chunkZ*16
		c.SetBlock(uint8(localX), int16(newY)-64, uint8(localZ), 0, blk.RuntimeID)
	}

	return chunks, nil
}

func (f *FuHongV6) GetChunksNBT(posList []define.ChunkPos) (map[define.ChunkPos]map[define.BlockPos]map[string]any, error) {
	result := make(map[define.ChunkPos]map[define.BlockPos]map[string]any, len(posList))
	for _, pos := range posList {
		if _, exists := result[pos]; !exists {
			result[pos] = make(map[define.BlockPos]map[string]any)
		}
	}

	if len(result) == 0 {
		return result, nil
	}

	offsetX := int(f.offsetPos.X())
	offsetY := int(f.offsetPos.Y())
	offsetZ := int(f.offsetPos.Z())

	for _, blk := range f.blocks {
		if blk.NBT == nil {
			continue
		}

		nbt, err := f.convertNBT(blk)
		if err != nil || nbt == nil {
			continue
		}

		newX := blk.LocalX + offsetX
		newY := blk.LocalY + offsetY
		newZ := blk.LocalZ + offsetZ

		chunkX := floorDiv(newX, 16)
		chunkZ := floorDiv(newZ, 16)
		chunkPos := define.ChunkPos{int32(chunkX), int32(chunkZ)}

		chunkNBT, exists := result[chunkPos]
		if !exists {
			continue
		}

		localX := newX - chunkX*16
		localZ := newZ - chunkZ*16
		blockPos := define.BlockPos{int32(localX), chunkLocalYFromWorld(newY), int32(localZ)}
		chunkNBT[blockPos] = nbt
	}

	return result, nil
}

func (f *FuHongV6) convertNBT(blk fuHongV6Block) (map[string]any, error) {
	if blk.NBT == nil {
		return nil, nil
	}

	name, _, ok := block.RuntimeIDToState(blk.RuntimeID)
	if !ok {
		return nil, fmt.Errorf("未知方块 RuntimeID: %d", blk.RuntimeID)
	}

	lowerName := strings.ToLower(name)

	// 命令方块
	if strings.Contains(lowerName, "command_block") {
		if arr, ok := blk.NBT.([]any); ok {
			return f.buildCommandNBT(arr, blk.Aux), nil
		}
	}

	// 告示牌
	if strings.Contains(lowerName, "sign") {
		if str, ok := blk.NBT.(string); ok {
			return f.buildSignNBT(name, str), nil
		}
		if arr, ok := blk.NBT.([]any); ok && len(arr) > 0 {
			if str, ok := arr[0].(string); ok {
				return f.buildSignNBT(name, str), nil
			}
		}
	}

	// 容器（包括潜影盒）
	if chestBlockNameToID(strings.TrimSpace(name)) != "" || strings.Contains(lowerName, "shulker_box") {
		if items, ok := blk.NBT.([]any); ok {
			// 检查是否是嵌套数组格式
			if len(items) == 1 {
				if innerItems, ok := items[0].([]any); ok {
					return f.buildContainerNBT(name, innerItems, blk.Aux), nil
				}
			}
			return f.buildContainerNBT(name, items, blk.Aux), nil
		}
	}

	return nil, nil
}

func (f *FuHongV6) buildCommandNBT(arr []any, aux int) map[string]any {
	if len(arr) == 0 {
		return nil
	}

	command := fmt.Sprint(arr[0])

	tickDelay := 0
	if len(arr) > 1 {
		if delay, err := toInt(arr[1]); err == nil {
			tickDelay = delay
		}
	}

	auto := byte(0)
	if len(arr) > 2 {
		auto = boolToByte(arr[2] != 0)
	}

	customName := ""
	if len(arr) > 3 {
		customName = fmt.Sprint(arr[3])
	}

	version := int32(19)
	if kbdxExecuteRegex.MatchString(command) {
		version = 38
	}

	// 从 aux 中提取条件模式（bit 3）
	conditionalMode := byte((aux >> 3) & 1)

	return map[string]any{
		"id":                 "CommandBlock",
		"Command":            command,
		"CustomName":         customName,
		"ExecuteOnFirstTick": byte(1),
		"auto":               auto,
		"TickDelay":          int32(tickDelay),
		"conditionalMode":    conditionalMode,
		"TrackOutput":        byte(1),
		"Version":            version,
	}
}

func (f *FuHongV6) buildSignNBT(name, text string) map[string]any {
	lower := strings.ToLower(strings.TrimSpace(name))
	id := "Sign"
	if strings.HasSuffix(lower, "hanging_sign") {
		id = "HangingSign"
	}

	return map[string]any{
		"id":        id,
		"IsWaxed":   byte(0),
		"isMovable": byte(1),
		"BackText": map[string]any{
			"FilteredText":       "",
			"HideGlowOutline":    byte(0),
			"IgnoreLighting":     byte(0),
			"PersistFormatting":  byte(1),
			"SignTextColor":      int32(-16777216),
			"Text":               "",
			"TextOwner":          "",
		},
		"FrontText": map[string]any{
			"FilteredText":       "",
			"HideGlowOutline":    byte(0),
			"IgnoreLighting":     byte(0),
			"PersistFormatting":  byte(1),
			"SignTextColor":      int32(-16777216),
			"Text":               text,
			"TextOwner":          "",
		},
	}
}

func (f *FuHongV6) buildContainerNBT(name string, items []any, aux int) map[string]any {
	id := chestBlockNameToID(strings.TrimSpace(name))
	if id == "" {
		// 检查是否是潜影盒
		if strings.Contains(strings.ToLower(name), "shulker_box") {
			id = "ShulkerBox"
		} else {
			return nil
		}
	}

	out := make([]map[string]any, 0, len(items))
	for _, rawItem := range items {
		itemTuple, ok := rawItem.([]any)
		if !ok || len(itemTuple) < 4 {
			continue
		}

		itemName := fmt.Sprint(itemTuple[0])
		if itemName != "" && !strings.Contains(itemName, ":") {
			itemName = "minecraft:" + itemName
		}

		damage, _ := toInt(itemTuple[1])
		count, _ := toInt(itemTuple[2])
		slot, _ := toInt(itemTuple[3])

		item := map[string]any{
			"Name":   itemName,
			"Damage": int16(damage),
			"Count":  byte(count),
			"Slot":   byte(slot),
		}

		// 附魔处理
		if len(itemTuple) >= 5 {
			if ench, ok := itemTuple[4].([]any); ok && len(ench) > 0 {
				item["tag"] = f.buildEnchantments(ench)
			}
		}

		// keep on death 处理
		if len(itemTuple) >= 6 {
			if keep, err := toInt(itemTuple[5]); err == nil && keep != 0 {
				if item["tag"] == nil {
					item["tag"] = map[string]any{}
				}
				if tag, ok := item["tag"].(map[string]any); ok {
					tag["minecraft:keep_on_death"] = map[string]any{}
				}
			}
		}

		out = append(out, item)
	}

	nbt := map[string]any{
		"id":        id,
		"Findable":  byte(0),
		"IsOpened":  byte(0),
		"isMovable": byte(1),
		"Items":     out,
	}

	// 潜影盒强制设置 facing 为 1
	if strings.Contains(strings.ToLower(id), "shulker_box") {
		nbt["facing"] = byte(1)
	}

	return nbt
}

func (f *FuHongV6) buildEnchantments(ench []any) map[string]any {
	list := make([]map[string]any, 0, len(ench))
	for _, e := range ench {
		if pair, ok := e.([]any); ok && len(pair) >= 2 {
			id, _ := toInt(pair[0])
			lvl, _ := toInt(pair[1])
			list = append(list, map[string]any{
				"id":  int16(id),
				"lvl": int16(lvl),
			})
		}
	}
	return map[string]any{"ench": list}
}

func (f *FuHongV6) CountNonAirBlocks() (int, error) {
	return f.nonAirBlocks, nil
}

func (f *FuHongV6) Close() error {
	return nil
}

type fuHongV6ExportKey struct {
	Palette string
	Aux     int
}

type fuHongV6ExportGroup struct {
	palette   string
	aux       int
	xs        []int
	ys        []int
	zs        []int
	extras    []any
	hasExtras bool
}

type fuHongV6ResolvedPalette struct {
	paletteName string
	aux         int
}

func (f *FuHongV6) FromMCWorld(
	world *world.BedrockWorld,
	target *os.File,
	point1BlockPos define.BlockPos,
	point2BlockPos define.BlockPos,
	startCallback func(int),
	progressCallback func(),
) error {
	if _, err := target.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("重置目标文件指针失败: %w", err)
	}
	if err := target.Truncate(0); err != nil {
		return fmt.Errorf("清空目标文件失败: %w", err)
	}

	return f.WriteTo(world, target, point1BlockPos, point2BlockPos, startCallback, progressCallback)
}

func (f *FuHongV6) WriteTo(
	bw *world.BedrockWorld,
	w io.Writer,
	point1BlockPos define.BlockPos,
	point2BlockPos define.BlockPos,
	startCallback func(int),
	progressCallback func(),
) error {
	if bw == nil {
		return fmt.Errorf("bedrock 世界为 nil")
	}
	if f.paletteCache == nil {
		f.paletteCache = make(map[string]uint32)
	}
	if w == nil {
		return fmt.Errorf("writer 为 nil")
	}

	startBlockPos := define.BlockPos{
		min(point1BlockPos.X(), point2BlockPos.X()),
		min(point1BlockPos.Y(), point2BlockPos.Y()),
		min(point1BlockPos.Z(), point2BlockPos.Z()),
	}
	endBlockPos := define.BlockPos{
		max(point1BlockPos.X(), point2BlockPos.X()),
		max(point1BlockPos.Y(), point2BlockPos.Y()),
		max(point1BlockPos.Z(), point2BlockPos.Z()),
	}

	startX := int(startBlockPos.X())
	startY := int(startBlockPos.Y())
	startZ := int(startBlockPos.Z())
	endX := int(endBlockPos.X())
	endY := int(endBlockPos.Y())
	endZ := int(endBlockPos.Z())

	// V6 使用 32x32 的区块
	minChunkX := floorDiv(startX, 32)
	maxChunkX := floorDiv(endX, 32)
	minChunkZ := floorDiv(startZ, 32)
	maxChunkZ := floorDiv(endZ, 32)
	
	// 确保区块范围是连续的
	chunkXCount := maxChunkX - minChunkX + 1
	chunkZCount := maxChunkZ - minChunkZ + 1
	
	minSubChunkY := floorDiv(startY, 16)
	maxSubChunkY := floorDiv(endY, 16)
	subChunkYCount := maxSubChunkY - minSubChunkY + 1
	
	totalSubChunks := chunkXCount * chunkZCount * subChunkYCount
	if startCallback != nil {
		startCallback(totalSubChunks)
	}

	blocksList := []string{"minecraft:air"}
	paletteIndex := map[string]int{"minecraft:air": 0}
	resolvedCache := make(map[uint32]fuHongV6ResolvedPalette)

	if _, err := io.WriteString(w, `{"FuHongBuild":[`); err != nil {
		return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
	}
	_ = fuHongMaybeFlush(w)

	wroteAnyChunk := false
	totalBlocks := 0

	// 遍历所有 32x32 区块
	for cz := minChunkZ; cz <= maxChunkZ; cz++ {
		chunkWorldZStart := cz * 32
		chunkWorldZEnd := chunkWorldZStart + 31
		
		for cx := minChunkX; cx <= maxChunkX; cx++ {
			chunkWorldXStart := cx * 32
			chunkWorldXEnd := chunkWorldXStart + 31

			// 收集该 32x32 区块内所有方块的 NBT（对应 2x2 个 16x16 区块）
			nbtByWorldPos := make(map[define.BlockPos]map[string]any)
			for dz := 0; dz < 2; dz++ {
				for dx := 0; dx < 2; dx++ {
					subChunkX := cx*2 + dx
					subChunkZ := cz*2 + dz
					chunkPos := bwo_define.ChunkPos{int32(subChunkX), int32(subChunkZ)}

					chunkNBT, err := fuHongLoadChunkNBT(bw, chunkPos, startX, startY, startZ, endX, endY, endZ)
					if err != nil {
						return err
					}
					for pos, nbt := range chunkNBT {
						nbtByWorldPos[pos] = nbt
					}
				}
			}

			groups := make(map[fuHongV6ExportKey]*fuHongV6ExportGroup)

			// 遍历 Y 轴子区块
			for subY := minSubChunkY; subY <= maxSubChunkY; subY++ {
				subChunkWorldYStart := subY * 16
				subChunkWorldYEnd := subChunkWorldYStart + 15
				
				// 计算当前 32x32 区块与导出范围重叠的区域
				overlapXStart := max(chunkWorldXStart, startX)
				overlapXEnd := min(chunkWorldXEnd, endX)
				overlapZStart := max(chunkWorldZStart, startZ)
				overlapZEnd := min(chunkWorldZEnd, endZ)
				overlapYStart := max(subChunkWorldYStart, startY)
				overlapYEnd := min(subChunkWorldYEnd, endY)

				if overlapXStart <= overlapXEnd && overlapZStart <= overlapZEnd && overlapYStart <= overlapYEnd {
					// 遍历该 32x32 区块内的 2x2 个 16x16 区块
					for dz := 0; dz < 2; dz++ {
						for dx := 0; dx < 2; dx++ {
							subChunkX := cx*2 + dx
							subChunkZ := cz*2 + dz

							subChunkWorldXStart := subChunkX * 16
							subChunkWorldXEnd := subChunkWorldXStart + 15
							subChunkWorldZStart := subChunkZ * 16
							subChunkWorldZEnd := subChunkWorldZStart + 15

							// 计算这个 16x16 子区块与当前需要导出的范围的重叠区域
							subOverlapXStart := max(subChunkWorldXStart, overlapXStart)
							subOverlapXEnd := min(subChunkWorldXEnd, overlapXEnd)
							subOverlapZStart := max(subChunkWorldZStart, overlapZStart)
							subOverlapZEnd := min(subChunkWorldZEnd, overlapZEnd)

							if subOverlapXStart <= subOverlapXEnd && subOverlapZStart <= subOverlapZEnd {
								worldSubChunkPos := bwo_define.SubChunkPos{int32(subChunkX), int32(subY), int32(subChunkZ)}
								subChunk := bw.LoadSubChunk(bwo_define.DimensionIDOverworld, worldSubChunkPos)

								if subChunk != nil {
									for wy := overlapYStart; wy <= overlapYEnd; wy++ {
										localY := byte(wy - subChunkWorldYStart)

										for wz := subOverlapZStart; wz <= subOverlapZEnd; wz++ {
											localZ := byte(wz - subChunkWorldZStart)

											for wx := subOverlapXStart; wx <= subOverlapXEnd; wx++ {
												localX := byte(wx - subChunkWorldXStart)

												runtimeID := subChunk.Block(localX, localY, localZ, 0)
												if runtimeID == block.AirRuntimeID {
													continue
												}

												resolved, ok := resolvedCache[runtimeID]
												if !ok {
													paletteName, aux := f.resolveFuHongPalette(runtimeID)
													resolved = fuHongV6ResolvedPalette{paletteName: paletteName, aux: aux}
													resolvedCache[runtimeID] = resolved
												}

												if _, ok := paletteIndex[resolved.paletteName]; !ok {
													idx := len(blocksList)
													blocksList = append(blocksList, resolved.paletteName)
													paletteIndex[resolved.paletteName] = idx
												}

												groupKey := fuHongV6ExportKey{Palette: resolved.paletteName, Aux: resolved.aux}
												group, ok := groups[groupKey]
												if !ok {
													group = &fuHongV6ExportGroup{
														palette: resolved.paletteName,
														aux:     resolved.aux,
													}
													groups[groupKey] = group
												}

												// V6 使用相对于结构原点的绝对坐标
												relX := wx - startX
												relY := wy - startY
												relZ := wz - startZ

												group.xs = append(group.xs, relX)
												group.ys = append(group.ys, relY)
												group.zs = append(group.zs, relZ)
												totalBlocks++

												if fuHongNeedsExtras(resolved.paletteName) {
													blockPos := define.BlockPos{int32(wx), int32(wy), int32(wz)}
													extra := f.buildFuHongV6ExtraPayload(resolved.paletteName, nbtByWorldPos[blockPos])
													if extra != nil {
														group.extras = append(group.extras, extra)
													} else {
														group.extras = append(group.extras, []any{})
													}
													group.hasExtras = true
												}
											}
										}
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

			if len(groups) == 0 {
				continue
			}

			if wroteAnyChunk {
				if _, err := io.WriteString(w, ","); err != nil {
					return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
				}
			}

			// V6 的 startX/startZ 是相对于结构原点的偏移
			localChunkBaseX := chunkWorldXStart - startX
			localChunkBaseZ := chunkWorldZStart - startZ

			if _, err := io.WriteString(w, fmt.Sprintf(`{"startX":%d,"startZ":%d,"block":[`, localChunkBaseX, localChunkBaseZ)); err != nil {
				return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
			}

			groupKeys := make([]fuHongV6ExportKey, 0, len(groups))
			for k := range groups {
				groupKeys = append(groupKeys, k)
			}
			sort.Slice(groupKeys, func(i, j int) bool {
				if groupKeys[i].Palette != groupKeys[j].Palette {
					return groupKeys[i].Palette < groupKeys[j].Palette
				}
				return groupKeys[i].Aux < groupKeys[j].Aux
			})

			for idx, gk := range groupKeys {
				if idx > 0 {
					if _, err := io.WriteString(w, ","); err != nil {
						return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
					}
				}
				g := groups[gk]
				pIdx := paletteIndex[g.palette]
				entry := []any{pIdx, g.aux, g.xs, g.ys, g.zs}
				if g.hasExtras {
					entry = append(entry, g.extras)
				}
				b, err := json.Marshal(entry)
				if err != nil {
					return fmt.Errorf("序列化 FuHong V6 block 条目失败: %w", err)
				}
				if _, err := w.Write(b); err != nil {
					return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
				}
			}

			if _, err := io.WriteString(w, `]}`); err != nil {
				return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
			}
			wroteAnyChunk = true
			_ = fuHongMaybeFlush(w)
		}
	}

	if !wroteAnyChunk {
		return fmt.Errorf("未导出任何方块")
	}

	if _, err := io.WriteString(w, `],"BlocksList":`); err != nil {
		return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
	}
	bl, err := json.Marshal(blocksList)
	if err != nil {
		return fmt.Errorf("序列化 FuHong V6 BlocksList 失败: %w", err)
	}
	if _, err := w.Write(bl); err != nil {
		return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
	}
	if _, err := io.WriteString(w, fmt.Sprintf(`,"toobtotalBlocks":%d,"BlockCalculationPos":true,"TimeUsed":"0ms","Build_Info":`, totalBlocks)); err != nil {
		return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
	}

	// 构建 Build_Info
	buildInfo := map[string]any{
		"Version":      20260221,
		"UserName":     "WaterStructure",
		"ToolVersion":  "1.0",
		"PlayerName":   "WaterStructure",
		"Export_Mode":  "命令导出",
		"ExportOption": []bool{true, true, true, true},
		"TimeConsuming": "耗时:1ms",
		"Pos": map[string]int{
			"x": startX,
			"y": startY,
			"z": startZ,
		},
		"Time": time.Now().Format("2006-01-02 15:04:05"),
	}

	b, err := json.Marshal(buildInfo)
	if err != nil {
		return fmt.Errorf("序列化 Build_Info 失败: %w", err)
	}
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
	}
	if _, err := io.WriteString(w, `}`); err != nil {
		return fmt.Errorf("写入 FuHong V6 JSON 失败: %w", err)
	}
	_ = fuHongMaybeFlush(w)

	return nil
}

func (f *FuHongV6) resolveFuHongPalette(runtimeID uint32) (paletteName string, aux int) {
	name, properties, found := block.RuntimeIDToState(runtimeID)
	if !found || strings.TrimSpace(name) == "" {
		return "minecraft:unknown", 0
	}
	name = strings.TrimSpace(name)
	if !strings.Contains(name, ":") {
		name = "minecraft:" + name
	}

	// 优先尝试用 legacy aux 还原
	for candidateAux := 0; candidateAux <= 255; candidateAux++ {
		if f.runtimeIDFor(name, candidateAux) == runtimeID {
			return name, candidateAux
		}
	}

	// fallback：把 states 写进 BlocksList
	if len(properties) == 0 {
		return name, 0
	}
	return name + "[" + formatFuHongStates(properties) + "]", 0
}

func (f *FuHongV6) buildFuHongV6ExtraPayload(blockName string, nbt map[string]any) any {
	lowerName := strings.ToLower(blockName)

	if strings.Contains(lowerName, "command_block") {
		if nbt == nil {
			return []any{"", 0, 0, ""}
		}
		return f.buildFuHongV6CommandPayload(nbt)
	}

	if chestBlockNameToID(strings.TrimSpace(blockName)) != "" || strings.Contains(lowerName, "shulker_box") {
		if nbt == nil {
			return []any{}
		}
		return f.buildFuHongV6ContainerPayload(nbt)
	}

	if strings.Contains(lowerName, "sign") {
		if nbt == nil {
			return ""
		}
		if front, ok := nbt["FrontText"].(map[string]any); ok {
			if text, ok := front["Text"].(string); ok {
				return text
			}
		}
		if text, ok := nbt["Text"].(string); ok {
			return text
		}
		return ""
	}

	return nil
}

func (f *FuHongV6) buildFuHongV6CommandPayload(nbt map[string]any) []any {
	cmd, _ := nbt["Command"].(string)

	delay := 0
	switch v := nbt["TickDelay"].(type) {
	case int32:
		delay = int(v)
	case int64:
		delay = int(v)
	case int:
		delay = v
	}

	auto := 0
	switch v := nbt["auto"].(type) {
	case byte:
		if v != 0 {
			auto = 1
		}
	case bool:
		if v {
			auto = 1
		}
	}

	custom, _ := nbt["CustomName"].(string)

	return []any{cmd, delay, auto, custom}
}

func (f *FuHongV6) buildFuHongV6ContainerPayload(nbt map[string]any) []any {
	rawItems, ok := nbt["Items"]
	if !ok {
		return []any{}
	}

	items, ok := rawItems.([]any)
	if !ok {
		return []any{}
	}

	out := make([]any, 0, len(items))
	for _, rawItem := range items {
		itemMap, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}

		name, _ := itemMap["Name"].(string)
		if name == "" {
			continue
		}

		damage, _ := toInt(itemMap["Damage"])
		count, _ := toInt(itemMap["Count"])
		slot, _ := toInt(itemMap["Slot"])

		entry := []any{name, damage, count, slot}

		// 检查是否有附魔
		if tag, ok := itemMap["tag"].(map[string]any); ok {
			if ench, ok := tag["ench"].([]any); ok && len(ench) > 0 {
				entry = append(entry, ench)
			}
		}

		out = append(out, entry)
	}

	return out
}