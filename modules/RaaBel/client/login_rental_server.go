package client

import (
	"context"
	"fmt"
	"time"

	"github.com/LangTuStudio/RaaBel/core/bunker/auth"
	"github.com/LangTuStudio/RaaBel/core/minecraft"
)

const loginRetryCount = 3

// LoginRentalServer ..
func LoginRentalServer(cfg Config) (client *Client, err error) {
	cfg.RentalServerCode, cfg.RentalServerPasscode = NormalizeServerTarget(cfg.RentalServerCode, cfg.RentalServerPasscode)

	totalAttempts := loginRetryCount + 1
	for attempt := 1; attempt <= totalAttempts; attempt++ {
		client, err = loginRentalServerOnce(cfg)
		if err == nil {
			return client, nil
		}
	}
	return nil, err
}

func loginRentalServerOnce(cfg Config) (client *Client, err error) {
	client, err = ConnectRentalServer(cfg)
	if err != nil {
		return nil, err
	}
	err = CompleteChallenge(client)
	if err != nil {
		return nil, fmt.Errorf("LoginRentalServer: %v", err)
	}
	return client, nil
}

// ConnectRentalServer connects to the auth server and then establishes a Minecraft connection.
func ConnectRentalServer(cfg Config) (*Client, error) {
	cfg.RentalServerCode, cfg.RentalServerPasscode = NormalizeServerTarget(cfg.RentalServerCode, cfg.RentalServerPasscode)
	useLobbyConnection := IsOnlineGameTarget(cfg.RentalServerCode)
	skipMCPCheckChallenge := ShouldSkipMCPCheckChallenge(cfg.RentalServerCode)

	authClient, err := auth.CreateClient(&auth.ClientOptions{
		AuthServer: cfg.AuthServerAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("LoginRentalServer: %v", err)
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*30)
	defer cancelFunc()

	authenticator := auth.NewAccessWrapper(
		authClient,
		cfg.RentalServerCode,
		cfg.RentalServerPasscode,
		cfg.AuthServerToken,
		"", "",
	)

	var conn *minecraft.Conn
	if useLobbyConnection {
		conn, err = openTanLobbyConnection(ctx, authenticator)
	} else {
		conn, err = openConnection(ctx, authenticator)
	}
	if err != nil {
		return nil, fmt.Errorf("LoginRentalServer: %v", err)
	}

	return &Client{
		connection:            conn,
		authClient:            authClient,
		skipMCPCheckChallenge: skipMCPCheckChallenge,
	}, nil
}

// CompleteChallenge completes the rental-server challenge.
func CompleteChallenge(client *Client) error {
	if client.skipMCPCheckChallenge {
		client.ensureCachedPacket()
		return nil
	}

	err := NewChallengeSolver(client).CopeChallenge()
	if err != nil {
		return err
	}
	return nil
}
