package access_point

import (
	"context"
	"crypto/md5"
	"fmt"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/access_helper"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/info_collect_utils"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/internal/termlog"
	"github.com/LangTuStudio/Conbit/nodes"
	"github.com/LangTuStudio/Conbit/nodes/defines"
	"github.com/LangTuStudio/Conbit/nodes/underlay_conn"
)

const ENTRY_NAME = "omega_access_point"

func printAccessPointLine(msg string) {
	termlog.Infof("%s", msg)
}

func Entry(args *Args) (omegaCore Conbit.MicroOmega, node defines.Node, err error) {
	printAccessPointLine(i18n.T(i18n.S_Conbit_access_point_starting))
	impactOption := args.ImpactOption

	if err := info_collect_utils.ReadUserInfoAndUpdateImpactOptions(impactOption); err != nil {
		return nil, nil, err
	}

	accessOption := access_helper.DefaultOptions()
	accessOption.ImpactOption = args.ImpactOption
	accessOption.MakeBotCreative = true
	accessOption.DisableCommandBlock = false
	accessOption.ReasonWithPrivilegeStuff = true

	ctx := context.Background()
	{
		server, err := underlay_conn.NewServerFromBasicNet(args.AccessArgs.AccessPointAddr)
		if err != nil {
			return nil, nil, err
		}
		master := nodes.NewMasterNode(server)
		node = nodes.NewGroup("Conbit", master, false)
	}
	omegaCore, err = access_helper.ImpactServer(ctx, node, accessOption)
	if err != nil {
		return nil, nil, err
	}
	huid := defines.Empty
	if args.UserToken != "" {
		huid = defines.FromString(StrMD5Str(args.UserToken))
	}
	node.SetValue("HashedUserID", huid)
	node.SetValue("_ServerID", defines.FromString(args.ServerCode))
	node.SetTags("access-point-ready")
	node.PublishMessage("reboot", defines.FromString("reboot to refresh data"))
	printAccessPointLine(i18n.T(i18n.S_Conbit_access_point_ready))
	return omegaCore, node, nil
}

func StrMD5Str(data string) string {
	return BytesMD5Str([]byte(data))
}

func BytesMD5Str(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}
