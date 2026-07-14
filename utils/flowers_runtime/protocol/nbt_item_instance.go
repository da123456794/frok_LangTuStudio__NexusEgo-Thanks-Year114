package protocol

type ExtraData map[string]any

type Item struct {
	Count       byte           `mapstructure:"Count"`
	Damage      int16          `mapstructure:"Damage"`
	Name        string         `mapstructure:"Name"`
	WasPickedUp byte           `mapstructure:"WasPickedUp"`
	Block       map[string]any `mapstructure:"Block,omitempty"`
	Tag         map[string]any `mapstructure:"tag,omitempty"`
	ModBlock    map[string]any `mapstructure:"modBlock,omitempty"`
	CanDestroy  []any          `mapstructure:"CanDestroy,omitempty"`
	CanPlaceOn  []any          `mapstructure:"CanPlaceOn,omitempty"`
}

type ItemWithSlot struct {
	Slot byte `mapstructure:"Slot"`
	Item `mapstructure:",squash"`
}

func (i *ItemWithSlot) Marshal(r IO) {
	r.Uint8(&i.Slot)
	r.NBTItem(&i.Item)
}
