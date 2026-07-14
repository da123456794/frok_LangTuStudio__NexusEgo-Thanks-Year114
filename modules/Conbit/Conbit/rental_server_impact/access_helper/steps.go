package access_helper

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	tanauth "github.com/Happy2018new/nemc-tan-lobby-solver/bunker"
	"github.com/Happy2018new/nemc-tan-lobby-solver/protocol/service"
	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/fbauth"
	"github.com/LangTuStudio/Conbit/Conbit/minecraft_conn"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/challenges"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/minecraft/lang"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/minecraft_neo/cascade_conn/base_net"
	"github.com/LangTuStudio/Conbit/minecraft_neo/cascade_conn/byte_frame_conn"
	"github.com/LangTuStudio/Conbit/minecraft_neo/cascade_conn/packet_conn"
	"github.com/LangTuStudio/Conbit/minecraft_neo/login_and_spawn_core"
	"github.com/LangTuStudio/Conbit/minecraft_neo/login_and_spawn_core/options"
)

const accessPointGrowthLevel = 50

// Copied from phoenixbuilder/core/core
// func initializeMinecraftConnection(ctx context.Context, authenticator minecraft.Authenticator) (conn *minecraft.Conn, err error) {
// 	dialer := minecraft.Dialer{
// 		Authenticator: authenticator,
// 	}
// 	conn, err = dialer.DialContext(ctx, "raknet")
// 	if err != nil {
// 		return
// 	}

//		return
//	}

type tanLobbyAuthenticatorProvider interface {
	TanLobbyAuthenticator() tanauth.Authenticator
}

func loginAuthServer(ctx context.Context, authenticator Authenticator) (privateKey *ecdsa.PrivateKey, authResp map[string]any, err error) {
	infoLine(i18n.T(i18n.S_generating_client_key_pair))
	privateKey, _ = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	publicKey, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	infoLine(i18n.T(i18n.S_retrieving_client_information_from_auth_server))
	authResp, err = authenticator.GetAccess(ctx, publicKey)
	if err != nil {
		return nil, nil, err
	}
	serverMessage, _ := authResp["server_msg"].(string)
	if len(serverMessage) > 0 {
		noticeLine(i18n.T(i18n.S_message_from_auth_server) + " " + strings.TrimSpace(serverMessage))
	}
	return privateKey, authResp, nil
}

func loginMCServer(ctx context.Context, privateKey *ecdsa.PrivateKey, authResp map[string]any, targetNames serverTargetLogNames) (conn minecraft_conn.Conn, err error) {
	address, _ := authResp["ip_address"].(string)

	infoLine(i18n.T(i18n.S_establishing_raknet_connection))
	rakNetConn, err := base_net.RakNet.DialContext(ctx, address)
	if err != nil {
		return nil, err
	}

	infoLine(i18n.T(i18n.S_establishing_byte_frame_connection))
	byteFrameConn := byte_frame_conn.NewConnectionFromNet(rakNetConn)

	infoLine(i18n.T(i18n.S_generating_key_login_request))
	opt := options.NewDefaultOptions(address, authResp, privateKey)
	opt.ServerKindName = targetNames.minecraftKind

	return loginWithByteFrameConn(ctx, byteFrameConn, opt, authResp)
}

func loginTanLobbyMCServer(ctx context.Context, authenticator Authenticator, serverCode, serverPassword string, targetNames serverTargetLogNames) (conn minecraft_conn.Conn, err error) {
	provider, ok := authenticator.(tanLobbyAuthenticatorProvider)
	if !ok {
		return nil, fmt.Errorf("authenticator does not support TanLobby")
	}
	roomID := strings.TrimPrefix(normalizeServerTargetColon(serverCode), "TanLobby:")
	infoLine(i18n.T(i18n.S_establishing_raknet_connection))
	netConn, tanLobbyLoginResp, err := service.Dial(roomID, serverPassword, provider.TanLobbyAuthenticator())
	if err != nil {
		return nil, err
	}

	infoLine(i18n.T(i18n.S_establishing_byte_frame_connection))
	byteFrameConn := byte_frame_conn.NewConnectionFromNetWithoutHeader(newWaitClosedNetConn(netConn))

	infoLine(i18n.T(i18n.S_generating_key_login_request))
	privateKey, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	authResp := tanLobbyAuthRespToMap(tanLobbyLoginResp)
	opt := options.NewTanLobbyOptions(
		tanLobbyLoginResp.RaknetServerAddress,
		authResp,
		privateKey,
		tanLobbyLoginResp.UserUniqueID,
		tanLobbyLoginResp.UserPlayerName,
	)
	opt.ServerKindName = targetNames.minecraftKind
	return loginWithByteFrameConn(ctx, byteFrameConn, opt, authResp)
}

