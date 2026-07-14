package TerminalCommands

import (
	"nexus/utils/api/TerminalCommands/function"
	"nexus/utils/client"

	"github.com/pterm/pterm"
)

func Process(client *client.Client, words []string) bool {
	switch words[0] {
	case "help":
		return function.Help(client, words)
	case "say":
		if client.Conn != nil {
			return function.Say(client, words)
		}
	case "cexport", "export":
		return function.Cexport(client, words)
	case "exit":
		return function.Exit(client, words)
	default:
		pterm.Println(pterm.Red("未知命令,输入help查看可用命令"))
		return function.Help(client, words)
	}
	pterm.Println(pterm.Red("正在重连MC,您请等待重连完成后再输入命令"))
	return false
}
