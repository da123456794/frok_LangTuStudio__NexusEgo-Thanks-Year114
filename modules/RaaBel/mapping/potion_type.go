package mapping

// 此表描述了炼药锅中 PotionType 字段到 药水类型 的映射
var PotionTypeToItemName = map[int16]string{
	0: "minecraft:potion",           // 药水
	1: "minecraft:splash_potion",    // 喷溅药水
	2: "minecraft:lingering_potion", // 滞留药水
}
