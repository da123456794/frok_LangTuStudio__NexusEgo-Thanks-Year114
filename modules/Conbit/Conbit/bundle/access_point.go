package bundle

import (
	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/minecraft_conn"
	"github.com/LangTuStudio/Conbit/Conbit/modules/cmd_sender"
	"github.com/LangTuStudio/Conbit/Conbit/modules/core"
	"github.com/LangTuStudio/Conbit/Conbit/uqholder"
	"github.com/LangTuStudio/Conbit/nodes/defines"
)

func NewAccessPointMicroOmega(node defines.Node, conn minecraft_conn.Conn, preferAIQueryTarget bool) Conbit.UnReadyMicroOmega {
	interactCore := core.NewAccessPointInteractCore(node, conn)
	reactCore := core.NewAccessPointReactCore(node, conn)
	microUQHolder := uqholder.NewAccessPointMicroUQHolder(node, conn, reactCore)

	cmdSender := cmd_sender.NewAccessPointCmdSender(node, reactCore, interactCore)
	return NewMicroOmega(interactCore, reactCore, microUQHolder, cmdSender, node, true, preferAIQueryTarget)
}
