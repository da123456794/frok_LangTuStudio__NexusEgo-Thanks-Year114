package function

import (
	"fmt"

	"nexus/utils/client"

	"github.com/pterm/pterm"
)

func Help(client *client.Client, words []string) bool {
	Println_Help("help", "显示帮助菜单")
	Println_Help("exit", "退出程序")
	Println_Help("cdump", "操作cdump文件")
	Println_Help("cexport", "export region to mcworld/nexus")
	Println_Help("/xxx", "输入/开头的命令为MC指令")
	Println_Help(".xxx", "输入.开头的命令为MC指令（要求返回）")
	return true
}

func Println_Help(cmd string, tips string) {
	pterm.Println(fmt.Sprintf("%s%s %s", pterm.Green(cmd), pterm.Cyan(":"), pterm.Yellow(tips)))
}
