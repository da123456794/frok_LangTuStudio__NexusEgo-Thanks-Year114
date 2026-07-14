package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/mitchellh/mapstructure"
)

// TrialSpawnerSpawnData...
type TrialSpawnerSpawnData struct {
	TypeID string `mapstructure:"TypeId"`
}

// TrialSpawnerNBT...
type TrialSpawnerNBT struct {
	SpawnData TrialSpawnerSpawnData `mapstructure:"spawn_data"`
}

// 试炼刷怪笼
type TrialSpawner struct {
	DefaultBlock
	NBT TrialSpawnerNBT
}

func (t TrialSpawner) NeedSpecialHandle() bool {
	return len(t.NBT.SpawnData.TypeID) != 0
}

func (TrialSpawner) NeedCheckCompletely() bool {
	return true
}

func (t TrialSpawner) formatNBT(prefix string) string {
	return prefix + fmt.Sprintf("生成生物: %s\n", t.NBT.SpawnData.TypeID)
}

func (t *TrialSpawner) Format(prefix string) string {
	result := t.DefaultBlock.Format(prefix)
	if t.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += t.formatNBT(prefix + "\t")
	}
	return result
}

func (t *TrialSpawner) Parse(nbtMap map[string]any) error {
	var result TrialSpawnerNBT
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	t.NBT = result
	return nil
}

func (t TrialSpawner) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.String(&t.NBT.SpawnData.TypeID)

	return buf.Bytes()
}

func (t *TrialSpawner) FullStableBytes() []byte {
	return append(t.DefaultBlock.FullStableBytes(), t.NBTStableBytes()...)
}
