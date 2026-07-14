package function

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	clientType "nexus/utils/client"
	convertpkg "nexus/utils/convert"
	"nexus/utils/dimension"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
)

const exportChunkRetries = 1

func Cexport(client *clientType.Client, words []string) bool {
	if len(words) == 1 || words[1] == "help" {
		return Cexport_Help(client, words)
	}
	switch words[1] {
	case "save":
		return Cexport_Save(client, words)
	default:
		pterm.Println(pterm.Red("未知的导出命令, 请使用 /cexport help 获取帮助"))
		return true
	}
}

func Cexport_Help(client *clientType.Client, words []string) bool {
	pterm.Info.Println("cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension] - export mcworld/nexus")
	return true
}

func Cexport_Save(client *clientType.Client, words []string) bool {
	if len(words) < 9 {
		pterm.Println(pterm.Red("缺少参数. 请使用 /cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension]"))
		return true
	}
	filePath := words[2]
	if filepath.Ext(filePath) == "" {
		filePath += ".mcworld"
	} else {
		ext := strings.ToLower(filepath.Ext(filePath))
		if ext != ".mcworld" && ext != ".nexus" {
			pterm.Println(pterm.Red("only .mcworld or .nexus"))
			return true
		}
	}
	x1, err := strconv.Atoi(words[3])
	if err != nil {
		pterm.Println(pterm.Red("参数错误. 请使用 /cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension]"))
		return true
	}
	y1, err := strconv.Atoi(words[4])
	if err != nil {
		pterm.Println(pterm.Red("参数错误. 请使用 /cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension]"))
		return true
	}
	z1, err := strconv.Atoi(words[5])
	if err != nil {
		pterm.Println(pterm.Red("参数错误. 请使用 /cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension]"))
		return true
	}
	x2, err := strconv.Atoi(words[6])
	if err != nil {
		pterm.Println(pterm.Red("参数错误. 请使用 /cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension]"))
		return true
	}
	y2, err := strconv.Atoi(words[7])
	if err != nil {
		pterm.Println(pterm.Red("参数错误. 请使用 /cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension]"))
		return true
	}
	z2, err := strconv.Atoi(words[8])
	if err != nil {
		pterm.Println(pterm.Red("参数错误. 请使用 /cexport save <file_path> <x1> <y1> <z1> <x2> <y2> <z2> [dimension]"))
		return true
	}

	dimInput := ""
	if len(words) >= 10 {
		dimInput = words[9]
	}
	if dimInput != "" {
		dimInfo, err := dimension.Parse(dimInput)
		if err != nil {
			pterm.Println(pterm.Red(fmt.Sprintf("维度参数错误: %v", err)))
			return true
		}
		prevDimName := client.CommandDimension
		prevDimID := client.DimensionID
		client.CommandDimension = dimInfo.Name
		client.DimensionID = dimInfo.ID
		defer func() {
			client.CommandDimension = prevDimName
			client.DimensionID = prevDimID
		}()
	}

	output, err := ExportMCWorld(client, filePath, x1, y1, z1, x2, y2, z2, false)
	if err != nil {
		pterm.Println(pterm.Red(fmt.Sprintf("导出失败: %v", err)))
		return true
	}

	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".nexus":
		nexusPath, err := convertpkg.ConvertMCWorldToNexus(output, filePath, "", "")
		if err != nil {
			pterm.Println(pterm.Red(fmt.Sprintf("nexus convert failed: %v", err)))
			return true
		}
		_ = os.Remove(output)
		pterm.Println(pterm.Green("export done:", pterm.Blue(nexusPath)))
	default:
		pterm.Println(pterm.Green("export done:", pterm.Blue(output)))
	}
	return true
}

func ExportMCWorld(client *clientType.Client, filePath string, x1, y1, z1, x2, y2, z2 int, waitChunkLoad bool) (string, error) {
	config := DefaultOptimizedExportConfig()
	config.WaitChunkLoad = waitChunkLoad
	return ExportMCWorldOptimized(client, filePath, x1, y1, z1, x2, y2, z2, config)
}

func archiveWorldAsMCWorld(worldDir, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	bufWriter := bufio.NewWriterSize(outFile, 256*1024)
	zipWriter := zip.NewWriter(bufWriter)

	walkErr := filepath.Walk(worldDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(worldDir, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if relPath != "." {
				_, err := zipWriter.Create(relPath + "/")
				return err
			}
			return nil
		}
		writer, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(writer, file)
		return err
	})
	if walkErr != nil {
		zipWriter.Close()
		bufWriter.Flush()
		outFile.Close()
		return walkErr
	}
	if err := zipWriter.Close(); err != nil {
		bufWriter.Flush()
		outFile.Close()
		return err
	}
	if err := bufWriter.Flush(); err != nil {
		outFile.Close()
		return err
	}
	return outFile.Close()
}
