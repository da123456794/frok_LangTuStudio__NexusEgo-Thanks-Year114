package mainfunction

import (
	"encoding/json"
	"errors"

	conbitapi "nexus/utils/api/conbit"
	"nexus/utils/client"
	"nexus/utils/log"
	"nexus/utils/py_rpc"

	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/pterm/pterm"
)

func onPyRpc(p *packet.PyRpc, conn client.Conn, env *client.Client) {
	if p.Value == nil {
		return
	}

	content, err := py_rpc.Unmarshal(p.Value)
	if err != nil {
		if env.GameInterface == nil {
			pterm.Warning.Printf("onPyRpc: %v\n", err)
		} else {
			env.GameInterface.Output(pterm.Warning.Sprintf("onPyRpc: %v", err))
		}
		return
	}

	switch c := content.(type) {
	case *py_rpc.HeartBeat:
		c.Type = py_rpc.ClientToServerHeartBeat
		_ = conn.WritePacket(&packet.PyRpc{
			Value:         py_rpc.Marshal(c),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	case *py_rpc.StartType:
		access := env.Access
		if access == nil {
			break
		}
		resp, err := access.TransferData(c.Content)
		if err != nil {
			if errors.Is(err, conbitapi.ErrTransferHandledInternally) {
				break
			}
			log.Log.Warn("onPyRpc TransferData 失败，已忽略", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			break
		}
		c.Content = resp
		c.Type = py_rpc.StartTypeResponse
		_ = conn.WritePacket(&packet.PyRpc{
			Value:         py_rpc.Marshal(c),
			OperationType: packet.PyRpcOperationTypeSend,
		})
	case *py_rpc.GetMCPCheckNum:
		if env.GetCheckNumEverPassed {
			break
		}
		access := env.Access
		if access == nil {
			break
		}
		arg, _ := json.Marshal([]any{
			c.FirstArg,
			c.SecondArg.Arg,
			conn.GameData().EntityUniqueID,
		})
		ret, err := access.TransferCheckNum(string(arg))
		if err != nil {
			if errors.Is(err, conbitapi.ErrTransferHandledInternally) {
				env.GetCheckNumEverPassed = true
				break
			}
			log.Log.Warn("onPyRpc TransferCheckNum 失败，已忽略", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			break
		}
		retP := []any{}
		_ = json.Unmarshal([]byte(ret), &retP)
		if len(retP) > 7 {
			if ret6, ok := retP[6].(float64); ok {
				retP[6] = int64(ret6)
			}
		}
		_ = conn.WritePacket(&packet.PyRpc{
			Value:         py_rpc.Marshal(&py_rpc.SetMCPCheckNum{retP}),
			OperationType: packet.PyRpcOperationTypeSend,
		})
		env.GetCheckNumEverPassed = true
	}
}
