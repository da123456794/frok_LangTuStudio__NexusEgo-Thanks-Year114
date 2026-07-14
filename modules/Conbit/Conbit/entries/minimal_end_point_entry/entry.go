package minimal_end_point_entry

import (
	"os"
	"time"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/bundle"
	"github.com/LangTuStudio/Conbit/Conbit/chunks/define"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/internal/termlog"
	"github.com/LangTuStudio/Conbit/nodes"
	"github.com/LangTuStudio/Conbit/nodes/defines"
	"github.com/LangTuStudio/Conbit/nodes/underlay_conn"
)

const ENTRY_NAME = "omega_minimal_end_point"

func Entry(args *Args) {
	var node defines.Node
	{
		client, err := underlay_conn.NewClientFromBasicNet(args.AccessPointAddr, time.Second)
		if err != nil {
			termlog.Errorf("%v", err)
			return
		}
		slave, err := nodes.NewSlaveNode(client)
		if err != nil {
			termlog.Errorf("%v", err)
			return
		}
		node = nodes.NewGroup("Conbit", slave, false)
		node.ListenMessage("reboot", func(msg defines.Values) {
			reason, _ := msg.ToString()
			termlog.Infof("%s", reason)
			os.Exit(3)
		}, false)
		if !node.CheckNetTag("access-point") {
			termlog.Errorf("%s", i18n.T(i18n.S_no_access_point_in_network))
			return
		}
		for {
			if node.CheckNetTag("access-point-ready") {
				break
			}
			time.Sleep(time.Second)
		}
	}

	omegaCore, err := bundle.NewEndPointMicroOmega(node)
	if err != nil {
		termlog.Errorf("%v", err)
		return
	}
	_, _ = omegaCore.GetGameControl().SendWebSocketCmdNeedResponse("execute in overworld run tp @s 1024 200 1024").BlockGetResult()
	omegaCore.GetLowLevelAreaRequester().AttachSubChunkResultListener(func(scr Conbit.SubChunkResult) {})
	ret, err := omegaCore.GetLowLevelAreaRequester().
		LowLevelRequestChunk(define.ChunkPos{1024 >> 4, 1024 >> 4}).
		AutoDimension().
		FullY().
		X(0).
		ZRange(0, 3).
		GetResult().
		SetTimeout(time.Second * 3).
		BlockGetResult()
	if err != nil {
		termlog.Errorf("%v", err)
		return
	}
	termlog.Infof("%v", ret.AllOk())
	termlog.Infof("%v", ret.AllErrors())
	chunks := ret.ToChunks(nil)
	termlog.Infof("%v", chunks)

	if err := <-node.WaitClosed(); err != nil {
		termlog.Errorf("%v", err)
	}
}
