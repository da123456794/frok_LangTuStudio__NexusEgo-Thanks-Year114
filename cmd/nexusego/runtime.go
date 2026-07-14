package main

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptoRand "crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nexus/cmd/common"
	appcontrol "nexus/control"
	types "nexus/defines"
	"nexus/utils/api/TerminalCommands/function"
	ResourcesControl "nexus/utils/api/resources_control"
	NBTAssigner "nexus/utils/bdump/nbt_assigner"
	clientpkg "nexus/utils/client"
	consolepkg "nexus/utils/console"
	convertpkg "nexus/utils/convert"
	"nexus/utils/dimension"
	"nexus/utils/file"
	"nexus/utils/log"
	"nexus/utils/mainfunction"
	"nexus/utils/mapbuilder"
	"nexus/utils/netutil"

	nbt "github.com/LangTuStudio/Conbit/minecraft/nbt"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	starclient "github.com/LangTuStudio/RaaBel/client"
	service "github.com/LangTuStudio/RaaBel/std_server/service/src"
	"github.com/gin-gonic/gin"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/pterm/pterm"
)

var (
	FlowersPort  int
	FlowersReady bool
	flowersMu    sync.RWMutex
	flowersGen   atomic.Uint64
	flowersQuiet sync.Once
	ansiPattern  = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

	flowersOPRequestMu   sync.RWMutex
	flowersOPRequestSink chan<- string

	accessPointOutputMu     sync.RWMutex
	accessPointOutputClient func() *clientpkg.Client
)

const (
	autoReconnectImportLimit = 5
	flowersRestartDelay      = 3 * time.Second
	flowersHealthGracePeriod = 20 * time.Second
	flowersHealthMaxFailures = 3
	// Conbit 的无响应看门狗 5 秒后开始倒计时，并在 60 秒阈值关闭旧机器人。
	// 自动续导必须等旧看门狗收尾，否则新登录流会和旧倒计时互相干扰。
	importReconnectWatchdogDrainDelay       = 61 * time.Second
	accessPointNoResponseAbortWindowSeconds = 3.0
)

var (
	importStallAbortDelay = 62 * time.Second
	importStallCheckEvery = time.Second
)

const accessPointNoResponseImportReasonPrefix = "机器人无响应倒计时触发"

func validateToken(serverURL, token string) (bool, string) {
	reqBody, _ := json.Marshal(map[string]string{
		"login_token": token,
		"server_code": "00000000",
	})
	client := netutil.NewHTTPClient(10 * time.Second)
	resp, err := client.Post(serverURL+"api/phoenix/login", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return false, "无法连接验证服务器: " + err.Error()
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "读取验证服务器响应失败: " + err.Error()
	}

	var result struct {
		Success   bool   `json:"success"`
		Message   string `json:"message"`
		ServerMsg string `json:"server_msg"`
	}
	if json.Unmarshal(body, &result) != nil {
		return false, "解析验证服务器响应失败"
	}
	if !result.Success && strings.Contains(result.Message, "Token ") {
		return false, result.Message
	}
	if result.ServerMsg != "" {
		return true, result.ServerMsg
	}
	return true, "Token 验证成功"
}

func shouldAutoReconnectImport(client *clientpkg.Client) bool {
	if client == nil {
		return false
	}
	if client.Conn == nil {
		return true
	}
	if state, ok := client.Conn.(interface{ Closed() bool }); ok && state.Closed() {
		return true
	}
	errText := strings.ToLower(importReconnectReason(client))
	if errText == "" {
		return false
	}
	return strings.Contains(errText, "conn dead") ||
		strings.Contains(errText, "node dead") ||
		strings.Contains(errText, "packet listen routine finished") ||
		strings.Contains(errText, "packet connection closed") ||
		strings.Contains(errText, "connection closed") ||
		strings.Contains(errText, "connection unavailable") ||
		strings.Contains(errText, "use of closed network connection") ||
		strings.Contains(errText, "netconn closed") ||
		strings.Contains(errText, "i/o timeout") ||
		strings.Contains(errText, "forcibly closed") ||
		strings.Contains(errText, "connection reset") ||
		strings.Contains(errText, "mc server disconnect") ||
		strings.Contains(errText, "rental server disconnected") ||
		strings.Contains(errText, "租赁服主动断开") ||
		strings.Contains(errText, "与网易租赁服连接已经断开") ||
		strings.Contains(errText, "机器人已确认掉线") ||
		strings.Contains(errText, "机器人无响应") ||
		strings.Contains(errText, "权限突然被移除") ||
		strings.Contains(errText, "权限被移除") ||
		strings.Contains(errText, "/deop") ||
		(strings.Contains(errText, "op") && strings.Contains(errText, "权限")) ||
		(strings.Contains(errText, "op") && strings.Contains(errText, "permission")) ||
		(strings.Contains(errText, "持续") && strings.Contains(errText, "未能从网易租赁服获得数据")) ||
		strings.Contains(errText, "no response") ||
		strings.Contains(errText, "bot is down") ||
		strings.Contains(errText, "eof")
}

func connectImportClient(tasks *appcontrol.Task, config *appcontrol.Config) (*clientpkg.Client, error) {
	_ = os.RemoveAll("cache")
	return mainfunction.Start_Client_by_token(tasks.Server, tasks.Password, config.FBToken, config.ServerURL)
}

func importReconnectReason(client *clientpkg.Client) string {
	if client == nil {
		return ""
	}
	var parts []string
	if text := strings.TrimSpace(client.LastImportError); text != "" {
		parts = append(parts, text)
	}
	if client.Conn != nil {
		if state, ok := client.Conn.(interface{ CloseError() error }); ok {
			if err := state.CloseError(); err != nil && strings.TrimSpace(err.Error()) != "" {
				parts = append(parts, err.Error())
			}
		}
	}
	return strings.Join(parts, " | ")
}

func closeImportClient(client *clientpkg.Client) {
	if client == nil || client.Conn == nil {
		return
	}
	_ = client.Conn.Close()
}

func accessPointNoResponseImportReason(remaining float64) string {
	return fmt.Sprintf("%s，剩余 %.2f 秒，主动中断当前导入以等待接入点重启后断点续导", accessPointNoResponseImportReasonPrefix, remaining)
}

func parseAccessPointNoResponseImportReasonRemaining(reason string) (float64, bool) {
	reason = strings.TrimSpace(reason)
	if reason == "" || !strings.Contains(reason, accessPointNoResponseImportReasonPrefix) {
		return 0, false
	}
	start := strings.Index(reason, "剩余")
	if start < 0 {
		return 0, false
	}
	rest := reason[start+len("剩余"):]
	end := strings.Index(rest, "秒")
	if end < 0 {
		return 0, false
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(rest[:end]), 64)
	if err != nil {
		return 0, false
	}
	return seconds, true
}

func importReconnectWatchdogDrainDelayForReason(reason string) time.Duration {
	if remaining, ok := parseAccessPointNoResponseImportReasonRemaining(reason); ok {
		if remaining <= 0 {
			return 0
		}
		return time.Duration(math.Ceil(remaining+1)) * time.Second
	}
	reason = strings.ToLower(strings.TrimSpace(reason))
	if reason == "" {
		return importReconnectWatchdogDrainDelay
	}
	if strings.Contains(reason, "no response after") ||
		strings.Contains(reason, "bot is down") ||
		strings.Contains(reason, "机器人已确认掉线") ||
		strings.Contains(reason, "机器人无响应倒计时结束") ||
		(strings.Contains(reason, "持续") && strings.Contains(reason, "未能从网易租赁服获得数据")) {
		return 0
	}
	return importReconnectWatchdogDrainDelay
}

func waitImportReconnectWatchdogDrain(client *clientpkg.Client, needsNBTBot bool, progress *function.ImportGameProgress, reason string, sleeper func(time.Duration)) {
	delay := importReconnectWatchdogDrainDelayForReason(reason)
	if delay <= 0 {
		return
	}
	if sleeper == nil {
		sleeper = time.Sleep
	}
	if progress != nil {
		progress.SetPhase("等待旧机器人重启倒计时")
		progress.SetBuilderStatus("等待重启")
		if needsNBTBot {
			progress.SetNBTStatus("等待重启")
		}
		progress.SendToClientNow(client)
	}
	sleeper(delay)
}

func waitImportClientOP(client *clientpkg.Client, progress *function.ImportGameProgress) bool {
	log.Log.Info("等待机器人获得 OP 权限（<=180秒）")
	if progress != nil {
		progress.SetPhase("准备授权")
		progress.SetBuilderStatus("等待 OP")
	}
	for i := 0; i < 180; i++ {
		client.IsOP_loop.Lock()
		isOP := client.IsOP
		client.IsOP_loop.Unlock()
		if isOP {
			log.Log.Info(fmt.Sprintf("机器人 %s 已获得 OP 权限", client.Conn.IdentityData().DisplayName))
			if progress != nil {
				progress.SetBuilderStatus("在线 待命")
			}
			return true
		}
		log.Log.Info(fmt.Sprintf("请给机器人 %s 授予 OP 权限", client.Conn.IdentityData().DisplayName))
		time.Sleep(time.Second)
	}
	return false
}

func applyImportClientState(client *clientpkg.Client, tasks *appcontrol.Task, dimInfo dimension.Info) {
	tasks.CloseCommandBlock = true
	function.CleanupNexusTickingAreas(client)
	client.CommandDimension = dimInfo.Name
	client.DimensionID = dimInfo.ID

	if client.Cdump_Setting == nil {
		client.Cdump_Setting = &clientpkg.Cdump_Setting{
			Speed:      appcontrol.DefaultImportSpeed,
			RegionSize: 1,
		}
	}
	if tasks.ImportSpeed > 0 {
		client.Cdump_Setting.Speed = tasks.ImportSpeed
	}
	if tasks.UseFill {
		if tasks.RegionSize > 0 {
			client.Cdump_Setting.RegionSize = tasks.RegionSize
		} else {
			client.Cdump_Setting.RegionSize = 4
		}
	} else {
		client.Cdump_Setting.RegionSize = 1
	}
	client.Cdump_Setting.Clear_Building = tasks.ClearArea
	client.Cdump_Setting.Clear_Drops = tasks.ClearDrops
	client.Cdump_Setting.Close_Command_Block = tasks.CloseCommandBlock
	client.Cdump_Setting.Close_Sign = tasks.DefaultSignWax
}

func buildImportTaskConfig(tasks *appcontrol.Task, finalFileName string) types.Task {
	tasks.CloseCommandBlock = true
	return types.Task{
		FilePath:           finalFileName,
		XYZ:                [3]int{tasks.X, tasks.Y, tasks.Z},
		ImportNBT:          tasks.ImportNBT,
		ImportCommandBlock: tasks.ImportCommandBlock,
		UseFill:            tasks.UseFill,
		RegionSize:         tasks.RegionSize,
		ClearArea:          tasks.ClearArea,
		ClearDrops:         tasks.ClearDrops,
		AutoPlaceDenyBlock: tasks.AutoPlaceDenyBlock,
		AutoPlaceBorder:    tasks.AutoPlaceBorder,
		CloseCommandBlock:  tasks.CloseCommandBlock,
		DefaultSignWax:     tasks.DefaultSignWax,
		EnterRepairDirect:  tasks.EnterRepairDirect,
		CommandDataSpeed:   tasks.CommandDataSpeed,
		CropEnabled:        tasks.CropEnabled,
		CropMin:            tasks.CropMin,
		CropMax:            tasks.CropMax,
		Dimension:          tasks.Dimension,
		ResumeProcessed:    tasks.ResumeProcessed,
		ResumeTotal:        tasks.ResumeTotal,
	}
}

