package protocol

import (
	"bytes"
	"testing"
)

func TestSkinAllowsEmptyImageDataWhenReading(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	w := NewWriter(buf, 0)
	skinID := "netease-empty-skin"
	emptyString := ""
	emptyBytes := []byte(nil)
	width, height := uint32(64), uint32(64)
	zero := uint32(0)
	falseValue := false

	w.String(&skinID)
	w.String(&emptyString)
	w.ByteSlice(&emptyBytes)
	w.Uint32(&width)
	w.Uint32(&height)
	w.ByteSlice(&emptyBytes)
	w.Uint32(&zero)
	w.Uint32(&zero)
	w.Uint32(&zero)
	w.ByteSlice(&emptyBytes)
	w.ByteSlice(&emptyBytes)
	w.ByteSlice(&emptyBytes)
	w.ByteSlice(&emptyBytes)
	w.String(&emptyString)
	w.String(&emptyString)
	w.String(&emptyString)
	w.String(&emptyString)
	w.Uint32(&zero)
	w.Uint32(&zero)
	w.Bool(&falseValue)
	w.Bool(&falseValue)
	w.Bool(&falseValue)
	w.Bool(&falseValue)
	w.Bool(&falseValue)

	readBuf := bytes.NewBuffer(buf.Bytes())
	var skin Skin
	skin.Marshal(NewReader(readBuf, 0, false))

	if readBuf.Len() != 0 {
		t.Fatalf("skin decode left %v unread bytes", readBuf.Len())
	}
	if skin.SkinImageWidth != width || skin.SkinImageHeight != height || len(skin.SkinData) != 0 {
		t.Fatalf("unexpected decoded skin dimensions/data: %vx%v len=%v", skin.SkinImageWidth, skin.SkinImageHeight, len(skin.SkinData))
	}
}

func TestSkinRejectsEmptyImageDataWhenWriting(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected empty image data to be rejected while writing")
		}
	}()

	skin := Skin{SkinID: "invalid-empty-skin", SkinImageWidth: 64, SkinImageHeight: 64}
	skin.Marshal(NewWriter(bytes.NewBuffer(nil), 0))
}
