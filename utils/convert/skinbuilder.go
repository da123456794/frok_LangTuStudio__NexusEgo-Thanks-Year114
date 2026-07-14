package convert

import (
	"context"
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bwoblock "github.com/TriM-Organization/bedrock-world-operator/block"
	bwoworld "github.com/TriM-Organization/bedrock-world-operator/world"
	wsutils "github.com/Yeah114/WaterStructure/utils"
	wsblocks "github.com/Yeah114/blocks"
	"github.com/disintegration/imaging"

	"nexus/utils/log"
)

const (
	skinBuildY          = int32(-64)
	skinDefaultDataName = "skin"
)

type SkinArmType string

const (
	SkinArmClassic SkinArmType = "classic"
	SkinArmSlim    SkinArmType = "slim"
)

type SkinBlockSet string

const (
	SkinBlockSetConcrete   SkinBlockSet = "concrete"
	SkinBlockSetWool       SkinBlockSet = "wool"
	SkinBlockSetTerracotta SkinBlockSet = "terracotta"
	SkinBlockSetMixed      SkinBlockSet = "mixed"
	SkinBlockSetExtended   SkinBlockSet = "extended"
)

type SkinBuildOptions struct {
	Scale          int
	ArmType        SkinArmType
	BlockSet       SkinBlockSet
	AlphaCutoff    uint8
	OuterThickness int
	Solid          bool
	FillBlock      string
}

type skinImage struct {
	width  int
	height int
	unit   int
	img    interface {
		NRGBAAt(x, y int) color.NRGBA
	}
}

type skinFace string

const (
	skinFaceNorth skinFace = "north"
	skinFaceSouth skinFace = "south"
	skinFaceWest  skinFace = "west"
	skinFaceEast  skinFace = "east"
	skinFaceUp    skinFace = "up"
	skinFaceDown  skinFace = "down"
)

type skinFaceUV struct {
	u int
	v int
	w int
	h int
}

type skinPart struct {
	name      string
	origin    skinPos
	size      skinSize
	baseUV    map[skinFace]skinFaceUV
	overlayUV map[skinFace]skinFaceUV
}

type skinPos struct {
	x int
	y int
	z int
}

type skinSize struct {
	w int
	h int
	d int
}

type skinPaletteSpec struct {
	name string
	r    uint8
	g    uint8
	b    uint8
}

type skinPaletteEntry struct {
	name      string
	runtimeID uint32
	r         uint8
	g         uint8
	b         uint8
}

type skinPalette struct {
	entries []skinPaletteEntry
}

var (
	skinPaletteCacheMu sync.Mutex
	skinPaletteCache   = make(map[SkinBlockSet]skinPalette)
)

func DefaultSkinBuildOptions() SkinBuildOptions {
	return SkinBuildOptions{
		Scale:          2,
		ArmType:        SkinArmClassic,
		BlockSet:       SkinBlockSetMixed,
		AlphaCutoff:    16,
		OuterThickness: 1,
		Solid:          false,
		FillBlock:      "minecraft:stone",
	}
}

