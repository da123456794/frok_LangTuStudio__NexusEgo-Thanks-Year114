package minecraft

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptoRand "crypto/rand"
	"fmt"
	"log/slog"
	"math"
	"net"
	"time"

	"github.com/LangTuStudio/RaaBel/core/bunker/auth"
	"github.com/LangTuStudio/RaaBel/core/minecraft/internal"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/login"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
)

func DialNetConnContext(ctx context.Context, netConn net.Conn, identityData login.IdentityData, clientData login.ClientData) (*Conn, error) {
	var d Dialer
	return d.DialNetConnContext(ctx, netConn, identityData, clientData)
}

func (d Dialer) DialNetConnContext(ctx context.Context, netConn net.Conn, identityData login.IdentityData, clientData login.ClientData) (*Conn, error) {
	key, _ := ecdsa.GenerateKey(elliptic.P384(), cryptoRand.Reader)

	if d.ErrorLog == nil {
		d.ErrorLog = slog.New(internal.DiscardHandler{})
	}
	d.ErrorLog = d.ErrorLog.With("src", "dialer")
	if d.Protocol == nil {
		d.Protocol = DefaultProtocol
	}
	if d.FlushRate == 0 {
		d.FlushRate = time.Second / 20
	}

	conn := newConn(netConn, key, d.ErrorLog, d.Protocol, d.FlushRate, false)
	conn.pool = conn.proto.Packets(false)
	conn.identityData = identityData
	conn.clientData = clientData
	conn.packetFunc = d.PacketFunc
	conn.downloadResourcePack = d.DownloadResourcePack
	conn.cacheEnabled = d.EnableClientCache
	conn.disconnectOnInvalidPacket = d.DisconnectOnInvalidPackets
	conn.disconnectOnUnknownPacket = d.DisconnectOnUnknownPackets
	conn.maxDecompressedLen = math.MaxInt

	defaultIdentityData(&conn.identityData)
	if conn.clientData.ServerAddress == "" {
		conn.clientData.ServerAddress = netConn.RemoteAddr().String()
	}
	defaultClientData(&conn.clientData, auth.AuthResponse{RentalServerIP: conn.clientData.ServerAddress})
	setAndroidData(&conn.clientData)

	request := login.EncodeOffline(conn.identityData, conn.clientData, key, false)
	parsedIdentityData, _, _, err := login.Parse(request)
	if err != nil {
		return nil, fmt.Errorf("dial net conn: parse identity data: %w", err)
	}
	conn.identityData = parsedIdentityData

	readyForLogin, connected := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancelCause(ctx)
	go listenConn(conn, readyForLogin, connected, cancel)

	conn.expect(packet.IDNetworkSettings, packet.IDPlayStatus)
	if err := conn.WritePacket(&packet.RequestNetworkSettings{ClientProtocol: d.Protocol.ID()}); err != nil {
		return nil, conn.wrap(fmt.Errorf("send request network settings: %w", err), "dial net conn")
	}
	_ = conn.Flush()

	select {
	case <-ctx.Done():
		return nil, conn.wrap(context.Cause(ctx), "dial net conn")
	case <-conn.ctx.Done():
		return nil, conn.closeErr("dial net conn")
	case <-readyForLogin:
		conn.expect(packet.IDServerToClientHandshake, packet.IDPlayStatus)
		if err := conn.WritePacket(&packet.Login{ConnectionRequest: request, ClientProtocol: d.Protocol.ID()}); err != nil {
			return nil, conn.wrap(fmt.Errorf("send login: %w", err), "dial net conn")
		}
		_ = conn.Flush()

		select {
		case <-ctx.Done():
			return nil, conn.wrap(context.Cause(ctx), "dial net conn")
		case <-conn.ctx.Done():
			return nil, conn.closeErr("dial net conn")
		case <-connected:
			return conn, nil
		}
	}
}
