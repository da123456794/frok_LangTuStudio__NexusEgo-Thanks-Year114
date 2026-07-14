package main

import (
	"fmt"

	"github.com/Yeah114/blocks"
)

func main() {
	fmt.Println("=== Bedrock to Java Conversion Examples ===\n")

	// Example 1: RuntimeID → Java block string
	fmt.Println("Example 1: RuntimeID → Java block string")
	bedrockRtid := uint32(1798) // stone
	javaStr, found := blocks.RuntimeIDToJavaBlockStr(bedrockRtid)
	fmt.Printf("  Bedrock RuntimeID %d\n", bedrockRtid)
	fmt.Printf("  → Java: %s (found: %t)\n\n", javaStr, found)

	// Example 2: RuntimeID → Name and State String
	fmt.Println("Example 2: RuntimeID → Java name and state string")
	bedrockRtid2 := uint32(737) // oak_log
	javaName2, javaStateStr2, found2 := blocks.RuntimeIDToJavaBlockNameAndStateStr(bedrockRtid2)
	fmt.Printf("  Bedrock RuntimeID %d\n", bedrockRtid2)
	fmt.Printf("  → Java: %s [%s] (found: %t)\n\n", javaName2, javaStateStr2, found2)

	// Example 3: RuntimeID → Name and State Map
	fmt.Println("Example 3: RuntimeID → Java name and state map")
	bedrockRtid3 := uint32(738) // oak_log with different axis
	javaName3, javaProps3, found3 := blocks.RuntimeIDToJavaBlockNameAndState(bedrockRtid3)
	fmt.Printf("  Bedrock RuntimeID %d\n", bedrockRtid3)
	fmt.Printf("  → Java: %s %v (found: %t)\n\n", javaName3, javaProps3, found3)

	// Example 4: Block with complex properties
	fmt.Println("Example 4: Bedrock name+props → Java name+props")
	bedrockName := "oak_door"
	bedrockProps := map[string]any{
		"direction":       int32(3),
		"door_hinge_bit":  false,
		"open_bit":        false,
		"upper_block_bit": false,
	}
	javaName, javaProps, found := blocks.BedrockBlockNameAndStateToJavaBlock(bedrockName, bedrockProps)
	fmt.Printf("  Bedrock: %s %v\n", bedrockName, bedrockProps)
	fmt.Printf("  → Java: %s %v (found: %t)\n\n", javaName, javaProps, found)

	// Example 5: Block string conversion
	fmt.Println("Example 5: Bedrock block string → Java block string")
	bedrockBlockStr := "coral_block[coral_color=\"yellow\",dead_bit=0b]"
	javaBlockStr, found := blocks.BedrockBlockStrToJavaBlockStr(bedrockBlockStr)
	fmt.Printf("  Bedrock: %s\n", bedrockBlockStr)
	fmt.Printf("  → Java: %s (found: %t)\n\n", javaBlockStr, found)

	// Example 6: Various block types
	fmt.Println("Example 6: Multiple block type conversions")
	testBlocks := []uint32{
		9533,  // dirt
		6990,  // sand
		11742, // short_grass (tallgrass)
		6991,  // red_sand
		76,    // granite
	}
	for _, rtid := range testBlocks {
		bedrockName, bedrockState, _ := blocks.RuntimeIDToBlockNameAndStateStr(rtid)
		javaName, javaState, found := blocks.RuntimeIDToJavaBlockNameAndStateStr(rtid)
		fmt.Printf("  Bedrock: %s [%s]\n", bedrockName, bedrockState)
		fmt.Printf("  → Java: %s [%s] (found: %t)\n\n", javaName, javaState, found)
	}

	// Example 7: Edge cases
	fmt.Println("Example 7: Edge cases")

	// Air block
	airRtid := blocks.AIR_RUNTIMEID
	javaAir, found := blocks.RuntimeIDToJavaBlockStr(airRtid)
	fmt.Printf("  Air (RuntimeID %d): %s (found: %t)\n", airRtid, javaAir, found)

	// Invalid RuntimeID
	invalidRtid := uint32(999999)
	javaInvalid, found := blocks.RuntimeIDToJavaBlockStr(invalidRtid)
	fmt.Printf("  Invalid (RuntimeID %d): %s (found: %t)\n\n", invalidRtid, javaInvalid, found)

	fmt.Println("=== All Examples Complete ===")
}
