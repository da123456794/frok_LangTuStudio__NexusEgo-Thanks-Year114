package access_helper

import "testing"

func TestNormalizeServerTarget(t *testing.T) {
	t.Run("traditional rental code keeps passcode", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("44314120", "123456")
		if serverCode != "44314120" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "123456" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("invite code defaults to domain game", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("ABCDEF", "123456")
		if serverCode != "DomainGame:ABCDEF" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("lobby game target is passed through", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("LobbyGame:1234567890123456789", "654321")
		if serverCode != "LobbyGame:1234567890123456789" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "654321" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("english prefixes support full-width colon", func(t *testing.T) {
		tests := []struct {
			name             string
			serverCode       string
			serverPassword   string
			wantServerCode   string
			wantServerPasswd string
		}{
			{
				name:           "domain game",
				serverCode:     "DomainGame：ABCDEF",
				serverPassword: "123456",
				wantServerCode: "DomainGame:ABCDEF",
			},
			{
				name:           "pc domain game",
				serverCode:     "PCDomainGame：ABCDEF",
				serverPassword: "123456",
				wantServerCode: "PCDomainGame:ABCDEF",
			},
			{
				name:             "tan lobby",
				serverCode:       "TanLobby：123456",
				serverPassword:   "654321",
				wantServerCode:   "TanLobby:123456",
				wantServerPasswd: "654321",
			},
			{
				name:             "lobby game",
				serverCode:       "LobbyGame：1234567890123456789",
				serverPassword:   "654321",
				wantServerCode:   "LobbyGame:1234567890123456789",
				wantServerPasswd: "654321",
			},
			{
				name:             "network game",
				serverCode:       "NetworkGame：544895",
				serverPassword:   "654321",
				wantServerCode:   "NetworkGame:544895",
				wantServerPasswd: "654321",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				serverCode, serverPassword := NormalizeServerTarget(tt.serverCode, tt.serverPassword)
				if serverCode != tt.wantServerCode {
					t.Fatalf("unexpected server code: %q", serverCode)
				}
				if serverPassword != tt.wantServerPasswd {
					t.Fatalf("unexpected server password: %q", serverPassword)
				}
			})
		}
	})

	t.Run("chinese domain game prefix supports full-width colon", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("山头：ABCDEF", "123456")
		if serverCode != "DomainGame:ABCDEF" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("chinese local online game maps to tan lobby", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("本地联机:544895", "654321")
		if serverCode != "TanLobby:544895" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "654321" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("chinese online lobby maps to lobby game", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("联机大厅:1234567890123456789", "654321")
		if serverCode != "LobbyGame:1234567890123456789" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "654321" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("chinese network game maps to network game", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("网络游戏:544895", "654321")
		if serverCode != "NetworkGame:544895" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "654321" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("chinese lobby shorthand maps to lobby game", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("大厅:1234567890123456789", "654321")
		if serverCode != "LobbyGame:1234567890123456789" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "654321" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})

	t.Run("main city is passed through", func(t *testing.T) {
		serverCode, serverPassword := NormalizeServerTarget("MainCity", "  ")
		if serverCode != "MainCity" {
			t.Fatalf("unexpected server code: %q", serverCode)
		}
		if serverPassword != "" {
			t.Fatalf("unexpected server password: %q", serverPassword)
		}
	})
}

func TestIsDomainGameTarget(t *testing.T) {
	if !IsDomainGameTarget("DomainGame:ABCDEF") {
		t.Fatal("expected DomainGame target to be detected")
	}
	if !IsDomainGameTarget("DomainGame：ABCDEF") {
		t.Fatal("expected DomainGame target with full-width colon to be detected")
	}
	if !IsDomainGameTarget("PCDomainGame:ABCDEF") {
		t.Fatal("expected PCDomainGame target to be detected")
	}
	if IsDomainGameTarget("44314120") {
		t.Fatal("rental target should not be treated as domain game")
	}
}

func TestOnlineGameTargetDetection(t *testing.T) {
	if !IsTanLobbyTarget("TanLobby：123456") {
		t.Fatal("expected TanLobby target to be detected")
	}
	if !IsLobbyGameTarget("LobbyGame:1234567890123456789") {
		t.Fatal("expected LobbyGame target to be detected")
	}
	if !IsLobbyGameTarget("LobbyGame：1234567890123456789") {
		t.Fatal("expected LobbyGame target with full-width colon to be detected")
	}
	if !IsNetworkGameTarget("NetworkGame:544895") {
		t.Fatal("expected NetworkGame target to be detected")
	}
	if !IsOnlineGameTarget("TanLobby:123456") {
		t.Fatal("expected TanLobby target to be treated as online game")
	}
	if !IsOnlineGameTarget("LobbyGame:1234567890123456789") {
		t.Fatal("expected LobbyGame target to be treated as online game")
	}
	if !IsOnlineGameTarget("NetworkGame:544895") {
		t.Fatal("expected NetworkGame target to be treated as online game")
	}
	if IsOnlineGameTarget("DomainGame:ABCDEF") {
		t.Fatal("domain game target should not be treated as online game")
	}
}

func TestServerTargetLogNames(t *testing.T) {
	tests := []struct {
		name              string
		serverCode        string
		wantNeteaseKind   string
		wantMinecraftKind string
		wantCodeLabel     string
		wantLogValue      string
	}{
		{
			name:              "traditional rental server",
			serverCode:        "44314120",
			wantNeteaseKind:   "租赁服",
			wantMinecraftKind: "服务器",
			wantCodeLabel:     "服号: ",
			wantLogValue:      "44314120",
		},
		{
			name:              "domain game",
			serverCode:        "DomainGame:ABCDEF",
			wantNeteaseKind:   "山头",
			wantMinecraftKind: "山头",
			wantCodeLabel:     "邀请码 :",
			wantLogValue:      "ABCDEF",
		},
		{
			name:              "tan lobby",
			serverCode:        "TanLobby:272897",
			wantNeteaseKind:   "本地联机",
			wantMinecraftKind: "本地联机",
			wantCodeLabel:     "房间号 :",
			wantLogValue:      "272897",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := ServerTargetLogNames(tt.serverCode)
			if names.neteaseKind != tt.wantNeteaseKind {
				t.Fatalf("unexpected netease kind: %q", names.neteaseKind)
			}
			if names.minecraftKind != tt.wantMinecraftKind {
				t.Fatalf("unexpected minecraft kind: %q", names.minecraftKind)
			}
			if names.codeLabel != tt.wantCodeLabel {
				t.Fatalf("unexpected code label: %q", names.codeLabel)
			}
			if got := serverTargetValueForLog(tt.serverCode); got != tt.wantLogValue {
				t.Fatalf("unexpected log value: %q", got)
			}
		})
	}
}
