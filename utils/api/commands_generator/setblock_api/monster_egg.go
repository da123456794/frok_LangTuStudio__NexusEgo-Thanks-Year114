package setblock_api

import (
	"fmt"
	"strconv"
	"nexus/defines"
)

func Monster_egg(module *types.Module, config *types.MainConfig) string {
	Block := module.Block
	Point := module.Point
	Method := config.Method
	var data uint16 = 0
	if IsNum(Block.BlockStates) {
		data_1, _ := strconv.Atoi(Block.BlockStates)
		data = uint16(data_1)
	} else {
		data = Block.Data
	}
	block := ""
	switch data {
	case 0: //铏殌鐭冲ご
		block = "stone"
	case 1: //铏殌鍦嗙煶
		block = "cobblestone"
	case 2: //铏殌鐭崇爾
		block = "stone_brick"
	case 3: //铏殌鑻旂煶鐮?		block = "mossy_stone_brick"
	case 4: //铏殌瑁傜汗鐭崇爾
		block = "cracked_stone_brick"
	case 5: //铏殌闆曠汗鐭崇爾
		block = "chiseled_stone_brick"
	default:
		block = "stone"
	}
	return fmt.Sprintf("setblock %d %d %d %s 0 %s", Point.X, Point.Y, Point.Z, block, Method)
}
func Infested_deepslate(module *types.Module, config *types.MainConfig) string {
	Block := module.Block
	Point := module.Point
	Method := config.Method
	var data uint16 = 0
	if IsNum(Block.BlockStates) {
		data_1, _ := strconv.Atoi(Block.BlockStates)
		data = uint16(data_1)
	} else {
		data = Block.Data
	}
	return fmt.Sprintf("setblock %d %d %d deepslate %d %s", Point.X, Point.Y, Point.Z, data, Method)
}
