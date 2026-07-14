package service

import "testing"

func TestShouldWaitConsoleInitResponse(t *testing.T) {
	tests := []struct {
		name       string
		serverCode string
		want       bool
	}{
		{name: "rental server", serverCode: "12345678", want: true},
		{name: "domain game", serverCode: "DomainGame:ABCDEF", want: true},
		{name: "online lobby", serverCode: "LobbyGame:1234567890123456789", want: false},
		{name: "tan lobby", serverCode: "TanLobby:544895", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldWaitConsoleInitResponse(tt.serverCode); got != tt.want {
				t.Fatalf("shouldWaitConsoleInitResponse(%q) = %v, want %v", tt.serverCode, got, tt.want)
			}
		})
	}
}
