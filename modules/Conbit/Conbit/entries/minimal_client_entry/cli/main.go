package main

import (
	"github.com/LangTuStudio/Conbit/Conbit/entries/minimal_client_entry"
)

func main() {
	args := minimal_client_entry.GetArgs()
	minimal_client_entry.Entry(args)
}
