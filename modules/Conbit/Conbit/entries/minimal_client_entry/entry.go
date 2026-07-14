package minimal_client_entry

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/access_helper"
	"github.com/LangTuStudio/Conbit/Conbit/rental_server_impact/info_collect_utils"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/internal/termlog"
	"github.com/LangTuStudio/Conbit/nodes"
)

const ENTRY_NAME = "omega_minimal_client"

func printAccessPointLine(msg string) {
	termlog.Infof("%s", msg)
}

func Entry(args *access_helper.ImpactOption) {
	printAccessPointLine(i18n.T(i18n.S_Conbit_access_point_starting))

	if err := info_collect_utils.ReadUserInfoAndUpdateImpactOptions(args); err != nil {
		termlog.Errorf("%v", err)
		return
	}

	accessOption := access_helper.DefaultOptions()
	accessOption.ImpactOption = args
	accessOption.MakeBotCreative = true
	accessOption.DisableCommandBlock = false
	accessOption.ReasonWithPrivilegeStuff = true

	ctx := context.Background()
	node := nodes.NewLocalNode(ctx)
	node = nodes.NewGroup("Conbit-core", node, false)
	omegaCore, err := access_helper.ImpactServer(context.Background(), node, accessOption)
	if err != nil {
		termlog.Errorf("%v", err)
		return
	}

	go func() {
		for {
			time.Sleep(time.Second)
			ret, _ := omegaCore.GetGameControl().SendWebSocketCmdNeedResponse("tp @s ~~~").BlockGetResult()
			if ret == nil || ret.SuccessCount == 0 {
				termlog.Errorf("tp @s ~~~ fail, recv: %v", ret)
				omegaCore.CloseWithError(fmt.Errorf("tp @s ~~~ fail"))
				return
			}
		}
	}()
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print(">")
			line, err := reader.ReadString('\n')
			if err != nil {
				termlog.Errorf("%v", err)
				omegaCore.CloseWithError(err)
				return
			}
			if strings.HasPrefix(line, "/") {
				cmd := strings.TrimPrefix(line, "/")
				ret, _ := omegaCore.GetGameControl().SendWebSocketCmdNeedResponse(cmd).SetTimeout(time.Second).BlockGetResult()
				if ret == nil {
					termlog.Errorf("cmd not responsed")
				} else {
					bs, _ := json.Marshal(ret)
					termlog.Infof("%s", string(bs))
				}
			} else {
				termlog.Infof("try type /tp @s ~~~")
			}
		}
	}()
	omegaCore.GetBotAction().DropItemFromHotBar(3)
	if err := <-omegaCore.WaitClosed(); err != nil {
		termlog.Errorf("%v", err)
	}
}
