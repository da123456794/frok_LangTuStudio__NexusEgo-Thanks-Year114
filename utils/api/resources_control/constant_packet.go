package ResourcesControl

import (
	"strings"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	newgamedata "github.com/LangTuStudio/Conbit/minecraft_neo/game_data"

	"github.com/pterm/pterm"
)

const DebugPrintUnknownItem = true

type ConstantPacket struct {
	availableItems       []protocol.ItemEntry
	itemNetworkIDMapping map[int32]int
	itemNameMapping      map[string]int
	itemNameMappingInv   []string

	creativeContent    []protocol.CreativeItem
	creativeNIMapping  map[int32][]int
	creativeCNIMapping map[uint32]int

	commandItems        []string
	commandItemsMapping map[string]bool

	trimRecipeNetworkID uint32
}

func NewConstantPacket() *ConstantPacket {
	return &ConstantPacket{
		itemNetworkIDMapping: make(map[int32]int),
		itemNameMapping:      make(map[string]int),
		creativeNIMapping:    make(map[int32][]int),
		creativeCNIMapping:   make(map[uint32]int),
		commandItemsMapping:  make(map[string]bool),
	}
}

func (c ConstantPacket) AllCreativeContent() []protocol.CreativeItem {
	return c.creativeContent
}

func (c ConstantPacket) CreativeItemByCNI(creativeNetworkID uint32) protocol.CreativeItem {
	return c.creativeContent[c.creativeCNIMapping[creativeNetworkID]]
}

func (c ConstantPacket) CreativeItemByNI(networkID int32) []protocol.CreativeItem {
	result := make([]protocol.CreativeItem, 0)
	for _, index := range c.creativeNIMapping[networkID] {
		result = append(result, c.creativeContent[index])
	}
	return result
}

func (c ConstantPacket) CreativeItemByName(name string) []protocol.CreativeItem {
	name = strings.ToLower(name)
	if !strings.HasPrefix(name, "minecraft:") {
		name = "minecraft:" + name
	}
	return c.CreativeItemByNI(int32(c.ItemByName(name).RuntimeID))
}

func (c *ConstantPacket) onCreativeContent(p *packet.CreativeContent) {
	c.creativeContent = p.Items
	c.creativeNIMapping = make(map[int32][]int)
	c.creativeCNIMapping = make(map[uint32]int)
	for index, item := range p.Items {
		c.creativeNIMapping[item.Item.NetworkID] = append(c.creativeNIMapping[item.Item.NetworkID], index)
		c.creativeCNIMapping[item.CreativeItemNetworkID] = index
	}
}

func (c ConstantPacket) AllAvailableItems() []protocol.ItemEntry {
	return c.availableItems
}

func (c ConstantPacket) ItemByNetworkID(networkID int32) protocol.ItemEntry {
	return c.availableItems[c.itemNetworkIDMapping[networkID]]
}

func (c ConstantPacket) ItemByName(name string) protocol.ItemEntry {
	name = strings.ToLower(name)
	if !strings.HasPrefix(name, "minecraft:") {
		name = "minecraft:" + name
	}
	return c.availableItems[c.itemNameMapping[name]]
}

func (c ConstantPacket) ItemNameByNetworkID(networkID int32) string {
	return c.itemNameMappingInv[c.itemNetworkIDMapping[networkID]]
}

func (c *ConstantPacket) updateByGameData(data newgamedata.GameData) {
	c.availableItems = data.Items
	c.itemNetworkIDMapping = make(map[int32]int, len(c.availableItems))
	c.itemNameMapping = make(map[string]int, len(c.availableItems))
	c.itemNameMappingInv = make([]string, len(c.availableItems))
	for index, item := range c.availableItems {
		c.itemNetworkIDMapping[int32(item.RuntimeID)] = index
		c.itemNameMapping[item.Name] = index
		c.itemNameMappingInv[index] = item.Name
	}
}

func (c ConstantPacket) AllCommandItems() []string {
	return c.commandItems
}

func (c ConstantPacket) ItemCanGetByCommand(name string) bool {
	name = strings.ToLower(name)
	if !strings.HasPrefix(name, "minecraft:") {
		name = "minecraft:" + name
	}

	result := c.commandItemsMapping[name]
	if DebugPrintUnknownItem && !result {
		pterm.Warning.Printfln("ItemCanGetByCommand: Item %#v is unknown, due to it can not get by command", name)
	}
	return result
}

func (c *ConstantPacket) onAvailableCommands(p *packet.AvailableCommands) {
	c.commandItems = []string{"minecraft:written_book"}
	c.commandItemsMapping = map[string]bool{
		"minecraft:written_book": true,
	}
	for _, enum := range p.Enums {
		if enum.Type != "Item" {
			continue
		}
		for _, index := range enum.ValueIndices {
			itemName := p.EnumValues[index]
			if !strings.HasPrefix(itemName, "minecraft:") {
				continue
			}
			c.commandItems = append(c.commandItems, itemName)
			c.commandItemsMapping[itemName] = true
		}
		return
	}
}

func (c *ConstantPacket) TrimRecipeNetworkID() uint32 {
	return c.trimRecipeNetworkID
}

func (c *ConstantPacket) onCraftingData(p *packet.CraftingData) {
	for _, recipe := range p.Recipes {
		if data, ok := recipe.(*protocol.SmithingTrimRecipe); ok {
			c.trimRecipeNetworkID = data.RecipeNetworkID
		}
	}
}