func ConvertSkinToMCWorld(inputPath, outputDir string, opts SkinBuildOptions) (string, error) {
	opts = normalizeSkinBuildOptions(opts)

	skin, err := loadSkin(inputPath)
	if err != nil {
		return "", err
	}
	if err := validateSkinWorldHeight(skin, opts); err != nil {
		return "", err
	}

	palette, err := loadSkinPalette(opts.BlockSet)
	if err != nil {
		return "", err
	}
	fillRuntimeID, err := resolveSkinBlockRuntimeID(opts.FillBlock)
	if err != nil {
		return "", err
	}

	blocks, err := buildSkinStatueBlocks(skin, palette, fillRuntimeID, opts)
	if err != nil {
		return "", err
	}
	if len(blocks) == 0 {
		return "", fmt.Errorf("皮肤中没有可生成的可见像素")
	}

	width, height, length := skinStatueDimensions(skin, opts.Scale)
	width += opts.OuterThickness * 2
	height += opts.OuterThickness * 2
	length += opts.OuterThickness * 2

	worldPath, err := os.MkdirTemp("", "nexusego-skinbuilder-")
	if err != nil {
		return "", fmt.Errorf("创建临时世界失败: %w", err)
	}
	defer os.RemoveAll(worldPath)

	bw, err := bwoworld.Open(worldPath, nil)
	if err != nil {
		return "", fmt.Errorf("打开临时世界失败: %w", err)
	}

	mcWorld, err := wsutils.NewMCWorld(bw, context.Background())
	if err != nil {
		_ = bw.CloseWorld()
		return "", fmt.Errorf("初始化临时世界失败: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = mcWorld.Close()
		}
	}()

	shift := opts.OuterThickness
	for pos, runtimeID := range blocks {
		worldX := int32(pos.x + shift)
		worldY := skinBuildY + int32(pos.y+shift)
		worldZ := int32(pos.z + shift)
		if err := mcWorld.SetBlock(worldX, int16(worldY), worldZ, runtimeID); err != nil {
			return "", fmt.Errorf("写入雕塑方块失败: %w", err)
		}
	}

	levelName := buildSkinWorldName(inputPath, width, height, length)
	bw.LevelDat().LevelName = levelName
	mcWorld.Flush()
	if err := mcWorld.Close(); err != nil {
		return "", fmt.Errorf("关闭临时世界失败: %w", err)
	}
	closed = true

	log.Log.Info(fmt.Sprintf("皮肤尺寸: %dx%d", skin.width, skin.height))
	log.Log.Info(fmt.Sprintf("雕塑尺寸: %dx%dx%d", width, height, length))
	log.Log.Info(fmt.Sprintf("非空气方块: %d", len(blocks)))

	start := time.Now()
	outputPath, err := archiveWorldAsMCWorld(worldPath, levelName, outputDir)
	if err != nil {
		return "", err
	}
	log.Log.Info(fmt.Sprintf("SkinBuilder 转换完成，耗时 %.1fs", time.Since(start).Seconds()))
	log.Log.Info(fmt.Sprintf("MCWorld 文件已保存到: %s", outputPath))
	return outputPath, nil
}

func normalizeSkinBuildOptions(opts SkinBuildOptions) SkinBuildOptions {
	defaults := DefaultSkinBuildOptions()
	if opts.Scale <= 0 {
		opts.Scale = defaults.Scale
	}
	if strings.TrimSpace(string(opts.ArmType)) == "" {
		opts.ArmType = defaults.ArmType
	}
	if strings.TrimSpace(string(opts.BlockSet)) == "" {
		opts.BlockSet = defaults.BlockSet
	}
	if opts.AlphaCutoff == 0 {
		opts.AlphaCutoff = defaults.AlphaCutoff
	}
	if opts.OuterThickness <= 0 {
		opts.OuterThickness = defaults.OuterThickness
	}
	if strings.TrimSpace(opts.FillBlock) == "" {
		opts.FillBlock = defaults.FillBlock
	}
	return opts
}

func loadSkin(path string) (skinImage, error) {
	img, err := imaging.Open(path)
	if err != nil {
		return skinImage{}, fmt.Errorf("打开皮肤图片失败: %w", err)
	}
	nrgba := imageToNRGBA(img)
	width := nrgba.Bounds().Dx()
	height := nrgba.Bounds().Dy()
	if !((width == 64 && height == 64) || (width == 64 && height == 32) || (width == 128 && height == 128)) {
		return skinImage{}, fmt.Errorf("不支持的皮肤尺寸: %dx%d，仅支持 64x64 / 64x32 / 128x128", width, height)
	}
	unit := width / 64
	if unit < 1 {
		unit = 1
	}
	return skinImage{
		width:  width,
		height: height,
		unit:   unit,
		img:    nrgba,
	}, nil
}

func validateSkinWorldHeight(skin skinImage, opts SkinBuildOptions) error {
	_, height, _ := skinStatueDimensions(skin, opts.Scale)
	height += opts.OuterThickness * 2
	maxY := skinBuildY + int32(height) - 1
	if maxY > 319 {
		return fmt.Errorf("雕塑高度超出世界高度限制: 最高 Y=%d，请降低缩放比例", maxY)
	}
	return nil
}

