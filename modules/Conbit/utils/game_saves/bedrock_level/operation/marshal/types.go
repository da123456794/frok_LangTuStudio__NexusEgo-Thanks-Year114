package marshal

type paletteBlock struct {
	Name    string         `nbt:"name"`
	States  map[string]any `nbt:"states"`
	Version int32          `nbt:"version"`
}

const blockStateVersion int32 = 0x01150100