func markImportActivity(activity *atomic.Int64) {
	if activity == nil {
		return
	}
	activity.Store(time.Now().UnixNano())
}

func startImportStallWatchdog(clientProvider func() *clientpkg.Client, activity *atomic.Int64, stop <-chan struct{}) {
	if clientProvider == nil || activity == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(importStallCheckEvery)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}

			lastUnix := activity.Load()
			if lastUnix <= 0 {
				continue
			}
			last := time.Unix(0, lastUnix)
			if time.Since(last) < importStallAbortDelay {
				continue
			}
			client := clientProvider()
			if client == nil {
				continue
			}
			if !strings.Contains(client.LastImportError, accessPointNoResponseImportReasonPrefix) {
				if !shouldAutoReconnectImport(client) {
					client.LastImportError = accessPointNoResponseImportReason(55)
				} else if strings.TrimSpace(client.LastImportError) == "" {
					client.LastImportError = accessPointNoResponseImportReason(55)
				}
				if strings.TrimSpace(client.LastImportError) != "" {
					log.Log.Warn(client.LastImportError)
				}
			}
			closeImportClient(client)
			return
		}
	}()
}

func setAccessPointOutputClientProvider(provider func() *clientpkg.Client) {
	accessPointOutputMu.Lock()
	accessPointOutputClient = provider
	accessPointOutputMu.Unlock()
}

func accessPointOutputCurrentClient() *clientpkg.Client {
	accessPointOutputMu.RLock()
	provider := accessPointOutputClient
	accessPointOutputMu.RUnlock()
	if provider == nil {
		return nil
	}
	return provider()
}

func handleAccessPointOutput(line string) {
	countdown, ok := parseAccessPointNoResponseCountdown(line)
	if !ok {
		return
	}
	client := accessPointOutputCurrentClient()
	if client == nil {
		return
	}
	if strings.TrimSpace(client.LastImportError) == "" ||
		!strings.Contains(client.LastImportError, accessPointNoResponseImportReasonPrefix) {
		client.LastImportError = accessPointNoResponseImportReason(countdown)
		log.Log.Warn(client.LastImportError)
	}
	if countdown > accessPointNoResponseAbortWindowSeconds {
		return
	}
	closeImportClient(client)
}

func parseAccessPointNoResponseCountdown(line string) (float64, bool) {
	text := strings.Join(strings.Fields(strings.TrimSpace(stripANSI(line))), " ")
	if text == "" || !strings.Contains(text, "机器人无响应") || !strings.Contains(text, "将在") || !strings.Contains(text, "秒") {
		return 0, false
	}
	start := strings.Index(text, "将在")
	if start < 0 {
		return 0, false
	}
	rest := text[start+len("将在"):]
	end := strings.Index(rest, "秒")
	if end < 0 {
		return 0, false
	}
	value := strings.TrimSpace(rest[:end])
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return seconds, true
}

func runImportOnce(client *clientpkg.Client, filePath string, tasks *appcontrol.Task, taskConfig types.Task, progressSaver func(processed int, total int), gameProgress *function.ImportGameProgress) bool {
	var activity atomic.Int64
	markImportActivity(&activity)
	stopWatchdog := make(chan struct{})
	var stopOnce sync.Once
	startImportStallWatchdog(func() *clientpkg.Client { return client }, &activity, stopWatchdog)
	defer stopOnce.Do(func() { close(stopWatchdog) })

	progressWithActivity := func(processed int, total int) {
		markImportActivity(&activity)
		if progressSaver != nil {
			progressSaver(processed, total)
		}
	}
	if gameProgress != nil {
		gameProgress.SetActivityHook(func() {
			markImportActivity(&activity)
		})
		defer gameProgress.SetActivityHook(nil)
	}

	ok := function.Cdump_import(client, filePath, tasks.X, tasks.Y, tasks.Z, false, tasks.NZ, nil, "", taskConfig, progressWithActivity, gameProgress)
	markImportActivity(&activity)
	return ok
}

func prepareImportClient(client *clientpkg.Client, tasks *appcontrol.Task, dimInfo dimension.Info, gameProgress *function.ImportGameProgress) bool {
	if !waitImportClientOP(client, gameProgress) {
		return false
	}
	applyImportClientState(client, tasks, dimInfo)
	return true
}

func recoverImportClient(tasks *appcontrol.Task, config *appcontrol.Config, dimInfo dimension.Info, needsNBTBot bool, clientRef **clientpkg.Client, gameProgress *function.ImportGameProgress, attempt int) bool {
	if clientRef == nil {
		return false
	}
	oldClient := *clientRef
	reason := importReconnectReason(oldClient)
	log.Log.Warn(fmt.Sprintf("导入连接中断，准备自动重连并从 %d%% 继续 (%d/%d)", tasks.NZ, attempt, autoReconnectImportLimit))
	if reason != "" {
		log.Log.Warn("导入连接中断原因: " + reason)
	}
	if gameProgress != nil {
		gameProgress.SetPhase("连接中断 正在恢复")
		gameProgress.SetBuilderStatus("等待重启")
		if needsNBTBot {
			gameProgress.SetNBTStatus("等待重启")
		}
		gameProgress.SendToClientNow(oldClient)
	}

	if needsNBTBot {
		invalidateFlowersRuntime()
	}
	closeImportClient(oldClient)
	waitImportReconnectWatchdogDrain(oldClient, needsNBTBot, gameProgress, reason, time.Sleep)

	reconnectedClient, reconnectErr := connectImportClient(tasks, config)
	if reconnectErr != nil {
		log.Log.Error("自动重连失败", log.Log.ArgsFromMap(map[string]any{
			"err": reconnectErr.Error(),
		}))
		return false
	}
	*clientRef = reconnectedClient

	if gameProgress != nil {
		gameProgress.SetPhase("主建机器人重新进服")
		gameProgress.SetBuilderStatus("等待 OP")
		if needsNBTBot {
			gameProgress.SetNBTStatus("等待主建")
		}
		gameProgress.SendToClientNow(reconnectedClient)
	}
	if !prepareImportClient(reconnectedClient, tasks, dimInfo, gameProgress) {
		return false
	}

	if !needsNBTBot {
		if gameProgress != nil {
			gameProgress.SetPhase("准备继续导入")
			gameProgress.SetBuilderStatus("运行中")
		}
		return true
	}

	if gameProgress != nil {
		gameProgress.SetPhase("等待两个机器人重新就绪")
		gameProgress.SetBuilderStatus("在线 等待 NBT")
		gameProgress.SetNBTStatus("重新进服")
	}
	if !restartFlowersForImport(tasks, clientRef, config, gameProgress) {
		return false
	}
	if gameProgress != nil {
		gameProgress.SetPhase("准备继续导入")
		gameProgress.SetBuilderStatus("运行中")
		gameProgress.SetNBTStatus("在线待命")
	}
	return true
}

func startTask(console *consolepkg.Console_input, tasks *appcontrol.Task, config *appcontrol.Config) {
	log.Log.Info("正在读取任务信息...")
	if tasks == nil {
		return
	}
	tasks.CloseCommandBlock = true

	taskType := strings.ToLower(strings.TrimSpace(tasks.TaskType))
	if taskType == "" {
		taskType = "import"
	}
	if taskType == "export" {
		startExportTask(console, tasks, config)
		return
	}
	if len(tasks.BatchImports) > 0 {
		startBatchImportTask(console, tasks, config)
		return
	}

	finalFileName := filepath.Base(tasks.FileName)
	if !strings.HasSuffix(strings.ToLower(finalFileName), ".mcworld") {
		log.Log.Error("只支持 .mcworld 文件")
		common.ExitAfterPrompt(nil, 0)
	}

	filePath := appcontrol.ResolveImportFilePath(finalFileName)
	if !file.Is_File(filePath) {
		log.Log.Error(fmt.Sprintf("文件不存在: %s", finalFileName))
		common.ExitAfterPrompt(nil, 0)
	}

	dimInfo, err := dimension.Parse(tasks.Dimension)
	if err != nil {
		log.Log.Error("维度参数错误: " + err.Error())
		common.WaitForExit(nil)
		return
	}

	taskConfig := buildImportTaskConfig(tasks, finalFileName)

	client, err := connectImportClient(tasks, config)
	if err != nil {
		log.Log.Error("连接客户端失败", log.Log.ArgsFromMap(map[string]any{
			"err": err.Error(),
		}))
		common.ExitAfterPrompt(nil, 0)
	}

	needsNBTBot := tasks.ImportNBT
	gameProgress := function.NewImportGameProgress("准备导入")
	if needsNBTBot {
		gameProgress.SetNBTStatus("准备登录")
	}
	stopGameProgress := gameProgress.Start(func() *clientpkg.Client { return client })
	defer stopGameProgress()
	setAccessPointOutputClientProvider(func() *clientpkg.Client { return client })
	defer setAccessPointOutputClientProvider(nil)

	if !prepareImportClient(client, tasks, dimInfo, gameProgress) {
		common.WaitForExit(nil)
		return
	}

	if tasks.EnterRepairDirect {
		if startRepairMode(client, tasks, filePath, gameProgress) {
			select {}
		}
		log.Log.Error("进入修补模式失败，程序退出")
		common.WaitForExit(nil)
		return
	}

	gameProgress.SetTitleMuted(appcontrol.ShouldSuppressLocalImportTitle(tasks))
	if needsNBTBot {
		gameProgress.SetPhase("等待 NBT 机器人就绪")
		gameProgress.SetBuilderStatus("在线 等待 NBT")
		gameProgress.SetNBTStatus("启动中")
		startFlowersForMachines(tasks, &client, config, gameProgress)
		gameProgress.SetPhase("等待开始")
		gameProgress.SetBuilderStatus("在线 待命")
		gameProgress.SetNBTStatus("在线待命")
		gameProgress.SendToClientNow(client)
	}

	stopImportCommands := startImportCommandListener(console, &client)
	if stopImportCommands != nil {
		log.Log.Info("导入期间可在终端输入 /xxx 发送命令，.xxx 等待返回")
		defer stopImportCommands()
	}

	progressSaver := newTaskProgressSaver(tasks)
	importOK := false
	for attempt := 0; attempt <= autoReconnectImportLimit; attempt++ {
		if runImportOnce(client, filePath, tasks, taskConfig, progressSaver, gameProgress) {
			importOK = true
			break
		}
		if !shouldAutoReconnectImport(client) || attempt >= autoReconnectImportLimit {
			break
		}
		if !recoverImportClient(tasks, config, dimInfo, needsNBTBot, &client, gameProgress, attempt+1) {
			break
		}
	}

	if importOK {
		gameProgress.SetTitleMuted(false)
		log.Log.Info("导入完成")
		gameProgress.SetPhase("导入完成")
		gameProgress.SetBuilderStatus("在线 待命")
		gameProgress.SendToClientNow(client)
		if needsNBTBot && tasks.ImportNBT && FlowersPort > 0 {
			if err := restoreFlowersConsole(FlowersPort); err != nil {
				log.Log.Error("恢复操作台区域失败: " + err.Error())
			}
		}
		_ = os.Remove(appcontrol.TaskFilePath(tasks))
		if startRepairMode(client, tasks, filePath, gameProgress) {
			select {}
		}
		log.Log.Error("进入修补模式失败，程序退出")
		common.WaitForExit(nil)
		return
	}

	if strings.TrimSpace(client.LastImportError) != "" {
		log.Log.Error("导入失败: " + client.LastImportError)
	} else {
		log.Log.Error("导入失败，原因未知")
	}
	gameProgress.SetTitleMuted(false)
	gameProgress.SetPhase("导入失败")
	gameProgress.SendToClientNow(client)
	common.WaitForExit(nil)
}

