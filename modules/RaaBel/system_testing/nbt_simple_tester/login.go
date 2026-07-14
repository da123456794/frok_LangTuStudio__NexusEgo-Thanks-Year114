package main

import (
	"fmt"
	"time"

	"github.com/LangTuStudio/RaaBel/client"
	"github.com/LangTuStudio/RaaBel/game_control/game_interface"
	"github.com/LangTuStudio/RaaBel/game_control/resources_control"
	"github.com/LangTuStudio/RaaBel/nbt_assigner/nbt_console"

	"github.com/pterm/pterm"
)

func SystemTestingLogin() {
	var err error
	tA := time.Now()

	cfg := client.Config{
		AuthServerAddress:    "...",
		AuthServerToken:      "...",
		RentalServerCode:     "48285363",
		RentalServerPasscode: "",
	}

	c, err = client.LoginRentalServer(cfg)
	if err != nil {
		panic(err)
	}

	resources = resources_control.NewResourcesControl(c)
	api, err = game_interface.NewGameInterface(resources, game_interface.DefaultMaintainer)
	if err != nil {
		panic(fmt.Sprintf("SystemTestingLogin: Failed on init game interface, and the err is %v", err))
	}

	console, err = nbt_console.NewConsole(api, 0, [3]int32{23, 12, -21})
	if err != nil {
		panic(fmt.Sprintf("SystemTestingLogin: Failed on init new console, and the err is %v", err))
	}

	pterm.Success.Printfln("SystemTestingLogin: PASS (Time used = %v)", time.Since(tA))
}