func skinStatueDimensions(skin skinImage, scale int) (int, int, int) {
	return 16 * skin.unit * scale, 32 * skin.unit * scale, 8 * skin.unit * scale
}

func uvRectsFromNet(u0, v0, w, h, d int, unit int) map[skinFace]skinFaceUV {
	u0 *= unit
	v0 *= unit
	w *= unit
	h *= unit
	d *= unit
	return map[skinFace]skinFaceUV{
		skinFaceUp:    {u: u0 + d, v: v0, w: w, h: d},
		skinFaceDown:  {u: u0 + d + w, v: v0, w: w, h: d},
		skinFaceWest:  {u: u0, v: v0 + d, w: d, h: h},
		skinFaceSouth: {u: u0 + d, v: v0 + d, w: w, h: h},
		skinFaceEast:  {u: u0 + d + w, v: v0 + d, w: d, h: h},
		skinFaceNorth: {u: u0 + d + w + d, v: v0 + d, w: w, h: h},
	}
}

func skinPartsForClassic(skin skinImage, scale int) []skinPart {
	unit := skin.unit
	headBase := uvRectsFromNet(0, 0, 8, 8, 8, unit)
	bodyBase := uvRectsFromNet(16, 16, 8, 12, 4, unit)
	rightArmBase := uvRectsFromNet(40, 16, 4, 12, 4, unit)
	rightLegBase := uvRectsFromNet(0, 16, 4, 12, 4, unit)

	isModern := skin.width == 64*unit && skin.height == 64*unit
	leftArmBase := rightArmBase
	leftLegBase := rightLegBase
	if isModern {
		leftArmBase = uvRectsFromNet(32, 48, 4, 12, 4, unit)
		leftLegBase = uvRectsFromNet(16, 48, 4, 12, 4, unit)
	}

	var headOverlay map[skinFace]skinFaceUV
	var bodyOverlay map[skinFace]skinFaceUV
	var rightArmOverlay map[skinFace]skinFaceUV
	var rightLegOverlay map[skinFace]skinFaceUV
	var leftLegOverlay map[skinFace]skinFaceUV
	var leftArmOverlay map[skinFace]skinFaceUV
	if isModern {
		headOverlay = uvRectsFromNet(32, 0, 8, 8, 8, unit)
		bodyOverlay = uvRectsFromNet(16, 32, 8, 12, 4, unit)
		rightArmOverlay = uvRectsFromNet(40, 32, 4, 12, 4, unit)
		rightLegOverlay = uvRectsFromNet(0, 32, 4, 12, 4, unit)
		leftLegOverlay = uvRectsFromNet(0, 48, 4, 12, 4, unit)
		leftArmOverlay = uvRectsFromNet(48, 48, 4, 12, 4, unit)
	}

	s := scale * unit
	return []skinPart{
		{name: "head", origin: skinPos{4 * s, 24 * s, 0}, size: skinSize{8 * s, 8 * s, 8 * s}, baseUV: headBase, overlayUV: headOverlay},
		{name: "body", origin: skinPos{4 * s, 12 * s, 2 * s}, size: skinSize{8 * s, 12 * s, 4 * s}, baseUV: bodyBase, overlayUV: bodyOverlay},
		{name: "arm_right", origin: skinPos{0, 12 * s, 2 * s}, size: skinSize{4 * s, 12 * s, 4 * s}, baseUV: rightArmBase, overlayUV: rightArmOverlay},
		{name: "arm_left", origin: skinPos{12 * s, 12 * s, 2 * s}, size: skinSize{4 * s, 12 * s, 4 * s}, baseUV: leftArmBase, overlayUV: leftArmOverlay},
		{name: "leg_right", origin: skinPos{4 * s, 0, 2 * s}, size: skinSize{4 * s, 12 * s, 4 * s}, baseUV: rightLegBase, overlayUV: rightLegOverlay},
		{name: "leg_left", origin: skinPos{8 * s, 0, 2 * s}, size: skinSize{4 * s, 12 * s, 4 * s}, baseUV: leftLegBase, overlayUV: leftLegOverlay},
	}
}

