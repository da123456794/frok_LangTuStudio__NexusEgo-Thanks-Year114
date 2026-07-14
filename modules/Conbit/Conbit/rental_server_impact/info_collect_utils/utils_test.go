package info_collect_utils

import "testing"

func TestTranslateInputToAuthServer(t *testing.T) {
	t.Run("official auth server", func(t *testing.T) {
		authServer, _, err := TranslateInputToAuthServer("1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authServer != AUTH_SERVER_FB_OFFICIAL {
			t.Fatalf("unexpected auth server: %q", authServer)
		}
	})

	t.Run("custom selection", func(t *testing.T) {
		authServer, _, err := TranslateInputToAuthServer("3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authServer != AUTH_SERVER_CUSTOM {
			t.Fatalf("unexpected auth server: %q", authServer)
		}
	})

	t.Run("direct custom url", func(t *testing.T) {
		authServer, _, err := TranslateInputToAuthServer("http://127.0.0.1:1200/")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authServer != "http://127.0.0.1:1200" {
			t.Fatalf("unexpected auth server: %q", authServer)
		}
	})
}

func TestLooksLikeUserTokenCandidate(t *testing.T) {
	t.Run("json token", func(t *testing.T) {
		if !looksLikeUserTokenCandidate(`{"sauth_json":"..."}`) {
			t.Fatal("expected json token to be detected")
		}
	})

	t.Run("cookie prefix", func(t *testing.T) {
		if !looksLikeUserTokenCandidate("cookie:{\"sauth_json\":\"...\"}") {
			t.Fatal("expected cookie-prefixed token to be detected")
		}
	})

	t.Run("legacy token", func(t *testing.T) {
		if !looksLikeUserTokenCandidate("w9/abcdefg") {
			t.Fatal("expected legacy token to be detected")
		}
	})

	t.Run("plain username", func(t *testing.T) {
		if looksLikeUserTokenCandidate("demo-user") {
			t.Fatal("plain username should not be treated as token")
		}
	})
}
