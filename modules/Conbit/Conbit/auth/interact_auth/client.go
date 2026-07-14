package interact_auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// HumanInteract 负责和用户交互。
type HumanInteract interface {
	Query(prompt string) (string, error)
	Tell(msg string) error
}

// ResultWrap 用于传递调用结果。
type ResultWrap[T any] struct {
	Value T
	Err   string
}

func (r ResultWrap[T]) ToJson() ([]byte, error) {
	return json.Marshal(r)
}

func UnWrapResultFromJson[T any](data string) (T, error) {
	var wrap ResultWrap[T]
	if err := json.Unmarshal([]byte(data), &wrap); err != nil {
		var zero T
		return zero, err
	}
	if wrap.Err != "" {
		var zero T
		return zero, errors.New(wrap.Err)
	}
	return wrap.Value, nil
}

// wsMessage 为 WebSocket 交互消息。
type wsMessage struct {
	Type   string          `json:"Type"`
	ID     uint64          `json:"ID"`
	Method string          `json:"Method,omitempty"`
	Data   json.RawMessage `json:"Data,omitempty"`
}

// WsInteractor 管理 WebSocket 交互。
type WsInteractor struct {
	conn     *websocket.Conn
	mu       sync.Mutex
	nextID   uint64
	pending  map[uint64]chan string
	interact HumanInteract
	closed   bool
	writeMu  sync.Mutex
}

func newWsInteractor(authServer string, interact HumanInteract) (*WsInteractor, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(context.Background(), authServer, http.Header{})
	if err != nil {
		return nil, err
	}

	w := &WsInteractor{
		conn:     conn,
		pending:  map[uint64]chan string{},
		interact: interact,
	}
	go w.receiveLoop()
	return w, nil
}

func (w *WsInteractor) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.conn.Close()
}

func (w *WsInteractor) call(method string, args map[string]interface{}) (string, error) {
	w.mu.Lock()
	w.nextID++
	id := w.nextID
	ch := make(chan string, 1)
	w.pending[id] = ch
	w.mu.Unlock()

	payload, err := json.Marshal(args)
	if err != nil {
		return "", err
	}

	w.writeMu.Lock()
	err = w.conn.WriteJSON(wsMessage{
		Type:   "Call",
		ID:     id,
		Method: method,
		Data:   payload,
	})
	w.writeMu.Unlock()
	if err != nil {
		return "", err
	}

	result := <-ch
	return result, nil
}

func (w *WsInteractor) receiveLoop() {
	for {
		var msg wsMessage
		if err := w.conn.ReadJSON(&msg); err != nil {
			if w.closed {
				return
			}
			panic(err)
		}

		switch msg.Type {
		case "Result":
			w.mu.Lock()
			ch := w.pending[msg.ID]
			delete(w.pending, msg.ID)
			w.mu.Unlock()
			if ch != nil {
				ch <- string(msg.Data)
			}
		case "Interact":
			var content string
			_ = json.Unmarshal(msg.Data, &content)
			if msg.Method == "Tell" {
				_ = w.interact.Tell(content)
				continue
			}
			answer, err := w.interact.Query(content)
			result := ResultWrap[string]{Value: answer}
			if err != nil {
				result.Err = err.Error()
			}
			resp, _ := result.ToJson()
			w.writeMu.Lock()
			_ = w.conn.WriteJSON(wsMessage{
				Type: "InteractResponse",
				ID:   msg.ID,
				Data: resp,
			})
			w.writeMu.Unlock()
		}
	}
}

// Account 描述远程账户操作集合。
type Account interface {
	ChangeUserName(newName string) error
	GetCurrentUsingMod() (*UsingMod, error)
	GetLauncherLevel() (int, int, int, error)
	GetMCPCheckNum(arg1, arg2, arg3 string) (string, error)
	LoginRentalServer(arg1, arg2, arg3 string) (string, string, error)
	TransferStartType(startType string) (string, error)
}

// RemoteAccount 是 Account 的默认实现。
type RemoteAccount struct {
	AccountID  string
	interactor *WsInteractor
}

func (a RemoteAccount) ChangeUserName(newName string) error {
	_, err := accountMethod[string](a, "ChangeUserName", []string{newName})
	return err
}

func (a RemoteAccount) GetCurrentUsingMod() (*UsingMod, error) {
	return accountMethod[*UsingMod](a, "GetCurrentUsingMod", nil)
}

func (a RemoteAccount) GetLauncherLevel() (int, int, int, error) {
	values, err := accountMethod[[]int](a, "GetLauncherLevel", nil)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(values) < 3 {
		return 0, 0, 0, fmt.Errorf("invalid launcher level")
	}
	return values[0], values[1], values[2], nil
}

