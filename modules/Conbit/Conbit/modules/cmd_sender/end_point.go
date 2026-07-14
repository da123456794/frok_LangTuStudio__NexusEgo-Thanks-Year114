package cmd_sender

import (
	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/LangTuStudio/Conbit/nodes/defines"
	"github.com/LangTuStudio/Conbit/utils/async_wrapper"

	"github.com/google/uuid"
)

func init() {
	if false {
		func(sender Conbit.CmdSender) {}(&EndPointCmdSender{})
	}
}

type EndPointCmdSender struct {
	*CmdSenderBasic
	node defines.APINode
}

func NewEndPointCmdSender(node defines.APINode, reactable Conbit.ReactCore, interactable Conbit.InteractCore) Conbit.CmdSender {
	c := &EndPointCmdSender{
		CmdSenderBasic: NewCmdSenderBasic(reactable, interactable),
		node:           node,
	}
	return c
}

func (c *EndPointCmdSender) SendPlayerCmdNeedResponse(cmd string) async_wrapper.AsyncResult[*packet.CommandOutput] {
	ud, _ := uuid.NewUUID()
	args := defines.FromString(cmd).Extend(defines.FromUUID(ud))
	return async_wrapper.NewAsyncWrapper(func(ac *async_wrapper.AsyncController[*packet.CommandOutput]) {
		c.cbByUUID.Set(ud.String(), func(co *packet.CommandOutput) {
			ac.SetResult(co)
		})
		ac.SetCancelHook(func() {
			c.cbByUUID.Delete(ud.String())
		})
		c.node.CallOmitResponse("send-player-command", args)
	}, false)
}

func (c *EndPointCmdSender) SendAICommandNeedResponse(runtimeid string, cmd string) async_wrapper.AsyncResult[*packet.CommandOutput] {
	ud, _ := uuid.NewUUID()
	args := defines.FromString(runtimeid).Extend(defines.FromString(cmd), defines.FromUUID(ud))
	return async_wrapper.NewAsyncWrapper(func(ac *async_wrapper.AsyncController[*packet.CommandOutput]) {
		c.cbByUUID.Set(ud.String(), func(co *packet.CommandOutput) {
			ac.SetResult(co)
		})
		ac.SetCancelHook(func() {
			c.cbByUUID.Delete(ud.String())
		})
		c.node.CallOmitResponse("send-ai-command", args)
	}, false)
}