func startBatchImportTask(console *consolepkg.Console_input, tasks *appcontrol.Task, config *appcontrol.Config) {
	if len(tasks.BatchImports) == 0 {
		return
	}
	tasks.CloseCommandBlock = true
	if tasks.EnterRepairDirect {
		log.Log.Warn("批量导入不支持直接进入修补模式，已自动关闭")
		tasks.EnterRepairDirect = false
	}

	for i := range tasks.BatchImports {
		item := &tasks.BatchImports[i]
		item.FileName = filepath.Base(item.FileName)
		if !strings.HasSuffix(strings.ToLower(item.FileName), ".mcworld") {
			log.Log.Error("批量导入只支持 .mcworld 文件")
			common.WaitForExit(nil)
			return
		}
		if !file.Is_File(appcontrol.ResolveImportFilePath(item.FileName)) {
			log.Log.Error(fmt.Sprintf("批量导入文件不存在: %s", item.FileName))
			common.WaitForExit(nil)
			return
		}
	}

	dimInfo, err := dimension.Parse(tasks.Dimension)
	if err != nil {
		log.Log.Error("维度参数错误: " + err.Error())
		common.WaitForExit(nil)
		return
	}

	client, err := connectImportClient(tasks, config)
	if err != nil {
		log.Log.Error("连接客户端失败", log.Log.ArgsFromMap(map[string]any{
			"err": err.Error(),
		}))
		common.ExitAfterPrompt(nil, 0)
	}

	needsNBTBot := tasks.ImportNBT
	gameProgress := function.NewImportGameProgress("准备导入")
	if needsNBTBot {
		gameProgress.SetNBTStatus("准备登录")
	}
	stopGameProgress := gameProgress.Start(func() *clientpkg.Client { return client })
	defer stopGameProgress()
	setAccessPointOutputClientProvider(func() *clientpkg.Client { return client })
	defer setAccessPointOutputClientProvider(nil)

	if !prepareImportClient(client, tasks, dimInfo, gameProgress) {
		common.WaitForExit(nil)
		return
	}

	gameProgress.SetTitleMuted(appcontrol.ShouldSuppressLocalImportTitle(tasks))
	if needsNBTBot {
		gameProgress.SetPhase("等待 NBT 机器人就绪")
		gameProgress.SetBuilderStatus("在线 等待 NBT")
		gameProgress.SetNBTStatus("启动中")
		startFlowersForMachines(tasks, &client, config, gameProgress)
		gameProgress.SetPhase("等待开始")
		gameProgress.SetBuilderStatus("在线 待命")
		gameProgress.SetNBTStatus("在线待命")
		gameProgress.SendToClientNow(client)
	}

	stopImportCommands := startImportCommandListener(console, &client)
	if stopImportCommands != nil {
		log.Log.Info("导入期间可在终端输入 /xxx 发送命令，.xxx 等待返回")
		defer stopImportCommands()
	}

	for index := range tasks.BatchImports {
		item := &tasks.BatchImports[index]
		if item.NZ >= 100 || (item.ResumeTotal > 0 && item.ResumeProcessed >= item.ResumeTotal) {
			log.Log.Info(fmt.Sprintf("跳过已完成的一条龙建筑 [%d/%d]: %s", index+1, len(tasks.BatchImports), batchItemDisplayName(*item)))
			continue
		}
		filePath := appcontrol.ResolveImportFilePath(item.FileName)
		applyBatchImportItem(tasks, item)
		applyImportClientState(client, tasks, dimInfo)
		taskConfig := buildImportTaskConfig(tasks, item.FileName)
		progressSaver := newBatchTaskProgressSaver(tasks, index)

		log.Log.Info(fmt.Sprintf("开始导入一条龙建筑 [%d/%d]: %s -> %d %d %d", index+1, len(tasks.BatchImports), batchItemDisplayName(*item), item.X, item.Y, item.Z))
		if item.ResumeProcessed > 0 && item.ResumeTotal > 0 {
			log.Log.Info(fmt.Sprintf("将从该建筑断点 %d/%d 区块继续导入", item.ResumeProcessed, item.ResumeTotal))
		} else if item.NZ > 0 {
			log.Log.Info(fmt.Sprintf("将从该建筑断点 %d%% 继续导入", item.NZ))
		}
		importOK := false
		for attempt := 0; attempt <= autoReconnectImportLimit; attempt++ {
			if runImportOnce(client, filePath, tasks, taskConfig, progressSaver, gameProgress) {
				importOK = true
				break
			}
			if !shouldAutoReconnectImport(client) || attempt >= autoReconnectImportLimit {
				break
			}
			if !recoverImportClient(tasks, config, dimInfo, needsNBTBot, &client, gameProgress, attempt+1) {
				break
			}
		}

		if !importOK {
			if strings.TrimSpace(client.LastImportError) != "" {
				log.Log.Error("导入失败: " + client.LastImportError)
			} else {
				log.Log.Error("导入失败，原因未知")
			}
			gameProgress.SetTitleMuted(false)
			gameProgress.SetPhase("导入失败")
			gameProgress.SendToClientNow(client)
			common.WaitForExit(nil)
			return
		}

		item.NZ = 100
		item.ResumeProcessed = 0
		item.ResumeTotal = 0
		tasks.NZ = 0
		tasks.ResumeProcessed = 0
		tasks.ResumeTotal = 0
		saveTaskProgress(tasks)
		log.Log.Info(fmt.Sprintf("一条龙建筑导入完成 [%d/%d]: %s", index+1, len(tasks.BatchImports), batchItemDisplayName(*item)))
	}

	log.Log.Info("一条龙批量导入完成")
	gameProgress.SetTitleMuted(false)
	gameProgress.SetPhase("导入完成")
	gameProgress.SetBuilderStatus("在线 待命")
	gameProgress.SendToClientNow(client)
	if needsNBTBot && tasks.ImportNBT && FlowersPort > 0 {
		if err := restoreFlowersConsole(FlowersPort); err != nil {
			log.Log.Error("恢复操作台区域失败: " + err.Error())
		}
	}
	_ = os.Remove(appcontrol.TaskFilePath(tasks))
}

func applyBatchImportItem(task *appcontrol.Task, item *appcontrol.BatchImportItem) {
	task.FileName = item.FileName
	task.X = item.X
	task.Y = item.Y
	task.Z = item.Z
	task.NZ = item.NZ
	task.ResumeProcessed = item.ResumeProcessed
	task.ResumeTotal = item.ResumeTotal
	task.CropEnabled = item.CropEnabled
	task.CropMin = item.CropMin
	task.CropMax = item.CropMax
}

func batchItemDisplayName(item appcontrol.BatchImportItem) string {
	name := strings.TrimSpace(item.DisplayName)
	if name != "" {
		return name
	}
	return strings.TrimSuffix(filepath.Base(item.FileName), filepath.Ext(item.FileName))
}

