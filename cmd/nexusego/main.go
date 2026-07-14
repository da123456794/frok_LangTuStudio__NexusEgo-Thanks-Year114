package main

import (
	cmdargs "nexus/cmd/args"
	"nexus/cmd/cli"
	"nexus/constants"
	"nexus/control"
)

func main() {
	options := cmdargs.Parse()
	app := control.NewApp(constants.ServerURL, validateToken, startTask)
	app.MapBuilderRunner = startMapBuilder
	cli.Run(app, options)
}
