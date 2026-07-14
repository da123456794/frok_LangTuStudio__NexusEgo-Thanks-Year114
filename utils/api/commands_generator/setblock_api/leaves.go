package setblock_api

import (
	"fmt"
	"strconv"

	types "nexus/defines"
)

func Leaves(module *types.Module, config *types.MainConfig) string {
	block := module.Block
	point := module.Point
	method := config.Method
	var data uint16
	if IsNum(block.BlockStates) {
		data1, _ := strconv.Atoi(block.BlockStates)
		data = uint16(data1)
	} else {
		data = block.Data
	}
	switch data {
	case 0: // 橡树树叶
	case 1: // 云杉树叶
	case 2: // 白桦树叶
	case 3: // 丛林树叶
	case 4: // 橡树树叶（检查枯萎）
		data -= 4
	case 5: // 云杉树叶（检查枯萎）
		data -= 4
	case 6: // 白桦树叶（检查枯萎）
		data -= 4
	case 7: // 丛林树叶（检查枯萎）
		data -= 4
	case 8: // 橡树树叶（不枯萎）
		data -= 8
	case 9: // 云杉树叶（不枯萎）
		data -= 8
	case 10: // 白桦树叶（不枯萎）
		data -= 8
	case 11: // 丛林树叶（不枯萎）
		data -= 8
	case 12: // 橡树树叶（不枯萎且检查枯萎）
		data -= 12
	case 13: // 云杉树叶（不枯萎且检查枯萎）
		data -= 12
	case 14: // 白桦树叶（不枯萎且检查枯萎）
		data -= 12
	case 15: // 丛林树叶（不枯萎且检查枯萎）
		data -= 12
	}
	return fmt.Sprintf("setblock %d %d %d leaves %d %s", point.X, point.Y, point.Z, data, method)
}

func Leaves2(module *types.Module, config *types.MainConfig) string {
	block := module.Block
	point := module.Point
	method := config.Method
	var data uint16
	if IsNum(block.BlockStates) {
		data1, _ := strconv.Atoi(block.BlockStates)
		data = uint16(data1)
	} else {
		data = block.Data
	}
	switch data {
	case 0: // 金合欢树叶
	case 1: // 深色橡树叶
	case 4: // 金合欢树叶（检查枯萎）
		data = 0
	case 5: // 深色橡树叶（检查枯萎）
		data = 1
	case 8: // 金合欢树叶（不枯萎）
		data = 0
	case 9: // 深色橡树叶（不枯萎）
		data = 1
	case 12: // 金合欢树叶（不枯萎且检查枯萎）
		data = 0
	case 13: // 深色橡树叶（不枯萎且检查枯萎）
		data = 1
	default:
		data = 0
	}
	return fmt.Sprintf("setblock %d %d %d leaves2 %d %s", point.X, point.Y, point.Z, data, method)
}

func Azalea_leaves(module *types.Module, config *types.MainConfig) string {
	point := module.Point
	method := config.Method
	return fmt.Sprintf("setblock %d %d %d azalea_leaves 0 %s", point.X, point.Y, point.Z, method)
}

func Azalea_leaves_flowered(module *types.Module, config *types.MainConfig) string {
	point := module.Point
	method := config.Method
	return fmt.Sprintf("setblock %d %d %d azalea_leaves_flowered 0 %s", point.X, point.Y, point.Z, method)
}
