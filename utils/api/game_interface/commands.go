package GameInterface

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	ResourcesControl "nexus/utils/api/resources_control"
	"nexus/utils/py_rpc"
	cts "nexus/utils/py_rpc/mod_event/client_to_server"
	cts_mc "nexus/utils/py_rpc/mod_event/client_to_server/minecraft"
	cts_mc_a "nexus/utils/py_rpc/mod_event/client_to_server/minecraft/ai_command"
	mei "nexus/utils/py_rpc/mod_event/interface"

	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	"github.com/google/uuid"
)

const defaultAwaitChangesCount = 2

func (g *GameInterface) buildCommandRespond(
	resp *packet.CommandOutput,
	err error,
	errorPrefix string,
	commandType string,
) ResourcesControl.CommandRespond {
	result := ResourcesControl.CommandRespond{
		Respond: resp,
		Type:    commandType,
	}
	if err == nil {
		return result
	}
	result.ErrorType = ResourcesControl.ErrCommandRequestOthers
	if errors.Is(err, context.DeadlineExceeded) {
		result.ErrorType = ResourcesControl.ErrCommandRequestTimeOut
	}
	result.Error = fmt.Errorf("%s: %v", errorPrefix, err)
	return result
}

func (g *GameInterface) SendSettingsCommand(command string, dimensional bool) error {
	if dimensional {
		command = fmt.Sprintf(
			`execute as @a[name="%s"] at @s run %s`,
			g.ClientInfo.DisplayName,
			command,
		)
	}
	if g != nil && g.CommandSender != nil {
		g.CommandSender.SendAICommandOmitResponse(
			fmt.Sprintf("%d", g.ClientInfo.EntityRuntimeID),
			"execute run "+command,
		)
		return nil
	}
	return g.SendAICommand("execute run "+command, false)
}

func (g *GameInterface) send_command(command string, uniqueID uuid.UUID, origin uint32) error {
	pkt := packet.CommandRequest{
		CommandLine: command,
		CommandOrigin: protocol.CommandOrigin{
			Origin: origin,
			UUID:   uniqueID,
		},
		Internal:  false,
		UnLimited: false,
		Version:   0x23,
	}
	if origin == protocol.CommandOriginAutomationPlayer {
		pkt.CommandOrigin.RequestID = DefaultCommandRequestID
	}
	if err := g.WritePacket(&pkt); err != nil {
		return fmt.Errorf("send_command: %v", err)
	}
	return nil
}

func (g *GameInterface) send_netease_ai_command(command string, uniqueID uuid.UUID) error {
	event := cts_mc_a.ExecuteCommandEvent{
		CommandLine:      command,
		CommandRequestID: uniqueID,
	}
	module := cts_mc.AICommand{Module: &mei.DefaultModule{Event: &event}}
	park := cts.Minecraft{Default: mei.Default{Module: &module}}
	if err := g.WritePacket(&packet.PyRpc{
		Value: py_rpc.Marshal(&py_rpc.ModEvent{
			Package: &park,
			Type:    py_rpc.ModEventClientToServer,
		}),
		OperationType: packet.PyRpcOperationTypeSend,
	}); err != nil {
		return fmt.Errorf("send_netease_ai_command: %v", err)
	}
	return nil
}

