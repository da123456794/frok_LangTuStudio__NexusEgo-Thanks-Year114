package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
	"github.com/mitchellh/mapstructure"
)

// MobSpawnerNBT...
type MobSpawnerNBT struct {
	EntityIdentifier string `mapstructure:"EntityIdentifier"`
}

// 刷怪笼
type MobSpawner struct {
	DefaultBlock
	NBT MobSpawnerNBT
}

func (m MobSpawner) NeedSpecialHandle() bool {
	return len(m.NBT.EntityIdentifier) != 0
}

func (MobSpawner) NeedCheckCompletely() bool {
	return true
}

func (m MobSpawner) formatNBT(prefix string) string {
	return prefix + fmt.Sprintf("生成生物: %s\n", m.NBT.EntityIdentifier)
}

func (m *MobSpawner) Format(prefix string) string {
	result := m.DefaultBlock.Format(prefix)
	if m.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += m.formatNBT(prefix + "\t")
	}
	return result
}

func (m *MobSpawner) Parse(nbtMap map[string]any) error {
	var result MobSpawnerNBT
	if err := mapstructure.Decode(nbtMap, &result); err != nil {
		return fmt.Errorf("Parse: %v", err)
	}
	m.NBT = result
	return nil
}

func (m MobSpawner) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.String(&m.NBT.EntityIdentifier)

	return buf.Bytes()
}

func (m *MobSpawner) FullStableBytes() []byte {
	return append(m.DefaultBlock.FullStableBytes(), m.NBTStableBytes()...)
}
