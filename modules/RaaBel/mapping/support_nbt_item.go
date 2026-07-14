package mapping

const (
	SupportNBTItemTypeBook uint8 = iota
	SupportNBTItemTypeBanner
	SupportNBTItemTypeShield
	SupportNBTItemTypeFirework
	SupportNBTItemTypeBundle
)

// 此表描述了现阶段已经支持了的特殊物品，如烟花等物品。
// 键代表物品名，而值代表这种物品应该归属的类型
var SupportItemsPool = map[string]uint8{
	// 成书
	"minecraft:writable_book": SupportNBTItemTypeBook,
	"minecraft:written_book":  SupportNBTItemTypeBook,
	// 旗帜
	"minecraft:banner": SupportNBTItemTypeBanner,
	// 盾牌
	"minecraft:shield": SupportNBTItemTypeShield,
	// 烟花火箭
	"minecraft:firework_rocket": SupportNBTItemTypeFirework,
	// 收纳袋
	"minecraft:bundle":            SupportNBTItemTypeBundle,
	"minecraft:white_bundle":      SupportNBTItemTypeBundle,
	"minecraft:orange_bundle":     SupportNBTItemTypeBundle,
	"minecraft:magenta_bundle":    SupportNBTItemTypeBundle,
	"minecraft:light_blue_bundle": SupportNBTItemTypeBundle,
	"minecraft:yellow_bundle":     SupportNBTItemTypeBundle,
	"minecraft:lime_bundle":       SupportNBTItemTypeBundle,
	"minecraft:pink_bundle":       SupportNBTItemTypeBundle,
	"minecraft:gray_bundle":       SupportNBTItemTypeBundle,
	"minecraft:light_gray_bundle": SupportNBTItemTypeBundle,
	"minecraft:cyan_bundle":       SupportNBTItemTypeBundle,
	"minecraft:purple_bundle":     SupportNBTItemTypeBundle,
	"minecraft:blue_bundle":       SupportNBTItemTypeBundle,
	"minecraft:brown_bundle":      SupportNBTItemTypeBundle,
	"minecraft:green_bundle":      SupportNBTItemTypeBundle,
	"minecraft:red_bundle":        SupportNBTItemTypeBundle,
	"minecraft:black_bundle":      SupportNBTItemTypeBundle,
}