func startExportTask(console *consolepkg.Console_input, tasks *appcontrol.Task, config *appcontrol.Config) {
	if !appcontrol.CanExportServerCode(tasks.Server, config != nil && config.AllowPrefixedExport) {
		log.Log.Error(appcontrol.ExportServerCodeRequirement)
		common.WaitForExit(nil)
		return
	}
	tasks.ExportFile = appcontrol.ResolveExportFilePath(tasks.ExportFile)
	if strings.TrimSpace(tasks.ExportFile) == "" {
		log.Log.Error("导出文件路径为空")
		common.WaitForExit(nil)
		return
	}
	if filepath.Ext(tasks.ExportFile) == "" {
		tasks.ExportFile += ".mcworld"
	} else {
		ext := strings.ToLower(filepath.Ext(tasks.ExportFile))
		if ext != ".mcworld" && ext != ".nexus" {
			tasks.ExportFile = strings.TrimSuffix(tasks.ExportFile, filepath.Ext(tasks.ExportFile)) + ".mcworld"
		}
	}

	outputDir := filepath.Dir(tasks.ExportFile)
	if outputDir != "" && outputDir != "." {
		_ = os.MkdirAll(outputDir, 0755)
	}

	_ = os.RemoveAll("cache")
	client, err := mainfunction.Start_Client_by_token(tasks.Server, tasks.Password, config.FBToken, config.ServerURL)
	if err != nil {
		log.Log.Error("连接客户端失败", log.Log.ArgsFromMap(map[string]any{
			"err": err.Error(),
		}))
		common.WaitForExit(nil)
		return
	}

	log.Log.Info("等待机器人获得 OP 权限（<=180秒）")
	isOP := false
	for i := 0; i < 180; i++ {
		client.IsOP_loop.Lock()
		isOP = client.IsOP
		client.IsOP_loop.Unlock()
		if isOP {
			log.Log.Info(fmt.Sprintf("机器人 %s 已获得 OP 权限", client.Conn.IdentityData().DisplayName))
			break
		} else {
			log.Log.Info(fmt.Sprintf("请给机器人 %s 授予 OP 权限", client.Conn.IdentityData().DisplayName))
		}
		time.Sleep(time.Second)
	}
	if !isOP {
		common.WaitForExit(nil)
		return
	}

	if shouldSkipExportTickingAreaCleanup(tasks, config) {
		log.Log.Info("前缀服务器导出，跳过导入常加载区块清理")
	} else {
		function.CleanupNexusTickingAreas(client)
	}

	dimInfo, err := dimension.Parse(tasks.Dimension)
	if err != nil {
		log.Log.Error("维度参数错误: " + err.Error())
		common.WaitForExit(nil)
		return
	}
	client.CommandDimension = dimInfo.Name
	client.DimensionID = dimInfo.ID

	minPos := tasks.ExportMin
	maxPos := tasks.ExportMax
	exportConfig := function.DefaultOptimizedExportConfig()
	exportConfig.SkipOperatorProbe = shouldSkipExportOperatorProbe(tasks, config)
	log.Log.Info("开始导出 mcworld")
	output, err := function.ExportMCWorldOptimized(client, tasks.ExportFile, minPos[0], minPos[1], minPos[2], maxPos[0], maxPos[1], maxPos[2], exportConfig)
	if err != nil {
		log.Log.Error("导出失败", log.Log.ArgsFromMap(map[string]any{
			"error": err.Error(),
		}))
		common.WaitForExit(nil)
		return
	}

	switch strings.ToLower(filepath.Ext(tasks.ExportFile)) {
	case ".nexus":
		author := strings.TrimSpace(tasks.ExportAuthor)
		password := strings.TrimSpace(tasks.ExportPassword)
		nexusPath, err := convertpkg.ConvertMCWorldToNexus(output, tasks.ExportFile, author, password)
		if err != nil {
			log.Log.Error("nexus 转换失败", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			common.WaitForExit(nil)
			return
		}
		_ = os.Remove(output)
		log.Log.Info(fmt.Sprintf("导出完成: %s", nexusPath))
	default:
		log.Log.Info(fmt.Sprintf("导出完成: %s", output))
	}

	_ = os.Remove(appcontrol.TaskFilePath(tasks))
}

func shouldSkipExportTickingAreaCleanup(tasks *appcontrol.Task, config *appcontrol.Config) bool {
	return isAuthorizedPrefixedExport(tasks, config)
}

func shouldSkipExportOperatorProbe(tasks *appcontrol.Task, config *appcontrol.Config) bool {
	return isAuthorizedPrefixedExport(tasks, config)
}

func isAuthorizedPrefixedExport(tasks *appcontrol.Task, config *appcontrol.Config) bool {
	return tasks != nil &&
		config != nil &&
		config.AllowPrefixedExport &&
		!appcontrol.IsExportServerCode(tasks.Server) &&
		appcontrol.CanExportServerCode(tasks.Server, true)
}

func resolveImportBuildOrigin(task *appcontrol.Task) types.Position {
	if task == nil {
		return types.Position{}
	}
	origin := types.Position{X: task.X, Y: task.Y, Z: task.Z}
	if task.AutoPlaceBorder {
		origin.X++
		origin.Z++
	}
	if task.AutoPlaceDenyBlock {
		origin.Y++
	}
	return origin
}

func startRepairMode(client *clientpkg.Client, tasks *appcontrol.Task, filePath string, gameProgress *function.ImportGameProgress) bool {
	if gameProgress != nil {
		gameProgress.SetTitleMuted(false)
	}
	if client.RepairCtx == nil {
		client.RepairCtx = &clientpkg.RepairContext{}
	}

	buildSize, ok := repairBuildSizeFromContext(client.RepairCtx)
	if !ok {
		parsedSize, err := readBuildSize(filePath)
		if err != nil {
			if manualSize, manualErr := promptMCWorldSize(); manualErr == nil {
				buildSize = manualSize
			} else {
				log.Log.Error("读取建筑尺寸失败，无法进入修补模式", log.Log.ArgsFromMap(map[string]any{
					"error": err.Error(),
				}))
				return false
			}
		} else {
			buildSize = parsedSize
		}
	}

	absPath, _ := filepath.Abs(filePath)
	regionSize := client.Cdump_Setting.RegionSize
	if regionSize <= 0 {
		if tasks.UseFill {
			regionSize = 4
		} else {
			regionSize = 1
		}
	}
	buildOrigin := resolveImportBuildOrigin(tasks)

	client.RepairCtx.Setup(
		absPath,
		buildOrigin,
		buildSize,
		regionSize,
		tasks.UseFill,
		client.Cdump_Setting,
		types.Position{X: tasks.X, Y: tasks.Y, Z: tasks.Z},
		tasks.AutoPlaceDenyBlock,
		tasks.AutoPlaceBorder,
	)
	client.RepairCtx.ChatEnabled = true
	client.RepairCtx.ImportCommandBlock = tasks.ImportCommandBlock
	client.RepairCtx.CommandDataSpeed = tasks.CommandDataSpeed

	intro := "当前处于修补模式\n" +
		"请移动到缺失的区块范围内，发送“修补”\n" +
		"如需先清空再重导，发送“清理修补”\n" +
		"随后 NexusEgo[fixer] 将为您修补\n" +
		"!! 注意 !! 任何人都拥有该权限\n" +
		"发送“完成”或“exit”以退出修补模式"
	client.GameInterface.SendWSCommandWithResponse(
		fmt.Sprintf(`tellraw @a {"rawtext":[{"text":%s}]}`, strconv.Quote(intro)),
		ResourcesControl.CommandRequestOptions{TimeOut: 3 * time.Second},
	)
	function.ShowRepairModeIdle(client, gameProgress, tasks.ImportNBT)
	log.Log.Info("已进入修补模式，聊天栏输入“修补”即可修补所在区域")
	return true
}

func repairBuildSizeFromContext(ctx *clientpkg.RepairContext) ([3]int, bool) {
	var zero [3]int
	if ctx == nil || !ctx.Enabled {
		return zero, false
	}
	size := ctx.BuildSize
	if size[0] <= 0 || size[1] <= 0 || size[2] <= 0 {
		return zero, false
	}
	return size, true
}

func readBuildSize(path string) ([3]int, error) {
	if !strings.HasSuffix(strings.ToLower(path), ".mcworld") {
		return [3]int{}, fmt.Errorf("只支持 .mcworld 文件")
	}
	return readMCWorldSizeFromName(path)
}

func readMCWorldSizeFromName(path string) ([3]int, error) {
	var zero [3]int
	minPos, maxPos, ok := parseMCWorldBoundsFromName(path)
	if !ok {
		return zero, fmt.Errorf("无法从文件名解析坐标，请使用格式 @[x1,y1,z1]~[x2,y2,z2]")
	}

	width := absInt(maxPos[0]-minPos[0]) + 1
	height := absInt(maxPos[1]-minPos[1]) + 1
	length := absInt(maxPos[2]-minPos[2]) + 1
	return [3]int{width, height, length}, nil
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func parseMCWorldBoundsFromName(path string) ([3]int, [3]int, bool) {
	var minPos [3]int
	var maxPos [3]int
	base := filepath.Base(path)
	pattern := regexp.MustCompile(`@\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]~\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]`)
	matches := pattern.FindStringSubmatch(base)
	if len(matches) != 7 {
		return minPos, maxPos, false
	}

	x1, _ := strconv.Atoi(matches[1])
	y1, _ := strconv.Atoi(matches[2])
	z1, _ := strconv.Atoi(matches[3])
	x2, _ := strconv.Atoi(matches[4])
	y2, _ := strconv.Atoi(matches[5])
	z2, _ := strconv.Atoi(matches[6])

	minPos = [3]int{minInt(x1, x2), minInt(y1, y2), minInt(z1, z2)}
	maxPos = [3]int{maxInt(x1, x2), maxInt(y1, y2), maxInt(z1, z2)}
	return minPos, maxPos, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func promptMCWorldSize() ([3]int, error) {
	var zero [3]int
	fmt.Println("无法从 .mcworld 文件名解析坐标，请输入两个对角坐标，格式: x1 y1 z1 x2 y2 z2")
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("请输入坐标: ")
		line, err := reader.ReadString('\n')
		if err != nil {
			return zero, err
		}

		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 6 {
			fmt.Println("需要输入 6 个整数，请重新输入。")
			continue
		}

		values := make([]int, 6)
		ok := true
		for i, field := range fields {
			value, err := strconv.Atoi(field)
			if err != nil {
				ok = false
				break
			}
			values[i] = value
		}
		if !ok {
			fmt.Println("坐标必须为整数，请重新输入。")
			continue
		}

		minX, maxX := values[0], values[3]
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		minY, maxY := values[1], values[4]
		if minY > maxY {
			minY, maxY = maxY, minY
		}
		minZ, maxZ := values[2], values[5]
		if minZ > maxZ {
			minZ, maxZ = maxZ, minZ
		}

		return [3]int{maxX - minX + 1, maxY - minY + 1, maxZ - minZ + 1}, nil
	}
}

const importResumeRollbackChunks = 1

type asyncProgressSaveRequest struct {
	processed int
	total     int
}

func newAsyncProgressSaver(save func(processed int, total int)) func(processed int, total int) {
	if save == nil {
		return nil
	}
	ch := make(chan asyncProgressSaveRequest, 1)
	go func() {
		for req := range ch {
			latest := req
			draining := true
			for draining {
				select {
				case next := <-ch:
					latest = next
				default:
					draining = false
				}
			}
			save(latest.processed, latest.total)
		}
	}()
	return func(processed int, total int) {
		req := asyncProgressSaveRequest{processed: processed, total: total}
		select {
		case ch <- req:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- req:
			default:
			}
		}
	}
}

func newTaskProgressSaver(tasks *appcontrol.Task) func(processed int, total int) {
	var (
		mu                 sync.Mutex
		lastSaved          = tasks.NZ
		lastSavedProcessed = tasks.ResumeProcessed
		lastWrite          time.Time
	)

	return newAsyncProgressSaver(func(processed int, total int) {
		if processed <= 0 {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if total <= 0 {
			return
		}

		safeProcessed := processed - importResumeRollbackChunks
		if safeProcessed < 0 {
			safeProcessed = 0
		}
		percent := int(float64(safeProcessed) * 100 / float64(total))
		if percent > 100 {
			percent = 100
		}
		forceSave := processed >= total
		if safeProcessed <= lastSavedProcessed && percent <= lastSaved {
			return
		}
		if !forceSave && percent <= lastSaved && time.Since(lastWrite) < 2*time.Second {
			return
		}

		tasks.NZ = percent
		tasks.ResumeProcessed = safeProcessed
		tasks.ResumeTotal = total
		data, err := json.Marshal(tasks)
		if err != nil {
			log.Log.Error("保存断点进度失败", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			return
		}
		if err := os.WriteFile(appcontrol.TaskFilePath(tasks), data, 0655); err != nil {
			log.Log.Error("写入断点进度失败", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			return
		}
		lastSaved = percent
		lastSavedProcessed = safeProcessed
		lastWrite = time.Now()
	})
}

func newBatchTaskProgressSaver(tasks *appcontrol.Task, itemIndex int) func(processed int, total int) {
	var (
		mu                 sync.Mutex
		lastSaved          int
		lastSavedProcessed int
		lastWrite          time.Time
	)
	if itemIndex >= 0 && itemIndex < len(tasks.BatchImports) {
		lastSaved = tasks.BatchImports[itemIndex].NZ
		lastSavedProcessed = tasks.BatchImports[itemIndex].ResumeProcessed
	}

	return newAsyncProgressSaver(func(processed int, total int) {
		if processed <= 0 || total <= 0 || itemIndex < 0 || itemIndex >= len(tasks.BatchImports) {
			return
		}
		mu.Lock()
		defer mu.Unlock()

		safeProcessed := processed - importResumeRollbackChunks
		if safeProcessed < 0 {
			safeProcessed = 0
		}
		percent := int(float64(safeProcessed) * 100 / float64(total))
		if percent > 100 {
			percent = 100
		}
		forceSave := processed >= total
		if safeProcessed <= lastSavedProcessed && percent <= lastSaved {
			return
		}
		if !forceSave && percent <= lastSaved && time.Since(lastWrite) < 2*time.Second {
			return
		}

		tasks.BatchImports[itemIndex].NZ = percent
		tasks.BatchImports[itemIndex].ResumeProcessed = safeProcessed
		tasks.BatchImports[itemIndex].ResumeTotal = total
		tasks.NZ = percent
		tasks.ResumeProcessed = safeProcessed
		tasks.ResumeTotal = total
		if err := saveTaskProgress(tasks); err != nil {
			log.Log.Error("保存批量断点进度失败", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			return
		}
		lastSaved = percent
		lastSavedProcessed = safeProcessed
		lastWrite = time.Now()
	})
}

func saveTaskProgress(tasks *appcontrol.Task) error {
	data, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	return os.WriteFile(appcontrol.TaskFilePath(tasks), data, 0655)
}

func startImportCommandListener(console *consolepkg.Console_input, clientRef **clientpkg.Client) func() {
	if console == nil || clientRef == nil || *clientRef == nil || (*clientRef).GameInterface == nil {
		return nil
	}

	commandQueue := make(chan string, 32)
	stop := make(chan struct{})

	console.SetFallbackInputHandler(func(line string) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			return
		}
		select {
		case commandQueue <- trimmed:
		default:
			log.Log.Warn("导入命令队列已满，已丢弃输入")
		}
	})

	go func() {
		for {
			select {
			case <-stop:
				return
			case line := <-commandQueue:
				handleImportCommand(*clientRef, line)
			}
		}
	}()

	return func() {
		console.SetFallbackInputHandler(nil)
		close(stop)
	}
}

func handleImportCommand(client *clientpkg.Client, line string) {
	if client == nil || client.GameInterface == nil {
		return
	}

	rawInput := strings.TrimSpace(line)
	trimmed := rawInput
	if trimmed == "" {
		return
	}

	waitResponse := false
	switch {
	case strings.HasPrefix(trimmed, "."):
		waitResponse = true
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "."))
	case strings.HasPrefix(trimmed, "/"):
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "/"))
	default:
		log.Log.Warn("请输入以 / 或 . 开头的命令")
		return
	}

	trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "/"))
	if trimmed == "" {
		return
	}

	command := client.WrapCommandInDimension(trimmed)
	if waitResponse {
		resp := client.GameInterface.SendWSCommandWithResponse(
			command,
			ResourcesControl.CommandRequestOptions{TimeOut: 5 * time.Second},
		)
		if resp.Error != nil {
			log.Log.Error("命令执行失败", log.Log.ArgsFromMap(map[string]any{
				"error": resp.Error.Error(),
				"input": rawInput,
			}))
			return
		}
		if resp.Respond != nil && len(resp.Respond.OutputMessages) > 0 {
			for _, msg := range resp.Respond.OutputMessages {
				if msg.Message == "" {
					continue
				}
				log.Log.Info("命令回显", log.Log.ArgsFromMap(map[string]any{
					"message": humanizeCommandMessage(msg.Message),
				}))
			}
			log.Log.Info("命令执行完成", log.Log.ArgsFromMap(map[string]any{
				"input": rawInput,
			}))
		} else {
			log.Log.Info("命令执行完成", log.Log.ArgsFromMap(map[string]any{
				"input": rawInput,
				"exec":  command,
			}))
		}
		return
	}

	if err := client.GameInterface.SendWSCommand(command); err != nil {
		log.Log.Error("命令发送失败", log.Log.ArgsFromMap(map[string]any{
			"error": err.Error(),
			"input": rawInput,
		}))
		return
	}
	log.Log.Info("命令已发送", log.Log.ArgsFromMap(map[string]any{
		"input": rawInput,
	}))
}

