package client

import resources_control "nexus/utils/api/resources_control"

type GameInterface interface {
	SendAICommand(string, bool) error
	SendAICommandWithResponse(string, resources_control.CommandRequestOptions) resources_control.CommandRespond
	SendSettingsCommand(string, bool) error
	SendCommand(string) error
	SendWSCommand(string) error
	SendCommandWithResponse(string, resources_control.CommandRequestOptions) resources_control.CommandRespond
	SendWSCommandWithResponse(string, resources_control.CommandRequestOptions) resources_control.CommandRespond
	SendCommandWithOrigin(string, uint32, resources_control.CommandRequestOptions) resources_control.CommandRespond

	SetBlock([3]int32, string, string) error
	SetBlockAsync([3]int32, string, string) error

	SendChat(string) error
	Output(string) error
	Title(string) error
}
