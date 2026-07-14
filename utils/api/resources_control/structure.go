package ResourcesControl

import (
	"fmt"
	"time"

	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

// 提交结构请求
func (m *mcstructure) WriteRequest(structureName string) {
	m.respLock.Lock()
	defer m.respLock.Unlock()
	m.expectedStructure = structureName
	m.resp = make(chan packet.StructureTemplateDataResponse, 1)
}

// 向结构请求写入返回值 resp 。
// 属于私有实现。
func (m *mcstructure) writeResponse(data packet.StructureTemplateDataResponse) bool {
	m.respLock.Lock()
	defer m.respLock.Unlock()
	if m.expectedStructure == "" {
		return false
	}
	select {
	case m.resp <- data:
	default:
	}
	close(m.resp)
	m.expectedStructure = ""
	return true
}

// 从管道读取结构请求的返回值
func (m *mcstructure) LoadResponse() packet.StructureTemplateDataResponse {
	return <-m.resp
}

// 从管道读取结构请求的返回值，并在超时后返回错误。
func (m *mcstructure) LoadResponseWithTimeout(timeout time.Duration) (packet.StructureTemplateDataResponse, error) {
	if timeout <= 0 {
		return m.LoadResponse(), nil
	}
	select {
	case resp := <-m.resp:
		return resp, nil
	case <-time.After(timeout):
		m.respLock.Lock()
		if m.expectedStructure != "" {
			m.expectedStructure = ""
		}
		m.respLock.Unlock()
		return packet.StructureTemplateDataResponse{}, fmt.Errorf("structure response timeout after %s", timeout)
	}
}