func skinPartsForSlim(skin skinImage, scale int) ([]skinPart, error) {
	unit := skin.unit
	if skin.width != 64*unit || skin.height != 64*unit {
		return nil, fmt.Errorf("非现代皮肤不支持 slim 细手臂")
	}

	armWidth := 3
	headBase := uvRectsFromNet(0, 0, 8, 8, 8, unit)
	bodyBase := uvRectsFromNet(16, 16, 8, 12, 4, unit)
	rightArmBase := uvRectsFromNet(40, 16, armWidth, 12, 4, unit)
	leftArmBase := uvRectsFromNet(32, 48, armWidth, 12, 4, unit)
	rightLegBase := uvRectsFromNet(0, 16, 4, 12, 4, unit)
	leftLegBase := uvRectsFromNet(16, 48, 4, 12, 4, unit)

	headOverlay := uvRectsFromNet(32, 0, 8, 8, 8, unit)
	bodyOverlay := uvRectsFromNet(16, 32, 8, 12, 4, unit)
	rightArmOverlay := uvRectsFromNet(40, 32, armWidth, 12, 4, unit)
	rightLegOverlay := uvRectsFromNet(0, 32, 4, 12, 4, unit)
	leftLegOverlay := uvRectsFromNet(0, 48, 4, 12, 4, unit)
	leftArmOverlay := uvRectsFromNet(48, 48, armWidth, 12, 4, unit)

	s := scale * unit
	return []skinPart{
		{name: "head", origin: skinPos{4 * s, 24 * s, 0}, size: skinSize{8 * s, 8 * s, 8 * s}, baseUV: headBase, overlayUV: headOverlay},
		{name: "body", origin: skinPos{4 * s, 12 * s, 2 * s}, size: skinSize{8 * s, 12 * s, 4 * s}, baseUV: bodyBase, overlayUV: bodyOverlay},
		{name: "arm_right", origin: skinPos{(4 - armWidth) * s, 12 * s, 2 * s}, size: skinSize{armWidth * s, 12 * s, 4 * s}, baseUV: rightArmBase, overlayUV: rightArmOverlay},
		{name: "arm_left", origin: skinPos{12 * s, 12 * s, 2 * s}, size: skinSize{armWidth * s, 12 * s, 4 * s}, baseUV: leftArmBase, overlayUV: leftArmOverlay},
		{name: "leg_right", origin: skinPos{4 * s, 0, 2 * s}, size: skinSize{4 * s, 12 * s, 4 * s}, baseUV: rightLegBase, overlayUV: rightLegOverlay},
		{name: "leg_left", origin: skinPos{8 * s, 0, 2 * s}, size: skinSize{4 * s, 12 * s, 4 * s}, baseUV: leftLegBase, overlayUV: leftLegOverlay},
	}, nil
}

func skinParts(skin skinImage, scale int, armType SkinArmType) ([]skinPart, error) {
	switch armType {
	case SkinArmClassic:
		return skinPartsForClassic(skin, scale), nil
	case SkinArmSlim:
		return skinPartsForSlim(skin, scale)
	default:
		return nil, fmt.Errorf("未知手臂类型: %s", armType)
	}
}

