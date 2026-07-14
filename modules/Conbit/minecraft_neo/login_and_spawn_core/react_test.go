package login_and_spawn_core

import (
	"sync"
	"testing"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/minecraft_neo/can_close"
	"github.com/LangTuStudio/Conbit/minecraft_neo/login_and_spawn_core/options"
	"github.com/google/uuid"
)

type testPacketConn struct {
	can_close.CanCloseWithError
	mu      sync.Mutex
	written []packet.Packet
}

func newTestPacketConn() *testPacketConn {
	return &testPacketConn{CanCloseWithError: can_close.NewClose(func() {})}
}

func (c *testPacketConn) ListenRoutine(func(packet.Packet, []byte)) {}

func (c *testPacketConn) WritePacket(pk packet.Packet) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.written = append(c.written, pk)
	return nil
}

func (c *testPacketConn) SetShieldID(int32) {}

func (c *testPacketConn) GetShieldID() int32 { return 0 }

func (c *testPacketConn) EnableEncryption([32]byte) {}

func (c *testPacketConn) EnableCompression(packet.Compression) {}

func (c *testPacketConn) Flush() error { return nil }

func TestHandleResourcePackStackIgnoresUndownloadedPacks(t *testing.T) {
	conn := newTestPacketConn()
	core := NewLoginAndSpawnCore(conn, &options.Options{})
	pack := protocol.StackResourcePack{UUID: "80a59c71-b088-462a-b338-6e24415f4158", Version: "0.0.4636"}

	if err := core.handleResourcePackStack(&packet.ResourcePackStack{TexturePacks: []protocol.StackResourcePack{pack}}); err != nil {
		t.Fatalf("handleResourcePackStack returned error: %v", err)
	}
	if !core.hasPack(pack.UUID, pack.Version, false) {
		t.Fatal("undownloaded pack should be remembered as ignored")
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.written) != 1 {
		t.Fatalf("expected one packet written, got %v", len(conn.written))
	}
	resp, ok := conn.written[0].(*packet.ResourcePackClientResponse)
	if !ok {
		t.Fatalf("expected ResourcePackClientResponse, got %T", conn.written[0])
	}
	if resp.Response != packet.PackResponseCompleted {
		t.Fatalf("expected PackResponseCompleted, got %v", resp.Response)
	}
}

func TestHandleResourcePacksInfoSkipsServerPacks(t *testing.T) {
	conn := newTestPacketConn()
	core := NewLoginAndSpawnCore(conn, &options.Options{})
	packUUID := uuid.MustParse("80a59c71-b088-462a-b338-6e24415f4158")
	pack := protocol.TexturePackInfo{UUID: packUUID, Version: "1.0.0", Size: 1024, AddonPack: true}

	if err := core.handleResourcePacksInfo(&packet.ResourcePacksInfo{
		TexturePackRequired: true,
		HasAddons:           true,
		HasScripts:          true,
		TexturePacks:        []protocol.TexturePackInfo{pack},
	}); err != nil {
		t.Fatalf("handleResourcePacksInfo returned error: %v", err)
	}
	if !core.hasPack(pack.UUID.String(), pack.Version, false) {
		t.Fatal("server pack should be remembered as ignored")
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.written) != 1 {
		t.Fatalf("expected one packet written, got %v", len(conn.written))
	}
	resp, ok := conn.written[0].(*packet.ResourcePackClientResponse)
	if !ok {
		t.Fatalf("expected ResourcePackClientResponse, got %T", conn.written[0])
	}
	if resp.Response != packet.PackResponseAllPacksDownloaded {
		t.Fatalf("expected PackResponseAllPacksDownloaded, got %v", resp.Response)
	}
	if len(resp.PacksToDownload) != 0 {
		t.Fatalf("expected no packs to download, got %v", resp.PacksToDownload)
	}
}
