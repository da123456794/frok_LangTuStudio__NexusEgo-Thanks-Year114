package protocol

type NeteaseStartGameData struct {
	Unknown1 uint64  // 64位数据（功能标志/标识符）
	Unknown2 float32 // 32位浮点（参数/系数）
}

func (x *NeteaseStartGameData) Marshal(r IO) {
	r.Uint64(&x.Unknown1)
	r.Float32(&x.Unknown2)
}