func tanLobbyAuthRespToMap(tanLobbyLoginResp tanauth.TanLobbyLoginResponse) map[string]any {
	outfitInfo := make(map[string]any, len(tanLobbyLoginResp.BotComponent))
	for uuid, level := range tanLobbyLoginResp.BotComponent {
		if level == nil {
			outfitInfo[uuid] = nil
			continue
		}
		outfitInfo[uuid] = *level
	}
	return map[string]any{
		"growth_level": float64(accessPointGrowthLevel),
		"skin_info": map[string]any{
			"entity_id": tanLobbyLoginResp.BotSkin.ItemID,
			"res_url":   tanLobbyLoginResp.BotSkin.SkinDownloadURL,
			"is_slim":   tanLobbyLoginResp.BotSkin.SkinIsSlim,
		},
		"outfit_info": outfitInfo,
	}
}

func loginWithByteFrameConn(ctx context.Context, byteFrameConn *byte_frame_conn.ByteFrameConnection, opt *options.Options, authResp map[string]any) (minecraft_conn.Conn, error) {
	infoLine(i18n.T(i18n.S_generating_packet_connection))
	packetConn := packet_conn.NewPacketConn(byteFrameConn, false)

	readQueue := NewInfinityQueue()
	loginAndSpawnCore := login_and_spawn_core.NewLoginAndSpawnCore(packetConn, opt)
	go packetConn.ListenRoutine(func(pk packet.Packet, raw []byte) {
		// fmt.Println("read:", pk.ID())
		loginAndSpawnCore.Receive(pk)
		readQueue.PutPacket(pk, raw)
	})
	err := loginAndSpawnCore.Login(ctx)
	if err != nil {
		return nil, err
	}
	infoLine(i18n.T(i18n.S_login_accomplished))

	infoLine(i18n.T(i18n.S_sending_additional_information))
	packetConn.WritePacket(&packet.ClientCacheStatus{
		Enabled: false,
	})
	packetConn.WritePacket(&packet.NeteaseJson{
		Data: []byte(fmt.Sprintf(`{"eventName":"LOGIN_UID","resid":"","uid":"%s"}`, strconv.FormatInt(opt.IdentityData.Uid, 10))),
	})
	// conn.WritePacket(&packet.PyRpc{
	// 	Value: py_rpc.FromGo([]any{
	// 		"e",
	// 		[]any{},
	// 		nil,
	// 	}),
	// })
	outfitInfo, _ := authResp["outfit_info"].(map[string]any)
	usingModList := []string{}
	for uuid, level := range outfitInfo {
		usingModList = append(usingModList, uuid)
		if level == nil {
			delete(outfitInfo, uuid)
		}
	}
	packetConn.WritePacket(&packet.PyRpc{
		Value: []any{
			"SyncUsingMod",
			[]any{
				usingModList,
				opt.ClientData.SkinID,
				opt.ClientData.SkinItemID,
				true,
				outfitInfo,
			},
			nil,
		},
		OperationType: packet.PyRpcOperationTypeSend,
	})

	// Only this packet is necessary
	packetConn.WritePacket(&packet.PyRpc{
		Value: []any{
			"ClientLoadAddonsFinishedFromGac",
			[]any{},
			nil,
		},
		OperationType: packet.PyRpcOperationTypeSend,
	})

	// Generally, following packets are sent after "SetStartType"
	packetConn.WritePacket(&packet.PyRpc{
		Value: []any{
			"arenaGamePlayerFinishLoad",
			[]any{},
			nil,
		},
		OperationType: packet.PyRpcOperationTypeSend,
	})
	packetConn.WritePacket(&packet.PyRpc{
		Value: []any{
			"ModEventC2S",
			[]any{
				"Minecraft",
				"vipEventSystem",
				"PlayerUiInit",
				fmt.Sprintf("%d", loginAndSpawnCore.GameData().EntityUniqueID),
			},
			nil,
		},
		OperationType: packet.PyRpcOperationTypeSend,
	})
	packetConn.WritePacket(&packet.PyRpc{
		Value: []any{
			"ClientInitUIFinishedEventFromGac",
			[]any{},
			nil,
		},
		OperationType: packet.PyRpcOperationTypeSend,
	})
	packetConn.Flush()
	infoLine(i18n.T(i18n.S_packing_core))
	return &shallowWrap{
		byteFrameConn: byteFrameConn,
		PacketConn:    packetConn,
		Core:          loginAndSpawnCore,
		InfinityQueue: readQueue,
		identityData:  loginAndSpawnCore.IdentityData,
	}, nil
}

