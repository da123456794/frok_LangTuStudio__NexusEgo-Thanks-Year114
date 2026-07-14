// AIGC

package nbt_block

import (
	"fmt"
	"time"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/block_helper"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_cache"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"
	nbt_assigner_utils "github.com/LangTuStudio/RaaBel/nbt_assigner/utils"
	nbt_parser_block "github.com/LangTuStudio/RaaBel/nbt_parser/block"
)

// 信标
type Beacon struct {
	console *nbt_console.Console
	cache   *nbt_cache.NBTCacheSystem
	data    nbt_parser_block.Beacon
	offset  protocol.BlockPos
}

func (b Beacon) Offset() protocol.BlockPos {
	return b.offset
}

// buildEmeraldPyramid 在信标下方搭建指定层级的绿宝石金字塔。
// level 的合法值为 [0,4]。
func (b *Beacon) buildEmeraldPyramid(level int) error {
	if level <= 0 {
		return nil
	}
	if level > 4 {
		return fmt.Errorf("buildEmeraldPyramid: Invalid level = %d", level)
	}

	api := b.console.API()
	center := b.console.Center()

	for currentLevel := 1; currentLevel <= level; currentLevel++ {
		radius := int32(level - currentLevel + 1)
		y := center[1] - 1 + int32(currentLevel-1)

		for x := -radius; x <= radius; x++ {
			for z := -radius; z <= radius; z++ {
				blockPos := protocol.BlockPos{center[0] + x, y, center[2] + z}
				err := api.SetBlock().SetBlock(blockPos, "minecraft:emerald_block", "[]")
				if err != nil {
					return fmt.Errorf("buildEmeraldPyramid: %v", err)
				}
			}
		}
	}

	*b.console.NearBlockByIndex(
		nbt_console.ConsoleIndexCenterBlock,
		protocol.BlockPos{0, -1, 0},
	) = block_helper.NearBlock{Name: "minecraft:emerald_block"}
	return nil
}

// clearEmeraldPyramid 清理临时搭建的绿宝石金字塔。
func (b *Beacon) clearEmeraldPyramid(level int) error {
	if level <= 0 {
		return nil
	}
	if level > 4 {
		return fmt.Errorf("clearEmeraldPyramid: Invalid level = %d", level)
	}

	api := b.console.API()
	center := b.console.Center()

	for currentLevel := 1; currentLevel <= level; currentLevel++ {
		radius := int32(level - currentLevel + 1)
		y := center[1] - 1 + int32(currentLevel-1)

		for x := -radius; x <= radius; x++ {
			for z := -radius; z <= radius; z++ {
				blockName := "minecraft:air"
				if y == center[1]-1 {
					blockName = nbt_console.BaseBackground
				}

				blockPos := protocol.BlockPos{center[0] + x, y, center[2] + z}
				err := api.SetBlock().SetBlock(blockPos, blockName, "[]")
				if err != nil {
					return fmt.Errorf("clearEmeraldPyramid: %v", err)
				}
			}
		}
	}

	return nil
}

// requiredPyramidLevel 返回设置当前主/副效果所需的最小信标等级。
func (b *Beacon) requiredPyramidLevel() (int, error) {
	requiredLevelByPrimary := map[int32]int{
		0:  0,
		1:  1,
		3:  1,
		8:  2,
		11: 2,
		5:  3,
		10: 4,
	}

	primaryLevel, found := requiredLevelByPrimary[b.data.NBT.Primary]
	if !found {
		return 0, fmt.Errorf("requiredPyramidLevel: Invalid primary effect id = %d", b.data.NBT.Primary)
	}

	if b.data.NBT.Secondary != 0 {
		_, secondaryFound := requiredLevelByPrimary[b.data.NBT.Secondary]
		if !secondaryFound {
			return 0, fmt.Errorf("requiredPyramidLevel: Invalid secondary effect id = %d", b.data.NBT.Secondary)
		}
		return 4, nil
	}

	return primaryLevel, nil
}

// beaconOffsetByLevel 返回在“底层不低于中心 y-1，且向上抬层”策略下，
// 信标相对操作台中心的目标偏移。
func beaconOffsetByLevel(level int) protocol.BlockPos {
	if level <= 0 {
		return protocol.BlockPos{0, 0, 0}
	}
	return protocol.BlockPos{0, int32(level - 1), 0}
}