func (a RemoteAccount) GetMCPCheckNum(arg1, arg2, arg3 string) (string, error) {
	return accountMethod[string](a, "GetMCPCheckNum", []string{arg1, arg2, arg3})
}

func (a RemoteAccount) LoginRentalServer(arg1, arg2, arg3 string) (string, string, error) {
	values, err := accountMethod[[]string](a, "LoginRentalServer", []string{arg1, arg2, arg3})
	if err != nil {
		return "", "", err
	}
	if len(values) < 2 {
		return "", "", fmt.Errorf("invalid login response")
	}
	return values[0], values[1], nil
}

func (a RemoteAccount) TransferStartType(startType string) (string, error) {
	return accountMethod[string](a, "TransferStartType", []string{startType})
}

// UsingMod 代表当前使用的模组信息。
type UsingMod struct {
	Items map[string]*struct {
		ConfigUUID  string `json:"ConfigUUID,omitempty"`
		OutfitLevel *int   `json:"OutfitLevel,omitempty"`
	}
}

func (m *UsingMod) GetConfigUUID2OutfitLevel() map[string]int {
	result := map[string]int{}
	if m == nil {
		return result
	}
	for key, item := range m.Items {
		if item == nil || item.OutfitLevel == nil {
			result[key] = 0
			continue
		}
		if *item.OutfitLevel == 0 {
			result[key] = 2
			continue
		}
		if *item.OutfitLevel == 1 {
			result[key] = 1
			continue
		}
		result[key] = 0
	}
	return result
}

// AccountRet 是 UseAccount/LoginOnce 的结果。
type AccountRet struct {
	UpdatedFile []byte
	AccountID   string
}

func UseAccount(authServer string, accountFile []byte, interact HumanInteract) (Account, error) {
	ws, err := newWsInteractor(authServer, interact)
	if err != nil {
		return nil, err
	}
	result, err := ws.call("UseAccount", map[string]interface{}{
		"AccountFile": accountFile,
	})
	if err != nil {
		return nil, err
	}
	ret, err := UnWrapResultFromJson[AccountRet](result)
	if err != nil {
		return nil, err
	}
	return RemoteAccount{AccountID: ret.AccountID, interactor: ws}, nil
}

func LoginOnce(authServer string, accountFile []byte, interact HumanInteract) error {
	ws, err := newWsInteractor(authServer, interact)
	if err != nil {
		return err
	}
	_, err = ws.call("UseAccount", map[string]interface{}{
		"AccountFile": accountFile,
	})
	_ = ws.Close()
	return err
}

// AccessWrapper 为外部调用提供统一接口。
type AccessWrapper struct {
	Account Account
}

const accessPointGrowthLevel = 50

func (w *AccessWrapper) GetAccess(_ context.Context, serverCode, translationServer, serverPasscode string, clientPublicKey []byte) (map[string]interface{}, error) {
	if w.Account == nil {
		return nil, fmt.Errorf("nil account")
	}

	encodedKey := base64.StdEncoding.EncodeToString(clientPublicKey)
	ip, msg, err := w.Account.LoginRentalServer(serverCode, serverPasscode, encodedKey)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"ip_address": ip,
		"server_msg": msg,
	}

	if mod, err := w.Account.GetCurrentUsingMod(); err == nil {
		result["chainInfo"] = mod.GetConfigUUID2OutfitLevel()
	}

	result["growth_level"] = float64(accessPointGrowthLevel)

	result["res_url"] = translationServer
	result["is_slim"] = false
	result["entity_id"] = serverCode
	result["skin_info"] = serverPasscode

	return result, nil
}

// accountMethod 统一封装账户方法调用。
func accountMethod[T any](account RemoteAccount, method string, args []string) (T, error) {
	payload := map[string]interface{}{
		"ID":     account.AccountID,
		"Method": method,
		"Args":   args,
	}

	result, err := account.interactor.call("AccountMethod", payload)
	if err != nil {
		var zero T
		return zero, err
	}
	return UnWrapResultFromJson[T](result)
}

// InteractClient 提供给 auth.Client 的兼容接口。
type InteractClient interface {
	TransferCheckNum(name, passcode, checkNum string) (string, error)
	TransferData(data string) (string, error)
}

// SimpleInteractClient 复用 Account 能力。
type SimpleInteractClient struct {
	Account Account
}

func (c *SimpleInteractClient) TransferCheckNum(name, passcode, checkNum string) (string, error) {
	return c.Account.GetMCPCheckNum(name, passcode, checkNum)
}

func (c *SimpleInteractClient) TransferData(data string) (string, error) {
	return c.Account.TransferStartType(data)
}