func loginMCServerWithRetry(ctx context.Context, authenticator Authenticator, serverCode, serverPassword string, retryTimesRemains int) (conn minecraft_conn.Conn, err error) {
	targetNames := ServerTargetLogNames(serverCode)
	if IsTanLobbyTarget(serverCode) {
		return loginWithRetry(retryTimesRemains, targetNames, func() (minecraft_conn.Conn, error) {
			return loginTanLobbyMCServer(ctx, authenticator, serverCode, serverPassword, targetNames)
		})
	}
	privateKey, authResp, err := loginAuthServer(ctx, authenticator)
	if err != nil {
		return nil, err
	}
	// chain info will be vaild in a short time, so it can be used to re-login
	return loginWithRetry(retryTimesRemains, targetNames, func() (minecraft_conn.Conn, error) {
		return loginMCServer(ctx, privateKey, authResp, targetNames)
	})
}

func loginWithRetry(retryTimesRemains int, targetNames serverTargetLogNames, connect func() (minecraft_conn.Conn, error)) (conn minecraft_conn.Conn, err error) {
	retryTimes := 0
	for {
		conn, err = connect()
		if err == nil {
			break
		} else {
			errorLine(lang.LangFormat(lang.LANG_ZH_CN, err.Error()))
		}
		if retryTimesRemains <= 0 {
			break
		}
		retryTimes++
		infof(i18n.T(i18n.S_fail_connecting_to_mc_server_retrying), targetNames.neteaseKind, retryTimes, targetNames.neteaseKind)
		// wait for 1s
		time.Sleep(time.Second)
		retryTimesRemains--
	}
	if err != nil {
		return nil, err
	}
	doneLine(fmt.Sprintf(i18n.T(i18n.S_done_connecting_to_mc_server), targetNames.neteaseKind))
	return conn, nil
}

func makeAuthenticatorAndChallengeSolver(options *ImpactOption, writeBackFBToken bool) (authenticator Authenticator, challengeSolver challenges.CanSolveChallenge, err error) {
	clientOptions := fbauth.MakeDefaultClientOptions()
	clientOptions.AuthServer = options.AuthServer
	infof(i18n.T(i18n.S_connecting_to_auth_server)+": %v", options.AuthServer)
	fbClient, err := fbauth.CreateClient(clientOptions)
	if err != nil {
		return nil, nil, err
	}
	challengeSolver = fbClient
	doneLine(i18n.T(i18n.S_done_connecting_to_auth_server))
	hashedPassword := ""
	if options.UserToken == "" {
		psw_sum := sha256.Sum256([]byte(options.UserPassword))
		hashedPassword = hex.EncodeToString(psw_sum[:])
	}
	authenticator = fbauth.NewAccessWrapper(fbClient, options.ServerCode, options.ServerPassword, options.UserToken, options.UserName, hashedPassword, writeBackFBToken)
	return authenticator, challengeSolver, nil
}