func (g *GameInterface) send_command_with_options(
	command string,
	options ResourcesControl.CommandRequestOptions,
	origin *uint32,
) *ResourcesControl.CommandRespond {
	commandRequestID := ResourcesControl.GenerateUUID()

	commandType := ResourcesControl.CommandTypeStandard
	if origin == nil {
		commandType = ResourcesControl.CommandTypeAICommand
	}
	if err := g.Resources.Command.WriteRequest(commandRequestID, options, commandType); err != nil {
		return &ResourcesControl.CommandRespond{
			Error:     fmt.Errorf("send_command_with_options: %v", err),
			ErrorType: ResourcesControl.ErrCommandRequestOthers,
		}
	}

	var err error
	if origin == nil {
		err = g.send_netease_ai_command(command, commandRequestID)
	} else {
		err = g.send_command(command, commandRequestID, *origin)
	}
	if err != nil {
		return &ResourcesControl.CommandRespond{
			Error:     fmt.Errorf("send_command_with_options: %v", err),
			ErrorType: ResourcesControl.ErrCommandRequestOthers,
		}
	}
	if options.WithNoResponse {
		return nil
	}

	resp := g.Resources.Command.LoadResponseAndDelete(commandRequestID)
	if resp.Error != nil {
		resp.Error = fmt.Errorf("send_command_with_options: %v", resp.Error)
	}
	if origin == nil {
		return &resp
	}

	resp.Type = ResourcesControl.CommandTypeStandard
	if resp.Respond == nil {
		fakeResp := DefaultCommandOutput
		fakeResp.CommandOrigin.Origin = *origin
		fakeResp.CommandOrigin.UUID = commandRequestID
		fakeResp.OutputMessages = []protocol.CommandOutputMessage{
			{
				Success:    false,
				Message:    "commands.generic.syntax",
				Parameters: []string{"", command, ""},
			},
		}
		if *origin == protocol.CommandOriginAutomationPlayer {
			fakeResp.DataSet = "{\n   \"statusCode\" : -2147483648\n}\n"
		}
		resp.Respond = &fakeResp
	} else {
		resp.Respond.CommandOrigin.Origin = *origin
	}

	if *origin == protocol.CommandOriginAutomationPlayer {
		resp.Respond.CommandOrigin.RequestID = DefaultCommandRequestID
		resp.Respond.OutputType = packet.CommandOutputTypeDataSet
	} else {
		resp.Respond.CommandOrigin.RequestID = ""
		resp.Respond.OutputType = packet.CommandOutputTypeNone
		resp.Respond.DataSet = ""
	}
	return &resp
}

func (g *GameInterface) SendCommand(command string) error {
	if g != nil && g.CommandSender != nil {
		g.CommandSender.SendPlayerCmdOmitResponse(command)
		return nil
	}
	uniqueID, _ := uuid.NewUUID()
	if err := g.send_command(command, uniqueID, protocol.CommandOriginPlayer); err != nil {
		return fmt.Errorf("SendCommand: %v", err)
	}
	return nil
}

func (g *GameInterface) SendWSCommand(command string) error {
	if g != nil && g.CommandSender != nil {
		g.CommandSender.SendWebSocketCmdOmitResponse(command)
		return nil
	}
	uniqueID, _ := uuid.NewUUID()
	if err := g.send_command(command, uniqueID, protocol.CommandOriginAutomationPlayer); err != nil {
		return fmt.Errorf("SendWSCommand: %v", err)
	}
	return nil
}

func (g *GameInterface) SendAICommand(command string, dimensional bool) error {
	holder := g.Resources.Command.Occupy()
	defer g.Resources.Command.Release(holder)

	if dimensional {
		command = fmt.Sprintf(
			`execute as @a[name="%s"] at @s run %s`,
			g.ClientInfo.DisplayName,
			command,
		)
	}

	if g != nil && g.CommandSender != nil {
		g.CommandSender.SendAICommandOmitResponse(
			fmt.Sprintf("%d", g.ClientInfo.EntityRuntimeID),
			command,
		)
		return nil
	}

	uniqueID, _ := uuid.NewUUID()
	if err := g.Resources.Command.WriteRequest(
		uniqueID,
		ResourcesControl.CommandRequestOptions{WithNoResponse: true},
		ResourcesControl.CommandTypeAICommand,
	); err != nil {
		return fmt.Errorf("SendAICommand: %v", err)
	}
	if err := g.send_netease_ai_command(command, uniqueID); err != nil {
		return fmt.Errorf("SendAICommand: %v", err)
	}
	return nil
}

func (g *GameInterface) SendCommandWithResponse(
	command string,
	options ResourcesControl.CommandRequestOptions,
) ResourcesControl.CommandRespond {
	if g != nil && g.CommandSender != nil {
		if options.WithNoResponse {
			g.CommandSender.SendPlayerCmdOmitResponse(command)
			return ResourcesControl.CommandRespond{Type: ResourcesControl.CommandTypeStandard}
		}
		async := g.CommandSender.SendPlayerCmdNeedResponse(command)
		if options.TimeOut > 0 {
			async = async.SetTimeout(options.TimeOut)
		}
		resp, err := async.BlockGetResult()
		return g.buildCommandRespond(resp, err, "SendCommandWithResponse", ResourcesControl.CommandTypeStandard)
	}
	origin := uint32(protocol.CommandOriginPlayer)
	resp := g.send_command_with_options(command, options, &origin)
	if resp.Error != nil {
		resp.Error = fmt.Errorf("SendCommandWithResponse: %v", resp.Error)
	}
	return *resp
}

