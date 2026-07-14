package protocol

type MapPixelsData interface{}

func lookupMapPixels(id uint8, x *MapPixelsData) bool {
	_ = id
	_ = x
	return false
}

func lookupMapPixelsType(x MapPixelsData, id *uint8) bool {
	_ = x
	_ = id
	return false
}