func copeWithRentalServerChallenge(ctx context.Context, omegaCore Conbit.MicroOmega, canSolveChallenge challenges.CanSolveChallenge) (err error) {
	infoLine(i18n.T(i18n.S_coping_with_rental_server_challenges))
	challengeSolver := challenges.NewPyRPCResponder(omegaCore, canSolveChallenge)

	err = challengeSolver.ChallengeCompete(ctx)
	if err != nil {
		return ErrFBChallengeSolvingTimeout
	}
	doneLine(i18n.T(i18n.S_done_coping_with_rental_server_challenges))
	return nil
}

func reasonWithPrivilegeStuff(ctx context.Context, omegaCore Conbit.MicroOmega, options *PrivilegeStuffOptions, targetNames serverTargetLogNames) (err error) {
	infoLine(fmt.Sprintf(i18n.T(i18n.S_checking_bot_op_permission_and_game_cheat_mode), targetNames.neteaseKind))
	helper := challenges.NewOperatorChallenge(omegaCore, func() {
		if options.OpPrivilegeRemovedCallBack != nil {
			options.OpPrivilegeRemovedCallBack()
		}
		if options.DieOnLosingOpPrivilege {
			omegaCore.CloseWithError(ErrBotOpPrivilegeRemoved)
		}
	})
	waitErr := make(chan error)
	go func() {
		waitErr <- helper.WaitForPrivilege(ctx)
	}()
	select {
	case err = <-waitErr:
	case err = <-omegaCore.WaitClosed():
	}
	if err != nil {
		return err
	}
	infoLine(fmt.Sprintf(i18n.T(i18n.S_done_checking_bot_op_permission_and_game_cheat_mode), targetNames.neteaseKind))
	return nil
}

func makeBotCreative(omegaCoreCtrl Conbit.GameCtrl) {
	waitor := make(chan struct{})
	infoLine(i18n.T(i18n.S_switching_bot_to_creative_mode))
	omegaCoreCtrl.SendWebSocketCmdNeedResponse("gamemode c @s").SetTimeout(time.Second * 3).AsyncGetResult(func(output *packet.CommandOutput, err error) {
		if err == nil && output != nil {
			doneLine(i18n.T(i18n.S_done_setting_bot_to_creative_mode))
			close(waitor)
		} else {
			panic("failed to set bot to creative mode")
		}
	})
	<-waitor
}

func disableCommandBlock(omegaCoreCtrl Conbit.GameCtrl) {
	omegaCoreCtrl.SendWOCmd("gamerule commandblocksenabled false")
	//	waitor := make(chan struct{})
	//	omegaCoreCtrl.SendPlayerCmdNeedResponse("gamerule commandblocksenabled false").AsyncGetResult(func(output *packet.CommandOutput, err error) {
	doneLine(i18n.T(i18n.S_done_setting_commandblocksenabled_false))
	//		close(waitor)
	//	})
	//
	// <-waitor
}

func waitDead(omegaCore Conbit.MicroOmega) {
	// SetTime packet will be sent by server every 256 ticks, even dodaylightcycle gamerule disabled
	threshold := time.Minute
	startTime := time.Now()
	lastReceivePacket := time.Now()
	omegaCore.GetGameListener().SetAnyPacketCallBack(func(p packet.Packet) {
		lastReceivePacket = time.Now()
	}, false)
	for {
		time.Sleep(time.Second)
		nowTime := time.Now()
		if lastReceivePacket.Add(time.Second * 5).Before(nowTime) {
			flyTime := nowTime.Sub(lastReceivePacket)
			deadTime := threshold - flyTime
			warnf(i18n.T(i18n.S_bot_no_resp_could_been_feeding_massive_data_reboot_count_down), float32(deadTime)/float32(time.Second))
			omegaCore.GetGameControl().SendWebSocketCmdOmitResponse("errorcmd")
		}
		if lastReceivePacket.Add(threshold).Before(nowTime) {
			omegaCore.CloseWithError(fmt.Errorf(i18n.T(i18n.S_no_response_after_a_long_time_bot_is_down), threshold, time.Since(startTime).Seconds()))
			break
		}
	}
}
