package packet

import (
	"bytes"
	"testing"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
)

func TestResourcePackStackReadsNeteaseTexturePacks(t *testing.T) {
	original := &ResourcePackStack{
		BaseGameVersion: "*",
		NeteaseTexturePacks: []protocol.StackResourcePack{
			{UUID: "0091565e-b3c9-475c-8ee7-270cd12e099c", Version: "0.3.2"},
			{UUID: "56cf7f31-dd21-403d-94f2-ec4243cdc6ea", Version: "0.0.12"},
		},
	}

	buf := bytes.NewBuffer(nil)
	original.Marshal(protocol.NewWriter(buf, 0))

	readBuf := bytes.NewBuffer(buf.Bytes())
	decoded := &ResourcePackStack{}
	decoded.Marshal(protocol.NewReader(readBuf, 0, false))

	if readBuf.Len() != 0 {
		t.Fatalf("resource pack stack decode left %v unread bytes", readBuf.Len())
	}
	if len(decoded.NeteaseTexturePacks) != len(original.NeteaseTexturePacks) {
		t.Fatalf("expected %v netease texture packs, got %v", len(original.NeteaseTexturePacks), len(decoded.NeteaseTexturePacks))
	}
	if decoded.NeteaseTexturePacks[1].UUID != original.NeteaseTexturePacks[1].UUID {
		t.Fatalf("unexpected decoded pack UUID: %q", decoded.NeteaseTexturePacks[1].UUID)
	}
}
