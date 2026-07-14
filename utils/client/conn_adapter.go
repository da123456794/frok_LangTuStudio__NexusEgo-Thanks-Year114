package client

import (
	"bytes"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	newlogin "github.com/LangTuStudio/Conbit/minecraft/protocol/login"
	oldpacket "github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	newgamedata "github.com/LangTuStudio/Conbit/minecraft_neo/game_data"
)

type Conn interface {
	GameData() newgamedata.GameData
	IdentityData() newlogin.IdentityData
	WritePacket(oldpacket.Packet) error
	ReadPacket() (oldpacket.Packet, error)
	Close() error
}

type OldPacketAlias = oldpacket.Packet

func DecodeNewPacketToOld(raw []byte, shieldID int32) (oldpacket.Packet, error) {
	buf := bytes.NewBuffer(raw)
	header := &oldpacket.Header{}
	if err := header.Read(buf); err != nil {
		return nil, err
	}
	pkFunc, ok := oldpacket.NewPool()[header.PacketID]
	if !ok {
		return &oldpacket.Unknown{PacketID: header.PacketID, Payload: buf.Bytes()}, nil
	}
	pk := pkFunc()
	reader := protocol.NewReader(buf, shieldID, false)
	defer func() {
		if rec := recover(); rec != nil {
			pk = &oldpacket.Unknown{
				PacketID: header.PacketID,
				Payload:  append([]byte(nil), buf.Bytes()...),
			}
		}
	}()
	pk.Marshal(reader)
	if buf.Len() != 0 {
		if shouldAcceptTrailingBytes(pk, buf.Bytes()) {
			return pk, nil
		}
		return &oldpacket.Unknown{
			PacketID: header.PacketID,
			Payload:  append([]byte(nil), buf.Bytes()...),
		}, nil
	}
	return pk, nil
}

func shouldAcceptTrailingBytes(pk oldpacket.Packet, unread []byte) bool {
	switch pk.(type) {
	case *oldpacket.StartGame, *oldpacket.ItemRegistry:
		return true
	case *oldpacket.ClientBoundMapItemData:
		return true
	case *oldpacket.MobArmourEquipment:
		return len(unread) == 1 && unread[0] == 0x00
	default:
		return false
	}
}

func EncodeOldPacketToNewRaw(pk oldpacket.Packet, shieldID int32) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	header := &oldpacket.Header{PacketID: pk.ID()}
	if err := header.Write(buf); err != nil {
		return nil, err
	}
	writer := protocol.NewWriter(buf, shieldID)
	pk.Marshal(writer)
	return buf.Bytes(), nil
}