func humanizeCommandMessage(message string) string {
	switch strings.TrimSpace(message) {
	case "commands.generic.syntax":
		return "命令语法错误"
	case "commands.generic.unknown":
		return "未知命令"
	case "commands.generic.permission":
		return "权限不足"
	case "commands.generic.exception":
		return "命令执行异常"
	default:
		return message
	}
}

// pickFlowersAuthServer 返回 NBT 处理器登录用的认证服与 token。
// NBT 处理器固定走 langtu.flyshop.chat 认证。
func pickFlowersAuthServer(serverCode string, config *appcontrol.Config) (string, string) {
	return "http://langtu.flyshop.chat", "NE/dfhgddgh21132"
}

type flowersHealth struct {
	Alive     bool   `json:"alive"`
	ErrorInfo string `json:"error_info"`
}

type flowersLaunch struct {
	port       int
	generation uint64
	crashed    chan error
}

func installFlowersQuietOutput() {
	flowersQuiet.Do(func() {
		originalStdout := os.Stdout
		reader, writer, err := os.Pipe()
		if err == nil {
			os.Stdout = writer
			go relayFilteredFlowersStdout(reader, originalStdout)
		}

		gin.SetMode(gin.ReleaseMode)
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = os.Stderr

		filter := flowersOutputFilter{writer: originalStdout}
		pterm.SetDefaultOutput(filter)
		pterm.Info.Writer = filter
		pterm.Warning.Writer = filter
		pterm.Success.Writer = filter
		pterm.Debug.Writer = filter
		pterm.Description.Writer = filter
		pterm.Error.Writer = os.Stderr
		pterm.Fatal.Writer = os.Stderr
	})
}

func relayFilteredFlowersStdout(reader *os.File, writer io.Writer) {
	buf := make([]byte, 4096)
	var pending string
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			pending += string(buf[:n])
			pending = writeCompleteFlowersOutputLines(pending, writer)
			if pending != "" && !couldBeHiddenFlowersPartial(pending) {
				_, _ = io.WriteString(writer, pending)
				pending = ""
			}
		}
		if err != nil {
			if pending != "" {
				handleAccessPointOutput(pending)
				requestFlowersOPFromOutput(pending)
				if !shouldHideFlowersOutput(pending) {
					_, _ = io.WriteString(writer, pending)
				}
			}
			return
		}
	}
}

func writeCompleteFlowersOutputLines(text string, writer io.Writer) string {
	for {
		idx := strings.IndexByte(text, '\n')
		if idx < 0 {
			return text
		}
		line := text[:idx+1]
		handleAccessPointOutput(line)
		requestFlowersOPFromOutput(line)
		if !shouldHideFlowersOutput(line) {
			_, _ = io.WriteString(writer, line)
		}
		text = text[idx+1:]
	}
}

func couldBeHiddenFlowersPartial(text string) bool {
	compact := strings.Join(strings.Fields(strings.TrimSpace(stripANSI(text))), " ")
	if compact == "" {
		return true
	}
	prefixes := []string{"[GIN", "[INFO", "[DONE", "WARNING", "SUCCESS"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(prefix, compact) || strings.HasPrefix(compact, prefix) {
			return true
		}
	}
	return false
}

type flowersOutputFilter struct {
	writer io.Writer
}

