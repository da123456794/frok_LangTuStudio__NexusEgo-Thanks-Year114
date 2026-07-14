package packet

import (
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
)

// Netease Packet
type UpdateBiome struct {
	// Netease
	Pos protocol.BlockPos
	// Netease
	BiomeData []protocol.NeteaseBiomeData
}

// ID ...
func (*UpdateBiome) ID() uint32 {
	return IDUpdateBiome
}

func (pk *UpdateBiome) Marshal(io protocol.IO) {
	io.UBlockPos(&pk.Pos)
	protocol.Slice(io, &pk.BiomeData)
}
