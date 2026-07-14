package console

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"nexus/utils/log"

	"github.com/pterm/pterm"
	"golang.org/x/term"
)

func New() *Console_input {
	return &Console_input{
		input_text: "",
		Input_chan: nil,
		system_io:  bufio.NewReader(os.Stdin),
		input_err:  nil,
		started:    false,
	}
}

var promptPrefix = ""
var promptTextStyle = pterm.NewStyle(pterm.FgWhite)

func printPrompt(text string) {
	if text == "" {
		return
	}
	trimmed := strings.TrimLeft(text, "\n")
	leading := len(text) - len(trimmed)
	if leading > 0 {
		pterm.Print(strings.Repeat("\n", leading))
		text = trimmed
	}
	if text == "" {
		return
	}
	pterm.Print(promptPrefix)
	promptTextStyle.Print(text)
}

type Console_input struct {
	input_text           string
	Input_chan           chan bool
	system_io            *bufio.Reader
	input_err            error
	started              bool
	fallbackInputHandler func(string)
}

func (console *Console_input) Start() {
	if !console.started {
		console.started = true
		go func() {
			console.input()
		}()
	}
}

func (console *Console_input) input() {
	for {
		text, err := console.system_io.ReadString('\n')
		if err != nil {
			console.input_err = err
			continue
		}
		line := strings.Trim(strings.Trim(text, "\n"), "\r")
		if console.Input_chan == nil {
			if console.fallbackInputHandler != nil {
				console.fallbackInputHandler(line)
				continue
			}
			log.Log.Warn("当前不需要输入任何东西")
			continue
		}

		console.input_text = line
		console.Input_chan <- true
	}
}

func (console *Console_input) Input(text string) (string, bool, error) {
	if !console.started {
		result, err := console.InputSync(text)
		return result, err == nil, err
	}

	printPrompt(text)
	console.Input_chan = make(chan bool)
	is_get_input := <-console.Input_chan
	if is_get_input {
		console.Input_chan = nil
		return console.input_text, true, console.input_err
	}

	console.Input_chan = nil
	return "", false, console.input_err
}

func (console *Console_input) InputInfo(text string) (string, bool, error) {
	infoPrefix := pterm.White("[") + pterm.Green("INFO") + pterm.White("]")
	var prompt string
	if runtime.GOOS == "linux" {
		prompt = fmt.Sprintf("%s %s\n", infoPrefix, pterm.White(text))
	} else {
		prompt = fmt.Sprintf("%s %s", infoPrefix, pterm.White(text))
	}
	if !console.started {
		fmt.Print(prompt)
		input, err := console.system_io.ReadString('\n')
		if err != nil {
			return "", false, err
		}
		return strings.TrimSpace(input), true, nil
	}

	fmt.Print(prompt)
	console.Input_chan = make(chan bool)
	is_get_input := <-console.Input_chan
	if is_get_input {
		console.Input_chan = nil
		return console.input_text, true, console.input_err
	}

	console.Input_chan = nil
	return "", false, console.input_err
}

func (console *Console_input) InputNoPrefix(text string) (string, bool, error) {
	if !console.started {
		fmt.Print(text)
		input, err := console.system_io.ReadString('\n')
		if err != nil {
			return "", false, err
		}
		return strings.TrimSpace(input), true, nil
	}

	if text != "" {
		if runtime.GOOS == "linux" {
			fmt.Println(text)
		} else {
			fmt.Print(text)
		}
	} else {
		fmt.Print(text)
	}
	console.Input_chan = make(chan bool)
	is_get_input := <-console.Input_chan
	if is_get_input {
		console.Input_chan = nil
		return console.input_text, true, console.input_err
	}

	console.Input_chan = nil
	return "", false, console.input_err
}

func (console *Console_input) SetFallbackInputHandler(handler func(string)) {
	console.fallbackInputHandler = handler
}

func (console *Console_input) InputSync(text string) (string, error) {
	printPrompt(text)
	input, err := console.system_io.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(input), nil
}

func (console *Console_input) InputPassword(text string) (string, error) {
	printPrompt(text)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(password)), nil
}
