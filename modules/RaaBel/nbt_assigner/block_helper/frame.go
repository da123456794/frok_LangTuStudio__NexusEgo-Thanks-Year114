package block_helper

import "github.com/LangTuStudio/RaaBel/utils"

// FrameBlockHelper 描述了一个物品展示框
type FrameBlockHelper struct{}

func (FrameBlockHelper) KnownBlockStates() bool {
	return true
}

func (FrameBlockHelper) BlockName() string {
	return "minecraft:frame"
}

func (FrameBlockHelper) BlockStates() map[string]any {
	return map[string]any{
		"facing_direction":     int32(1),
		"item_frame_map_bit":   byte(0),
		"item_frame_photo_bit": byte(0),
	}
}

func (f FrameBlockHelper) BlockStatesString() string {
	return utils.MarshalBlockStates(f.BlockStates())
}
