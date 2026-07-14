package packet_conn

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/minecraft_neo/can_close"
	"github.com/LangTuStudio/Conbit/minecraft_neo/cascade_conn/defines"
)

type PacketConn struct {
	can_close.CanCloseWithError
	FrameConn                  defines.ByteFrameConn
	shieldID                   int32
	disconnectOnInvalidPackets bool
	pool                       packet.Pool
}

func shouldIgnoreTrailingBytes(pk packet.Packet, unread []byte) bool {
	switch pk.(type) {
	case *packet.StartGame, *packet.ItemRegistry:
		return true
	case *packet.ClientBoundMapItemData:
		return true
	case *packet.ContainerClose, *packet.MobArmourEquipment:
		return len(unread) == 1 && unread[0] == 0x00
	default:
		return false
	}
}

func NewPacketConn(conn defines.ByteFrameConn, disconnectOnInvalidPackets bool) defines.PacketConn {
	c := &PacketConn{
		// close underlay conn on err
		CanCloseWithError:          can_close.NewClose(conn.Close),
		FrameConn:                  conn,
		disconnectOnInvalidPackets: disconnectOnInvalidPackets,
		pool:                       packet.NewPool(),
	}
	go func() {
		// close when underlay err
		err := <-conn.WaitClosed()
		c.CloseWithError(err)
	}()
	return c
}

func (conn *PacketConn) SetShieldID(newShieldID int32) {
	conn.shieldID = newShieldID
}

func (conn *PacketConn) GetShieldID() int32 {
	return conn.shieldID
}

func (conn *PacketConn) WritePacket(pk packet.Packet) error {
	if conn.Closed() {
		return fmt.Errorf("write packet on closed connection")
	}
	buf := bytes.NewBuffer([]byte{})
	hdr := &packet.Header{}
	hdr.PacketID = pk.ID()
	_ = hdr.Write(buf)
	// for _, converted := range conn.proto.ConvertFromLatest(pk, conn) {
	pk.Marshal(protocol.NewWriter(buf, conn.shieldID))
	// conn.bufferedSend = append(conn.bufferedSend, append([]byte(nil), buf.Bytes()...))
	conn.FrameConn.WriteBytePacket(buf.Bytes())
	// }
	return nil
}

func (conn *PacketConn) decode(data []byte) (pk packet.Packet, raw []byte) {
	pkData, err := parseData(data)
	if err != nil {
		fmt.Println("packet decode err: " + err.Error())
		conn.CloseWithError(err)
		return nil, data
	}
	pkFunc, ok := conn.pool[pkData.h.PacketID]
	if !ok {

		fmt.Printf("packet decode err: unknown packet %v\n", pkData.h.PacketID)
		if conn.disconnectOnInvalidPackets {
			if conn.disconnectOnInvalidPackets {
				conn.CloseWithError(err)
			}
		}
		return nil, data
	}
	pk = pkFunc()
	r := protocol.NewReader(pkData.payload, conn.shieldID, false)
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("panic decoding %T: %v", pk, rec)
				if shouldIgnoreDecodePanic(pk, rec) {
					pk = nil
					return
				}
				_ = os.WriteFile(fmt.Sprintf("ID%v_panic.bin", pkData.h.PacketID), data, 0755)
				fmt.Println("packet decode err: " + err.Error())
				if conn.disconnectOnInvalidPackets {
					conn.CloseWithError(err)
				}
				pk = nil
			}
		}()
		pk.Marshal(r)
	}()
	if pk == nil {
		return nil, data
	}
	if pkData.payload.Len() != 0 {
		if shouldIgnoreTrailingBytes(pk, pkData.payload.Bytes()) {
			_ = pkData.payload.Next(pkData.payload.Len())
		} else {
			err = fmt.Errorf("%T: %v unread bytes left: 0x%x", pk, pkData.payload.Len(), pkData.payload.Bytes())
			os.WriteFile(fmt.Sprintf("ID%v.bin", pkData.h.PacketID), data, 0755)
			p, err2 := filepath.Abs(fmt.Sprintf("ID%v.bin", pkData.h.PacketID))
			if err2 != nil {
				p = fmt.Sprintf("ID%v.bin", pkData.h.PacketID)
			}
			fmt.Printf("sample data save in: %v\n", p)
			fmt.Println("packet decode err: " + err.Error())
			if conn.disconnectOnInvalidPackets {
				conn.CloseWithError(err)
			}
		}
	}
	return pk, data
}

func shouldIgnoreDecodePanic(pk packet.Packet, rec any) bool {
	switch pk.(type) {
	case *packet.ClientBoundMapItemData:
		return fmt.Sprint(rec) == "EOF"
	default:
		return false
	}
}

func (conn *PacketConn) ListenRoutine(read func(pk packet.Packet, raw []byte)) {
	conn.FrameConn.ReadRoutine(func(data []byte) {
		pk, raw := conn.decode(data)
		if pk == nil {
			return
		}
		// fmt.Println("decoded:", pk.ID())
		read(pk, raw)
	})
	if err := conn.FrameConn.CloseError(); err != nil {
		conn.CloseWithError(err)
		return
	}
	if err := conn.CloseError(); err != nil {
		conn.CloseWithError(err)
		return
	}
	conn.CloseWithError(fmt.Errorf("packet connection closed"))
}

func (conn *PacketConn) Flush() error {
	return conn.FrameConn.Flush()
}

func (conn *PacketConn) EnableEncryption(key [32]byte) {
	conn.FrameConn.EnableEncryption(key)
}

func (conn *PacketConn) EnableCompression(algorithm packet.Compression) {
	conn.FrameConn.EnableCompression(algorithm)
}
