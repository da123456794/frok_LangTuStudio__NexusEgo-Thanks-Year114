package access_helper

import "strings"

const (
	domainGamePrefix         = "DomainGame"
	pcDomainGamePrefix       = "PCDomainGame"
	networkGamePrefix        = "NetworkGame"
	tanLobbyPrefix           = "TanLobby"
	lobbyGamePrefix          = "LobbyGame"
	chineseDomainPrefix      = "山头"
	chineseTanLobbyPrefix    = "本地联机"
	chineseLobbyGamePrefix   = "联机大厅"
	chineseLobbyPrefix       = "大厅"
	chineseNetworkGamePrefix = "网络游戏"
)

// NormalizeImpactOptionServerTarget 将接入目标统一成认证服务可识别的形式。
func NormalizeImpactOptionServerTarget(option *ImpactOption) {
	if option == nil {
		return
	}
	option.ServerCode, option.ServerPassword = NormalizeServerTarget(option.ServerCode, option.ServerPassword)
}

func IsDomainGameTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && (prefix == domainGamePrefix || prefix == pcDomainGamePrefix)
}

func IsTanLobbyTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && prefix == tanLobbyPrefix
}

func IsLobbyGameTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && prefix == lobbyGamePrefix
}

func IsNetworkGameTarget(serverCode string) bool {
	prefix, _, ok := splitServerTarget(serverCode)
	return ok && prefix == networkGamePrefix
}

func IsOnlineGameTarget(serverCode string) bool {
	return IsTanLobbyTarget(serverCode) || IsLobbyGameTarget(serverCode) || IsNetworkGameTarget(serverCode)
}

type serverTargetLogNames struct {
	neteaseKind   string
	minecraftKind string
	codeLabel     string
}

func ServerTargetLogNames(serverCode string) serverTargetLogNames {
	if IsDomainGameTarget(serverCode) {
		return serverTargetLogNames{
			neteaseKind:   "山头",
			minecraftKind: "山头",
			codeLabel:     "邀请码 :",
		}
	}
	if IsTanLobbyTarget(serverCode) {
		return serverTargetLogNames{
			neteaseKind:   "本地联机",
			minecraftKind: "本地联机",
			codeLabel:     "房间号 :",
		}
	}
	if IsLobbyGameTarget(serverCode) {
		return serverTargetLogNames{
			neteaseKind:   "联机大厅",
			minecraftKind: "联机大厅",
			codeLabel:     "房间号 :",
		}
	}
	if IsNetworkGameTarget(serverCode) {
		return serverTargetLogNames{
			neteaseKind:   "网络游戏",
			minecraftKind: "网络游戏",
			codeLabel:     "房间号 :",
		}
	}
	return serverTargetLogNames{
		neteaseKind:   "租赁服",
		minecraftKind: "服务器",
		codeLabel:     "服号: ",
	}
}

func serverTargetValueForLog(serverCode string) string {
	_, value, ok := splitServerTarget(serverCode)
	if !ok {
		return strings.TrimSpace(serverCode)
	}
	if value == "" {
		return strings.TrimSpace(serverCode)
	}
	return value
}

// NormalizeServerTarget 兼容传统租赁服号、山头邀请码、本地联机和联机大厅入口。
// 未显式带前缀且不是 4-8 位纯数字时，默认视为 DomainGame 邀请码。
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
		case chineseTanLobbyPrefix:
			return joinServerTarget(tanLobbyPrefix, value), serverPassword
		case chineseLobbyGamePrefix, chineseLobbyPrefix:
			return joinServerTarget(lobbyGamePrefix, value), serverPassword
		case chineseNetworkGamePrefix:
			return joinServerTarget(networkGamePrefix, value), serverPassword
		case domainGamePrefix, pcDomainGamePrefix:
			return joinServerTarget(prefix, value), ""
		case networkGamePrefix, tanLobbyPrefix, lobbyGamePrefix:
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
	return strings.ReplaceAll(strings.TrimSpace(serverCode), "：", ":")
}

func splitServerTarget(serverCode string) (prefix, value string, ok bool) {
	serverCode = normalizeServerTargetColon(serverCode)
	prefix, value, ok = strings.Cut(serverCode, ":")
	return strings.TrimSpace(prefix), strings.TrimSpace(value), ok
}

func joinServerTarget(prefix, value string) string {
	return prefix + ":" + strings.TrimSpace(value)
}
