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
		func(sender Conbit.CmdSender) {}(&AccessPointCmdSender{})
	}
}

type AccessPointCmdSender struct {
	*CmdSenderBasic
}

func NewAccessPointCmdSender(node defines.APINode, reactable Conbit.ReactCore, interactable Conbit.InteractCore) Conbit.CmdSender {
	c := &AccessPointCmdSender{
		CmdSenderBasic: NewCmdSenderBasic(reactable, interactable),
	}

	node.ExposeAPI("send-player-command").InstantAPI(func(args defines.Values) (result defines.Values, err error) {
		cmd, err := args.ToString()
		if err != nil {
			return
		}
		args = args.ConsumeHead()
		ud, err := args.ToUUID()
		if err != nil {
			return
		}
		c.SendPacket(c.packCmdWithUUID(cmd, ud, true))
		return defines.Empty, nil
	})
	node.ExposeAPI("send-ai-command").InstantAPI(func(args defines.Values) (result defines.Values, err error) {
		runtimeid, err := args.ToString()
		if err != nil {
			return
		}
		args = args.ConsumeHead()
		cmd, err := args.ToString()
		if err != nil {
			return
		}
		args = args.ConsumeHead()
		ud, err := args.ToUUID()
		if err != nil {
			return
		}
		c.SendPacket(c.packAICmdWithUUID(runtimeid, cmd, ud))
		return defines.Empty, nil
	})

	return c
}

func (c *AccessPointCmdSender) SendPlayerCmdOmitResponse(cmd string) {
	ud, _ := uuid.NewUUID()
	c.SendPacket(c.packCmdWithUUID(cmd, ud, true))
}

func (c *AccessPointCmdSender) SendPlayerCmdNeedResponse(cmd string) async_wrapper.AsyncResult[*packet.CommandOutput] {
	ud, _ := uuid.NewUUID()
	pkt := c.packCmdWithUUID(cmd, ud, true)
	return async_wrapper.NewAsyncWrapper(func(ac *async_wrapper.AsyncController[*packet.CommandOutput]) {
		c.cbByUUID.Set(ud.String(), func(co *packet.CommandOutput) {
			ac.SetResult(co)
		})
		ac.SetCancelHook(func() {
			c.cbByUUID.Delete(ud.String())
		})
		c.SendPacket(pkt)
	}, false)
}

func (c *AccessPointCmdSender) SendAICommandNeedResponse(runtimeid string, cmd string) async_wrapper.AsyncResult[*packet.CommandOutput] {
	ud, _ := uuid.NewUUID()
	pkt := c.packAICmdWithUUID(runtimeid, cmd, ud)
	return async_wrapper.NewAsyncWrapper(func(ac *async_wrapper.AsyncController[*packet.CommandOutput]) {
		c.cbByUUID.Set(ud.String(), func(co *packet.CommandOutput) {
			ac.SetResult(co)
		})
		ac.SetCancelHook(func() {
			c.cbByUUID.Delete(ud.String())
		})
		c.SendPacket(pkt)
	}, false)
}