func (f flowersOutputFilter) Write(p []byte) (int, error) {
	for _, line := range splitOutputLines(string(p)) {
		handleAccessPointOutput(line)
		requestFlowersOPFromOutput(line)
		if shouldHideFlowersOutput(line) {
			continue
		}
		if _, err := io.WriteString(f.writer, line); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

func splitOutputLines(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.SplitAfter(text, "\n")
	lines := parts[:0]
	for _, part := range parts {
		if part != "" {
			lines = append(lines, part)
		}
	}
	return lines
}

func setFlowersOPRequestSink(sink chan<- string) {
	flowersOPRequestMu.Lock()
	flowersOPRequestSink = sink
	flowersOPRequestMu.Unlock()
}

func requestFlowersOPFromOutput(line string) {
	botName := parseFlowersOPRequestName(line)
	if botName == "" {
		return
	}
	flowersOPRequestMu.RLock()
	sink := flowersOPRequestSink
	flowersOPRequestMu.RUnlock()
	if sink == nil {
		return
	}
	select {
	case sink <- botName:
	default:
	}
}

func parseFlowersOPRequestName(line string) string {
	text := strings.TrimSpace(stripANSI(line))
	if text == "" {
		return ""
	}
	candidates := []string{
		text,
		strings.Join(strings.Fields(text), " "),
	}
	for _, candidate := range candidates {
		if name := parseFlowersOPRequestNameFromText(candidate); name != "" {
			return name
		}
	}
	return ""
}

func parseFlowersOPRequestNameFromText(text string) string {
	lower := strings.ToLower(text)
	if !strings.Contains(text, "请给予") &&
		!strings.Contains(text, "给予") &&
		!strings.Contains(text, "give") &&
		!strings.Contains(lower, "op ") &&
		!strings.Contains(lower, "/op ") {
		return ""
	}
	if !strings.Contains(text, "管理员") &&
		!strings.Contains(text, "管理員") &&
		!strings.Contains(lower, "op") {
		return ""
	}
	if !strings.Contains(text, "请给予") && !strings.Contains(text, "给予") {
		if name := parseFlowersOPCommandTarget(text); name != "" {
			return name
		}
	}
	start := strings.Index(text, "请给予")
	if start < 0 {
		start = strings.Index(text, "给予")
	}
	if start < 0 {
		return parseFlowersOPCommandTarget(text)
	}
	advance := len("给予")
	if strings.HasPrefix(text[start:], "请给予") {
		advance = len("请给予")
	}
	rest := text[start+advance:]
	for _, suffix := range []string{"管理员权限", "管理员", "管理員權限", "管理員", "OP权限", "OP", "op权限", "op"} {
		if end := strings.Index(rest, suffix); end >= 0 {
			rest = rest[:end]
			break
		}
	}
	rest = strings.Trim(rest, " \t\r\n:：,，.。\"'`[]()（）<>")
	return sanitizeBotName(rest)
}

func parseFlowersOPCommandTarget(text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	for i, field := range fields {
		field = strings.Trim(strings.ToLower(field), " \t\r\n:：,，.。\"'`[]()（）<>")
		if field != "op" && field != "/op" {
			continue
		}
		if i+1 >= len(fields) {
			return ""
		}
		return sanitizeBotName(strings.Trim(fields[i+1], " \t\r\n:：,，.。\"'`[]()（）<>"))
	}
	return ""
}

func shouldHideFlowersOutput(line string) bool {
	text := strings.TrimSpace(stripANSI(line))
	if text == "" {
		return false
	}
	compact := strings.Join(strings.Fields(text), " ")
	lowerCompact := strings.ToLower(compact)
	if strings.HasPrefix(compact, "[GIN-debug]") ||
		strings.HasPrefix(compact, "[GIN]") ||
		strings.Contains(lowerCompact, "message from auth server") ||
		strings.Contains(lowerCompact, "auth server:") ||
		strings.Contains(lowerCompact, "raabel") ||
		strings.Contains(lowerCompact, "starshuttler") ||
		(strings.HasPrefix(compact, "[INFO]") && strings.Contains(compact, "NBT") && strings.Contains(compact, "OP")) ||
		strings.Contains(compact, "commands.generic.noTargetMatch") ||
		strings.Contains(compact, "commands.op.failed") ||
		(strings.Contains(compact, "缺少管理员权限") && strings.Contains(compact, "请给予")) ||
		strings.Contains(compact, "缺少管理员权限，请给予") ||
		strings.Contains(compact, "来自验证服务器的消息") ||
		strings.Contains(compact, "来自认证服务器的消息") ||
		strings.Contains(compact, "成功了，请您关闭调试信息后重新启动测试") ||
		strings.Contains(compact, "正在连接到验证服务器") ||
		strings.Contains(compact, "正在连接认证服") ||
		strings.Contains(compact, "成功与验证服务器建立连接") ||
		strings.Contains(compact, "认证服连接完成") ||
		strings.Contains(compact, "正在生成客户端密钥对") ||
		strings.Contains(compact, "正在从验证服务器取得机器人数据") ||
		strings.Contains(compact, "正在连接到 Minecraft") ||
		strings.Contains(compact, "正在连接 Minecraft") ||
		strings.Contains(compact, "正在建立 raknet 连接") ||
		strings.Contains(compact, "正在建立租赁服连接") ||
		strings.Contains(compact, "正在建立本地联机连接") ||
		strings.Contains(compact, "正在封装数据帧连接层") ||
		strings.Contains(compact, "正在封装数据包连接层") ||
		strings.Contains(compact, "正在生成关键登陆数据") ||
		strings.Contains(compact, "登陆序列完成") ||
		strings.Contains(compact, "正在发送附加消息") ||
		strings.Contains(compact, "正在发送登录后初始化数据") ||
		strings.Contains(compact, "登录后初始化完成") ||
		strings.Contains(compact, "正在打包关键数据") ||
		strings.Contains(compact, "成功连接到 Minecraft") ||
		strings.Contains(compact, "Minecraft 连接完成") ||
		strings.Contains(compact, "正在尝试完成网易要求的零知识机器人身份证明") ||
		strings.Contains(compact, "正在处理租赁服挑战校验") ||
		strings.Contains(compact, "正在检查机器人的权限和租赁服内作弊模式是否打开") ||
		strings.Contains(compact, "成功完成网易要求的零知识机器人身份证明") ||
		strings.Contains(compact, "租赁服挑战校验完成") {
		return true
	}
	return false
}

func stripANSI(text string) string {
	return ansiPattern.ReplaceAllString(text, "")
}

func startFlowersForMachinesLegacy(tasks *appcontrol.Task, clientRef **clientpkg.Client, config *appcontrol.Config) {
	installFlowersQuietOutput()
	if clientRef == nil || *clientRef == nil {
		log.Log.Error("NBT 处理器启动失败: 客户端不可用")
		return
	}
	client := *clientRef
	port, err := getFreePort()
	if err != nil {
		log.Log.Error("无法获取可用端口: " + err.Error())
		return
	}
	FlowersPort = port
	FlowersReady = false

	NBTAssigner.GetFlowersPort = func() int { return FlowersPort }
	NBTAssigner.GetFlowersReady = func() bool { return FlowersReady }
	NBTAssigner.PlaceNBTBlockAtPositionViaHTTP = func(port int, blockName string, blockStates string, blockNBT map[string]interface{}, dimensionID uint8, x, y, z int) (map[string]interface{}, error) {
		return placeNBTBlockAtPositionViaHTTP(port, blockName, blockStates, blockNBT, dimensionID, x, y, z)
	}
	NBTAssigner.SendCommand = func(cmd string) {
		log.Log.Info("发送 NBT 处理器命令: " + cmd)
		current := *clientRef
		if current == nil || current.GameInterface == nil {
			return
		}
		current.GameInterface.SendCommandWithResponse(cmd, ResourcesControl.CommandRequestOptions{})
	}

	dimensionID := detectCurrentDimension(client)
	consoleX := tasks.X
	consoleY := 310
	consoleZ := tasks.Z

	log.Log.Info("正在启动 NBT 处理器用于 NBT 处理...")

	crashed := make(chan bool, 1)
	opRequested := make(chan string, 1)
	setFlowersOPRequestSink(opRequested)
	botName, botNameErr := resolveFlowersBotName(tasks, config)
	if botNameErr != nil {
		log.Log.Warn("无法预获取 NBT 机器人名称，将等待 NBT 处理器管理员提示自动授权", log.Log.ArgsFromMap(map[string]any{
			"error": botNameErr.Error(),
		}))
	} else {
		_, watchErr := startFlowersOPWatcher(client, botName, opRequested)
		if watchErr != nil {
			log.Log.Warn("NBT 处理器 OP 监听启动失败", log.Log.ArgsFromMap(map[string]any{
				"error": watchErr.Error(),
			}))
		}
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Log.Error("NBT 处理器服务崩溃: " + fmt.Sprintf("%v", recovered))
				crashed <- true
			}
		}()

		authServer, authToken := pickFlowersAuthServer(tasks.Server, config)
		serverCode, serverPasscode := flowersServiceTarget(tasks.Server, tasks.Password)
		service.RunServer(
			serverCode,
			serverPasscode,
			authServer,
			authToken,
			port,
			int(dimensionID),
			consoleX,
			consoleY,
			consoleZ,
		)
	}()
	if botNameErr == nil && botName != "" {
		startFlowersOPRetry(botName, opRequested, 0, 90*time.Second)
	} else {
		log.Log.Warn("无法确定 NBT 机器人名称，正在等待 NBT 处理器管理员提示")
	}

	go func() {
		for {
			select {
			case botName := <-opRequested:
				current := *clientRef
				if !isClientCommandAvailable(current) {
					continue
				}
				ok, detail := sendOpCommand(current, "op "+botName)
				if !ok {
					if isCommandNoTargetMatch(detail) || isCommandOPAlreadyHandled(detail) || isTransientOPCommandFailure(detail) {
						continue
					}
					if detail == "" {
						detail = "未知错误"
					}
					log.Log.Error("发送 OP 命令失败: " + detail)
					continue
				}
				log.Log.Info("NBT 处理器 OP 命令已发送，机器人名称: " + botName)
			case <-crashed:
				return
			}
		}
	}()

	log.Log.Info("等待NBT处理器服务完全就绪...")
	deadline := time.Now().Add(60 * time.Second)
	for {
		select {
		case <-crashed:
			log.Log.Info("NBT 处理器已崩溃，程序即将退出")
			common.ExitAfterPrompt(nil, 1)
			return
		default:
		}
		if isFlowersServiceReady(port) {
			log.Log.Info("NBT处理器服务已完全就绪")
			FlowersReady = true
			return
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(time.Second)
	}

	log.Log.Error("NBT处理器服务在 60 秒内未能完成启动")
	common.ExitAfterPrompt(nil, 1)
}

func startFlowersForMachines(tasks *appcontrol.Task, clientRef **clientpkg.Client, config *appcontrol.Config, gameProgress *function.ImportGameProgress) {
	if !startFlowersRuntime(tasks, clientRef, config, gameProgress, true) {
		common.ExitAfterPrompt(nil, 1)
	}
}

func restartFlowersForImport(tasks *appcontrol.Task, clientRef **clientpkg.Client, config *appcontrol.Config, gameProgress *function.ImportGameProgress) bool {
	log.Log.Info("正在重新启动 NBT 机器人用于继续导入...")
	return startFlowersRuntime(tasks, clientRef, config, gameProgress, false)
}

func invalidateFlowersRuntime() {
	flowersGen.Add(1)
	setFlowersState(0, false)
	setFlowersOPRequestSink(nil)
	if err := service.CloseCurrentBotConnection(); err != nil {
		log.Log.Warn("关闭旧 NBT 机器人连接失败，继续尝试重新进服", log.Log.ArgsFromMap(map[string]any{
			"error": err.Error(),
		}))
	}
}

func startFlowersRuntime(tasks *appcontrol.Task, clientRef **clientpkg.Client, config *appcontrol.Config, gameProgress *function.ImportGameProgress, exitOnFailure bool) bool {
	installFlowersQuietOutput()
	if clientRef == nil || *clientRef == nil {
		log.Log.Error("NBT 处理器启动失败: 客户端不可用")
		return false
	}

	client := *clientRef
	generation := flowersGen.Add(1)
	setFlowersState(0, false)

	NBTAssigner.GetFlowersPort = getFlowersPort
	NBTAssigner.GetFlowersReady = getFlowersReady
	NBTAssigner.PlaceNBTBlockAtPositionViaHTTP = func(port int, blockName string, blockStates string, blockNBT map[string]interface{}, dimensionID uint8, x, y, z int) (map[string]interface{}, error) {
		return placeNBTBlockAtPositionViaHTTP(port, blockName, blockStates, blockNBT, dimensionID, x, y, z)
	}
	NBTAssigner.SendCommand = func(cmd string) {
		log.Log.Info("发送 NBT 处理器命令: " + cmd)
		current := *clientRef
		if current == nil || current.GameInterface == nil {
			return
		}
		current.GameInterface.SendCommandWithResponse(cmd, ResourcesControl.CommandRequestOptions{})
	}

	dimensionID := detectCurrentDimension(client)
	consoleX := tasks.X
	consoleY := 310
	consoleZ := tasks.Z

	opRequested := make(chan string, 8)
	setFlowersOPRequestSink(opRequested)
	botName, botNameErr := resolveFlowersBotName(tasks, config)
	if botNameErr != nil {
		log.Log.Warn("无法预获取 NBT 机器人名称，将等待 NBT 处理器管理员提示自动授权", log.Log.ArgsFromMap(map[string]any{
			"error": botNameErr.Error(),
		}))
	} else {
		stopOPWatcher, watchErr := startFlowersOPWatcher(client, botName, opRequested)
		if watchErr != nil {
			log.Log.Warn("NBT 处理器 OP 监听启动失败", log.Log.ArgsFromMap(map[string]any{
				"error": watchErr.Error(),
			}))
		} else {
			stopFlowersOPWatcherWhenStale(stopOPWatcher, generation)
		}
	}
	go grantFlowersOP(clientRef, opRequested, generation)

	log.Log.Info("正在启动 NBT 处理器用于 NBT 处理...")
	if gameProgress != nil {
		gameProgress.SetPhase("准备 NBT 机器人")
		gameProgress.SetNBTStatus("正在登录")
	}
	launch, ok := startFlowersLaunch(tasks, config, dimensionID, consoleX, consoleY, consoleZ, generation)
	if !ok {
		invalidateFlowersRuntime()
		return false
	}
	if botNameErr == nil && botName != "" {
		startFlowersOPRetry(botName, opRequested, generation, 90*time.Second)
	} else {
		log.Log.Warn("无法确定 NBT 机器人名称，正在等待 NBT 处理器管理员提示")
	}
	if gameProgress != nil {
		gameProgress.SetPhase("等待 NBT 机器人就绪")
		gameProgress.SetNBTStatus("等待就绪")
	}
	if !waitFlowersLaunchReady(launch, exitOnFailure, client, gameProgress) {
		invalidateFlowersRuntime()
		return false
	}
	if gameProgress != nil {
		gameProgress.SetNBTStatus("在线待命")
		gameProgress.SendToClientNow(client)
	}

	go superviseFlowersForMachines(tasks, config, dimensionID, consoleX, consoleY, consoleZ, launch)
	return true
}

func getFlowersPort() int {
	flowersMu.RLock()
	defer flowersMu.RUnlock()
	return FlowersPort
}

func getFlowersReady() bool {
	flowersMu.RLock()
	defer flowersMu.RUnlock()
	return FlowersReady
}

func setFlowersState(port int, ready bool) {
	flowersMu.Lock()
	FlowersPort = port
	FlowersReady = ready
	flowersMu.Unlock()
}

func isFlowersLaunchCurrent(launch flowersLaunch) bool {
	return isFlowersGenerationCurrent(launch.generation)
}

func setFlowersStateIfCurrent(launch flowersLaunch, port int, ready bool) {
	if isFlowersLaunchCurrent(launch) {
		setFlowersState(port, ready)
	}
}

func isFlowersGenerationCurrent(generation uint64) bool {
	return generation == 0 || flowersGen.Load() == generation
}

func stopFlowersOPWatcherWhenStale(stop func(), generation uint64) {
	if stop == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if !isFlowersGenerationCurrent(generation) {
				stop()
				return
			}
		}
	}()
}

