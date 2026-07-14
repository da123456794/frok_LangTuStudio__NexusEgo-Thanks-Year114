package mc_command_parser

import (
	"fmt"
	"strconv"
)

func ParseExecuteCommand(command string) (e *ExecuteCommand) {
	p := NewCommandParser(&command)
	r := p.reader
	if p.ExpectHeader("execute", true) {
		e = &ExecuteCommand{}
	} else {
		return
	}
	r.JumpSpace()
	e.Selector = p.ParseSelector()
	r.JumpSpace()
	e.Position = p.ParsePosition()
	r.JumpSpace()
	if p.ExpectHeader("detect", false) {
		r.JumpSpace()
		tmp := p.ParseDetectArgs()
		e.DetectArgs = &tmp
	}
	r.JumpSpace()
	e.SubCommand = command[r.Pointer():]
	return
}

func ParseBlockStates(blockStates string) (m map[string]interface{}) {
	version := 0
	p := NewCommandParser(&blockStates)
	r := p.reader
	r.JumpSpace()
	if r.Next(true) == "[" {
		m = make(map[string]interface{})
	} else {
		return
	}
	r.JumpSpace()
	switch r.Next(false) {
	case "]":
		return
	default:
		r.SetPtr(r.Pointer() - 1)
	}
	for {
		r.JumpSpace()
		if r.Next(false) != `"` {
			panic("ParseBlockStates: Invalid block states string")
		}
		key := r.ParseString()
		r.JumpSpace()
		switch r.Next(false) {
		case ":":
			if version == 2 {
				panic("ParseBlockStates: Invalid block states string")
			}
			version = 1
		case "=":
			if version == 1 {
				panic("ParseBlockStates: Invalid block states string")
			}
			version = 2
		default:
			panic("ParseBlockStates: Invalid block states string")
		}
		r.JumpSpace()
		switch r.Next(false) {
		case "+", "-", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			r.SetPtr(r.Pointer() - 1)
			intString, isInt := r.ParseNumber(true)
			if !isInt {
				panic("ParseBlockStates: The value of the key provided can not be a float")
			}
			num, err := strconv.ParseInt(intString, 10, 32)
			if err != nil {
				panic(fmt.Sprintf("ParseBlockStates: %v", err))
			}
			m[key] = int32(num)
		case `"`:
			m[key] = r.ParseString()
		case "t", "f", "T", "F":
			r.SetPtr(r.Pointer() - 1)
			boolean := r.ParseBool()
			if boolean {
				m[key] = byte(1)
			} else {
				m[key] = byte(0)
			}
		default:
			panic("ParseBlockStates: Invalid block states string")
		}
		r.JumpSpace()
		switch op := r.Next(false); op {
		case ",":
		case "]":
			return
		default:
			panic("ParseBlockStates: Invalid block states string")
		}
	}
}
