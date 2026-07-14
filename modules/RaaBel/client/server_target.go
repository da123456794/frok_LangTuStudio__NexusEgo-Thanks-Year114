package client

import "strings"

const (
	domainGamePrefix       = "DomainGame"
	pcDomainGamePrefix     = "PCDomainGame"
	networkGamePrefix      = "NetworkGame"
	lobbyGamePrefix        = "LobbyGame"
	tanLobbyPrefix         = "TanLobby"
	chineseDomainPrefix    = "\u5c71\u5934"
	chineseLobbyPrefix     = "\u5927\u5385"
	chineseLobbyGamePrefix = "\u8054\u673a\u5927\u5385"
	chineseNetworkPrefix   = "\u7f51\u7edc\u6e38\u620f"
	chineseTanLobbyPrefix  = "\u672c\u5730\u8054\u673a"
)

// NormalizeServerTarget converts human-friendly targets into auth-server targets.
func NormalizeServerTarget(serverCode, serverPassword string) (string, string) {
	serverCode = normalizeServerTargetColon(serverCode)
	serverPassword = strings.TrimSpace(serverPassword)
	if serverCode == "" {
		return "", serverPassword
	}

	if prefix, value, ok := splitServerTarget(serverCode); ok {
		switch prefix {
		case chineseDomainPrefix:
			return joinServerTarget(domainGamePrefix, value), ""
		case chineseLobbyGamePrefix, chineseLobbyPrefix:
			return joinServerTarget(lobbyGamePrefix, value), serverPassword
		case chineseNetworkPrefix:
			return joinServerTarget(networkGamePrefix, value), serverPassword
		case chineseTanLobbyPrefix:
			return joinServerTarget(tanLobbyPrefix, value), serverPassword
		case domainGamePrefix, pcDomainGamePrefix:
			return joinServerTarget(prefix, value), ""
		case networkGamePrefix:
			return joinServerTarget(prefix, value), serverPassword
		case lobbyGamePrefix:
			return joinServerTarget(prefix, value), serverPassword
		case tanLobbyPrefix:
			return joinServerTarget(prefix, value), serverPassword
		default:
			return joinServerTarget(prefix, value), serverPassword
		}
	}

	if isTraditionalRentalCode(serverCode) || serverCode == "MainCity" {
		return serverCode, serverPassword
	}

	return joinServerTarget(domainGamePrefix, serverCode), ""
}

func IsTanLobbyTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && (prefix == tanLobbyPrefix || prefix == chineseTanLobbyPrefix)
}

func IsNetworkGameTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && (prefix == networkGamePrefix || prefix == chineseNetworkPrefix)
}

func IsLobbyGameTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && (prefix == lobbyGamePrefix || prefix == chineseLobbyGamePrefix || prefix == chineseLobbyPrefix)
}

func IsOnlineGameTarget(serverCode string) bool {
	return IsTanLobbyTarget(serverCode) || IsLobbyGameTarget(serverCode) || IsNetworkGameTarget(serverCode)
}

func ShouldSkipMCPCheckChallenge(serverCode string) bool {
	serverCode, _ = NormalizeServerTarget(serverCode, "")
	return !isTraditionalRentalCode(serverCode) && serverCode != "MainCity"
}

func IsDomainGameTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && (prefix == domainGamePrefix || prefix == pcDomainGamePrefix || prefix == chineseDomainPrefix)
}

func isTraditionalRentalCode(serverCode string) bool {
	if len(serverCode) < 4 || len(serverCode) > 8 {
		return false
	}
	for _, ch := range serverCode {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func normalizeServerTargetColon(serverCode string) string {
	return strings.ReplaceAll(strings.TrimSpace(serverCode), "\uff1a", ":")
}

func splitServerTarget(serverCode string) (prefix, value string, ok bool) {
	serverCode = normalizeServerTargetColon(serverCode)
	prefix, value, ok = strings.Cut(serverCode, ":")
	return strings.TrimSpace(prefix), strings.TrimSpace(value), ok
}

func joinServerTarget(prefix, value string) string {
	return prefix + ":" + strings.TrimSpace(value)
}
