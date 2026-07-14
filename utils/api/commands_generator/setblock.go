package commands_generator

import (
	"fmt"
	"strings"
	"nexus/utils/api/commands_generator/setblock_api"
	"nexus/defines"
	"nexus/utils/client"
)

func SetBlockRequest(module *types.Module, config *types.MainConfig, client *client.Client) string {
	Point := module.Point
	Method := config.Method
	// fmt.Println(*module.Block.Name, Point.X, Point.Y, Point.Z)
	if module.Block != nil {
		baseName := strings.TrimPrefix(*module.Block.Name, "minecraft:")
		switch baseName {
		case "leaves":
			if client.Cdump_Setting.Stable_Foliage {
				return setblock_api.Leaves(module, config)
			}
		case "leaves2":
			if client.Cdump_Setting.Stable_Foliage {
				return setblock_api.Leaves2(module, config)
			}
		case "azalea_leaves":
			if client.Cdump_Setting.Stable_Foliage {
				return setblock_api.Azalea_leaves(module, config)
			}
		case "azalea_leaves_flowered":
			if client.Cdump_Setting.Stable_Foliage {
				return setblock_api.Azalea_leaves_flowered(module, config)
			}
		case "monster_egg":
			if client.Cdump_Setting.Silverfish_Blocks {
				return setblock_api.Monster_egg(module, config)
			}
		case "infested_deepslate":
			if client.Cdump_Setting.Silverfish_Blocks {
				return setblock_api.Infested_deepslate(module, config)
			}
		case "standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}

		case "spruce_standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "birch_standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "jungle_standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "acacia_standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "darkoak_standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "crimson_standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "warped_standing_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "spruce_wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "birch_wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "jungle_wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "acacia_wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "darkoak_wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "crimson_wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "warped_wall_sign":
			if !client.Cdump_Setting.No_NBT {
				return fmt.Sprintf("setblock %d %d %d light_block 15 %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "frame":
			// fmt.Println(fmt.Sprintf("setblock %d %d %d frame %s %s", Point.X, Point.Y, Point.Z, module.Block.BlockStates, Method))
			return fmt.Sprintf("setblock %d %d %d frame %s %s", Point.X, Point.Y, Point.Z, module.Block.BlockStates, Method)
		case "glow_frame":
			// fmt.Println(fmt.Sprintf("setblock %d %d %d glow_frame %s %s", Point.X, Point.Y, Point.Z, module.Block.BlockStates, Method))
			return fmt.Sprintf("setblock %d %d %d glow_frame %s %s", Point.X, Point.Y, Point.Z, module.Block.BlockStates, Method)

		// 绂佹姘存祦鍔ㄦ敮鎸?		case "water":
			if client.Cdump_Setting.Unbuilderwater {
				return fmt.Sprintf("setblock %d %d %d light_blue_stained_glass %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "flowing_water":
			if client.Cdump_Setting.Unbuilderwater {
				return fmt.Sprintf("setblock %d %d %d light_blue_stained_glass %s", Point.X, Point.Y, Point.Z, Method)
			}
		// 绂佹宀╂祮
		case "lava":
			if client.Cdump_Setting.Unbuildermagma {
				return fmt.Sprintf("setblock %d %d %d red_stained_glass %s", Point.X, Point.Y, Point.Z, Method)
			}
		case "flowing_lava":
			if client.Cdump_Setting.Unbuildermagma {
				return fmt.Sprintf("setblock %d %d %d red_stained_glass %s", Point.X, Point.Y, Point.Z, Method)
			}
		}

		return setblock_api.General_block(module, config)
	} else {
		return "kill @e[type=item]"
	}
}

// setblock 30047 -35 30359 glow_frame ["facing_direction":3,"item_frame_map_bit":false,"item_frame_photo_bit":false]
