package restful_auth

import (
	"context"
	"fmt"
)

// DebugClientAccessWrapper 用于本地调试。
type DebugClientAccessWrapper struct {
	AuthServer string
}

func (w *DebugClientAccessWrapper) GetAccess(_ context.Context, _ string, _ string, _ string, _ []byte) (map[string]interface{}, error) {
	result := map[string]interface{}{}
	result["ip_address"] = w.AuthServer
	result["server_msg"] = fmt.Sprintf("警告: 您正在使用 Debug 模式 (您将前往本地的 %s 服务器)", w.AuthServer)
	return result, nil
}
