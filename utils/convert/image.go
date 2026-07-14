package convert

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	bwoblock "github.com/TriM-Organization/bedrock-world-operator/block"
	bwoworld "github.com/TriM-Organization/bedrock-world-operator/world"
	wsutils "github.com/Yeah114/WaterStructure/utils"
	"github.com/disintegration/imaging"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"nexus/utils/log"
)

const (
	imageAlphaThreshold = uint8(96)
	imageBuildY         = int32(-64)
)

type imagePaletteSpec struct {
	Name string
	R    uint8
	G    uint8
	B    uint8
}

type imagePaletteEntry struct {
	RuntimeID uint32
	R         uint8
	G         uint8
	B         uint8
}

var imageInputExts = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".bmp":  {},
	".webp": {},
	".gif":  {},
}

var imagePaletteSpecs = []imagePaletteSpec{
	{Name: "minecraft:white_concrete", R: 207, G: 213, B: 214},
	{Name: "minecraft:light_gray_concrete", R: 125, G: 125, B: 115},
	{Name: "minecraft:gray_concrete", R: 54, G: 57, B: 61},
	{Name: "minecraft:black_concrete", R: 8, G: 10, B: 15},
	{Name: "minecraft:brown_concrete", R: 96, G: 59, B: 31},
	{Name: "minecraft:red_concrete", R: 142, G: 32, B: 32},
	{Name: "minecraft:orange_concrete", R: 224, G: 97, B: 0},
	{Name: "minecraft:yellow_concrete", R: 241, G: 175, B: 21},
	{Name: "minecraft:lime_concrete", R: 94, G: 168, B: 24},
	{Name: "minecraft:green_concrete", R: 73, G: 91, B: 36},
	{Name: "minecraft:cyan_concrete", R: 21, G: 119, B: 136},
	{Name: "minecraft:light_blue_concrete", R: 36, G: 137, B: 199},
	{Name: "minecraft:blue_concrete", R: 44, G: 46, B: 143},
	{Name: "minecraft:purple_concrete", R: 100, G: 31, B: 156},
	{Name: "minecraft:magenta_concrete", R: 169, G: 48, B: 159},
	{Name: "minecraft:pink_concrete", R: 214, G: 101, B: 143},
	{Name: "minecraft:white_wool", R: 234, G: 236, B: 237},
	{Name: "minecraft:light_gray_wool", R: 142, G: 142, B: 135},
	{Name: "minecraft:gray_wool", R: 62, G: 68, B: 71},
	{Name: "minecraft:black_wool", R: 23, G: 21, B: 26},
	{Name: "minecraft:brown_wool", R: 114, G: 71, B: 40},
	{Name: "minecraft:red_wool", R: 160, G: 39, B: 34},
	{Name: "minecraft:orange_wool", R: 240, G: 118, B: 19},
	{Name: "minecraft:yellow_wool", R: 249, G: 198, B: 39},
	{Name: "minecraft:lime_wool", R: 112, G: 185, B: 25},
	{Name: "minecraft:green_wool", R: 84, G: 109, B: 27},
	{Name: "minecraft:cyan_wool", R: 21, G: 137, B: 145},
	{Name: "minecraft:light_blue_wool", R: 58, G: 179, B: 218},
	{Name: "minecraft:blue_wool", R: 53, G: 57, B: 157},
	{Name: "minecraft:purple_wool", R: 122, G: 42, B: 172},
	{Name: "minecraft:magenta_wool", R: 189, G: 68, B: 179},
	{Name: "minecraft:pink_wool", R: 237, G: 141, B: 172},
}

var (
	imagePaletteOnce sync.Once
	imagePalette     []imagePaletteEntry
	imagePaletteErr  error
)

func IsImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	_, ok := imageInputExts[ext]
	return ok
}

func DefaultImageTargetWidth(inputPath string) (int, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return 0, fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, fmt.Errorf("decode image config: %w", err)
	}
	if cfg.Width <= 0 {
		return 0, fmt.Errorf("invalid image width: %d", cfg.Width)
	}
	return cfg.Width, nil
}