func buildSkinStatueBlocks(skin skinImage, palette skinPalette, fillRuntimeID uint32, opts SkinBuildOptions) (map[skinPos]uint32, error) {
	parts, err := skinParts(skin, opts.Scale, opts.ArmType)
	if err != nil {
		return nil, err
	}

	blocks := make(map[skinPos]uint32)
	for _, part := range parts {
		ox, oy, oz := part.origin.x, part.origin.y, part.origin.z
		sw, sh, sd := part.size.w, part.size.h, part.size.d
		w, h, d := sw/opts.Scale, sh/opts.Scale, sd/opts.Scale

		if opts.Solid {
			for yb := 0; yb < sh; yb++ {
				for zb := 0; zb < sd; zb++ {
					for xb := 0; xb < sw; xb++ {
						blocks[skinPos{ox + xb, oy + yb, oz + zb}] = fillRuntimeID
					}
				}
			}
		}

		for yb := 0; yb < sh; yb++ {
			for zb := 0; zb < sd; zb++ {
				for xb := 0; xb < sw; xb++ {
					face, ok := surfaceFaceForSkinBlock(xb, yb, zb, sw, sh, sd)
					if !ok {
						continue
					}

					xt, yt, zt := xb/opts.Scale, yb/opts.Scale, zb/opts.Scale
					uLocal, vLocal := skinFaceTexelCoords(face, xt, yt, zt, w, h, d)
					uv := part.baseUV[face]
					pixel := skin.img.NRGBAAt(uv.u+uLocal, uv.v+vLocal)
					if pixel.A < opts.AlphaCutoff {
						continue
					}
					blocks[skinPos{ox + xb, oy + yb, oz + zb}] = palette.nearestRuntimeID(pixel.R, pixel.G, pixel.B)
				}
			}
		}

		if part.overlayUV != nil {
			writeSkinOverlay(blocks, skin, palette, part, w, h, d, opts)
		}
	}
	return blocks, nil
}

func writeSkinOverlay(blocks map[skinPos]uint32, skin skinImage, palette skinPalette, part skinPart, w, h, d int, opts SkinBuildOptions) {
	t := opts.OuterThickness
	ox, oy, oz := part.origin.x, part.origin.y, part.origin.z
	sw, sh, sd := part.size.w, part.size.h, part.size.d

	for face, uv := range part.overlayUV {
		switch face {
		case skinFaceSouth, skinFaceNorth:
			for yt := 0; yt < h; yt++ {
				for xt := 0; xt < w; xt++ {
					zt := 0
					if face == skinFaceSouth {
						zt = d - 1
					}
					uLocal, vLocal := skinFaceTexelCoords(face, xt, yt, zt, w, h, d)
					pixel := skin.img.NRGBAAt(uv.u+uLocal, uv.v+vLocal)
					if pixel.A < opts.AlphaCutoff {
						continue
					}
					runtimeID := palette.nearestRuntimeID(pixel.R, pixel.G, pixel.B)
					x0, x1 := ox+xt*opts.Scale, ox+(xt+1)*opts.Scale
					y0, y1 := oy+yt*opts.Scale, oy+(yt+1)*opts.Scale
					z0, z1 := oz-t, oz
					if face == skinFaceSouth {
						z0, z1 = oz+sd, oz+sd+t
					}
					fillSkinOverlayBlocks(blocks, x0, x1, y0, y1, z0, z1, runtimeID)
				}
			}
		case skinFaceWest, skinFaceEast:
			for yt := 0; yt < h; yt++ {
				for zt := 0; zt < d; zt++ {
					xt := 0
					if face == skinFaceEast {
						xt = w - 1
					}
					uLocal, vLocal := skinFaceTexelCoords(face, xt, yt, zt, w, h, d)
					pixel := skin.img.NRGBAAt(uv.u+uLocal, uv.v+vLocal)
					if pixel.A < opts.AlphaCutoff {
						continue
					}
					runtimeID := palette.nearestRuntimeID(pixel.R, pixel.G, pixel.B)
					z0, z1 := oz+zt*opts.Scale, oz+(zt+1)*opts.Scale
					y0, y1 := oy+yt*opts.Scale, oy+(yt+1)*opts.Scale
					x0, x1 := ox-t, ox
					if face == skinFaceEast {
						x0, x1 = ox+sw, ox+sw+t
					}
					fillSkinOverlayBlocks(blocks, x0, x1, y0, y1, z0, z1, runtimeID)
				}
			}
		case skinFaceUp, skinFaceDown:
			for zt := 0; zt < d; zt++ {
				for xt := 0; xt < w; xt++ {
					yt := 0
					if face == skinFaceUp {
						yt = h - 1
					}
					uLocal, vLocal := skinFaceTexelCoords(face, xt, yt, zt, w, h, d)
					pixel := skin.img.NRGBAAt(uv.u+uLocal, uv.v+vLocal)
					if pixel.A < opts.AlphaCutoff {
						continue
					}
					runtimeID := palette.nearestRuntimeID(pixel.R, pixel.G, pixel.B)
					x0, x1 := ox+xt*opts.Scale, ox+(xt+1)*opts.Scale
					z0, z1 := oz+zt*opts.Scale, oz+(zt+1)*opts.Scale
					y0, y1 := oy-t, oy
					if face == skinFaceUp {
						y0, y1 = oy+sh, oy+sh+t
					}
					fillSkinOverlayBlocks(blocks, x0, x1, y0, y1, z0, z1, runtimeID)
				}
			}
		}
	}
}

