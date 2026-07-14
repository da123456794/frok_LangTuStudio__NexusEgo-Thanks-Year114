package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	tanauth "github.com/Happy2018new/nemc-tan-lobby-solver/bunker"
	I18n "github.com/LangTuStudio/RaaBel/core/bunker/i18n"
)

type AccessWrapper struct {
	ServerCode     string
	ServerPassword string
	Token          string
	Client         *Client
	Username       string
	Password       string
}

const accessPointGrowthLevel = 50

func NewAccessWrapper(Client *Client, ServerCode, ServerPassword, Token, username, password string) *AccessWrapper {
	return &AccessWrapper{
		Client:         Client,
		ServerCode:     ServerCode,
		ServerPassword: ServerPassword,
		Token:          Token,
		Username:       username,
		Password:       password,
	}
}

func (aw *AccessWrapper) GetAccess(ctx context.Context, publicKey []byte) (authResponse AuthResponse, err error) {
	pubKeyData := base64.StdEncoding.EncodeToString(publicKey)
	authResponse, err = aw.Client.Auth(ctx, aw.ServerCode, aw.ServerPassword, pubKeyData, aw.Token, aw.Username, aw.Password)
	if err != nil {
		return AuthResponse{}, err
	}
	authResponse.BotLevel = accessPointGrowthLevel
	aw.Client.GrowthLevel = accessPointGrowthLevel
	if len(authResponse.FBToken) != 0 {
		homedir, err := os.UserHomeDir()
		if err != nil {
			fmt.Println(I18n.T(I18n.Warning_UserHomeDir))
			homedir = "."
		}
		fbconfigdir := filepath.Join(homedir, ".config", "fastbuilder")
		os.MkdirAll(fbconfigdir, 0755)
		ptoken := filepath.Join(fbconfigdir, "fbtoken")
		// 0600: -rw-------
		token_file, err := os.OpenFile(ptoken, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return AuthResponse{}, err
		}
		_, err = token_file.WriteString(authResponse.FBToken)
		if err != nil {
			return AuthResponse{}, err
		}
		token_file.Close()
	}
	return
}

func (aw *AccessWrapper) TanLobbyGetAccess(roomID string) (tanauth.TanLobbyLoginResponse, error) {
	response, err := aw.Client.TanLobbyAuth(roomID, aw.Token)
	if err != nil {
		return tanauth.TanLobbyLoginResponse{}, fmt.Errorf("TanLobbyGetAccess: %v", err)
	}
	return response, nil
}

func (aw *AccessWrapper) TanLobbyGetCreate() (tanauth.TanLobbyCreateResponse, error) {
	response, err := aw.Client.TanLobbyCreate(aw.Token)
	if err != nil {
		return tanauth.TanLobbyCreateResponse{}, fmt.Errorf("TanLobbyGetCreate: %v", err)
	}
	return response, nil
}

type tanLobbyAuthenticator struct {
	aw *AccessWrapper
}

func NewTanLobbyAuthenticator(aw *AccessWrapper) tanauth.Authenticator {
	return &tanLobbyAuthenticator{aw: aw}
}

func (a *tanLobbyAuthenticator) GetAccess(roomID string) (tanauth.TanLobbyLoginResponse, error) {
	return a.aw.TanLobbyGetAccess(roomID)
}

func (a *tanLobbyAuthenticator) GetCreate() (tanauth.TanLobbyCreateResponse, error) {
	return a.aw.TanLobbyGetCreate()
}

func (a *tanLobbyAuthenticator) GetDebug(loginResponse string, raknetRand []byte) (tanauth.TanLobbyDebugResponse, error) {
	response, err := a.aw.Client.TanLobbyDebug(loginResponse, raknetRand, a.aw.Token)
	if err != nil {
		return tanauth.TanLobbyDebugResponse{}, fmt.Errorf("GetDebug: %v", err)
	}
	return response, nil
}

func (aw *AccessWrapper) TanLobbyAuthenticator() tanauth.Authenticator {
	return NewTanLobbyAuthenticator(aw)
}
