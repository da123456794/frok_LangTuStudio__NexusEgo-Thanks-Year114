package client

import (
	"github.com/LangTuStudio/RaaBel/core/bunker/auth"
	"github.com/LangTuStudio/RaaBel/core/minecraft"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
)

// ------------------------- Config -------------------------

// Config ..
type Config struct {
	AuthServerAddress    string
	AuthServerToken      string
	RentalServerCode     string
	RentalServerPasscode string
}

// ------------------------- Client -------------------------

// Client ..
type Client struct {
	connection            *minecraft.Conn
	authClient            *auth.Client
	getCheckNumEverPassed bool
	cachedPacket          chan packet.Packet
	skipMCPCheckChallenge bool
}

// Conn ..
func (c Client) Conn() *minecraft.Conn {
	return c.connection
}

// CachedPacket ..
func (c *Client) CachedPacket() chan packet.Packet {
	c.ensureCachedPacket()
	return c.cachedPacket
}

func (c *Client) ensureCachedPacket() {
	if c.cachedPacket != nil {
		return
	}
	cachedPacket := make(chan packet.Packet)
	close(cachedPacket)
	c.cachedPacket = cachedPacket
}

// ------------------------- MCPCheckChallengesSolver -------------------------

// MCPCheckChallengesSolver ..
type MCPCheckChallengesSolver struct {
	client *Client
}

// NewChallengeSolver ..
func NewChallengeSolver(client *Client) *MCPCheckChallengesSolver {
	return &MCPCheckChallengesSolver{client: client}
}
