package block_helper

import "github.com/LangTuStudio/RaaBel/utils"

// CauldronBlockHelper 描述了一个炼药锅
type CauldronBlockHelper struct {
	States map[string]any
}

func (CauldronBlockHelper) KnownBlockStates() bool {
	return true
}

func (CauldronBlockHelper) BlockName() string {
	return "minecraft:cauldron"
}

func (c CauldronBlockHelper) BlockStates() map[string]any {
	if c.States != nil {
		return c.States
	}
	return map[string]any{}
}

func (c CauldronBlockHelper) BlockStatesString() string {
	return utils.MarshalBlockStates(c.BlockStates())
}
