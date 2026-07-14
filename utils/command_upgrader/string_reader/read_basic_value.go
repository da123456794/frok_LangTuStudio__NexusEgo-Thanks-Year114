package string_reader

import (
	"encoding/json"
	"strings"
)

func (s *StringReader) JumpSpace() {
	for {
		switch s.Next(true) {
		case " ", "\n", "\t":
		case "":
			return
		default:
			s.SetPtr(s.Pointer() - 1)
			return
		}
	}
}

func (s *StringReader) ParseBool() (res bool) {
	if part1 := s.Sentence(4); len(part1) < 4 {
		panic("ParseBool: EOF")
	} else if strings.ToLower(part1) == "true" {
		res = true
	} else {
		part2 := s.Next(false)
		res = strings.ToLower(part1+part2) != "false"
		if res {
			panic("ParseBool: Invalid boolean")
		}
	}
	return
}

func (s *StringReader) ParseString() (res string) {
	older := s.Pointer() - 1
	for {
		switch s.Next(false) {
		case `\`:
			s.SetPtr(s.Pointer() + 1)
		case `"`:
			tmp := s.CutSentence(older)
			json.Unmarshal([]byte(tmp), &res)
			return res
		}
	}
}

func (s *StringReader) ParseNumber(omission bool) (res string, isInt bool) {
	isNegative := false
	isFirstOp := true
	isZero := true
	hasPoint := false

	switch op := s.Next(false); op {
	case "+":
	case "-":
		isNegative = true
	default:
		s.SetPtr(s.Pointer() - 1)
	}

	older := s.Pointer()
	func() {
		for {
			switch op := s.Next(true); op {
			case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				if isFirstOp {
					isFirstOp = false
				}
				if op == "0" && isZero {
					older++
				}
				if op != "0" && isZero {
					isZero = false
				}
			case ".":
				if isFirstOp || hasPoint {
					panic("ParseNumber: Invalid number")
				}
				if isZero {
					older--
					isZero = false
				}
				hasPoint = true
			case "-":
				panic("ParseNumber: Invalid number")
			default:
				if op != "" {
					s.SetPtr(s.Pointer() - 1)
				}
				res = s.CutSentence(older)
				if len(res) == 0 && !isFirstOp {
					res = "0"
				}
				return
			}
		}
	}()

	ptr := len(res)
	if ptr < 1 {
		panic("ParseNumber: EOF")
	}
	if res[ptr-1:ptr] == "." {
		panic("ParseNumber: EOF")
	}
	if hasPoint {
		for {
			switch res[ptr-1 : ptr] {
			case "0":
				ptr--
			default:
				res = res[:ptr]
				goto trimmed
			}
		}
	}
trimmed:
	if res[len(res)-1:] == "." {
		if omission {
			res = res[:len(res)-1]
		} else {
			res = res + "0"
		}
	}
	if res != "0" && res != "0.0" && isNegative {
		res = "-" + res
	}
	return res, !hasPoint
}
