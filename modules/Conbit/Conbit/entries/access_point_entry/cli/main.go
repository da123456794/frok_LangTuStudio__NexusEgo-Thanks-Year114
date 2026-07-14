package main

import (
	"os"

	access_point "github.com/LangTuStudio/Conbit/Conbit/entries/access_point_entry"
	"github.com/LangTuStudio/Conbit/internal/termlog"
)

func main() {
	args := access_point.GetArgs()
	omegaCore, _, err := access_point.Entry(args)
	if err != nil {
		termlog.Errorf("%v", err)
		os.Exit(1)
	}
	if err := <-omegaCore.WaitClosed(); err != nil {
		termlog.Errorf("%v", err)
		os.Exit(1)
	}
}
