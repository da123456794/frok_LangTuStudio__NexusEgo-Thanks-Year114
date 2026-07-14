package client

import (
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"nexus/utils/mirror/io/assembler"
	"sync"
)

type Client struct {
	//ClientInfo
	Access AccessContext

	GetCheckNumEverPassed bool
	CachedPacket          interface{}
	LRUMemoryChunkCacher  interface{}
	ChunkFeeder           interface{}
	Resources             interface{}
	ResourcesUpdater      interface{}
	GameInterface         GameInterface
	AvailableCommands     *packet.AvailableCommands
	Conn                  Conn
	Uid                   string // Uid
	Cdump_Setting         *Cdump_Setting
	ChunkAssembler        *assembler.Assembler
	SkipSubChunkCheck     bool
	GlobalFullConfig      interface{}
	IsOP_loop             sync.Mutex
	IsOP                  bool
	RepairCtx             *RepairContext
	CommandDimension      string
	DimensionID           int32
	LastImportError       string

	playerNamesMu     sync.RWMutex
	playerNamesByXUID map[string]string
	playerNamesByChat map[string]string
	playerNamesByName map[string]string
	playerNamesByUUID map[string]cachedPlayerName
}

func (client *Client) init() {

}

