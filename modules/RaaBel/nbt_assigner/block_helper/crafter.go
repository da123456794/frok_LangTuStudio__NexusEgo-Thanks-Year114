package block_helper

import "github.com/LangTuStudio/RaaBel/utils"

// CrafterBlockHelper 描述了一个合成器
type CrafterBlockHelper struct{}

func (CrafterBlockHelper) KnownBlockStates() bool {
	return true
}

func (CrafterBlockHelper) BlockName() string {
	return "minecraft:crafter"
}

func (CrafterBlockHelper) BlockStates() map[string]any {
	return map[string]any{
		"orientation":   "up_east",
		"crafting":      byte(0),
		"triggered_bit": byte(0),
	}
}

func (c CrafterBlockHelper) BlockStatesString() string {
	return utils.MarshalBlockStates(c.BlockStates())
}
