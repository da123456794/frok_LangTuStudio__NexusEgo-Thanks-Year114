package nbt_parser_block

import (
	"bytes"
	"fmt"

	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol"
)

// LodestoneNBT ..
type LodestoneNBT struct {
	HaveTrackingHandle bool
	TrackingHandle     int32
}

// 磁石
type Lodestone struct {
	DefaultBlock
	NBT LodestoneNBT
}

func (l Lodestone) NeedSpecialHandle() bool {
	return l.NBT.HaveTrackingHandle
}

func (Lodestone) NeedCheckCompletely() bool {
	return true
}

func (l Lodestone) formatNBT(prefix string) string {
	return prefix + fmt.Sprintf("磁石 ID: %d\n", l.NBT.TrackingHandle)
}

func (l *Lodestone) Format(prefix string) string {
	result := l.DefaultBlock.Format(prefix)
	if l.NeedSpecialHandle() {
		result += prefix + "附加数据: \n"
		result += l.formatNBT(prefix + "\t")
	}
	return result
}

func (l *Lodestone) Parse(nbtMap map[string]any) error {
	result := LodestoneNBT{}
	result.TrackingHandle, result.HaveTrackingHandle = nbtMap["trackingHandle"].(int32)
	l.NBT = result
	return nil
}

func (l Lodestone) NBTStableBytes() []byte {
	buf := bytes.NewBuffer(nil)
	w := protocol.NewWriter(buf, 0)

	w.Bool(&l.NBT.HaveTrackingHandle)

	return buf.Bytes()
}

func (l *Lodestone) FullStableBytes() []byte {
	return append(l.DefaultBlock.FullStableBytes(), l.NBTStableBytes()...)
}
