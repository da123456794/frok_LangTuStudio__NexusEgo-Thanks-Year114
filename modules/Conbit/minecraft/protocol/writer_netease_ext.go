package protocol

import (
	"bytes"
	"fmt"
	"image/color"

	"github.com/LangTuStudio/Conbit/minecraft/nbt"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/ugorji/go/codec"
)

// UBlockPos writes a BlockPos with Y encoded as an unsigned varint32.
func (w *Writer) UBlockPos(x *BlockPos) {
	w.Varint32(&x[0])
	y := uint32(x[1])
	w.Varuint32(&y)
	w.Varint32(&x[2])
}

// USubChunkPos writes a SubChunkPos with Y encoded as an unsigned varint32.
func (w *Writer) USubChunkPos(x *SubChunkPos) {
	w.Varint32(&x[0])
	y := uint32(x[1])
	w.Varuint32(&y)
	w.Varint32(&x[2])
}

// USoundPos writes a sound position using the NetEase/PhoenixBuilder unsigned block position flavour.
func (w *Writer) USoundPos(x *mgl32.Vec3) {
	b := BlockPos{int32((*x)[0] * 8), int32((*x)[1] * 8), int32((*x)[2] * 8)}
	w.UBlockPos(&b)
}

// NBTWithLength writes a length-prefixed NBT compound.
func (w *Writer) NBTWithLength(m *map[string]any) {
	if m == nil || *m == nil {
		var zero int32
		w.Varint32(&zero)
		return
	}
	data, err := nbt.MarshalEncoding(*m, nbt.NetworkLittleEndian)
	if err != nil {
		panic(err)
	}
	length := int32(len(data))
	w.Varint32(&length)
	w.Bytes(&data)
}

// MsgPack writes a NetEase/Python style msgpack payload.
func (w *Writer) MsgPack(x *any) {
	var msgPackBytes []byte
	if err := codec.NewEncoderBytes(&msgPackBytes, &codec.MsgpackHandle{}).Encode(x); err != nil {
		panic(fmt.Sprintf("(w *Writer) MsgPack: %v", err))
	}
	w.ByteSlice(&msgPackBytes)
}

// NeteasePixels writes a pixel slice in the same format as the legacy NetEase map packet flavour.
func (w *Writer) NeteasePixels(x *[]color.RGBA) {
	FuncSlice(w, x, w.VarRGBA)
}

// CompressedBiomeDefinitions writes the legacy compressed biome dictionary format used by the NetEase fork.
func (w *Writer) CompressedBiomeDefinitions(x *map[string]any) {
	decompressed, err := nbt.Marshal(*x)
	if err != nil {
		w.panicf("error marshaling nbt: %v", err)
	}

	var compressed []byte
	buf := bytes.NewBuffer(compressed)
	bufWriter := NewWriter(buf, w.shieldID)

	header := []byte("COMPRESSED")
	bufWriter.Bytes(&header)

	var dictionaryLength uint16
	bufWriter.Uint16(&dictionaryLength)
	for _, b := range decompressed {
		bufWriter.Uint8(&b)
		if b == 0xff {
			dictionaryIndex := int16(1)
			bufWriter.Int16(&dictionaryIndex)
		}
	}

	compressed = buf.Bytes()
	length := uint32(len(compressed))
	w.Varuint32(&length)
	w.Bytes(&compressed)
}