func ConvertImageToMCWorld(inputPath, outputDir string, targetWidth int) (string, error) {
	if err := ensureImagePalette(); err != nil {
		return "", err
	}

	src, err := imaging.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open image: %w", err)
	}

	srcWidth := src.Bounds().Dx()
	srcHeight := src.Bounds().Dy()
	if srcWidth <= 0 || srcHeight <= 0 {
		return "", fmt.Errorf("invalid image size: %dx%d", srcWidth, srcHeight)
	}

	if targetWidth <= 0 {
		targetWidth = srcWidth
	}
	targetHeight := int(math.Round(float64(srcHeight) * float64(targetWidth) / float64(srcWidth)))
	if targetHeight < 1 {
		targetHeight = 1
	}

	resized := src
	if targetWidth != srcWidth || targetHeight != srcHeight {
		resized = imaging.Resize(src, targetWidth, targetHeight, imaging.NearestNeighbor)
	}
	pixels := imageToNRGBA(resized)

	worldPath, err := os.MkdirTemp("", "nexusego-image-")
	if err != nil {
		return "", fmt.Errorf("create temp world: %w", err)
	}
	defer os.RemoveAll(worldPath)

	bw, err := bwoworld.Open(worldPath, nil)
	if err != nil {
		return "", fmt.Errorf("open temp world: %w", err)
	}

	mcWorld, err := wsutils.NewMCWorld(bw, context.Background())
	if err != nil {
		_ = bw.CloseWorld()
		return "", fmt.Errorf("init temp world: %w", err)
	}

	colorCache := make(map[uint32]uint32, 256)
	nonAirBlocks := 0
	for z := 0; z < targetHeight; z++ {
		for x := 0; x < targetWidth; x++ {
			pixel := pixels.NRGBAAt(x, z)
			if pixel.A < imageAlphaThreshold {
				continue
			}

			runtimeID := nearestImageRuntimeID(pixel.R, pixel.G, pixel.B, colorCache)
			if err := mcWorld.SetBlock(int32(x), int16(imageBuildY), int32(z), runtimeID); err != nil {
				return "", fmt.Errorf("set block: %w", err)
			}
			nonAirBlocks++
		}
	}

	if nonAirBlocks == 0 {
		return "", fmt.Errorf("image contains no visible pixels")
	}

	mcWorld.Flush()
	levelName := buildImageWorldName(inputPath, targetWidth, targetHeight)
	bw.LevelDat().LevelName = levelName
	if err := mcWorld.Close(); err != nil {
		return "", fmt.Errorf("close world: %w", err)
	}

	log.Log.Info(fmt.Sprintf("结构类型: %s", imageStructureType(inputPath)))
	log.Log.Info(fmt.Sprintf("图片尺寸: %dx%d -> %dx%d", srcWidth, srcHeight, targetWidth, targetHeight))
	log.Log.Info(fmt.Sprintf("非空气方块: %d", nonAirBlocks))

	outputPath, err := archiveWorldAsMCWorld(worldPath, levelName, outputDir)
	if err != nil {
		return "", err
	}
	log.Log.Info(fmt.Sprintf("MCWorld 文件已保存到: %s", outputPath))
	return outputPath, nil
}

func ensureImagePalette() error {
	imagePaletteOnce.Do(func() {
		imagePalette = make([]imagePaletteEntry, 0, len(imagePaletteSpecs))
		for _, spec := range imagePaletteSpecs {
			runtimeID, ok := bwoblock.StateToRuntimeID(spec.Name, nil)
			if !ok {
				imagePaletteErr = fmt.Errorf("unknown block runtime id: %s", spec.Name)
				return
			}
			imagePalette = append(imagePalette, imagePaletteEntry{
				RuntimeID: runtimeID,
				R:         spec.R,
				G:         spec.G,
				B:         spec.B,
			})
		}
	})
	return imagePaletteErr
}

func nearestImageRuntimeID(r, g, b uint8, cache map[uint32]uint32) uint32 {
	key := uint32(r)<<16 | uint32(g)<<8 | uint32(b)
	if runtimeID, ok := cache[key]; ok {
		return runtimeID
	}

	bestRuntimeID := imagePalette[0].RuntimeID
	bestDistance := int(^uint(0) >> 1)
	for _, entry := range imagePalette {
		dr := int(r) - int(entry.R)
		dg := int(g) - int(entry.G)
		db := int(b) - int(entry.B)
		distance := dr*dr + dg*dg + db*db
		if distance < bestDistance {
			bestDistance = distance
			bestRuntimeID = entry.RuntimeID
		}
	}

	cache[key] = bestRuntimeID
	return bestRuntimeID
}

func buildImageWorldName(inputPath string, width, height int) string {
	imageName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if strings.TrimSpace(imageName) == "" {
		imageName = "image"
	}
	return fmt.Sprintf("%s@[0,%d,0]~[%d,%d,%d]", imageName, imageBuildY, width-1, imageBuildY, height-1)
}

func imageStructureType(inputPath string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(inputPath))), ".")
	if ext == "" {
		return "image"
	}
	return ext
}

func imageToNRGBA(img image.Image) *image.NRGBA {
	if nrgba, ok := img.(*image.NRGBA); ok {
		return nrgba
	}
	return imaging.Clone(img)
}
