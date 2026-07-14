package player_interact

import (
	"fmt"
	"sync"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"

	"github.com/google/uuid"
)

func init() {
	if false {
		func(playerKit Conbit.PlayerInteract) {}(&PlayerInteract{})
	}
}

type PlayerInteract struct {
	playersUQ             Conbit.PlayersInfoHolder
	botBasicUQ            Conbit.BotBasicInfoHolder
	cmdSender             Conbit.CmdSender
	info                  Conbit.InfoSender
	gameIntractable       Conbit.GameIntractable
	chatCbs               []func(chat *Conbit.GameChat)
	commandBlockTellCbs   map[string][]func(*Conbit.GameChat)
	nextMsgListenerChan   map[string]chan *Conbit.GameChat
	specificItemMsgCbs    map[string][]func(*Conbit.GameChat)
	playerChangeListeners []func(Conbit.PlayerKit, string)
	cachedPlayers         map[uuid.UUID]Conbit.PlayerKit
	mu                    sync.Mutex
}

func NewPlayerInteract(
	reactCore Conbit.ReactCore,
	playersUQ Conbit.PlayersInfoHolder,
	botBasicUQ Conbit.BotBasicInfoHolder,
	cmdSender Conbit.CmdSender,
	info Conbit.InfoSender,
	gameIntractable Conbit.GameIntractable,
) Conbit.PlayerInteract {
	i := &PlayerInteract{
		playersUQ:             playersUQ,
		botBasicUQ:            botBasicUQ,
		cmdSender:             cmdSender,
		info:                  info,
		gameIntractable:       gameIntractable,
		chatCbs:               make([]func(chat *Conbit.GameChat), 0),
		commandBlockTellCbs:   make(map[string][]func(*Conbit.GameChat)),
		nextMsgListenerChan:   make(map[string]chan *Conbit.GameChat),
		specificItemMsgCbs:    make(map[string][]func(*Conbit.GameChat)),
		cachedPlayers:         make(map[uuid.UUID]Conbit.PlayerKit),
		playerChangeListeners: make([]func(Conbit.PlayerKit, string), 0),
		mu:                    sync.Mutex{},
	}
	reactCore.SetTypedPacketCallBack(packet.IDText, func(p packet.Packet) {
		i.onTextPacket(p.(*packet.Text))
	}, true)
	reactCore.SetTypedPacketCallBack(packet.IDPlayerList, func(p packet.Packet) {
		i.onPlayerList(p.(*packet.PlayerList))
	}, true)
	for _, player := range playersUQ.GetAllOnlinePlayers() {
		uuid, _ := player.GetUUID()
		name, _ := player.GetUsername()
		i.cachedPlayers[uuid] = &PlayerKit{player, name, i}
	}
	return i
}

func (i *PlayerInteract) onPlayerList(pk *packet.PlayerList) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if pk.ActionType == packet.PlayerListActionAdd {
		for _, entry := range pk.Entries {
			i.onAddPlayer(entry.UUID)
		}
	} else {
		for _, entry := range pk.Entries {
			i.onRemovePlayer(entry.UUID)
		}
	}

}

func (i *PlayerInteract) onAddPlayer(uid uuid.UUID) {
	player, found := i.playersUQ.GetPlayerByUUID(uid)
	if !found {
		fmt.Printf("player not found: %s", uid.String())
	}
	name, _ := player.GetUsername()
	i.cachedPlayers[uid] = &PlayerKit{player, name, i}
	for _, cb := range i.playerChangeListeners {
		go cb(i.cachedPlayers[uid], "online")
	}
}

func (i *PlayerInteract) onRemovePlayer(uid uuid.UUID) {
	player, found := i.cachedPlayers[uid]
	if !found {
		return
	}
	name, found := player.GetUsername()
	if found {
		if i.nextMsgListenerChan[name] != nil {
			close(i.nextMsgListenerChan[name])
			delete(i.nextMsgListenerChan, name)
		}
	}
	for _, cb := range i.playerChangeListeners {
		go cb(player, "offline")
	}
	delete(i.cachedPlayers, uid)
}

func (i *PlayerInteract) ListenPlayerChange(cb func(player Conbit.PlayerKit, action string)) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.playerChangeListeners = append(i.playerChangeListeners, cb)
	func() {
		tmp := make([]Conbit.PlayerKit, 0, len(i.cachedPlayers))
		for _, player := range i.cachedPlayers {
			tmp = append(tmp, player)
		}
		for _, player := range tmp {
			go cb(player, "exist")
		}
	}()
}

func (i *PlayerInteract) ListAllPlayers() []Conbit.PlayerKit {
	players := make([]Conbit.PlayerKit, 0)
	for _, player := range i.playersUQ.GetAllOnlinePlayers() {
		name, _ := player.GetUsername()
		players = append(players, &PlayerKit{player, name, i})
	}
	return players
}

func (i *PlayerInteract) GetPlayerKit(name string) (playerKit Conbit.PlayerKit, found bool) {
	player, found := i.playersUQ.GetPlayerByName(name)
	if !found {
		return nil, false
	}
	return &PlayerKit{player, name, i}, true
}

func (i *PlayerInteract) GetPlayerKitByUUID(uuid uuid.UUID) (playerKit Conbit.PlayerKit, found bool) {
	player, found := i.playersUQ.GetPlayerByUUID(uuid)
	if !found {
		return nil, false
	}
	name, _ := player.GetUsername()
	return &PlayerKit{player, name, i}, true
}

func (i *PlayerInteract) GetPlayerKitByUUIDString(uuidStr string) (playerKit Conbit.PlayerKit, found bool) {
	player, found := i.playersUQ.GetPlayerByUUIDString(uuidStr)
	if !found {
		return nil, false
	}
	name, _ := player.GetUsername()
	return &PlayerKit{player, name, i}, true
}

func (i *PlayerInteract) GetPlayerKitByUniqueID(uniqueID int64) (playerKit Conbit.PlayerKit, found bool) {
	player, found := i.playersUQ.GetPlayerByUniqueID(uniqueID)
	if !found {
		return nil, false
	}
	name, _ := player.GetUsername()
	return &PlayerKit{player, name, i}, true
}
