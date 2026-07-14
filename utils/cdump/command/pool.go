package command

var CDumpCommandPool_1 map[uint8]func() Command = map[uint8]func() Command{
	0x00: func() Command { return &Zero{} },
	0x01: func() Command { return &XPlus{} },
	0x02: func() Command { return &XMinus{} },
	0x03: func() Command { return &YPlus{} },
	0x04: func() Command { return &YMinus{} },
	0x05: func() Command { return &ZPlus{} },
	0x06: func() Command { return &ZMinus{} },
	0x07: func() Command { return &XPlusN{} },
	0x08: func() Command { return &XMinusN{} },
	0x09: func() Command { return &YPlusN{} },
	0x0a: func() Command { return &YMinusN{} },
	0x0b: func() Command { return &ZPlusN{} },
	0x0c: func() Command { return &ZMinusN{} },

	0x0d: func() Command { return &CreateConstantString{} },
	0x0e: func() Command { return &PlaceBlockWithBlockStates{} },
	0x0f: func() Command { return &PlaceBlock{} },
	// 0x10: func() Command { return &PlaceBlockWithBlockStates{} },
	0x11: func() Command { return &PlaceRuntimeBlock{} },
	0x12: func() Command { return &PlaceRuntimeBlockU32{} },
	0x13: func() Command { return &CommandBlockData{} },
	0x14: func() Command { return &ChestData{} },
	0x15: func() Command { return &NBTData{} },

	0x16: func() Command { return &End{} },
}