func fillSkinOverlayBlocks(blocks map[skinPos]uint32, x0, x1, y0, y1, z0, z1 int, runtimeID uint32) {
	for yb := y0; yb < y1; yb++ {
		for xb := x0; xb < x1; xb++ {
			for zb := z0; zb < z1; zb++ {
				blocks[skinPos{xb, yb, zb}] = runtimeID
			}
		}
	}
}

func surfaceFaceForSkinBlock(x, y, z, w, h, d int) (skinFace, bool) {
	if z == d-1 {
		return skinFaceSouth, true
	}
	if z == 0 {
		return skinFaceNorth, true
	}
	if x == 0 {
		return skinFaceWest, true
	}
	if x == w-1 {
		return skinFaceEast, true
	}
	if y == h-1 {
		return skinFaceUp, true
	}
	if y == 0 {
		return skinFaceDown, true
	}
	return "", false
}

func skinFaceTexelCoords(face skinFace, x, y, z, w, h, d int) (int, int) {
	switch face {
	case skinFaceSouth:
		return x, h - 1 - y
	case skinFaceNorth:
		return w - 1 - x, h - 1 - y
	case skinFaceWest:
		return z, h - 1 - y
	case skinFaceEast:
		return d - 1 - z, h - 1 - y
	case skinFaceUp:
		return x, z
	case skinFaceDown:
		return x, d - 1 - z
	default:
		return 0, 0
	}
}

func loadSkinPalette(blockSet SkinBlockSet) (skinPalette, error) {
	skinPaletteCacheMu.Lock()
	defer skinPaletteCacheMu.Unlock()
	if palette, ok := skinPaletteCache[blockSet]; ok {
		return palette, nil
	}

	specs, err := skinPaletteSpecsForBlockSet(blockSet)
	if err != nil {
		return skinPalette{}, err
	}
	entries := make([]skinPaletteEntry, 0, len(specs))
	for _, spec := range specs {
		runtimeID, err := resolveSkinBlockRuntimeID(spec.name)
		if err != nil {
			log.Log.Warn("SkinBuilder 调色板方块不可用，已跳过", log.Log.ArgsFromMap(map[string]any{"block": spec.name, "error": err.Error()}))
			continue
		}
		entries = append(entries, skinPaletteEntry{name: spec.name, runtimeID: runtimeID, r: spec.r, g: spec.g, b: spec.b})
	}
	if len(entries) == 0 {
		return skinPalette{}, fmt.Errorf("SkinBuilder 调色板没有可用方块")
	}

	palette := skinPalette{entries: entries}
	skinPaletteCache[blockSet] = palette
	return palette, nil
}

func resolveSkinBlockRuntimeID(name string) (uint32, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("方块名为空")
	}
	if runtimeID, ok := bwoblock.StateToRuntimeID(name, nil); ok {
		return runtimeID, nil
	}
	if runtimeID, ok := wsblocks.BlockStrToRuntimeID(name); ok {
		baseName, properties, found := wsblocks.RuntimeIDToState(runtimeID)
		if found {
			if bedrockRuntimeID, ok := bwoblock.StateToRuntimeID("minecraft:"+baseName, properties); ok {
				return bedrockRuntimeID, nil
			}
		}
	}
	return 0, fmt.Errorf("无法解析方块运行时 ID: %s", name)
}

