package client

import (
	"strings"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/google/uuid"
)

type cachedPlayerName struct {
	XUID           string
	PlatformChatID string
	Name           string
}

// UpdatePlayerNameCache 维护在线玩家真实用户名缓存，供聊天显示名异常时回查。
func (client *Client) UpdatePlayerNameCache(entries []protocol.PlayerListEntry, remove bool) {
	if client == nil {
		return
	}

	client.playerNamesMu.Lock()
	defer client.playerNamesMu.Unlock()
	client.ensurePlayerNameCacheLocked()

	for _, entry := range entries {
		key := entry.UUID.String()
		if remove {
			client.removePlayerNameLocked(key)
			continue
		}

		name := normalizeCachedPlayerName(entry.Username)
		if name == "" {
			continue
		}
		client.removePlayerNameLocked(key)
		cached := cachedPlayerName{
			XUID:           strings.TrimSpace(entry.XUID),
			PlatformChatID: strings.TrimSpace(entry.PlatformChatID),
			Name:           name,
		}
		client.playerNamesByUUID[key] = cached
		if cached.XUID != "" {
			client.playerNamesByXUID[cached.XUID] = name
		}
		if cached.PlatformChatID != "" {
			client.playerNamesByChat[cached.PlatformChatID] = name
		}
		client.playerNamesByName[strings.ToLower(name)] = name
	}
}

// ResolvePlayerName 优先通过 XUID 精确解析聊天发送者，失败后再使用显示名兜底。
func (client *Client) ResolvePlayerName(xuid, platformChatID, rawName string) string {
	if client == nil {
		return normalizeCachedPlayerName(rawName)
	}

	xuid = strings.TrimSpace(xuid)
	platformChatID = strings.TrimSpace(platformChatID)
	client.playerNamesMu.RLock()
	if xuid != "" {
		if name := client.playerNamesByXUID[xuid]; name != "" {
			client.playerNamesMu.RUnlock()
			return name
		}
	}
	if platformChatID != "" {
		if name := client.playerNamesByChat[platformChatID]; name != "" {
			client.playerNamesMu.RUnlock()
			return name
		}
	}
	candidate := normalizeCachedPlayerName(rawName)
	if candidate != "" {
		if name := client.playerNamesByName[strings.ToLower(candidate)]; name != "" {
			client.playerNamesMu.RUnlock()
			return name
		}
		if name := client.matchCachedPlayerNameSuffixLocked(candidate); name != "" {
			client.playerNamesMu.RUnlock()
			return name
		}
	}
	client.playerNamesMu.RUnlock()
	return candidate
}

func (client *Client) ensurePlayerNameCacheLocked() {
	if client.playerNamesByXUID == nil {
		client.playerNamesByXUID = make(map[string]string)
	}
	if client.playerNamesByChat == nil {
		client.playerNamesByChat = make(map[string]string)
	}
	if client.playerNamesByName == nil {
		client.playerNamesByName = make(map[string]string)
	}
	if client.playerNamesByUUID == nil {
		client.playerNamesByUUID = make(map[string]cachedPlayerName)
	}
}

func (client *Client) removePlayerNameLocked(uuidKey string) {
	if uuidKey == "" || uuidKey == uuid.Nil.String() {
		return
	}
	cached, ok := client.playerNamesByUUID[uuidKey]
	if !ok {
		return
	}
	delete(client.playerNamesByUUID, uuidKey)
	if cached.XUID != "" {
		delete(client.playerNamesByXUID, cached.XUID)
	}
	if cached.PlatformChatID != "" {
		delete(client.playerNamesByChat, cached.PlatformChatID)
	}
	if cached.Name != "" {
		delete(client.playerNamesByName, strings.ToLower(cached.Name))
	}
}

func (client *Client) matchCachedPlayerNameSuffixLocked(candidate string) string {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	if candidate == "" {
		return ""
	}

	var matched string
	for _, cached := range client.playerNamesByUUID {
		name := strings.ToLower(cached.Name)
		if name == "" || !strings.HasSuffix(candidate, name) {
			continue
		}
		prefix := strings.TrimSuffix(candidate, name)
		if strings.TrimSpace(prefix) != "" {
			runes := []rune(prefix)
			if !isPlayerNameBoundary(runes[len(runes)-1]) {
				continue
			}
		}
		if matched != "" && !strings.EqualFold(matched, cached.Name) {
			return ""
		}
		matched = cached.Name
	}
	return matched
}

func isPlayerNameBoundary(r rune) bool {
	return r == ' ' || r == '<' || r == '>' || r == '[' || r == ']' ||
		r == '(' || r == ')' || r == ':' || r == '：' || r == '|' ||
		r == '/' || r == '\\' || r == '-' || r == '_' || r == '·'
}

func normalizeCachedPlayerName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var cleaned []rune
	skipFormat := false
	for _, r := range raw {
		if skipFormat {
			skipFormat = false
			continue
		}
		if r == '§' {
			skipFormat = true
			continue
		}
		if r == '\n' || r == '\r' || r == '\t' {
			continue
		}
		cleaned = append(cleaned, r)
	}
	return strings.TrimSpace(string(cleaned))
}

