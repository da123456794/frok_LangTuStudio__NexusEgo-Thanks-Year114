package restful_auth

import "context"

// NemcAccessWrapper 目前作为 AccessWrapper 的轻量封装。
type NemcAccessWrapper struct {
	Client     *Client
	AuthServer string
}

func (w *NemcAccessWrapper) GetAccess(ctx context.Context, serverCode, translationServer, serverPasscode string, clientPublicKey []byte) (map[string]interface{}, error) {
	wrapper := &AccessWrapper{
		Client:     w.Client,
		AuthServer: w.AuthServer,
	}
	return wrapper.GetAccess(ctx, serverCode, translationServer, serverPasscode, clientPublicKey)
}