func (p skinPalette) nearestRuntimeID(r, g, b uint8) uint32 {
	best := p.entries[0]
	bestDistance := math.MaxInt
	for _, entry := range p.entries {
		dr := int(r) - int(entry.r)
		dg := int(g) - int(entry.g)
		db := int(b) - int(entry.b)
		distance := dr*dr + dg*dg + db*db
		if distance < bestDistance {
			bestDistance = distance
			best = entry
		}
	}
	return best.runtimeID
}

func skinPaletteSpecsForBlockSet(blockSet SkinBlockSet) ([]skinPaletteSpec, error) {
	concrete := paletteWithSuffix("concrete", adjustedSkinColors(skinBaseColors16, 1.00, 0.00))
	wool := paletteWithSuffix("wool", adjustedSkinColors(skinBaseColors16, 1.08, 0.04))
	terracotta := paletteWithSuffix("terracotta", adjustedSkinColors(skinBaseColors16, 0.90, 0.20))
	flesh := []skinPaletteSpec{
		{name: "minecraft:bone_block", r: 225, g: 214, b: 184},
		{name: "minecraft:mushroom_stem", r: 210, g: 206, b: 196},
		{name: "minecraft:clay", r: 160, g: 166, b: 179},
		{name: "minecraft:terracotta", r: 152, g: 94, b: 68},
		{name: "minecraft:packed_mud", r: 142, g: 123, b: 104},
	}
	extra := []skinPaletteSpec{
		{name: "minecraft:snow_block", r: 250, g: 252, b: 252},
		{name: "minecraft:quartz_block", r: 236, g: 233, b: 227},
		{name: "minecraft:calcite", r: 224, g: 227, b: 230},
		{name: "minecraft:iron_block", r: 220, g: 220, b: 220},
		{name: "minecraft:smooth_stone", r: 158, g: 158, b: 158},
		{name: "minecraft:stone", r: 125, g: 125, b: 125},
		{name: "minecraft:andesite", r: 136, g: 136, b: 137},
		{name: "minecraft:diorite", r: 184, g: 184, b: 186},
		{name: "minecraft:granite", r: 150, g: 103, b: 85},
		{name: "minecraft:tuff", r: 108, g: 109, b: 102},
		{name: "minecraft:dripstone_block", r: 134, g: 107, b: 92},
		{name: "minecraft:deepslate", r: 60, g: 60, b: 63},
		{name: "minecraft:blackstone", r: 44, g: 39, b: 45},
		{name: "minecraft:coal_block", r: 17, g: 17, b: 17},
		{name: "minecraft:obsidian", r: 20, g: 18, b: 30},
		{name: "minecraft:sandstone", r: 216, g: 203, b: 155},
		{name: "minecraft:smooth_sandstone", r: 220, g: 206, b: 159},
		{name: "minecraft:red_sandstone", r: 190, g: 102, b: 33},
		{name: "minecraft:smooth_red_sandstone", r: 188, g: 100, b: 30},
		{name: "minecraft:end_stone", r: 219, g: 222, b: 158},
		{name: "minecraft:ochre_froglight", r: 246, g: 217, b: 148},
		{name: "minecraft:pearlescent_froglight", r: 240, g: 236, b: 226},
		{name: "minecraft:verdant_froglight", r: 205, g: 233, b: 182},
		{name: "minecraft:oak_planks", r: 162, g: 131, b: 79},
		{name: "minecraft:spruce_planks", r: 114, g: 84, b: 48},
		{name: "minecraft:birch_planks", r: 206, g: 200, b: 144},
		{name: "minecraft:jungle_planks", r: 160, g: 115, b: 80},
		{name: "minecraft:acacia_planks", r: 169, g: 89, b: 51},
		{name: "minecraft:dark_oak_planks", r: 66, g: 43, b: 20},
		{name: "minecraft:mangrove_planks", r: 120, g: 54, b: 48},
		{name: "minecraft:crimson_planks", r: 122, g: 57, b: 84},
		{name: "minecraft:warped_planks", r: 44, g: 109, b: 89},
		{name: "minecraft:bamboo_planks", r: 203, g: 176, b: 84},
		{name: "minecraft:cherry_planks", r: 228, g: 166, b: 156},
		{name: "minecraft:amethyst_block", r: 133, g: 97, b: 191},
		{name: "minecraft:lapis_block", r: 30, g: 67, b: 140},
		{name: "minecraft:gold_block", r: 247, g: 208, b: 61},
		{name: "minecraft:redstone_block", r: 175, g: 27, b: 27},
		{name: "minecraft:packed_ice", r: 164, g: 200, b: 252},
		{name: "minecraft:blue_ice", r: 109, g: 168, b: 255},
	}

	switch blockSet {
	case SkinBlockSetConcrete:
		return concrete, nil
	case SkinBlockSetWool:
		return wool, nil
	case SkinBlockSetTerracotta:
		return terracotta, nil
	case SkinBlockSetMixed:
		return appendSpecs(concrete, wool, terracotta, flesh), nil
	case SkinBlockSetExtended:
		return appendSpecs(concrete, wool, terracotta, flesh, extra), nil
	default:
		return nil, fmt.Errorf("未知方块调色板: %s", blockSet)
	}
}

