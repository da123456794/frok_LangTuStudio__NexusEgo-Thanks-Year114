package nbt_parser_item

import (
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/utils"

	"github.com/mitchellh/mapstructure"
)

// SingleItemEnch 是物品持有的单个魔咒数据
type SingleItemEnch struct {
	ID    int16 `mapstructure:"id"`  // 魔咒 ID
	Level int16 `mapstructure:"lvl"` // 魔咒等级
}

// Marshal ..
func (s *SingleItemEnch) Marshal(io protocol.IO) {
	io.Int16(&s.ID)
	io.Int16(&s.Level)
}

// parseItemEnchList ..
func parseItemEnchList(enchList []any) (result []SingleItemEnch, err error) {
	for _, value := range enchList {
		var singleItemEnch SingleItemEnch

		val, ok := value.(map[string]any)
		if !ok {
			continue
		}

		err = mapstructure.Decode(&val, &singleItemEnch)
		if err != nil {
			return nil, fmt.Errorf("ParseItemEnchList: %v", err)
		}

		result = append(result, singleItemEnch)
	}
	return
}

// ParseItemEnchList ..
func ParseItemEnchList(nbtMap map[string]any) (result []SingleItemEnch, err error) {
	tag, ok := nbtMap["tag"].(map[string]any)
	if !ok {
		return
	}

	ench, ok := tag["ench"].([]any)
	if !ok {
		return
	}

	result, err = parseItemEnchList(ench)
	if err != nil {
		return nil, fmt.Errorf("ParseItemEnchList: %v", err)
	}

	return
}

// ParseItemEnchListNetwork ..
func ParseItemEnchListNetwork(item protocol.ItemStack) (result []SingleItemEnch, err error) {
	if item.NBTData == nil {
		return
	}

	ench, ok := item.NBTData["ench"].([]any)
	if !ok {
		return
	}

	result, err = parseItemEnchList(ench)
	if err != nil {
		return nil, fmt.Errorf("ParseItemEnchListNetwork: %v", err)
	}

	return
}

// ItemEnhanceData 是物品的增强数据，
// 例如物品组件、显示名称和附魔属性
type ItemEnhanceData struct {
	// 该物品的物品组件数据
	ItemComponent utils.ItemComponent
	// 该物品的显示名称。
	// 如果为空，则不存在
	DisplayName string
	// 该物品的附魔属性
	EnchList []SingleItemEnch
	// 该物品的自定义颜色
	// 如果为 nil，则不存在
	CustomColor *[3]uint8
	// 该物品的显示说明
	Lore []string
}

// ParseItemEnhance ..
func ParseItemEnhance(nbtMap map[string]any) (result ItemEnhanceData, err error) {
	result.ItemComponent = utils.ParseItemComponent(nbtMap)

	result.EnchList, err = ParseItemEnchList(nbtMap)
	if err != nil {
		return result, fmt.Errorf("ParseItemEnhance: %v", err)
	}

	tag, ok := nbtMap["tag"].(map[string]any)
	if !ok {
		return
	}

	customColor, ok := tag["customColor"].(int32)
	if ok {
		color, _ := utils.DecodeVarRGBA(customColor)
		result.CustomColor = &color
	}

	display, ok := tag["display"].(map[string]any)
	if ok {
		result.DisplayName, _ = display["Name"].(string)
	}
	lore, _ := display["Lore"].([]any)
	for _, v := range lore {
		l, ok := v.(string)
		if !ok {
			continue
		}
		if l != "(+DATA)" {
			result.DisplayName += "\n" + l
			continue
		}
		result.Lore = append(result.Lore, l)
	}

	return
}

// ParseItemEnhanceNetwork ..
func ParseItemEnhanceNetwork(item protocol.ItemStack) (result ItemEnhanceData, err error) {
	result.ItemComponent = utils.ParseItemComponentNetwork(item)

	result.EnchList, err = ParseItemEnchListNetwork(item)
	if err != nil {
		return result, fmt.Errorf("ParseItemEnhanceNetwork: %v", err)
	}

	if item.NBTData == nil {
		return
	}

	customColor, ok := item.NBTData["customColor"].(int32)
	if ok {
		color, _ := utils.DecodeVarRGBA(customColor)
		result.CustomColor = &color
	}

	display, ok := item.NBTData["display"].(map[string]any)
	if ok {
		result.DisplayName, _ = display["Name"].(string)
	}
	lore, _ := display["Lore"].([]any)
	for _, v := range lore {
		l, ok := v.(string)
		if !ok {
			continue
		}
		if l != "(+DATA)" {
			result.DisplayName += "\n" + l
			continue
		}
		result.Lore = append(result.Lore, l)
	}

	return
}