func (g *GameInterface) SendWSCommandWithResponse(
	command string,
	options ResourcesControl.CommandRequestOptions,
) ResourcesControl.CommandRespond {
	if g != nil && g.CommandSender != nil {
		if options.WithNoResponse {
			g.CommandSender.SendWebSocketCmdOmitResponse(command)
			return ResourcesControl.CommandRespond{Type: ResourcesControl.CommandTypeStandard}
		}
		async := g.CommandSender.SendWebSocketCmdNeedResponse(command)
		if options.TimeOut > 0 {
			async = async.SetTimeout(options.TimeOut)
		}
		resp, err := async.BlockGetResult()
		return g.buildCommandRespond(resp, err, "SendWSCommandWithResponse", ResourcesControl.CommandTypeStandard)
	}
	origin := uint32(protocol.CommandOriginAutomationPlayer)
	resp := g.send_command_with_options(command, options, &origin)
	if resp.Error != nil {
		resp.Error = fmt.Errorf("SendWSCommandWithResponse: %v", resp.Error)
	}
	return *resp
}

func (g *GameInterface) SendCommandWithOrigin(
	command string,
	origin uint32,
	options ResourcesControl.CommandRequestOptions,
) ResourcesControl.CommandRespond {
	if g != nil && g.CommandSender != nil {
		switch origin {
		case protocol.CommandOriginAutomationPlayer,
			protocol.CommandOriginDedicatedServer,
			protocol.CommandOriginDevConsole,
			protocol.CommandOriginEntityServer,
			protocol.CommandOriginGameDirectorEntityServer:
			return g.SendWSCommandWithResponse(command, options)
		case protocol.CommandOriginPlayer:
			return g.SendCommandWithResponse(command, options)
		}
	}
	resp := g.send_command_with_options(command, options, &origin)
	if resp.Error != nil {
		resp.Error = fmt.Errorf("SendCommandWithOrigin: %v", resp.Error)
	}
	return *resp
}

func (g *GameInterface) SendAICommandWithResponse(
	command string,
	options ResourcesControl.CommandRequestOptions,
) ResourcesControl.CommandRespond {
	if g != nil && g.CommandSender != nil {
		if options.WithNoResponse {
			g.CommandSender.SendAICommandOmitResponse(
				fmt.Sprintf("%d", g.ClientInfo.EntityRuntimeID),
				command,
			)
			return ResourcesControl.CommandRespond{Type: ResourcesControl.CommandTypeAICommand}
		}
		async := g.CommandSender.SendAICommandNeedResponse(
			fmt.Sprintf("%d", g.ClientInfo.EntityRuntimeID),
			command,
		)
		if options.TimeOut > 0 {
			async = async.SetTimeout(options.TimeOut)
		}
		resp, err := async.BlockGetResult()
		return g.buildCommandRespond(resp, err, "SendAICommandWithResponse", ResourcesControl.CommandTypeAICommand)
	}
	resp := g.send_command_with_options(command, options, nil)
	if resp.Error != nil {
		resp.Error = fmt.Errorf("SendAICommandWithResponse: %v", resp.Error)
	}
	return *resp
}

func (g *GameInterface) AwaitChangesGeneral() error {
	for i := 0; i < defaultAwaitChangesCount; i++ {
		resp := g.SendWSCommandWithResponse("", ResourcesControl.CommandRequestOptions{
			TimeOut: time.Second * 5,
		})
		if resp.Error != nil {
			return fmt.Errorf("AwaitChangesGeneral: %v", resp.Error)
		}
	}
	return nil
}

func (i *GameInterface) Output(content string) error {
	return nil
}

func (i *GameInterface) SendChat(content string) error {
	return i.WritePacket(&packet.Text{
		TextType:         packet.TextTypeChat,
		NeedsTranslation: false,
		SourceName:       i.ClientInfo.DisplayName,
		Message:          content,
		XUID:             i.ClientInfo.XUID,
		PlatformChatID:   "",
		NeteaseExtraData: []string{"PlayerId", fmt.Sprintf("%d", i.ClientInfo.EntityRuntimeID)},
	})
}

func (i *GameInterface) Title(message string) error {
	titleStruct := map[string]any{
		"rawtext": []any{
			map[string]any{
				"text": message,
			},
		},
	}
	jsonContent, _ := json.Marshal(titleStruct)
	return i.SendSettingsCommand(fmt.Sprintf("titleraw @a actionbar %s", jsonContent), false)
}
