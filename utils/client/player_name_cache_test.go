package client

import (
	"testing"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/google/uuid"
)

func TestResolvePlayerNameUsesXUID(t *testing.T) {
	cl := &Client{}
	cl.UpdatePlayerNameCache([]protocol.PlayerListEntry{{
		UUID:     uuid.New(),
		Username: "HGYU",
		XUID:     "12345",
	}}, false)

	if got := cl.ResolvePlayerName("12345", "", "结合 HGYU"); got != "HGYU" {
		t.Fatalf("ResolvePlayerName by XUID got %q, want HGYU", got)
	}
}

func TestResolvePlayerNameMatchesDisplaySuffix(t *testing.T) {
	cl := &Client{}
	cl.UpdatePlayerNameCache([]protocol.PlayerListEntry{{
		UUID:     uuid.New(),
		Username: "HGYU",
	}}, false)

	if got := cl.ResolvePlayerName("", "", "结合 HGYU"); got != "HGYU" {
		t.Fatalf("ResolvePlayerName by display suffix got %q, want HGYU", got)
	}
}

func TestResolvePlayerNameUsesPlatformChatID(t *testing.T) {
	cl := &Client{}
	cl.UpdatePlayerNameCache([]protocol.PlayerListEntry{{
		UUID:           uuid.New(),
		Username:       "HGYU",
		PlatformChatID: "chat-1",
	}}, false)

	if got := cl.ResolvePlayerName("", "chat-1", "结合 HGYU"); got != "HGYU" {
		t.Fatalf("ResolvePlayerName by PlatformChatID got %q, want HGYU", got)
	}
}

func TestResolvePlayerNameKeepsAmbiguousDisplayName(t *testing.T) {
	cl := &Client{}
	cl.UpdatePlayerNameCache([]protocol.PlayerListEntry{
		{UUID: uuid.New(), Username: "B"},
		{UUID: uuid.New(), Username: "A B"},
	}, false)

	if got := cl.ResolvePlayerName("", "", "VIP A B"); got != "VIP A B" {
		t.Fatalf("ambiguous ResolvePlayerName got %q, want original display name", got)
	}
}

