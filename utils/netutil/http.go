package netutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var defaultDNSServers = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
	"223.5.5.5:53",
	"114.114.114.114:53",
}

func dnsServers() []string {
	raw := strings.TrimSpace(strings.Join([]string{
		strings.TrimSpace(getenv("NEXUS_DNS")),
		strings.TrimSpace(getenv("NEXUS_DNS_SERVERS")),
	}, ","))
	if raw == "," || raw == "" {
		return append([]string(nil), defaultDNSServers...)
	}

	parts := strings.Split(raw, ",")
	servers := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, ":") {
			part += ":53"
		}
		servers = append(servers, part)
	}
	if len(servers) == 0 {
		return append([]string(nil), defaultDNSServers...)
	}
	return servers
}

func getenv(key string) string {
	return os.Getenv(key)
}

func newResolver() *net.Resolver {
	servers := dnsServers()
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 3 * time.Second}
			network = "udp4"
			var lastErr error
			for _, server := range servers {
				conn, err := dialer.DialContext(ctx, network, server)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("no dns servers configured")
			}
			return nil, lastErr
		},
	}
}

func NewTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			Resolver:  newResolver(),
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: NewTransport(),
	}
}
