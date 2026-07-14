package mainfunction

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	coreConbit "github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	types "nexus/defines"
	"nexus/utils/api/TerminalCommands/function"
	conbitapi "nexus/utils/api/conbit"
	GameInterface "nexus/utils/api/game_interface"
	resources_control "nexus/utils/api/resources_control"
	"nexus/utils/client"
	"nexus/utils/log"
	"nexus/utils/mirror/io/global"
	"nexus/utils/mirror/io/lru"
	"nexus/utils/webclient"
)

const defaultAuthServer = "https://user.fastbuilder.pro"

func initClientRuntime(cl *client.Client, conn client.Conn) {
	cl.Conn = conn
	// NewAccessPoint 返回前已经通过权限挑战，避免导入层重复等待 OP。
	cl.IsOP = true
	cl.CachedPacket = make(<-chan packet.Packet)
	cl.Resources = &resources_control.Resources{}
	cl.ResourcesUpdater = cl.Resources.(*resources_control.Resources).Init()
	cl.Resources.(*resources_control.Resources).BindRuntime(
		conn.WritePacket,
		resources_control.BotInfo{
			BotName:         conn.IdentityData().DisplayName,
			XUID:            conn.IdentityData().XUID,
			EntityUniqueID:  conn.GameData().EntityUniqueID,
			EntityRuntimeID: conn.GameData().EntityRuntimeID,
		},
		conn.GameData(),
	)
	gameInterface := &GameInterface.GameInterface{
		WritePacket: conn.WritePacket,
		ClientInfo: GameInterface.ClientInfo{
			DisplayName:     conn.IdentityData().DisplayName,
			ClientIdentity:  conn.IdentityData().Identity,
			XUID:            conn.IdentityData().XUID,
			EntityRuntimeID: conn.GameData().EntityRuntimeID,
			EntityUniqueID:  conn.GameData().EntityUniqueID,
		},
		Resources: cl.Resources.(*resources_control.Resources),
	}
	if provider, ok := any(conn).(interface {
		CommandSender() coreConbit.CmdSender
	}); ok {
		gameInterface.CommandSender = provider.CommandSender()
	}
	cl.GameInterface = gameInterface
}

func launchWorker(conn client.Conn, cl *client.Client, onPanic func(any)) {
	go func() {
		defer func() {
			if err2 := recover(); err2 != nil {
				if onPanic != nil {
					onPanic(err2)
					return
				}
				panic(err2)
			}
		}()
		EnterWorkerThread(conn, cl, nil)
	}()
}

func Start_Client(server string, password string) (*client.Client, error) {
	conn, err := conbitapi.NewAccessPoint(server, password, "", defaultAuthServer)
	if err != nil {
		return nil, err
	}
	yxdrClient := client.Client{
		Access:               conn,
		LRUMemoryChunkCacher: lru.NewLRUMemoryChunkCacher(12, false),
		ChunkFeeder:          global.NewChunkFeeder(),
		IsOP_loop:            sync.Mutex{},
		Cdump_Setting:        client.New_Cdump_Setting(),
	}
	initClientRuntime(&yxdrClient, conn)
	launchWorker(conn, &yxdrClient, func(err2 any) {
		_ = yxdrClient.GameInterface.SendWSCommand("kick NexusEgo")
		panic(err2)
	})
	return &yxdrClient, nil
}

