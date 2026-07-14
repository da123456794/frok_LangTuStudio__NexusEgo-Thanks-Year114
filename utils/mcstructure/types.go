package mcstructure

import (
	"encoding/json"
	"fmt"
)

// Area 描述一个结构的起点坐标及尺寸
type Area struct {
	BeginX int
	BeginY int
	BeginZ int
	SizeX  int
	SizeY  int
	SizeZ  int
}

// BlockPos 描述一个单个方块的位置
type BlockPos struct {
	X int
	Y int
	Z int
}

// UnmarshalBlockStates 解析方块状态字符串为 map
func UnmarshalBlockStates(blockStates string) (map[string]interface{}, error) {
	if blockStates == "" {
		return make(map[string]interface{}), nil
	}

	var result map[string]interface{}
	err := json.Unmarshal([]byte(blockStates), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block states: %w", err)
	}

	return result, nil
}

// MarshalBlockStates 将方块状态 map 序列化为字符串
func MarshalBlockStates(blockStates map[string]interface{}) (string, error) {
	if len(blockStates) == 0 {
		return "", nil
	}

	data, err := json.Marshal(blockStates)
	if err != nil {
		return "", fmt.Errorf("failed to marshal block states: %w", err)
	}

	return string(data), nil
}
