package cli

import (
	"nexus/cmd/common"
	"nexus/control"
)

func Run(app *control.App, opts control.CLIOptions) {
	common.ShowProgramInfo()
	console := common.NewConsole()
	console.Start()
	app.RunWithConsole(console, opts)
}
