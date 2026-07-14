package utils

// DirectionToYaw 将方块状态中的 direction 转换为玩家朝向。
func DirectionToYaw(direction int32) float32 {
	fixedDirection := ((direction % 4) + 4) % 4

	switch fixedDirection {
	case 0:
		return 180
	case 1:
		return -90
	case 2:
		return 0
	case 3:
		return 90
	default:
		panic("DirectionToYaw: Should never happened")
	}
}
