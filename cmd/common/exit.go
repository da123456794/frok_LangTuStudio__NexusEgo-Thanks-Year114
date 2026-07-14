package common

import (
	"bufio"
	"os"

	"nexus/utils/log"
)

func WaitForExit(console *Console) {
	log.Log.Info("按回车键退出...")
	if console != nil {
		console.Input("")
		return
	}
	_, _ = bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func ExitAfterPrompt(console *Console, code int) {
	WaitForExit(console)
	os.Exit(code)
}
