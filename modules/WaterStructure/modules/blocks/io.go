package blocks

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Yeah114/blocks/describe"
)

func RuntimeIDToBlock(runtimeID uint32) (block *describe.Block, found bool) {
	block = MC_CURRENT.BlockByRtid(runtimeID)
	return block, block != nil
}

func LegacyBlockToRuntimeID(name string, data uint16) (runtimeID uint32, found bool) {
	return DefaultAnyToNemcConvertor.TryBestSearchByLegacyValue(describe.BlockNameForSearch(name), data)
}

func RuntimeIDToState(runtimeID uint32) (baseName string, properties map[string]any, found bool) {
	block, found := RuntimeIDToBlock(runtimeID)
	if !found {
		return "air", nil, false
	}
	return block.ShortName(), block.States().ToNBT(), true
}

// coral_block ["coral_color":"yellow", "dead_bit":false] true
func RuntimeIDToBlockNameWithStateStr(runtimeID uint32) (blockNameWithState string, found bool) {
	block, found := RuntimeIDToBlock(runtimeID)
	if !found {
		return "air []", false
	}
	return block.BedrockString(), true
}

func RuntimeIDToBlockNameAndStateStr(runtimeID uint32) (blockName, blockState string, found bool) {
	block, found := RuntimeIDToBlock(runtimeID)
	if !found {
		return "air", "[]", false
	}
	return block.ShortName(), block.States().BedrockString(true), true
}

func BlockNameAndStateToRuntimeID(name string, properties map[string]any) (runtimeID uint32, found bool) {
	props, err := describe.PropsForSearchFromNbt(properties)
	if err != nil {
		// legacy capability
		fmt.Println(err)
		return uint32(AIR_RUNTIMEID), false
	}
	rtid, _, found := DefaultAnyToNemcConvertor.TryBestSearchByState(describe.BlockNameForSearch(name), props)
	return rtid, found
}

func BlockNameAndStateStrToRuntimeID(name string, stateStr string) (runtimeID uint32, found bool) {
	props, err := describe.PropsForSearchFromStr(stateStr)
	if err != nil {
		// legacy capability
		fmt.Println(err)
		return uint32(AIR_RUNTIMEID), false
	}
	rtid, _, found := DefaultAnyToNemcConvertor.TryBestSearchByState(describe.BlockNameForSearch(name), props)
	return rtid, found
}

func BlockStrToRuntimeID(blockNameWithOrWithoutState string) (runtimeID uint32, found bool) {
	blockNameWithOrWithoutState = strings.TrimSpace(blockNameWithOrWithoutState)
	ss := strings.Split(blockNameWithOrWithoutState, " ")
	if len(ss) > 1 {
		cleanedSS := []string{}
		for _, s := range ss {
			if s == "" {
				continue
			}
			cleanedSS = append(cleanedSS, s)
		}
		if len(cleanedSS) == 2 {
			val, err := strconv.Atoi(cleanedSS[1])
			if err == nil {
				rtid, found := DefaultAnyToNemcConvertor.TryBestSearchByLegacyValue(describe.BlockNameForSearch(cleanedSS[0]), uint16(val))
				return rtid, found
			}
		}
	}
	blockName, blockProps := ConvertStringToBlockNameAndPropsForSearch(blockNameWithOrWithoutState)
	rtid, _, found := DefaultAnyToNemcConvertor.TryBestSearchByState(blockName, blockProps)
	return rtid, found
}

// JavaBlockStrToRuntimeID converts Java block strings to RuntimeID
// It uses the same conversion logic as BlockStrToRuntimeID
var JavaBlockStrToRuntimeID = BlockStrToRuntimeID

func SchematicToRuntimeID(block uint8, value uint8) uint32 {
	//value = value & 0xF
	return quickSchematicMapping[block][value]
}

func GetSchematicMapping() [256][256]uint32 {
	return quickSchematicMapping
}

