package fbauth

import "testing"

func TestNormalizeAuthServerURL(t *testing.T) {
	t.Run("accept custom http server", func(t *testing.T) {
		authServer, err := NormalizeAuthServerURL(" http://127.0.0.1:1200/ ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authServer != "http://127.0.0.1:1200" {
			t.Fatalf("unexpected auth server: %q", authServer)
		}
	})

	t.Run("accept custom https server", func(t *testing.T) {
		authServer, err := NormalizeAuthServerURL("https://auth.example.com/base")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authServer != "https://auth.example.com/base" {
			t.Fatalf("unexpected auth server: %q", authServer)
		}
	})

	t.Run("fix swapped scheme separator", func(t *testing.T) {
		authServer, err := NormalizeAuthServerURL("http//:127.0.0.1:1200")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authServer != "http://127.0.0.1:1200" {
			t.Fatalf("unexpected auth server: %q", authServer)
		}
	})

	t.Run("reject invalid scheme", func(t *testing.T) {
		if _, err := NormalizeAuthServerURL("ftp://auth.example.com"); err == nil {
			t.Fatal("expected invalid scheme to fail")
		}
	})

	t.Run("reject missing host", func(t *testing.T) {
		if _, err := NormalizeAuthServerURL("http:///api"); err == nil {
			t.Fatal("expected missing host to fail")
		}
	})
}
