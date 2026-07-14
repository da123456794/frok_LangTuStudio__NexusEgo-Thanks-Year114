package packet_conn

import (
	"testing"

	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

func TestShouldIgnoreTrailingBytes(t *testing.T) {
	if !shouldIgnoreTrailingBytes(&packet.StartGame{}, []byte{0x01, 0x02}) {
		t.Fatal("StartGame trailing bytes should be tolerated")
	}
	if !shouldIgnoreTrailingBytes(&packet.ItemRegistry{}, []byte{0x01}) {
		t.Fatal("ItemRegistry trailing bytes should be tolerated")
	}
	if !shouldIgnoreTrailingBytes(&packet.ContainerClose{}, []byte{0x00}) {
		t.Fatal("ContainerClose single zero padding should be tolerated")
	}
	if shouldIgnoreTrailingBytes(&packet.ContainerClose{}, []byte{0x01}) {
		t.Fatal("unexpected ContainerClose trailing bytes should not be tolerated")
	}
	if !shouldIgnoreTrailingBytes(&packet.MobArmourEquipment{}, []byte{0x00}) {
		t.Fatal("MobArmourEquipment single zero padding should be tolerated")
	}
	if shouldIgnoreTrailingBytes(&packet.MobArmourEquipment{}, []byte{0x01}) {
		t.Fatal("unexpected MobArmourEquipment trailing bytes should not be tolerated")
	}
	if shouldIgnoreTrailingBytes(&packet.MovePlayer{}, []byte{0x00}) {
		t.Fatal("unrelated packet trailing bytes should not be tolerated")
	}
}
