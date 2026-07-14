package minecraft_conn

import (
	"github.com/LangTuStudio/Conbit/minecraft/protocol/login"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/minecraft_neo/can_close"
	"github.com/LangTuStudio/Conbit/minecraft_neo/game_data"
)

type Conn interface {
	GameData() game_data.GameData
	IdentityData() login.IdentityData
	GetShieldID() int32
	ReadPacketAndBytes() (packet.Packet, []byte)
	WritePacket(packet.Packet)
	WriteBytePacket([]byte)
	Flush() error
	can_close.CanCloseWithError
}
