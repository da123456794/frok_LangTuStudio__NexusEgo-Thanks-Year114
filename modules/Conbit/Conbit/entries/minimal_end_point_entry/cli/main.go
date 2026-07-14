package main

import "github.com/LangTuStudio/Conbit/Conbit/entries/minimal_end_point_entry"

func main() {
	args := minimal_end_point_entry.GetArgs()
	minimal_end_point_entry.Entry(args)
}
