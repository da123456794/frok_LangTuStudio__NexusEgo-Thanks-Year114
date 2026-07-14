package protocol

type Enchant struct {
	ID         int16  `mapstructure:"id"`
	Level      int16  `mapstructure:"lvl"`
	ModEnchant string `mapstructure:"modEnchant"`
}

func (e *Enchant) Marshal(r IO) {
	NBTInt(&e.ID, r.Uint16)
	NBTInt(&e.Level, r.Uint16)
	r.String(&e.ModEnchant)
}
