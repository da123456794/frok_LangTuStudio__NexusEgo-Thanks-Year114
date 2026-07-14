package client

import "testing"

func TestNormalizeServerTargetOnlineEntrances(t *testing.T) {
	tests := []struct {
		name         string
		serverCode   string
		password     string
		wantCode     string
		wantPassword string
	}{
		{
			name:         "local tan lobby",
			serverCode:   "本地联机:544895",
			password:     "654321",
			wantCode:     "TanLobby:544895",
			wantPassword: "654321",
		},
		{
			name:         "online lobby",
			serverCode:   "联机大厅:1234567890123456789",
			password:     "654321",
			wantCode:     "LobbyGame:1234567890123456789",
			wantPassword: "654321",
		},
		{
			name:         "short online lobby alias",
			serverCode:   "大厅：1234567890123456789",
			password:     "654321",
			wantCode:     "LobbyGame:1234567890123456789",
			wantPassword: "654321",
		},
		{
			name:         "network game",
			serverCode:   "网络游戏:544895",
			password:     "654321",
			wantCode:     "NetworkGame:544895",
			wantPassword: "654321",
		},
		{
			name:         "domain game",
			serverCode:   "山头:ABCDEF",
			password:     "654321",
			wantCode:     "DomainGame:ABCDEF",
			wantPassword: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCode, gotPassword := NormalizeServerTarget(tt.serverCode, tt.password)
			if gotCode != tt.wantCode {
				t.Fatalf("unexpected server code: got %q, want %q", gotCode, tt.wantCode)
			}
			if gotPassword != tt.wantPassword {
				t.Fatalf("unexpected password: got %q, want %q", gotPassword, tt.wantPassword)
			}
		})
	}
}

func TestOnlineTargetDetection(t *testing.T) {
	if !IsOnlineGameTarget("TanLobby:544895") {
		t.Fatal("expected TanLobby to be online target")
	}
	if !IsOnlineGameTarget("LobbyGame:1234567890123456789") {
		t.Fatal("expected LobbyGame to be online target")
	}
	if !IsOnlineGameTarget("NetworkGame:544895") {
		t.Fatal("expected NetworkGame to be online target")
	}
	if IsOnlineGameTarget("DomainGame:ABCDEF") {
		t.Fatal("expected DomainGame not to be online target")
	}
}

func TestMCPChallengeSkipDetection(t *testing.T) {
	tests := []struct {
		name       string
		serverCode string
		want       bool
	}{
		{name: "rental server", serverCode: "12345678", want: false},
		{name: "main city", serverCode: "MainCity", want: false},
		{name: "domain game", serverCode: "DomainGame:ABCDEF", want: true},
		{name: "domain game chinese", serverCode: "山头:ABCDEF", want: true},
		{name: "online lobby", serverCode: "LobbyGame:1234567890123456789", want: true},
		{name: "online lobby chinese", serverCode: "联机大厅:1234567890123456789", want: true},
		{name: "tan lobby", serverCode: "TanLobby:544895", want: true},
		{name: "tan lobby chinese", serverCode: "本地联机:544895", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldSkipMCPCheckChallenge(tt.serverCode); got != tt.want {
				t.Fatalf("ShouldSkipMCPCheckChallenge(%q) = %v, want %v", tt.serverCode, got, tt.want)
			}
		})
	}
}
