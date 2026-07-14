package utils

import (
	"math"

	"github.com/LangTuStudio/RaaBel/mapping"
)

// MaxCauldronMixDyeCount 指示当前支持的最大混色染料数。
const MaxCauldronMixDyeCount = 4

// CauldronDyeRecipe 描述一条炼药锅染色配方。
// 为了节约内存，Dyes 存储的是染料 ID。
type CauldronDyeRecipe struct {
	Count uint8
	Dyes  [MaxCauldronMixDyeCount]uint8
}

// CauldronColorToRecipe 描述 RGB 颜色到炼药锅染料配方的映射。
// 对于同色的多种配方，只保留染料数量最少的一条。
var CauldronColorToRecipe map[[3]uint8]CauldronDyeRecipe

func init() {
	initCauldronColorToRecipe()
}

// 递归生成 1~maxLen 长度的所有染料序列，并混合、记录配方
func generateAllRecipes(
	maxLen int,
	currentIDs []uint8,
	currentColor [3]uint8,
) {
	// 先把当前长度的配方记下来（只要长度 ≤ 最大）
	if len(currentIDs) > 0 {
		recordCauldronRecipe(currentColor, currentIDs...)
	}

	// 已经到最大长度，不再往下
	if len(currentIDs) >= maxLen {
		return
	}

	totalDyes := len(mapping.DefaultDyeColor)
	for id := 0; id < totalDyes; id++ {
		dyeID := uint8(id)
		dyeColor := mapping.DefaultDyeColor[id]

		// 往下扩展一步
		newIDs := append(currentIDs, dyeID)
		var newColor [3]uint8

		if len(currentIDs) == 0 {
			newColor = dyeColor
		} else {
			newColor = MixTwoColors(currentColor, dyeColor)
		}

		generateAllRecipes(maxLen, newIDs, newColor)
	}
}

// initCauldronColorToRecipe 根据 MaxCauldronMixDyeCount 自动穷举所有混色
func initCauldronColorToRecipe() {
	CauldronColorToRecipe = make(map[[3]uint8]CauldronDyeRecipe)
	// 自动生成 1 ~ MaxCauldronMixDyeCount 次的所有配方
	generateAllRecipes(MaxCauldronMixDyeCount, nil, [3]uint8{})
}

// recordCauldronRecipe 记录 color 对应的 recipeIDs
// 若该颜色已有更短配方，则保持已有配方
func recordCauldronRecipe(color [3]uint8, recipeIDs ...uint8) {
	old, ok := CauldronColorToRecipe[color]
	if ok && int(old.Count) <= len(recipeIDs) {
		return
	}

	recipe := CauldronDyeRecipe{Count: uint8(len(recipeIDs))}
	copy(recipe.Dyes[:], recipeIDs)
	CauldronColorToRecipe[color] = recipe
}

// SearchCauldronDyeRecipeByColor 查找最近颜色的配方
func SearchCauldronDyeRecipeByColor(color [3]uint8) (recipe CauldronDyeRecipe, found bool) {
	minDist := math.Inf(1)
	var bestColor [3]uint8
	found = false

	// 遍历所有预计算颜色，找距离最近的
	for c := range CauldronColorToRecipe {
		dist := CalculateColorDistance(color, c)
		if dist < minDist {
			minDist = dist
			bestColor = c
			found = true
		}
	}

	if found {
		recipe = CauldronColorToRecipe[bestColor]
	}
	return recipe, found
}

// SearchCauldronDyeIDsByColor 通过 RGB 颜色查询最近的染料 ID 序列
func SearchCauldronDyeIDsByColor(color [3]uint8) (ids []uint8, found bool) {
	recipe, found := SearchCauldronDyeRecipeByColor(color)
	if !found {
		return nil, false
	}
	result := make([]uint8, recipe.Count)
	copy(result, recipe.Dyes[:recipe.Count])
	return result, true
}
