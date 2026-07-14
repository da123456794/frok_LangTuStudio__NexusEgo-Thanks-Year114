package mapping

const (
	FireworkShapeSmallSphere uint8 = iota // 小型球状
	FireworkShapeHugeSphere               // 大型球状
	FireworkShapeStar                     // 星形
	FireworkShapeCreeperHead              // 苦力怕脸形
	FireworkShapeBurst                    // 爆裂形
)

// FireworkIngredient 记录烟花配方中的材料信息。
type FireworkIngredient struct {
	Name     string
	Metadata int16
}

// 此表描述了烟花颜色值到 染料物品名 的映射
var FireworkColorToDyeName = BannerColorToDyeName

// 此表描述了烟花爆炸类型到 额外材料 的映射
var FireworkShapeToIngredient = map[uint8]FireworkIngredient{
	FireworkShapeHugeSphere:  {Name: "minecraft:fire_charge"},
	FireworkShapeStar:        {Name: "minecraft:gold_nugget"},
	FireworkShapeCreeperHead: {Name: "minecraft:skull", Metadata: 4},
	FireworkShapeBurst:       {Name: "minecraft:feather"},
}