func appendSpecs(groups ...[]skinPaletteSpec) []skinPaletteSpec {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	out := make([]skinPaletteSpec, 0, total)
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

func paletteWithSuffix(blockSuffix string, colors []skinPaletteSpec) []skinPaletteSpec {
	out := make([]skinPaletteSpec, 0, len(colors))
	for _, spec := range colors {
		out = append(out, skinPaletteSpec{
			name: fmt.Sprintf("minecraft:%s_%s", spec.name, blockSuffix),
			r:    spec.r,
			g:    spec.g,
			b:    spec.b,
		})
	}
	return out
}

func adjustedSkinColors(colors []skinPaletteSpec, brightness, desaturate float64) []skinPaletteSpec {
	out := make([]skinPaletteSpec, 0, len(colors))
	for _, spec := range colors {
		r, g, b := adjustSkinRGB(spec.r, spec.g, spec.b, brightness, desaturate)
		out = append(out, skinPaletteSpec{name: spec.name, r: r, g: g, b: b})
	}
	return out
}

func adjustSkinRGB(r, g, b uint8, brightness, desaturate float64) (uint8, uint8, uint8) {
	rf, gf, bf := float64(r), float64(g), float64(b)
	lum := 0.2126*rf + 0.7152*gf + 0.0722*bf
	rf = ((1 - desaturate) * rf) + (desaturate * lum)
	gf = ((1 - desaturate) * gf) + (desaturate * lum)
	bf = ((1 - desaturate) * bf) + (desaturate * lum)
	rf *= brightness
	gf *= brightness
	bf *= brightness
	return clampSkinByte(rf), clampSkinByte(gf), clampSkinByte(bf)
}

func clampSkinByte(value float64) uint8 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(math.Round(value))
}

func buildSkinWorldName(inputPath string, width, height, length int) string {
	name := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if strings.TrimSpace(name) == "" {
		name = skinDefaultDataName
	}
	maxY := skinBuildY + int32(height) - 1
	return fmt.Sprintf("%s@[0,%d,0]~[%d,%d,%d]", name, skinBuildY, width-1, maxY, length-1)
}

var skinBaseColors16 = []skinPaletteSpec{
	{name: "white", r: 207, g: 213, b: 214},
	{name: "orange", r: 224, g: 97, b: 0},
	{name: "magenta", r: 169, g: 48, b: 159},
	{name: "light_blue", r: 36, g: 137, b: 199},
	{name: "yellow", r: 241, g: 175, b: 21},
	{name: "lime", r: 94, g: 168, b: 24},
	{name: "pink", r: 213, g: 101, b: 142},
	{name: "gray", r: 54, g: 57, b: 61},
	{name: "light_gray", r: 125, g: 125, b: 115},
	{name: "cyan", r: 21, g: 119, b: 136},
	{name: "purple", r: 100, g: 31, b: 156},
	{name: "blue", r: 44, g: 46, b: 143},
	{name: "brown", r: 96, g: 59, b: 31},
	{name: "green", r: 73, g: 91, b: 36},
	{name: "red", r: 142, g: 32, b: 32},
	{name: "black", r: 8, g: 10, b: 15},
}