func Start_Client_by_yx(server string, password string, task *types.Task, task_name string, webclient *webclient.Webclient) (*client.Client, error) {
	conn, err := conbitapi.NewAccessPoint(server, password, "", defaultAuthServer)
	if err != nil {
		webclient.Update_task_now_operation(task_name, task.UserID, err.Error())
		return nil, err
	}
	cdumpSettings := client.New_Cdump_Setting()
	cdumpSettings.Get_Parameter_From_Json(task.High_import_Setting)
	yxdrClient := client.Client{
		Access:               conn,
		Conn:                 conn,
		LRUMemoryChunkCacher: lru.NewLRUMemoryChunkCacher(12, false),
		ChunkFeeder:          global.NewChunkFeeder(),
		Cdump_Setting:        cdumpSettings,
		IsOP_loop:            sync.Mutex{},
	}
	initClientRuntime(&yxdrClient, conn)
	launchWorker(conn, &yxdrClient, func(err2 any) {
		webclient.Update_task_now_operation(task_name, task.UserID, fmt.Sprint(err2))
		_ = yxdrClient.GameInterface.SendWSCommand("kick NexusEgo")
		os.Exit(0)
	})
	return &yxdrClient, nil
}

func Start_Client_by_token(server string, password, token, yz string) (*client.Client, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Log.Info(fmt.Sprintf("第 %d/%d 次尝试连接服务器...", attempt, maxRetries))
			time.Sleep(3 * time.Second)
		}

		cl, err := startClientByTokenOnce(server, password, token, yz)
		if err == nil {
			if attempt > 1 {
				log.Log.Info(fmt.Sprintf("第 %d 次尝试连接成功", attempt))
			}
			return cl, nil
		}

		lastErr = err
		log.Log.Error(fmt.Sprintf("连接失败 (%d/%d) (%s)", attempt, maxRetries, err.Error()))
	}

	return nil, fmt.Errorf("连接服务器失败，已重试 %d 次: %w", maxRetries, lastErr)
}

func startClientByTokenOnce(server string, password, token, yz string) (*client.Client, error) {
	conn, err := conbitapi.NewAccessPoint(server, password, token, yz)
	if err != nil {
		return nil, err
	}
	yxdrClient := client.Client{
		Access:               conn,
		Conn:                 conn,
		LRUMemoryChunkCacher: lru.NewLRUMemoryChunkCacher(12, false),
		ChunkFeeder:          global.NewChunkFeeder(),
		Cdump_Setting:        client.New_Cdump_Setting(),
		IsOP_loop:            sync.Mutex{},
	}
	initClientRuntime(&yxdrClient, conn)
	launchWorker(conn, &yxdrClient, nil)
	return &yxdrClient, nil
}

func Import_task(task *types.Task, task_name string, webclient *webclient.Webclient) (is_true bool, err error) {
	defer func() {
		if err2 := recover(); err2 != nil {
			log.Log.Info("上传后文件ID", log.Log.ArgsFromMap(map[string]interface{}{
				"任务":   task_name,
				"文件ID": task.FileID,
			}))
			return
		}
	}()

	cl, err := Start_Client_by_yx(task.Server, task.Password, task, task_name, webclient)
	if err != nil {
		return false, err
	}
	if cl == nil {
		webclient.Update_task_now_operation(task_name, task.UserID, "导入完成")
		return false, errors.New("连接失败")
	}

	isOP := false
	for i := 0; i < 180; i++ {
		cl.IsOP_loop.Lock()
		isOP = cl.IsOP
		cl.IsOP_loop.Unlock()
		log.Log.Info(fmt.Sprintf("请给予机器人 %s OP权限", cl.Conn.IdentityData().DisplayName))
		if isOP {
			break
		}
		time.Sleep(time.Second)
	}
	if !isOP {
		webclient.Stop_task_import_file_by_no_op(task_name, task.UserID)
		return false, errors.New("机器人不是OP")
	}

	function.CleanupNexusTickingAreas(cl)
	if function.Cdump_import(cl, "./import_file/"+task.FilePath, task.XYZ[0], task.XYZ[1], task.XYZ[2], false, 0, webclient, task_name, *task, nil, nil) {
		webclient.Update_task_now_operation(task_name, task.UserID, "导入完成")
		_ = cl.GameInterface.SendWSCommand("kick NexusEgo")
		_ = cl.Conn.WritePacket(&packet.Disconnect{HideDisconnectionScreen: false})
		return true, nil
	}
	return false, errors.New("导入失败")
}
