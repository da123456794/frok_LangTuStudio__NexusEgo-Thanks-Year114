package client

import (
	"context"
	"fmt"
	"net"
	"time"

	tanauth "github.com/Happy2018new/nemc-tan-lobby-solver/bunker"
	tanservice "github.com/Happy2018new/nemc-tan-lobby-solver/protocol/service"
	"github.com/LangTuStudio/RaaBel/core/bunker/auth"
	"github.com/LangTuStudio/RaaBel/core/minecraft"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/login"
	"github.com/LangTuStudio/RaaBel/core/minecraft/protocol/packet"
	"github.com/LangTuStudio/RaaBel/core/py_rpc"
	cts "github.com/LangTuStudio/RaaBel/core/py_rpc/mod_event/client_to_server"
	cts_mc "github.com/LangTuStudio/RaaBel/core/py_rpc/mod_event/client_to_server/minecraft"
	cts_mc_p "github.com/LangTuStudio/RaaBel/core/py_rpc/mod_event/client_to_server/minecraft/preset"
	cts_mc_v "github.com/LangTuStudio/RaaBel/core/py_rpc/mod_event/client_to_server/minecraft/vip_event_system"
	mei "github.com/LangTuStudio/RaaBel/core/py_rpc/mod_event/interface"
	"github.com/google/uuid"
)

func openConnection(
	ctx context.Context,
	authenticator minecraft.Authenticator,
) (conn *minecraft.Conn, err error) {
	var dialer minecraft.Dialer
	var authResponse auth.AuthResponse

	dialer = minecraft.Dialer{
		Authenticator: authenticator,
	}
	conn, authResponse, err = dialer.DialContext(ctx, "raknet")
	if err != nil {
		return nil, err
	}

	err = postLogin(conn, authResponse.BotComponent)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func openTanLobbyConnection(
	ctx context.Context,
	authenticator *auth.AccessWrapper,
) (conn *minecraft.Conn, err error) {
	var (
		netConn               net.Conn
		roomCode              string
		roomID                string
		tanLobbyLoginResponse tanauth.TanLobbyLoginResponse
		identityData          login.IdentityData
		clientData            login.ClientData
	)

	roomCode = normalizeServerTargetColon(authenticator.ServerCode)
	_, roomID, _ = splitServerTarget(roomCode)
	netConn, tanLobbyLoginResponse, err = tanservice.DialContext(ctx, roomID, authenticator.ServerPassword, authenticator.TanLobbyAuthenticator())
	if err != nil {
		return nil, err
	}

	identityData = login.IdentityData{
		Uid:         int64(tanLobbyLoginResponse.UserUniqueID),
		Identity:    uuid.NewString(),
		DisplayName: tanLobbyLoginResponse.UserPlayerName,
	}
	clientData = login.ClientData{
		ServerAddress: tanLobbyLoginResponse.RaknetServerAddress,
	}
	loginCtx, cancelLogin := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelLogin()
	conn, err = minecraft.DialNetConnContext(loginCtx, netConn, identityData, clientData)
	if err != nil {
		_ = netConn.Close()
		return nil, err
	}

	err = postLogin(conn, tanLobbyLoginResponse.BotComponent)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func postLogin(conn *minecraft.Conn, botComponent map[string]*int) error {
	runtimeid := fmt.Sprintf("%d", conn.GameData().EntityUniqueID)
	conn.WritePacket(&packet.ServerBoundLoadingScreen{
		Type: packet.LoadingScreenTypeStart,
	})
	conn.WritePacket(&packet.ServerBoundLoadingScreen{
		Type: packet.LoadingScreenTypeEnd,
	})

	conn.WritePacket(&packet.ClientCacheStatus{
		Enabled: false,
	})

	conn.WritePacket(&packet.NeteaseJson{
		Data: []byte(
			fmt.Sprintf(
				`{"eventName":"LOGIN_UID","resid":"","uid":"%d"}`,
				conn.IdentityData().Uid,
			),
		),
	})

	conn.WritePacket(&packet.PyRpc{
		Value: []any{
			"e",
			[]any{},
			nil,
		},
		OperationType: packet.PyRpcOperationTypeSend,
	})

	{
		modUUIDs := make([]any, 0)
		outfitInfo := make(map[string]int64, 0)
		for modUUID, outfitType := range botComponent {
			modUUIDs = append(modUUIDs, modUUID)
			if outfitType != nil {
				outfitInfo[modUUID] = int64(*outfitType)
			}
		}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.SyncUsingMod{
				modUUIDs,
				conn.ClientData().SkinID,
				"",
				true,
				outfitInfo,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}

	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.SyncVipSkinUUID{nil}),
		OperationType: packet.PyRpcOperationTypeSend,
	})
	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.ClientLoadAddonsFinishedFromGac{}),
		OperationType: packet.PyRpcOperationTypeSend,
	})

	{
		event := cts_mc_p.GetLoadedInstances{PlayerRuntimeID: runtimeid}
		module := cts_mc.Preset{Module: &mei.DefaultModule{Event: &event}}
		park := cts.Minecraft{Default: mei.Default{Module: &module}}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.ModEvent{
				Package: &park,
				Type:    py_rpc.ModEventClientToServer,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}

	conn.WritePacket(&packet.PyRpc{
		Value:         py_rpc.Marshal(&py_rpc.ArenaGamePlayerFinishLoad{}),
		OperationType: packet.PyRpcOperationTypeSend,
	})

	{
		event := cts_mc_v.PlayerUiInit{RuntimeID: runtimeid}
		module := cts_mc.VIPEventSystem{Module: &mei.DefaultModule{Event: &event}}
		park := cts.Minecraft{Default: mei.Default{Module: &module}}
		conn.WritePacket(&packet.PyRpc{
			Value: py_rpc.Marshal(&py_rpc.ModEvent{
				Package: &park,
				Type:    py_rpc.ModEventClientToServer,
			}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	}

	conn.WritePacket(&packet.PyRpc{
		Value: []any{
			"ClientInitUIFinishedEventFromGac",
			[]any{},
			nil,
		},
		OperationType: packet.PyRpcOperationTypeSend,
	})
	conn.WritePacket(&packet.PlayerHotBar{
		SelectedHotBarSlot: 0,
		WindowID:           0,
		SelectHotBarSlot:   true,
	})

	return nil
}
