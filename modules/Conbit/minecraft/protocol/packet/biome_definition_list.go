package packet

import (
	"bytes"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
)

// BiomeDefinitionList is sent by the server to let the client know all biomes that are available and
// implemented on the server side. When enabled, it also includes information for the client to
// accurately recreate the server-side generation in vanilla worlds/servers for increased performance.
type BiomeDefinitionList struct {
	// BiomeDefinitions is a list of biomes that are available on the server.
	BiomeDefinitions []protocol.BiomeDefinition
	// StringList is a makeshift dictionary implementation Mojang created to try and reduce the size of the
	// overall packet. It is a list of common strings that are used in the biome definitions, such as
	// biome names, float values or query expressions.
	StringList []string
	// SerialisedBiomeDefinitions is kept for local compatibility with the legacy compressed biome dictionary.
	SerialisedBiomeDefinitions map[string]any
	// RawPayload stores the unread biome payload when the server uses a layout that is not modelled locally.
	RawPayload []byte
}

// ID ...
func (*BiomeDefinitionList) ID() uint32 {
	return IDBiomeDefinitionList
}

func (pk *BiomeDefinitionList) Marshal(io protocol.IO) {
	if reader, isReader := io.(*protocol.Reader); isReader {
		peek := reader.PeekBytes(16)
		buf := bytes.NewBuffer(peek)
		var compressedLen uint32
		if err := protocol.Varuint32(buf, &compressedLen); err == nil && buf.Len() >= len("COMPRESSED") && string(buf.Next(len("COMPRESSED"))) == "COMPRESSED" {
			io.CompressedBiomeDefinitions(&pk.SerialisedBiomeDefinitions)
			return
		}
		pk.RawPayload = reader.TakeRemainingBytes()
		return
	}
	if pk.SerialisedBiomeDefinitions != nil {
		io.CompressedBiomeDefinitions(&pk.SerialisedBiomeDefinitions)
		return
	}
	protocol.Slice(io, &pk.BiomeDefinitions)
	protocol.FuncSlice(io, &pk.StringList, io.String)
}