func isClientCommandAvailable(client *clientpkg.Client) bool {
	if client == nil || client.GameInterface == nil || client.Conn == nil {
		return false
	}
	if state, ok := client.Conn.(interface{ Closed() bool }); ok && state.Closed() {
		return false
	}
	return true
}

func grantFlowersOP(clientRef **clientpkg.Client, opRequested <-chan string, generation uint64) {
	granted := map[string]struct{}{}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if !isFlowersGenerationCurrent(generation) {
			return
		}
		var botName string
		select {
		case <-ticker.C:
			continue
		case name, ok := <-opRequested:
			if !ok {
				return
			}
			botName = name
		}
		if !isFlowersGenerationCurrent(generation) {
			return
		}
		botName = sanitizeBotName(botName)
		if botName == "" {
			continue
		}
		key := strings.ToLower(botName)
		if _, ok := granted[key]; ok {
			continue
		}
		current := *clientRef
		if !isClientCommandAvailable(current) {
			continue
		}
		ok, detail := sendOpCommand(current, "op "+botName)
		if !ok {
			if isCommandNoTargetMatch(detail) || isTransientOPCommandFailure(detail) {
				continue
			}
			if detail == "" {
				detail = "未知错误"
			}
			log.Log.Error("发送 OP 命令失败: " + detail)
			continue
		}
		granted[key] = struct{}{}
		log.Log.Info("NBT 处理器 OP 命令已发送，机器人名称: " + botName)
	}
}

func startFlowersOPRetry(botName string, requested chan<- string, generation uint64, duration time.Duration) {
	botName = sanitizeBotName(botName)
	if botName == "" || requested == nil || duration <= 0 {
		return
	}
	go func() {
		timer := time.NewTimer(duration)
		ticker := time.NewTicker(3 * time.Second)
		defer timer.Stop()
		defer ticker.Stop()
		for {
			if !isFlowersGenerationCurrent(generation) {
				return
			}
			select {
			case requested <- botName:
			default:
			}
			select {
			case <-timer.C:
				return
			case <-ticker.C:
			}
		}
	}()
}

type flowersAuthResponse struct {
	SuccessStates bool   `json:"success"`
	ServerMessage string `json:"server_msg,omitempty"`
	Message       string `json:"message,omitempty"`
	Translation   int    `json:"translation,omitempty"`
	ChainInfo     string `json:"chainInfo"`
}

type flowersChainData struct {
	Chain []string `json:"chain"`
}

type flowersIdentityClaims struct {
	ExtraData struct {
		DisplayName string `json:"displayName"`
	} `json:"extraData"`
}

type flowersTanLobbyAuthResponse struct {
	Success        bool   `json:"success"`
	ErrorInfo      string `json:"error_info"`
	UserPlayerName string `json:"user_player_name"`
}

func flowersBotNameAuthTarget(serverCode, serverPasscode string) (string, string, bool) {
	serverCode, serverPasscode = starclient.NormalizeServerTarget(serverCode, serverPasscode)
	return serverCode, serverPasscode, starclient.IsTanLobbyTarget(serverCode)
}

func flowersServiceTarget(serverCode, serverPasscode string) (string, string) {
	return starclient.NormalizeServerTarget(serverCode, serverPasscode)
}

func resolveFlowersBotName(tasks *appcontrol.Task, config *appcontrol.Config) (string, error) {
	if tasks == nil || config == nil {
		return "", fmt.Errorf("任务或配置为空")
	}
	authServer, authToken := pickFlowersAuthServer(tasks.Server, config)
	secret, err := fetchFlowersAuthSecret(authServer)
	if err != nil {
		return "", err
	}
	serverCode, serverPasscode, isTanLobby := flowersBotNameAuthTarget(tasks.Server, tasks.Password)
	if isTanLobby {
		name, err := requestFlowersTanLobbyBotName(authServer, secret, authToken, serverCode)
		if err != nil {
			return "", err
		}
		name = sanitizeBotName(name)
		if name == "" {
			return "", fmt.Errorf("NBT 本地联机认证响应未包含有效机器人名称")
		}
		return name, nil
	}

	publicKey, err := generateFlowersPublicKey()
	if err != nil {
		return "", err
	}
	authResp, err := requestFlowersAuth(authServer, secret, authToken, serverCode, serverPasscode, publicKey)
	if err != nil {
		return "", err
	}
	name, err := parseFlowersBotNameFromChain(authResp.ChainInfo)
	if err != nil {
		return "", err
	}
	name = sanitizeBotName(name)
	if name == "" {
		return "", fmt.Errorf("认证链未包含有效机器人名称")
	}
	return name, nil
}

func requestFlowersTanLobbyBotName(authServer, secret, token, serverCode string) (string, error) {
	_, roomID, ok := strings.Cut(strings.TrimSpace(serverCode), ":")
	if !ok || strings.TrimSpace(roomID) == "" {
		return "", fmt.Errorf("NBT 本地联机入口无效: %s", serverCode)
	}
	reqBody, err := json.Marshal(map[string]string{
		"login_token": token,
		"room_id":     strings.TrimSpace(roomID),
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(authServer, "/")+"/api/phoenix/tan_lobby_login", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+secret)

	client := netutil.NewHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 NBT 本地联机认证信息失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求 NBT 本地联机认证信息失败，HTTP 状态 %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var result flowersTanLobbyAuthResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if !result.Success {
		if strings.TrimSpace(result.ErrorInfo) != "" {
			return "", fmt.Errorf("%s", result.ErrorInfo)
		}
		return "", fmt.Errorf("NBT 本地联机认证信息请求失败")
	}
	return result.UserPlayerName, nil
}

func generateFlowersPublicKey() (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), cryptoRand.Reader)
	if err != nil {
		return "", err
	}
	publicKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(publicKey), nil
}

func fetchFlowersAuthSecret(authServer string) (string, error) {
	resp, err := http.Get(strings.TrimRight(authServer, "/") + "/api/new")
	if err != nil {
		return "", fmt.Errorf("请求 NBT 认证 secret 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求 NBT 认证 secret 失败，HTTP 状态 %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return strings.TrimSpace(string(body)), nil
}

func requestFlowersAuth(authServer, secret, token, serverCode, serverPasscode, publicKey string) (flowersAuthResponse, error) {
	var result flowersAuthResponse
	reqBody, err := json.Marshal(map[string]string{
		"login_token":       token,
		"server_code":       serverCode,
		"server_passcode":   serverPasscode,
		"client_public_key": publicKey,
	})
	if err != nil {
		return result, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(authServer, "/")+"/api/phoenix/login", bytes.NewBuffer(reqBody))
	if err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+secret)

	client := netutil.NewHTTPClient(30 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("请求 NBT 认证信息失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, err
	}
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("请求 NBT 认证信息失败，HTTP 状态 %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, err
	}
	if !result.SuccessStates {
		if strings.TrimSpace(result.Message) != "" {
			return result, fmt.Errorf("%s", result.Message)
		}
		return result, fmt.Errorf("NBT 认证信息请求失败")
	}
	return result, nil
}

func parseFlowersBotNameFromChain(chainInfo string) (string, error) {
	var chain flowersChainData
	if err := json.Unmarshal([]byte(chainInfo), &chain); err != nil {
		return "", fmt.Errorf("解析 NBT 认证链失败: %w", err)
	}
	if len(chain.Chain) < 2 {
		return "", fmt.Errorf("NBT 认证链长度不足")
	}
	token, err := jwt.ParseSigned(chain.Chain[1], []jose.SignatureAlgorithm{jose.ES384})
	if err != nil {
		return "", fmt.Errorf("解析 NBT 机器人凭证失败: %w", err)
	}
	var claims flowersIdentityClaims
	if err := token.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return "", fmt.Errorf("解析 NBT 机器人身份失败: %w", err)
	}
	if strings.TrimSpace(claims.ExtraData.DisplayName) != "" {
		return claims.ExtraData.DisplayName, nil
	}
	return "", fmt.Errorf("NBT 认证链未包含 displayName")
}

func startFlowersLaunch(tasks *appcontrol.Task, config *appcontrol.Config, dimensionID int, consoleX, consoleY, consoleZ int, generation uint64) (flowersLaunch, bool) {
	port, err := getFreePort()
	if err != nil {
		log.Log.Error("无法获取可用端口: " + err.Error())
		return flowersLaunch{}, false
	}

	launch := flowersLaunch{
		port:       port,
		generation: generation,
		crashed:    make(chan error, 1),
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err := fmt.Errorf("%v", recovered)
				log.Log.Error("NBT 处理器服务崩溃: " + err.Error())
				launch.crashed <- err
				return
			}
			launch.crashed <- fmt.Errorf("NBT 处理器服务已退出")
		}()

		authServer, authToken := pickFlowersAuthServer(tasks.Server, config)
		serverCode, serverPasscode := flowersServiceTarget(tasks.Server, tasks.Password)
		service.RunServer(
			serverCode,
			serverPasscode,
			authServer,
			authToken,
			port,
			dimensionID,
			consoleX,
			consoleY,
			consoleZ,
		)
	}()
	return launch, true
}

func waitFlowersLaunchReady(launch flowersLaunch, exitOnCrash bool, client *clientpkg.Client, gameProgress *function.ImportGameProgress) bool {
	log.Log.Info("等待 NBT 处理器服务完全就绪...")
	deadline := time.Now().Add(60 * time.Second)
	for {
		if !isFlowersLaunchCurrent(launch) {
			return false
		}
		select {
		case err := <-launch.crashed:
			setFlowersStateIfCurrent(launch, 0, false)
			log.Log.Error("NBT 处理器已崩溃: " + err.Error())
			return false
		default:
		}
		if isFlowersServiceReady(launch.port) {
			setFlowersStateIfCurrent(launch, launch.port, true)
			log.Log.Info("NBT 处理器服务已完全就绪")
			if gameProgress != nil {
				gameProgress.SetPhase("等待开始")
				gameProgress.SetBuilderStatus("在线 待命")
				gameProgress.SetNBTStatus("在线待命")
				gameProgress.SendToClientNow(client)
			}
			return true
		}
		if time.Now().After(deadline) {
			setFlowersStateIfCurrent(launch, 0, false)
			if exitOnCrash {
				log.Log.Error("NBT 处理器服务在 60 秒内未能完成启动")
			}
			return false
		}
		time.Sleep(time.Second)
	}
}