func RuntimeIDToSchematic(runtimeID uint32) (block uint8, value uint8, found bool) {
	if schematic, exists := runtimeIDToSchematic[runtimeID]; exists {
		return schematic[0], schematic[1], true
	}
	return 0, 0, false
}

func ConvertStringToBlockNameAndPropsForSearch(blockString string) (blockNameForSearch describe.BaseWithNameSpace, propsForSearch *describe.PropsForSearch) {
	blockString = strings.ReplaceAll(blockString, "{", "[")
	inFrags := strings.Split(blockString, "[")
	inBlockName, inBlockState := inFrags[0], ""
	if len(inFrags) > 1 {
		inBlockState = inFrags[1]
	}
	if len(inBlockState) > 0 {
		if inBlockState[len(inBlockState)-1] == ']' || inBlockState[len(inBlockState)-1] == '}' {
			inBlockState = inBlockState[:len(inBlockState)-1]
		}
	}
	inBlockStateForSearch, err := describe.PropsForSearchFromStr(inBlockState)
	if err != nil {
		// legacy capability
		fmt.Println(err)
	}
	return describe.BlockNameForSearch(inBlockName), inBlockStateForSearch
}

// RuntimeIDToJavaBlockStr converts Bedrock RuntimeID to Java block string
func RuntimeIDToJavaBlockStr(runtimeID uint32) (javaBlockStr string, found bool) {
	block, found := RuntimeIDToBlock(runtimeID)
	if !found {
		return "minecraft:air", false
	}
	javaBlock, _, found := BedrockToJavaConvertor.TryBestSearchByState(block.NameForSearch(), block.StatesForSearch())
	if !found {
		return "minecraft:air", false
	}
	return javaBlock.String(), true
}

// RuntimeIDToJavaBlockNameAndState converts Bedrock RuntimeID to Java block name and properties
func RuntimeIDToJavaBlockNameAndState(runtimeID uint32) (name string, properties map[string]any, found bool) {
	block, found := RuntimeIDToBlock(runtimeID)
	if !found {
		return "air", nil, false
	}
	javaBlock, _, found := BedrockToJavaConvertor.TryBestSearchByState(block.NameForSearch(), block.StatesForSearch())
	if !found {
		return "air", nil, false
	}
	return javaBlock.Name(), javaBlock.ToNBT(), true
}

// BedrockBlockStrToJavaBlockStr converts Bedrock block string to Java block string
func BedrockBlockStrToJavaBlockStr(bedrockBlockStr string) (javaBlockStr string, found bool) {
	blockName, blockProps := ConvertStringToBlockNameAndPropsForSearch(bedrockBlockStr)
	javaBlock, _, found := BedrockToJavaConvertor.TryBestSearchByState(blockName, blockProps)
	if !found {
		return "minecraft:air", false
	}
	return javaBlock.String(), true
}

// BedrockBlockNameAndStateToJavaBlock converts Bedrock block name and properties to Java block
func BedrockBlockNameAndStateToJavaBlock(name string, properties map[string]any) (javaName string, javaProperties map[string]any, found bool) {
	props, err := describe.PropsForSearchFromNbt(properties)
	if err != nil {
		return "air", nil, false
	}
	javaBlock, _, found := BedrockToJavaConvertor.TryBestSearchByState(describe.BlockNameForSearch(name), props)
	if !found {
		return "air", nil, false
	}
	return javaBlock.Name(), javaBlock.ToNBT(), true
}

// RuntimeIDToJavaBlockNameAndStateStr converts Bedrock RuntimeID to Java block name and state string
func RuntimeIDToJavaBlockNameAndStateStr(runtimeID uint32) (blockName, blockState string, found bool) {
	block, found := RuntimeIDToBlock(runtimeID)
	if !found {
		return "air", "[]", false
	}
	javaBlock, _, found := BedrockToJavaConvertor.TryBestSearchByState(block.NameForSearch(), block.StatesForSearch())
	if !found {
		return "air", "[]", false
	}
	return javaBlock.Name(), javaBlock.SNBT(), true
}
