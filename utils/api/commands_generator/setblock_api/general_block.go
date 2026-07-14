package setblock_api

import (
	"fmt"
	"strconv"

	types "nexus/defines"
)

func General_block(module *types.Module, config *types.MainConfig) string {
	block := module.Block
	point := module.Point
	method := config.Method

	if len(block.BlockStates) == 0 {
		return fmt.Sprintf("setblock %d %d %d %s %d %s", point.X, point.Y, point.Z, *block.Name, block.Data, method)
	}
	if block.BlockStates[0] == '[' || IsNum(block.BlockStates) {
		return fmt.Sprintf("setblock %d %d %d %s %s %s", point.X, point.Y, point.Z, *block.Name, block.BlockStates, method)
	}

	return fmt.Sprintf("setblock %d %d %d %s %d %s", point.X, point.Y, point.Z, *block.Name, block.Data, method)
}

func IsNum(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
