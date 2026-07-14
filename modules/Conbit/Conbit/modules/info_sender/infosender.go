package info_sender

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

func init() {
	if false {
		func(sender Conbit.InfoSender) {}(&InfoSender{})
	}
}

type InfoSender struct {
	Conbit.InteractCore
	Conbit.CmdSender
	Conbit.BotBasicInfoHolder
}

func NewInfoSender(interactable Conbit.InteractCore, cmdSender Conbit.CmdSender, info Conbit.BotBasicInfoHolder) Conbit.InfoSender {
	return &InfoSender{
		InteractCore:       interactable,
		CmdSender:          cmdSender,
		BotBasicInfoHolder: info,
	}
}

func (i *InfoSender) BotSay(msg string) {
	pk := &packet.Text{
		TextType:         packet.TextTypeChat,
		NeedsTranslation: false,
		SourceName:       i.GetBotName(),
		Message:          msg,
		XUID:             "",
		NeteaseExtraData: []string{"PlayerId", fmt.Sprintf("%d", i.GetBotRuntimeID())},
	}
	i.SendPacket(pk)
}

func (i *InfoSender) SayTo(target string, msg string) {
	content := toJsonRawString(msg)
	if strings.HasPrefix(target, "@") {
		i.SendWOCmd(fmt.Sprintf("tellraw %v %v", target, content))
	} else {
		i.SendWOCmd(fmt.Sprintf("tellraw \"%v\" %v", target, content))
	}
}

func (i *InfoSender) RawSayTo(target string, msg string) {
	if strings.HasPrefix(target, "@") {
		i.SendWOCmd(fmt.Sprintf("tellraw %v %v", target, msg))
	} else {
		i.SendWOCmd(fmt.Sprintf("tellraw \"%v\" %v", target, msg))
	}
}

type TellrawItem struct {
	Text string `json:"text"`
}
type TellrawStruct struct {
	RawText []TellrawItem `json:"rawtext"`
}

func toJsonRawString(line string) string {
	final := &TellrawStruct{
		RawText: []TellrawItem{{Text: line}},
	}
	content, _ := json.Marshal(final)
	return string(content)
}

func (i *InfoSender) ActionBarTo(target string, msg string) {
	content := toJsonRawString(msg)
	if strings.HasPrefix(target, "@") {
		i.SendWOCmd(fmt.Sprintf("titleraw %v actionbar %v", target, content))
	} else {
		i.SendWOCmd(fmt.Sprintf("titleraw \"%v\" actionbar %v", target, content))
	}
}

func (i *InfoSender) TitleTo(target string, msg string) {
	content := toJsonRawString(msg)
	if strings.HasPrefix(target, "@") {
		i.SendWOCmd(fmt.Sprintf("titleraw %v title %v", target, content))
	} else {
		i.SendWOCmd(fmt.Sprintf("titleraw \"%v\" title %v", target, content))
	}
}

func (i *InfoSender) SubTitleTo(target string, subTitle string, title string) {
	i.TitleTo(target, title)
	content := toJsonRawString(subTitle)
	if strings.HasPrefix(target, "@") {
		i.SendWOCmd(fmt.Sprintf("titleraw %v subtitle %v", target, content))
	} else {
		i.SendWOCmd(fmt.Sprintf("titleraw \"%v\" subtitle %v", target, content))
	}
}
