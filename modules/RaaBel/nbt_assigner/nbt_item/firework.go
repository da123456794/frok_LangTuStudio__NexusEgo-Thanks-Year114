package nbt_item

import (
	"fmt"
	"slices"

	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_hash "github.com/LangTuStudio/RaaBel/nbt_parser/hash"
	nbt_parser_interface "github.com/LangTuStudio/RaaBel/nbt_parser/interface"
	nbt_parser_item "github.com/LangTuStudio/RaaBel/nbt_parser/item"
)

const (
	fireworkMaxIngredientCount = 9
)

type fireworkIngredient struct {
	Name string
	Meta int16
}

// 烟花火箭
type Firework struct {
	api   *nbt_console.Console
	items []nbt_parser_item.FireworkRocket
}

func (f *Firework) Append(item ...nbt_parser_interface.Item) {
	for _, value := range item {
		val, ok := value.(*nbt_parser_item.FireworkRocket)
		if !ok {
			continue
		}
		f.items = append(f.items, *val)
	}
}

func (f *Firework) Make() (resultSlot map[uint64]resources_control.SlotID, err error) {
	if len(f.items) == 0 {
		return nil, nil
	}

	resultSlot = make(map[uint64]resources_control.SlotID)

	crafterIndex, err := f.api.FindOrGenerateNewCrafter()
	if err != nil {
		return nil, fmt.Errorf("Make: %v", err)
	}

	for _, target := range f.items {
		slot, err := f.makeOne(crafterIndex, target)
		if err != nil {
			return nil, fmt.Errorf("Make: %v", err)
		}
		resultSlot[nbt_hash.NBTItemNBTHash(&target)] = slot
	}

	f.items = nil
	return resultSlot, nil
}

func (f *Firework) makeOne(crafterIndex int, target nbt_parser_item.FireworkRocket) (resources_control.SlotID, error) {
	starSlots, err := f.makeFireworkStars(crafterIndex, target.NBT.Explosions)
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}

	flight := target.NBT.Flight
	if flight < 1 || flight > 3 {
		return 0, fmt.Errorf("makeOne: Invalid firework flight = %d; only 1~3 can be crafted", flight)
	}

	ingredients := make([]fireworkIngredient, 0, 1+flight)
	ingredients = append(ingredients, fireworkIngredient{Name: "minecraft:paper"})
	for range flight {
		ingredients = append(ingredients, fireworkIngredient{Name: "minecraft:gunpowder"})
	}

	ingredientSlots, err := f.placeIngredients(ingredients, starSlots)
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}
	craftingInputs := append(slices.Clone(starSlots), ingredientSlots...)

	if len(craftingInputs) > fireworkMaxIngredientCount {
		return 0, fmt.Errorf(
			"makeOne: Too many rocket ingredients: stars=%d, flight=%d",
			len(starSlots),
			flight,
		)
	}

	resultSlot, err := f.craftByCrafter(crafterIndex, craftingInputs, nil)
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}

	stack, existed := f.api.API().Resources().Inventories().GetItemStack(resources_control.WindowNameInventory, resultSlot)
	if !existed {
		panic("makeOne: Should nerver happened")
	}

	item, err := nbt_parser_interface.ParseItemNetwork(stack.Stack, "minecraft:firework_rocket")
	if err != nil {
		return 0, fmt.Errorf("makeOne: %v", err)
	}

	wantHash := nbt_hash.NBTItemNBTHash(&target)
	getHash := nbt_hash.NBTItemNBTHash(item)
	if wantHash != getHash {
		return 0, fmt.Errorf("makeOne: Craft result mismatch (want=%d, get=%d)", wantHash, getHash)
	}

	return resultSlot, nil
}

func (f *Firework) makeFireworkStars(
	crafterIndex int,
	explosions []nbt_parser_item.FireworkExplosion,
) ([]resources_control.SlotID, error) {
	if len(explosions) == 0 {
		return nil, nil
	}

	starSlots := make([]resources_control.SlotID, 0, len(explosions))
	for _, explosion := range explosions {
		starSlot, err := f.makeOneFireworkStar(crafterIndex, explosion, starSlots)
		if err != nil {
			return nil, fmt.Errorf("makeFireworkStars: %v", err)
		}
		starSlots = append(starSlots, starSlot)
	}

	return starSlots, nil
}

func (f *Firework) makeOneFireworkStar(
	crafterIndex int,
	explosion nbt_parser_item.FireworkExplosion,
	keepSlots []resources_control.SlotID,
) (resources_control.SlotID, error) {
	ingredients, err := f.baseExplosionIngredients(explosion)
	if err != nil {
		return 0, fmt.Errorf("makeOneFireworkStar: %v", err)
	}

	ingredientSlots, err := f.placeIngredients(ingredients, keepSlots)
	if err != nil {
		return 0, fmt.Errorf("makeOneFireworkStar: %v", err)
	}

	starSlot, err := f.craftByCrafter(crafterIndex, ingredientSlots, keepSlots)
	if err != nil {
		return 0, fmt.Errorf("makeOneFireworkStar: %v", err)
	}

	if len(explosion.FireworkFade) == 0 {
		return starSlot, nil
	}

	fadeIngredients := make([]fireworkIngredient, 0, len(explosion.FireworkFade))
	for _, color := range explosion.FireworkFade {
		dyeName, ok := mapping.FireworkColorToDyeName[int32(color)]
		if !ok {
			return 0, fmt.Errorf("makeOneFireworkStar: Invalid fade color = %d", color)
		}
		fadeIngredients = append(fadeIngredients, fireworkIngredient{Name: dyeName})
	}

	fadeDyeSlots, err := f.placeIngredients(fadeIngredients, append(slices.Clone(keepSlots), starSlot))
	if err != nil {
		return 0, fmt.Errorf("makeOneFireworkStar: %v", err)
	}

	fadeInputs := append([]resources_control.SlotID{starSlot}, fadeDyeSlots...)
	starSlot, err = f.craftByCrafter(crafterIndex, fadeInputs, keepSlots)
	if err != nil {
		return 0, fmt.Errorf("makeOneFireworkStar: %v", err)
	}

	return starSlot, nil
}

