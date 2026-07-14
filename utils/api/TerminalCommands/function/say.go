package function

import (
	"strings"

	"nexus/utils/client"

	"github.com/pterm/pterm"
)

func Say(client *client.Client, words []string) bool {
	if len(words) < 2 {
		pterm.Println(pterm.Red("请输入要发送的消息!"))
		return false
	}
	client.GameInterface.SendChat(strings.Join(words[1:], " "))
	pterm.Println(pterm.Green("消息发送成功!"))
	return true
}