func superviseFlowersForMachines(tasks *appcontrol.Task, config *appcontrol.Config, dimensionID int, consoleX, consoleY, consoleZ int, launch flowersLaunch) {
	current := launch
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	readySince := time.Now()
	healthFailures := 0

	for {
		if !isFlowersLaunchCurrent(current) {
			return
		}
		select {
		case err := <-current.crashed:
			log.Log.Warn("NBT 处理器退出，准备自动重连", log.Log.ArgsFromMap(map[string]any{
				"error": err.Error(),
			}))
			setFlowersStateIfCurrent(current, 0, false)
		case <-ticker.C:
			if !isFlowersLaunchCurrent(current) {
				return
			}
			if time.Since(readySince) < flowersHealthGracePeriod {
				continue
			}
			if isFlowersServiceReady(current.port) {
				healthFailures = 0
				continue
			}
			healthFailures++
			if healthFailures < flowersHealthMaxFailures {
				continue
			}
			setFlowersStateIfCurrent(current, 0, false)
			log.Log.Warn("NBT 处理器连续健康检查失败，准备自动重连", log.Log.ArgsFromMap(map[string]any{
				"failures": healthFailures,
				"port":     current.port,
			}))
		}

		if elapsed := time.Since(readySince); elapsed < importReconnectWatchdogDrainDelay {
			time.Sleep(importReconnectWatchdogDrainDelay - elapsed)
		}
		time.Sleep(flowersRestartDelay)
		if !isFlowersLaunchCurrent(current) {
			return
		}
		next, ok := startFlowersLaunch(tasks, config, dimensionID, consoleX, consoleY, consoleZ, current.generation)
		if !ok {
			continue
		}
		if !waitFlowersLaunchReady(next, false, nil, nil) {
			continue
		}
		current = next
		readySince = time.Now()
		healthFailures = 0
		log.Log.Info(fmt.Sprintf("NBT 处理器已自动重连，端口切换为 %d", current.port))
	}
}

func isFlowersServiceReady(port int) bool {
	url := fmt.Sprintf("http://localhost:%d/check_alive", port)
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var health flowersHealth
	if err := json.Unmarshal(body, &health); err == nil {
		return health.Alive
	}
	bodyText := strings.TrimSpace(string(body))
	return bodyText == "true" || strings.EqualFold(bodyText, "ok")
}

func startFlowersOPWatcher(client *clientpkg.Client, botName string, requested chan<- string) (func(), error) {
	if client == nil || client.Resources == nil {
		return nil, fmt.Errorf("客户端资源不可用")
	}
	res, ok := client.Resources.(*ResourcesControl.Resources)
	if !ok || res == nil {
		return nil, fmt.Errorf("资源不可用")
	}
	targetName := sanitizeBotName(botName)
	if targetName == "" {
		return nil, fmt.Errorf("NBT 机器人名称为空")
	}

	listenerID, packets := res.Listener.CreateNewListen([]uint32{packet.IDPlayerList}, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		selfName := sanitizeBotName(client.Conn.IdentityData().DisplayName)
		for pk := range packets {
			playerList, ok := pk.(*packet.PlayerList)
			if !ok || playerList.ActionType != packet.PlayerListActionAdd {
				continue
			}
			for _, entry := range playerList.Entries {
				name := sanitizeBotName(entry.Username)
				if name == "" || strings.EqualFold(name, selfName) {
					continue
				}
				if !strings.EqualFold(name, targetName) {
					continue
				}
				select {
				case requested <- name:
				default:
				}
			}
		}
	}()

	return func() {
		_ = res.Listener.StopAndDestroy(listenerID)
		<-done
	}, nil
}

func sanitizeBotName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	var builder strings.Builder
	builder.Grow(len(runes))
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\u00a7' {
			if i+1 < len(runes) {
				i++
			}
			continue
		}
		if runes[i] == '\n' || runes[i] == '\r' || runes[i] == '\t' {
			continue
		}
		builder.WriteRune(runes[i])
	}
	return strings.TrimSpace(builder.String())
}

func sendOpCommand(client *clientpkg.Client, command string) (bool, string) {
	if client == nil || client.GameInterface == nil {
		return false, "客户端不可用"
	}
	opts := ResourcesControl.CommandRequestOptions{TimeOut: 5 * time.Second}

	commands := []string{command}
	if !strings.HasPrefix(strings.TrimSpace(command), "/") {
		commands = append(commands, "/"+command)
	}

	var fallbackDetail string
	recordDetail := func(detail string) {
		if strings.TrimSpace(detail) == "" {
			return
		}
		if fallbackDetail == "" || isCommandNoTargetMatch(fallbackDetail) {
			fallbackDetail = detail
		}
	}

	origins := []uint32{
		protocol.CommandOriginDedicatedServer,
		protocol.CommandOriginDevConsole,
		protocol.CommandOriginGameDirectorEntityServer,
		protocol.CommandOriginEntityServer,
	}

	for _, cmd := range commands {
		resp := client.GameInterface.SendCommandWithResponse(cmd, opts)
		if ok, detail := commandSucceeded(resp); ok {
			return true, ""
		} else {
			recordDetail(detail)
		}

		resp = client.GameInterface.SendWSCommandWithResponse(cmd, opts)
		if ok, detail := commandSucceeded(resp); ok {
			return true, ""
		} else {
			recordDetail(detail)
		}

		for _, origin := range origins {
			resp = client.GameInterface.SendCommandWithOrigin(cmd, origin, opts)
			if ok, detail := commandSucceeded(resp); ok {
				return true, ""
			} else {
				recordDetail(detail)
			}
		}
	}
	return false, fallbackDetail
}

func commandSucceeded(resp ResourcesControl.CommandRespond) (bool, string) {
	if resp.Error != nil {
		return false, resp.Error.Error()
	}
	if resp.Respond == nil || len(resp.Respond.OutputMessages) == 0 {
		return true, ""
	}
	for _, msg := range resp.Respond.OutputMessages {
		if msg.Success {
			return true, ""
		}
		if msg.Message != "" {
			if isCommandOPAlreadyHandled(msg.Message) {
				return true, ""
			}
			return false, humanizeCommandMessage(msg.Message)
		}
	}
	return false, "命令执行失败"
}

func isCommandNoTargetMatch(detail string) bool {
	return strings.Contains(strings.TrimSpace(detail), "commands.generic.noTargetMatch")
}

func isCommandOPAlreadyHandled(detail string) bool {
	return strings.Contains(strings.TrimSpace(detail), "commands.op.failed")
}

func isTransientOPCommandFailure(detail string) bool {
	text := strings.ToLower(strings.TrimSpace(detail))
	if text == "" {
		return false
	}
	return strings.Contains(text, "context deadline exceeded") ||
		strings.Contains(text, "timeout") ||
		strings.Contains(text, "conn dead") ||
		strings.Contains(text, "connection closed") ||
		strings.Contains(text, "closed network connection") ||
		strings.Contains(text, "connection unavailable") ||
		strings.Contains(text, "i/o timeout")
}

type restoreConsoleResponse struct {
	Success   bool   `json:"success"`
	ErrorInfo string `json:"error_info,omitempty"`
}

func restoreFlowersConsole(port int) error {
	if port <= 0 {
		return fmt.Errorf("NBT 处理器端口无效: %d", port)
	}

	url := fmt.Sprintf("http://localhost:%d/restore_console", port)
	resp, err := http.Post(url, "application/json", bytes.NewBufferString("{}"))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("恢复操作台区域失败，HTTP 状态 %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var result restoreConsoleResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if !result.Success {
		if strings.TrimSpace(result.ErrorInfo) == "" {
			return fmt.Errorf("恢复操作台区域失败")
		}
		return fmt.Errorf("%s", result.ErrorInfo)
	}
	return nil
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func detectCurrentDimension(client *clientpkg.Client) int {
	if client == nil {
		return 0
	}
	return int(client.DimensionID)
}

func placeNBTBlockViaHTTP(port int, blockName string, blockStates map[string]interface{}, blockNBT map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("placeNBTBlockViaHTTP 已弃用，请使用 placeNBTBlockAtPositionViaHTTP")
}

func placeNBTBlockAtPositionViaHTTP(port int, blockName string, blockStates string, blockNBT map[string]interface{}, dimensionID uint8, x, y, z int) (map[string]interface{}, error) {
	requestData := map[string]interface{}{
		"block_name":              blockName,
		"block_states_string":     blockStates,
		"block_nbt_base64_string": "",
		"dimension":               dimensionID,
		"x":                       x,
		"y":                       y,
		"z":                       z,
	}
	if len(blockNBT) > 0 {
		buffer := new(bytes.Buffer)
		encoder := nbt.NewEncoderWithEncoding(buffer, nbt.LittleEndian)
		if err := encoder.Encode(blockNBT); err != nil {
			return nil, fmt.Errorf("使用 LittleEndian 编码 NBT 数据失败: %v", err)
		}
		requestData["block_nbt_base64_string"] = base64.StdEncoding.EncodeToString(buffer.Bytes())
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求数据失败: %v", err)
	}
	url := fmt.Sprintf("http://localhost:%d/place_nbt_block", port)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("发送 HTTP 请求失败: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应内容失败: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	if success, ok := response["success"].(bool); !ok || !success {
		return nil, fmt.Errorf("放置 NBT 方块失败: %v", response["error_info"])
	}
	return response, nil
}

// startMapBuilder 是 MapBuilderRunner 的实现：在主流程之外启动 NexusEgo-MapBuilder。
// 复用主机器人接入点（Conbit），不再单开 NBT 处理器。
//
// 流程：先把所有配置问完（服务器号/密码 + 媒体类型/路径/帧率等），
// 再登录进服 + 等 OP，最后让玩家定地图区域并播放。
func startMapBuilder(console *consolepkg.Console_input, config *appcontrol.Config) {
	log.Log.Info("启动 NexusEgo-MapBuilder")
	mapbuilder.SetConsole(console)
	mapbuilder.MediaDir = appcontrol.StorageFileDir()
	if !file.Is_Dir(mapbuilder.MediaDir) {
		_ = os.MkdirAll(mapbuilder.MediaDir, 0755)
	}

	log.Log.Info("服务器配置")

	server, password := appcontrol.PromptServerConfig(console)

	mapCfg := mapbuilder.AskMapConfig()
	mediaCfg := mapbuilder.AskMediaConfig()

	log.Log.Info("正在连接服务器...")
	client, err := mainfunction.Start_Client_by_token(server, password, config.FBToken, config.ServerURL)
	if err != nil {
		log.Log.Error("连接客户端失败: " + err.Error())
		common.WaitForExit(nil)
		return
	}

	log.Log.Info("等待机器人获得 OP 权限（<=180秒）")
	gotOP := false
	for i := 0; i < 180; i++ {
		client.IsOP_loop.Lock()
		isOP := client.IsOP
		client.IsOP_loop.Unlock()
		if isOP {
			log.Log.Info(fmt.Sprintf("机器人 %s 已获得 OP 权限", client.Conn.IdentityData().DisplayName))
			gotOP = true
			break
		}
		log.Log.Info(fmt.Sprintf("请给机器人 %s 授予 OP 权限", client.Conn.IdentityData().DisplayName))
		time.Sleep(time.Second)
	}
	if !gotOP {
		log.Log.Error("等待 OP 权限超时，MapBuilder 启动失败")
		common.WaitForExit(nil)
		return
	}

	api := mapbuilder.NewNexusAPI(client)
	mapbuilder.RunWithPreparedAll(api, mapCfg, mediaCfg)

	_ = client.GameInterface.SendWSCommand("kick NexusEgo-MapBuilder")
	common.WaitForExit(nil)
}
