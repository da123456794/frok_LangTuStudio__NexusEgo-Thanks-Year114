package bundle

import (
	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/modules/cmd_sender"
	"github.com/LangTuStudio/Conbit/Conbit/modules/core"
	"github.com/LangTuStudio/Conbit/Conbit/uqholder"
	"github.com/LangTuStudio/Conbit/nodes/defines"
)

func NewEndPointMicroOmega(node defines.Node) (Conbit.MicroOmega, error) {
	reactCore := core.NewEndPointReactCore(node)
	interactCore, err := core.NewEndPointInteractCore(node, reactCore)
	if err != nil {
		return nil, err
	}
	microUQHolder, err := uqholder.NewEndPointMicroUQHolder(node, reactCore)
	if err != nil {
		return nil, err
	}
	cmdSender := cmd_sender.NewEndPointCmdSender(node, reactCore, interactCore)
	unReadyMicroOmega := NewMicroOmega(interactCore, reactCore, microUQHolder, cmdSender, node, false, false)
	// access must have passed challenge
	unReadyMicroOmega.NotifyChallengePassed()
	return unReadyMicroOmega, nil
}