func (f *Firework) baseExplosionIngredients(explosion nbt_parser_item.FireworkExplosion) ([]fireworkIngredient, error) {
	if len(explosion.FireworkColor) == 0 {
		return nil, fmt.Errorf("baseExplosionIngredients: Missing base colors")
	}

	ingredients := make([]fireworkIngredient, 0, len(explosion.FireworkColor)+4)
	ingredients = append(ingredients, fireworkIngredient{Name: "minecraft:gunpowder"})

	for _, color := range explosion.FireworkColor {
		dyeName, ok := mapping.FireworkColorToDyeName[int32(color)]
		if !ok {
			return nil, fmt.Errorf("baseExplosionIngredients: Invalid color = %d", color)
		}
		ingredients = append(ingredients, fireworkIngredient{Name: dyeName})
	}

	if explosion.FireworkType != mapping.FireworkShapeSmallSphere {
		extraIngredient, ok := mapping.FireworkShapeToIngredient[explosion.FireworkType]
		if !ok {
			return nil, fmt.Errorf("baseExplosionIngredients: Unsupported firework type = %d", explosion.FireworkType)
		}
		ingredients = append(ingredients, fireworkIngredient{Name: extraIngredient.Name, Meta: extraIngredient.Metadata})
	}

	if explosion.FireworkTrail {
		ingredients = append(ingredients, fireworkIngredient{Name: "minecraft:diamond"})
	}
	if explosion.FireworkFlicker {
		ingredients = append(ingredients, fireworkIngredient{Name: "minecraft:glowstone_dust"})
	}

	if len(ingredients) > fireworkMaxIngredientCount {
		return nil, fmt.Errorf("baseExplosionIngredients: Too many ingredients in one explosion")
	}

	return ingredients, nil
}

func (f *Firework) placeIngredients(
	ingredients []fireworkIngredient,
	exclusion []resources_control.SlotID,
) ([]resources_control.SlotID, error) {
	if len(ingredients) == 0 {
		return nil, nil
	}

	api := f.api.API()
	slots := make([]resources_control.SlotID, 0, len(ingredients))

	for _, ingredient := range ingredients {
		exclude := append(slices.Clone(exclusion), slots...)
		slot := f.api.FindInventorySlot(exclude)

		err := api.Replaceitem().ReplaceitemInInventory(
			"@s",
			game_interface.ReplacePathInventory,
			game_interface.ReplaceitemInfo{
				Name:     ingredient.Name,
				Count:    1,
				MetaData: ingredient.Meta,
				Slot:     slot,
			},
			"",
			true,
		)
		if err != nil {
			return nil, fmt.Errorf("placeIngredients: %v", err)
		}

		f.api.UseInventorySlot(nbt_console.RequesterUser, slot, true)
		slots = append(slots, slot)
	}

	return slots, nil
}

func (f *Firework) craftByCrafter(
	crafterIndex int,
	ingredientSlots []resources_control.SlotID,
	exclusion []resources_control.SlotID,
) (resources_control.SlotID, error) {
	if len(ingredientSlots) == 0 {
		return 0, fmt.Errorf("craftByCrafter: ingredientSlots is empty")
	}
	if len(ingredientSlots) > fireworkMaxIngredientCount {
		return 0, fmt.Errorf("craftByCrafter: Too many ingredients")
	}

	api := f.api.API()
	resultExclusion := append(slices.Clone(exclusion), ingredientSlots...)
	resultSlot := f.api.FindInventorySlot(resultExclusion)

	err := api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathInventory,
		game_interface.ReplaceitemInfo{
			Name:     "minecraft:air",
			Count:    1,
			MetaData: 0,
			Slot:     resultSlot,
		},
		"",
		true,
	)
	if err != nil {
		return 0, fmt.Errorf("craftByCrafter: %v", err)
	}
	f.api.UseInventorySlot(nbt_console.RequesterUser, resultSlot, false)

	err = api.Commands().AwaitChangesGeneral()
	if err != nil {
		return 0, fmt.Errorf("craftByCrafter: %v", err)
	}

	inventoryToCrafterSlotMapping := make(map[resources_control.SlotID]resources_control.SlotID)
	for index, inventorySlot := range ingredientSlots {
		inventoryToCrafterSlotMapping[inventorySlot] = resources_control.SlotID(index)
	}

	err = f.api.CraftByCrafter(crafterIndex, inventoryToCrafterSlotMapping, resultSlot)
	if err != nil {
		return 0, fmt.Errorf("craftByCrafter: %v", err)
	}

	return resultSlot, nil
}
