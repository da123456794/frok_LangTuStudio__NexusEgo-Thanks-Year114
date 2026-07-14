package restful_auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"

	"github.com/LangTuStudio/Conbit/i18n"
)

func i18nMsg(key string) string {
	if msg, ok := i18n.I18nDict[key]; ok {
		return msg
	}
	return key
}

// Config 用于创建 REST 认证客户端。
type Config struct {
	AuthServer string
	UID        string
	ServerCode string
	// TranslationServer 仅在需要时使用。
	TranslationServer string
}

// Client 负责与认证服务器交互。
type Client struct {
	UID               string
	AuthServer        string
	ServerCode        string
	TranslationServer string
	httpClient        http.Client
}

func (c *Client) GetUID() string {
	return c.UID
}

// secretLoadingTransport 在请求头中注入认证密钥。
type secretLoadingTransport string

func (s secretLoadingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	secret := fmt.Sprintf("Bearer %s", string(s))
	key := http.CanonicalHeaderKey("Authorization")
	req.Header.Add(key, secret)
	return http.DefaultTransport.RoundTrip(req)
}

func CreateClient(cfg *Config) (*Client, error) {
	url := fmt.Sprintf("%s/api/new", cfg.AuthServer)
	resp, err := http.DefaultClient.Get(url)
	if err != nil {
		return nil, errors.New(i18nMsg("cannot establish http connection with auth server api"))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusOK {
		secret := string(body)
		client := &Client{
			UID:               cfg.UID,
			AuthServer:        cfg.AuthServer,
			ServerCode:        cfg.ServerCode,
			TranslationServer: cfg.TranslationServer,
		}
		client.httpClient = http.Client{Transport: secretLoadingTransport(secret)}
		return client, nil
	}
	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, errors.New(i18nMsg("auth server is down (http code 503)"))
	}

	return nil, parseError(string(body))
}

func parseError(body string) error {
	reg := regexp.MustCompile(`^(\d{3} [a-zA-Z ]+)\n\n(.*?)($|\n)`)
	if match := reg.FindStringSubmatch(body); len(match) == 4 {
		return fmt.Errorf("%v: %v", match[1], match[2])
	}
	return fmt.Errorf(i18nMsg("unknown error happened in parsing auth server response: %v"), body)
}

func jsonDecodeResp(resp *http.Response) (map[string]interface{}, error) {
	if resp.StatusCode == http.StatusServiceUnavailable {
		return nil, errors.New(i18nMsg("auth server is down (http code 503)"))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseError(string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf(i18nMsg("error parsing auth server API response: %v"), err)
	}

	if ok, _ := result["success"].(bool); ok {
		return result, nil
	}
	if ok, _ := result["ok"].(bool); ok {
		return result, nil
	}
	if msg, ok := result["msg"].(string); ok {
		return nil, fmt.Errorf(i18nMsg("fail to transfer check num: %s"), msg)
	}
	if msg, ok := result["error"].(string); ok {
		return nil, fmt.Errorf(i18nMsg("fail to transfer check num: %s"), msg)
	}
	if msg, ok := result["message"].(string); ok {
		return nil, fmt.Errorf(i18nMsg("fail to transfer check num: %s"), msg)
	}
	return nil, fmt.Errorf(i18nMsg("fail to transfer check num: %s"), "")
}

func (c *Client) authWithExtra(ctx context.Context, username, loginToken, passwordHash, password, serverPasscode, clientPublicKey, serverCode, translationServer string) (map[string]interface{}, error) {
	payload := map[string]interface{}{}
	if loginToken != "" {
		payload["login_token"] = loginToken
	} else if username != "" {
		payload["username"] = username
		payload["password"] = passwordHash
	}
	if serverCode != "" {
		payload["server_code"] = serverCode
	}
	if translationServer != "" {
		payload["translationserver"] = translationServer
	}
	if serverPasscode != "" {
		payload["server_passcode"] = serverPasscode
	}
	if clientPublicKey != "" {
		payload["client_public_key"] = clientPublicKey
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/phoenix/login", c.AuthServer)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	return jsonDecodeResp(resp)
}

func (c *Client) Auth(ctx context.Context, username, loginToken, passwordHash, password, serverPasscode string) (map[string]interface{}, error) {
	return c.authWithExtra(ctx, username, loginToken, passwordHash, password, serverPasscode, "", c.ServerCode, c.TranslationServer)
}

func (c *Client) TransferCheckNum(name, passcode string, checkNum int) (string, error) {
	payload := struct {
		Name     string `json:"name"`
		Passcode string `json:"passcode"`
		CheckNum int    `json:"check_num"`
	}{
		Name:     name,
		Passcode: passcode,
		CheckNum: checkNum,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/api/phoenix/transfer_check_num", c.AuthServer)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	result, err := jsonDecodeResp(resp)
	if err != nil {
		return "", err
	}
	if value, ok := result["value"].(string); ok {
		return value, nil
	}
	return "", fmt.Errorf(i18nMsg("fail to transfer check num: %s"), "")
}

func (c *Client) TransferData(content string) (string, error) {
	url := fmt.Sprintf("%s/api/phoenix/transfer_start_type?content=%s", c.AuthServer, content)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return "", err
	}
	result, err := jsonDecodeResp(resp)
	if err != nil {
		return "", err
	}
	if value, ok := result["data"].(string); ok {
		return value, nil
	}
	if msg, ok := result["message"].(string); ok {
		return "", fmt.Errorf(i18nMsg("fail to transfer check num: %s"), msg)
	}
	return "", fmt.Errorf(i18nMsg("fail to transfer check num: %s"), "")
}

// AccessWrapper 负责组织登录访问所需的参数。
type AccessWrapper struct {
	Client         *Client
	AuthServer     string
	UserName       string
	UserToken      string
	PasswordHash   string
	Password       string
	ServerCode     string
	Translation    string
	ServerPasscode string
	SaveToken      bool
}

func (w *AccessWrapper) GetAccess(ctx context.Context, serverCode, translationServer, serverPasscode string, clientPublicKey []byte) (map[string]interface{}, error) {
	if w.Client == nil {
		return nil, fmt.Errorf("nil client")
	}
	encodedKey := base64.StdEncoding.EncodeToString(clientPublicKey)
	return w.Client.authWithExtra(
		ctx,
		w.UserName,
		w.UserToken,
		w.PasswordHash,
		w.Password,
		serverPasscode,
		encodedKey,
		serverCode,
		translationServer,
	)
}
