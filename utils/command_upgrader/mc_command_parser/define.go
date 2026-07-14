package mc_command_parser

import "nexus/utils/command_upgrader/string_reader"

type CommandParser struct {
	reader *string_reader.StringReader
}

func NewCommandParser(command *string) *CommandParser {
	return &CommandParser{
		reader: string_reader.NewStringReader(command),
	}
}

const BlockStatesDefaultSeparator = ":"

type Selector struct {
	Main string
	Sub  *string
}

type DetectArgs struct {
	BlockPosition [3]string
	BlockName     string
	BlockData     string
}

type ExecuteCommand struct {
	Selector   Selector
	Position   [3]string
	DetectArgs *DetectArgs
	SubCommand string
}
