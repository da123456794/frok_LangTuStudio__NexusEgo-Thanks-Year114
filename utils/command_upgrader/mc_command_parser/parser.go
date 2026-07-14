package mc_command_parser

import "strings"

func (p *CommandParser) ExpectHeader(expect string, isCommandHeader bool) (is bool) {
	r := p.reader
	r.JumpSpace()
	if isCommandHeader {
		switch r.Next(true) {
		case "/":
			r.JumpSpace()
		case "":
		default:
			r.SetPtr(r.Pointer() - 1)
		}
	}
	older := r.Pointer()
	l := len(expect)
	if len(r.String()) >= r.Pointer()+l && strings.ToLower(r.Sentence(l)) == expect {
		return true
	}
	r.SetPtr(older)
	return false
}

func (p *CommandParser) ParseSelector() (selector Selector) {
	r := p.reader
	switch r.Next(false) {
	case `@`:
		older := r.Pointer() - 1
		func() {
			for {
				switch r.Next(false) {
				case " ", "[", "~", "^", "\n", "+", "\t":
					r.SetPtr(r.Pointer() - 1)
					selector.Main = r.CutSentence(older)
					return
				}
			}
		}()
		r.JumpSpace()
		switch r.Next(true) {
		case "[", "":
		default:
			r.SetPtr(r.Pointer() - 1)
			return
		}
		older = r.Pointer() - 1
		for {
			switch r.Next(false) {
			case `"`:
				r.ParseString()
			case `]`:
				tmp := r.CutSentence(older)
				selector.Sub = &tmp
				return
			}
		}
	case `"`:
		selector.Main = r.ParseString()
		return
	default:
		older := r.Pointer() - 1
		for {
			switch op := r.Next(true); op {
			case " ", "\n", "+", "\t", "":
				if op != "" {
					r.SetPtr(r.Pointer() - 1)
				}
				if selector.Main = r.CutSentence(older); len(selector.Main) == 0 {
					panic("ParseSelector: EOF")
				}
				return
			}
		}
	}
}

func (p *CommandParser) ParsePosition() (pos [3]string) {
	r := p.reader
	for i := 0; i < 3; i++ {
		if i > 0 {
			r.JumpSpace()
		}
		switch op := r.Next(false); op {
		case "~", "^":
			pos[i] = op
		default:
			r.SetPtr(r.Pointer() - 1)
		}
		switch op := r.Next(true); op {
		case "+", "-", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			r.SetPtr(r.Pointer() - 1)
			if len(pos[i]) == 0 && i != 1 {
				tmp, _ := r.ParseNumber(false)
				pos[i] = pos[i] + tmp
			} else {
				tmp, _ := r.ParseNumber(true)
				pos[i] = pos[i] + tmp
			}
			if pos[i] == "~0" || pos[i] == "^0" {
				pos[i] = pos[i][0:1]
			}
		default:
			if i != 2 {
				switch op {
				case "~", "^":
					r.SetPtr(r.Pointer() - 1)
				case " ", "\n", "\t":
				case "":
					if len(pos[i]) == 0 {
						panic("ParsePosition: EOF")
					}
				default:
					panic("ParsePosition: Invalid position")
				}
			} else if len(pos[i]) == 0 {
				panic("ParsePosition: Invalid position")
			} else if op != "" {
				r.SetPtr(r.Pointer() - 1)
			}
		}
	}
	return
}

func (p *CommandParser) ParseDetectArgs() (detectArgs DetectArgs) {
	var isInt bool
	r := p.reader
	detectArgs.BlockPosition = p.ParsePosition()
	r.JumpSpace()
	older := r.Pointer()
	func() {
		for {
			switch r.Next(true) {
			case " ", "\n", "\t":
				r.SetPtr(r.Pointer() - 1)
				return
			case "":
				return
			case "+":
				panic("ParseDetectArgs: Invalid block data value")
			}
		}
	}()
	detectArgs.BlockName = r.CutSentence(older)
	r.JumpSpace()
	if detectArgs.BlockData, isInt = r.ParseNumber(true); !isInt {
		panic("ParseDetectArgs: Block data provided must be an integer")
	}
	return
}
