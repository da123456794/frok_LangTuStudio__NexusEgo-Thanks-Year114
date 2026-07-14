package packet

import (
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
)

// UpdateAbilities is a packet sent from the server to the client to update the abilities of the player. It, along with
// the UpdateAdventureSettings packet, are replacements of the AdventureSettings packet since v1.19.10.
type UpdateAbilities struct {
	// AbilityData represents various data about the abilities of a player, such as ability layers or permissions.
	AbilityData protocol.AbilityData

	Unknown1 int32
	// Netease
	Unknown2 int64
	// p.s.) 这里实际上也同是瞎蒙的
}

// ID ...
func (*UpdateAbilities) ID() uint32 {
	return IDUpdateAbilities
}

func (pk *UpdateAbilities) Marshal(io protocol.IO) {
	protocol.Single(io, &pk.AbilityData)

	//Netease
	io.Int32(&pk.Unknown1)
	io.Int64(&pk.Unknown2)
}
