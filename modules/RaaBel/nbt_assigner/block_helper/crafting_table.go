package block_helper

// CraftingTableBlockHelper 描述了一个工作台
type CraftingTableBlockHelper struct{}

func (CraftingTableBlockHelper) KnownBlockStates() bool {
	return true
}

func (CraftingTableBlockHelper) BlockName() string {
	return "minecraft:crafting_table"
}

func (CraftingTableBlockHelper) BlockStates() map[string]any {
	return map[string]any{}
}

func (CraftingTableBlockHelper) BlockStatesString() string {
	return "[]"
}
