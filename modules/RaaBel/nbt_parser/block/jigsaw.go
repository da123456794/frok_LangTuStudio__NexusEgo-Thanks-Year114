package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/LangTuStudio/RaaBel/mapping"
	"github.com/mitchellh/mapstructure"
)

const (
	DefaultJigsawFinalState = "minecraft:air"
	DefaultJigsawJoint      = "rollable"
	DefaultJigsawName       = "minecraft:empty"
	DefaultJigsawTarget     = "minecraft:empty"
	DefaultJigsawTargetPool = "minecraft:empty"
)

// JigsawNBT ..
type JigsawNBT struct {
	FinalState        string `mapstructure:"final_state"`
	Joint             string `mapstructure:"joint"`
	Name              string `mapstructure:"name"`
	PlacementPriority int32  `mapstructure:"placement_priority"`
	SelectionPriority int32  `mapstructure:"selection_priority"`
	Target            string `mapstructure:"target"`
	TargetPool        string `mapstructure:"target_pool"`
}

// 拼图方块
type Jigsaw struct {
	DefaultBlock
	NBT JigsawNBT
}

func (j Jigsaw) NeedSpecialHandle() bool {
	if j.NBT.FinalState != DefaultJigsawFinalState {
		return true
	}
	if j.NBT.Joint != DefaultJigsawJoint {
		return true
	}
	if j.NBT.Name != DefaultJigsawName {
		return true
	}
	if j.NBT.Target != DefaultJigsawTarget {
		return true
	}
	if j.NBT.TargetPool != DefaultJigsawTargetPool {
		return true
	}
	if j.NBT.PlacementPriority != 0 {
		return true
	}
	if j.NBT.SelectionPriority != 0 {
		return true
	}
	return false
}

func (Jigsaw) NeedCheckCompletely() bool {
	return false
}

func (j Jigsaw) formatNBT(prefix string) string {
	result := ""
	joint := j.NBT.Joint
	if value, ok := mapping.JigsawJointFormat[j.NBT.Joint]; ok {
		joint = value
	}
	result += prefix + fmt.Sprintf("目标池: %s\n", j.NBT.TargetPool)
	result += prefix + fmt.Sprintf("名称: %s\n", j.NBT.Name)
	result += prefix + fmt.Sprintf("目标名称: %s\n", j.NBT.Target)
	result += prefix + fmt.Sprintf("变为: %s\n", j.NBT.FinalState)
	result += prefix + fmt.Sprintf("选择优先级: %d\n", j.NBT.SelectionPriority)
	result += prefix + fmt.Sprintf("放置优先级: %d\n", j.NBT.PlacementPriority)
	result += prefix + fmt.Sprintf("连点类型: %s\n", joint)
	return result
}

func (j *Jigsaw) Format(prefix string) string {
	result := j.DefaultBlock.Format(prefix)
	if j.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += j.formatNBT(prefix + "\t")
	}
	return result
}

func (j *Jigsaw) Parse(nbtMap map[string]any) error {
	result := JigsawNBT{
		FinalState: DefaultJigsawFinalState,
		Joint:      DefaultJigsawJoint,
		Name:       DefaultJigsawName,
		Target:     DefaultJigsawTarget,
		TargetPool: DefaultJigsawTargetPool,
	}
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	j.NBT = result
	return nil
}

func (j Jigsaw) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.String(&j.NBT.FinalState)
	w.String(&j.NBT.Joint)
	w.String(&j.NBT.Name)
	w.Varint32(&j.NBT.PlacementPriority)
	w.Varint32(&j.NBT.SelectionPriority)
	w.String(&j.NBT.Target)
	w.String(&j.NBT.TargetPool)

	return buf.Bytes()
}

func (j *Jigsaw) FullStableBytes() []byte {
	return append(j.DefaultBlock.FullStableBytes(), j.NBTStableBytes()...)
}
