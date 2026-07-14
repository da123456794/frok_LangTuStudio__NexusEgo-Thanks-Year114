package convert

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertSkinToMCWorld(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "test_skin.png")
	outputDir := filepath.Join(tempDir, "out")

	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 3), B: 96, A: 255})
		}
	}

	file, err := os.Create(inputPath)
	if err != nil {
		t.Fatalf("create skin: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("encode skin: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close skin: %v", err)
	}

	outputPath, err := ConvertSkinToMCWorld(inputPath, outputDir, SkinBuildOptions{
		Scale:          1,
		ArmType:        SkinArmClassic,
		BlockSet:       SkinBlockSetConcrete,
		AlphaCutoff:    16,
		OuterThickness: 1,
	})
	if err != nil {
		t.Fatalf("convert skin: %v", err)
	}
	if !strings.HasSuffix(strings.ToLower(outputPath), ".mcworld") {
		t.Fatalf("unexpected output extension: %s", outputPath)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if !strings.Contains(filepath.Base(outputPath), "@[0,-64,0]~") {
		t.Fatalf("output name missing bounds: %s", outputPath)
	}
}