// moveCenterBeaconToOffset 将操作台中心处的信标平移到 offset 指示的位置。
func (b *Beacon) moveCenterBeaconToOffset(offset protocol.BlockPos) error {
	if offset == (protocol.BlockPos{0, 0, 0}) {
		return nil
	}

	api := b.console.API()
	center := b.console.Center()
	target := protocol.BlockPos{
		center[0] + offset[0],
		center[1] + offset[1],
		center[2] + offset[2],
	}

	uniqueID, err := api.StructureBackup().BackupStructure(center)
	if err != nil {
		return fmt.Errorf("moveCenterBeaconToOffset: %v", err)
	}
	defer api.StructureBackup().DeleteStructure(uniqueID)

	err = api.StructureBackup().RevertStructure(uniqueID, target)
	if err != nil {
		return fmt.Errorf("moveCenterBeaconToOffset: %v", err)
	}

	err = api.SetBlock().SetBlock(center, "minecraft:air", "[]")
	if err != nil {
		return fmt.Errorf("moveCenterBeaconToOffset: %v", err)
	}

	b.console.UseHelperBlock(
		nbt_console.RequesterUser,
		nbt_console.ConsoleIndexCenterBlock,
		block_helper.Air{},
	)
	return nil
}

// openBeaconByOffset 按相对操作台中心的偏移打开信标。
func (b *Beacon) openBeaconByOffset(offset protocol.BlockPos) (bool, error) {
	api := b.console.API()
	blockPos := b.console.BlockPosByOffset(offset)

	err := b.console.CanReachOrMove(blockPos)
	if err != nil {
		return false, fmt.Errorf("openBeaconByOffset: %v", err)
	}

	return api.ContainerOpenAndClose().OpenContainer(
		game_interface.UseItemOnBlocks{
			HotbarSlotID: b.console.HotbarSlotID(),
			BotPos:       b.console.Position(),
			BlockPos:     blockPos,
			BlockName:    b.data.BlockName(),
			BlockStates:  b.data.BlockStates(),
		},
		false,
	)
}

// waitBeaconLevelRecalculation 等待信标至少完成一次等级重计算。
// Dragonfly 侧信标每 80 tick（约 4 秒）重算一次等级。
func (b *Beacon) waitBeaconLevelRecalculation() error {
	err := b.console.API().Commands().AwaitChangesGeneral()
	if err != nil {
		return fmt.Errorf("waitBeaconLevelRecalculation: %v", err)
	}

	time.Sleep(5 * time.Second)

	err = b.console.API().Commands().AwaitChangesGeneral()
	if err != nil {
		return fmt.Errorf("waitBeaconLevelRecalculation: %v", err)
	}

	return nil
}

func (b *Beacon) Make() error {
	api := b.console.API()
	b.offset = protocol.BlockPos{0, 0, 0}

	// 生成信标
	err := nbt_assigner_utils.SpawnNewEmptyBlock(
		b.console,
		b.cache,
		nbt_assigner_utils.EmptyBlockData{
			Name:               b.data.BlockName(),
			States:             b.data.BlockStates(),
			IsCanOpenConatiner: true,
			BlockCustomName:    b.data.NBT.CustomName,
		},
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 如果信标上没有效果，则应当直接返回值
	if b.data.NBT.Primary == 0 && b.data.NBT.Secondary == 0 {
		return nil
	}

	requiredLevel, err := b.requiredPyramidLevel()
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	beaconOffset := beaconOffsetByLevel(requiredLevel)
	err = b.moveCenterBeaconToOffset(beaconOffset)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	b.offset = beaconOffset

	// 先搭好满足当前效果的金字塔，再等待信标等级刷新。
	err = b.buildEmeraldPyramid(requiredLevel)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	defer b.clearEmeraldPyramid(requiredLevel)

	err = b.waitBeaconLevelRecalculation()
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}

	// 准备信标效果交付物品
	paymentSlot := b.console.FindInventorySlot(nil)
	err = api.Replaceitem().ReplaceitemInInventory(
		"@s",
		game_interface.ReplacePathInventory,
		game_interface.ReplaceitemInfo{
			Name:  "minecraft:emerald",
			Count: 1,
			Slot:  paymentSlot,
		},
		"",
		true,
	)
	if err != nil {
		return fmt.Errorf("Make: %v", err)
	}
	b.console.UseInventorySlot(nbt_console.RequesterUser, paymentSlot, true)

	// 交付物品。
	// 部分服务端会在信标刚建好时短暂保留旧等级，这里做有限重试。
	const maxPaymentAttempt = 3
	for attempt := 1; attempt <= maxPaymentAttempt; attempt++ {
		success, err := b.openBeaconByOffset(beaconOffset)
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		if !success {
			return fmt.Errorf("Make: Failed to open the beacon")
		}

		success, _, _, err = api.ItemStackOperation().OpenTransaction().
			BeaconPaymentFromInventory(paymentSlot, b.data.NBT.Primary, b.data.NBT.Secondary).
			Commit()

		closeErr := api.ContainerOpenAndClose().CloseContainer()
		if closeErr != nil {
			return fmt.Errorf("Make: %v", closeErr)
		}
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
		if success {
			break
		}

		if attempt == maxPaymentAttempt {
			return fmt.Errorf("Make: Beacon payment rejected by server")
		}

		err = b.waitBeaconLevelRecalculation()
		if err != nil {
			return fmt.Errorf("Make: %v", err)
		}
	}

	b.console.UseInventorySlot(nbt_console.RequesterUser, paymentSlot, false)

	return nil
}
