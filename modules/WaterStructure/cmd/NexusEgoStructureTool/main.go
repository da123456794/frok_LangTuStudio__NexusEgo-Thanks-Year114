package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/TriM-Organization/bedrock-world-operator/world"
	"github.com/schollz/progressbar/v3"

	"github.com/Yeah114/WaterStructure/define"
	"github.com/Yeah114/WaterStructure/structure"
)

func main() {
	start := time.Now()
	defer func() {
		fmt.Printf("转换耗时: %v\n", time.Since(start))
	}()

	var names []string
	for name := range structure.StructureNamePool {
		names = append(names, name)
	}
	fmt.Printf("支持的结构文件格式: %s\n", strings.Join(names, " "))

	reader := bufio.NewReader(os.Stdin)
	srcPath := readLine(reader, "请输入源结构文件路径: ")
	if srcPath == "" {
		panic("未提供源文件路径")
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		panic(fmt.Sprintf("打开源文件失败: %v", err))
	}
	defer srcFile.Close()

	var srcStruct structure.Structure
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == ".nexus" {
		srcPwd := readLine(reader, "检测到源文件为 .nexus，如有密码请输入(可留空): ")
		n := &structure.Nexus{Password: strings.TrimSpace(srcPwd)}
		if err := n.FromFile(srcFile); err != nil {
			panic(fmt.Sprintf("解析源 nexus 失败: %v", err))
		}
		srcStruct = n
	} else {
		srcStruct, err = structure.StructureFromFile(srcFile)
		if err != nil {
			panic(fmt.Sprintf("解析源文件失败: %v", err))
		}
	}
	if srcStruct != nil {
		defer srcStruct.Close()
	}

	fmt.Printf("检测到源文件类型: %s\n", srcStruct.Name())

	outPath := readLine(reader, "请输入输出 nexus 文件路径(留空默认同目录同名): ")
	if outPath == "" {
		outPath = strings.TrimSuffix(srcPath, filepath.Ext(srcPath)) + ".nexus"
	}
	if filepath.Ext(outPath) == "" {
		outPath += ".nexus"
	} else if strings.ToLower(filepath.Ext(outPath)) != ".nexus" {
		outPath = strings.TrimSuffix(outPath, filepath.Ext(outPath)) + ".nexus"
	}

	outDir := filepath.Dir(outPath)
	if outDir != "" && outDir != "." {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			panic(fmt.Sprintf("创建输出目录失败: %v", err))
		}
	}

	author := strings.TrimSpace(readLine(reader, "请输入作者(留空默认 nexus): "))
	if author == "" {
		author = "nexus"
	}
	password := strings.TrimSpace(readLine(reader, "请输入输出密码(可留空): "))

	tempDir, err := os.MkdirTemp("", "ws_nexus_*")
	if err != nil {
		panic(fmt.Sprintf("创建临时世界失败: %v", err))
	}
	defer os.RemoveAll(tempDir)

	bedrockWorld, err := world.Open(tempDir, nil)
	if err != nil {
		panic(fmt.Sprintf("打开临时世界失败: %v", err))
	}
	defer func() {
		_ = bedrockWorld.CloseWorld()
		_ = bedrockWorld.Close()
	}()

	startSubChunkPos := define.SubChunkPos{0, -4, 0}
	var bar *progressbar.ProgressBar
	fmt.Println("正在写入临时世界...")
	if err := srcStruct.ToMCWorld(
		bedrockWorld,
		startSubChunkPos,
		func(total int) {
			bar = buildBar(total, "写入世界")
		},
		func() {
			if bar != nil {
				_ = bar.Add(1)
			}
		},
	); err != nil {
		panic(fmt.Sprintf("写入临时世界失败: %v", err))
	}
	if bar != nil {
		_ = bar.Finish()
	}

	size := srcStruct.GetSize()
	startBlockPos := define.BlockPos{
		startSubChunkPos.X() * 16,
		startSubChunkPos.Y() * 16,
		startSubChunkPos.Z() * 16,
	}
	endBlockPos := define.BlockPos{
		startBlockPos.X() + int32(size.Width) - 1,
		startBlockPos.Y() + int32(size.Height) - 1,
		startBlockPos.Z() + int32(size.Length) - 1,
	}

	targetFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		panic(fmt.Sprintf("创建输出文件失败: %v", err))
	}
	defer targetFile.Close()

	fmt.Println("正在导出 nexus...")
	bar = nil
	nexus := &structure.Nexus{
		Author:   author,
		Password: password,
	}
	if err := nexus.FromMCWorld(
		bedrockWorld,
		targetFile,
		startBlockPos,
		endBlockPos,
		func(total int) {
			bar = buildBar(total, "导出 nexus")
		},
		func() {
			if bar != nil {
				_ = bar.Add(1)
			}
		},
	); err != nil {
		panic(fmt.Sprintf("导出 nexus 失败: %v", err))
	}
	if bar != nil {
		_ = bar.Finish()
	}

	fmt.Printf("转换完成，输出文件: %s\n", outPath)
}

func readLine(r *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	text, _ := r.ReadString('\n')
	return strings.TrimSpace(text)
}

func buildBar(total int, desc string) *progressbar.ProgressBar {
	if total <= 0 {
		return nil
	}
	return progressbar.NewOptions(
		total,
		progressbar.OptionSetWriter(os.Stdout),
		progressbar.OptionSetDescription(desc),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "#",
			SaucerHead:    ">",
			SaucerPadding: "-",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
	)
}
