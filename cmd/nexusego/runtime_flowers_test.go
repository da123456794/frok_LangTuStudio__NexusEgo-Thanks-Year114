package main

import (
	"sync/atomic"
	"testing"
	"time"

	appcontrol "nexus/control"
	ResourcesControl "nexus/utils/api/resources_control"
	clientpkg "nexus/utils/client"

	newlogin "github.com/LangTuStudio/Conbit/minecraft/protocol/login"
	oldpacket "github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
	newgamedata "github.com/LangTuStudio/Conbit/minecraft_neo/game_data"
	starclient "github.com/LangTuStudio/RaaBel/client"
)

type openTestConn struct{}

func (openTestConn) GameData() newgamedata.GameData        { return newgamedata.GameData{} }
func (openTestConn) IdentityData() newlogin.IdentityData   { return newlogin.IdentityData{} }
func (openTestConn) WritePacket(oldpacket.Packet) error    { return nil }
func (openTestConn) ReadPacket() (oldpacket.Packet, error) { return nil, nil }
func (openTestConn) Close() error                          { return nil }

type closedTestConn struct {
	openTestConn
}

func (closedTestConn) Closed() bool { return true }

type closeTrackingConn struct {
	closed atomic.Bool
}

func (c *closeTrackingConn) GameData() newgamedata.GameData        { return newgamedata.GameData{} }
func (c *closeTrackingConn) IdentityData() newlogin.IdentityData   { return newlogin.IdentityData{} }
func (c *closeTrackingConn) WritePacket(oldpacket.Packet) error    { return nil }
func (c *closeTrackingConn) ReadPacket() (oldpacket.Packet, error) { return nil, nil }
func (c *closeTrackingConn) Close() error {
	c.closed.Store(true)
	return nil
}

type noopGameInterface struct{}

func (noopGameInterface) SendAICommand(string, bool) error { return nil }
func (noopGameInterface) SendAICommandWithResponse(string, ResourcesControl.CommandRequestOptions) ResourcesControl.CommandRespond {
	return ResourcesControl.CommandRespond{}
}
func (noopGameInterface) SendSettingsCommand(string, bool) error { return nil }
func (noopGameInterface) SendCommand(string) error               { return nil }
func (noopGameInterface) SendWSCommand(string) error             { return nil }
func (noopGameInterface) SendCommandWithResponse(string, ResourcesControl.CommandRequestOptions) ResourcesControl.CommandRespond {
	return ResourcesControl.CommandRespond{}
}
func (noopGameInterface) SendWSCommandWithResponse(string, ResourcesControl.CommandRequestOptions) ResourcesControl.CommandRespond {
	return ResourcesControl.CommandRespond{}
}
func (noopGameInterface) SendCommandWithOrigin(string, uint32, ResourcesControl.CommandRequestOptions) ResourcesControl.CommandRespond {
	return ResourcesControl.CommandRespond{}
}
func (noopGameInterface) SetBlock([3]int32, string, string) error      { return nil }
func (noopGameInterface) SetBlockAsync([3]int32, string, string) error { return nil }
func (noopGameInterface) SendChat(string) error                        { return nil }
func (noopGameInterface) Output(string) error                          { return nil }
func (noopGameInterface) Title(string) error                           { return nil }

func TestParseFlowersOPRequestName(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "compact admin prompt",
			line: "[INFO] 缺少管理员权限，请给予NBTBot管理员",
			want: "NBTBot",
		},
		{
			name: "spaced admin prompt",
			line: "缺少管理员权限，请给予 NBTBot 管理员权限",
			want: "NBTBot",
		},
		{
			name: "op prompt with color code",
			line: "WARNING 请给予 §aBot_01 OP权限",
			want: "Bot_01",
		},
		{
			name: "bare op command",
			line: "[INFO] /op §bNBT_Agent",
			want: "NBT_Agent",
		},
		{
			name: "english op prompt",
			line: "please give op MachineBot",
			want: "MachineBot",
		},
		{
			name: "unrelated output",
			line: "正在连接到 Minecraft",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseFlowersOPRequestName(tt.line); got != tt.want {
				t.Fatalf("parseFlowersOPRequestName(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestShouldSkipExportTickingAreaCleanup(t *testing.T) {
	tests := []struct {
		name   string
		task   *appcontrol.Task
		config *appcontrol.Config
		want   bool
	}{
		{name: "nil task", config: &appcontrol.Config{AllowPrefixedExport: true}, want: false},
		{name: "nil config", task: &appcontrol.Task{Server: "domain:ABCDEF"}, want: false},
		{name: "rental server", task: &appcontrol.Task{Server: "12345678"}, config: &appcontrol.Config{AllowPrefixedExport: true}, want: false},
		{name: "prefixed without permission", task: &appcontrol.Task{Server: "domain:ABCDEF"}, config: &appcontrol.Config{}, want: false},
		{name: "domain with permission", task: &appcontrol.Task{Server: "domain:ABCDEF"}, config: &appcontrol.Config{AllowPrefixedExport: true}, want: true},
		{name: "local with permission", task: &appcontrol.Task{Server: "local:544895"}, config: &appcontrol.Config{AllowPrefixedExport: true}, want: true},
		{name: "unsupported with permission", task: &appcontrol.Task{Server: "unsupported:544895"}, config: &appcontrol.Config{AllowPrefixedExport: true}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldSkipExportTickingAreaCleanup(tt.task, tt.config); got != tt.want {
				t.Fatalf("shouldSkipExportTickingAreaCleanup() = %v, want %v", got, tt.want)
			}
			if got := shouldSkipExportOperatorProbe(tt.task, tt.config); got != tt.want {
				t.Fatalf("shouldSkipExportOperatorProbe() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldHideFlowersOutputForCurrentLoginFlow(t *testing.T) {
	tests := []string{
		"[INFO] 正在连接认证服: http://langtu.flyshop.chat\n",
		"[DONE] 认证服连接完成\n",
		"[INFO] 正在连接 Minecraft (目标: DomainGame:4424D9EE8C6 密码: false)...\n",
		"[INFO] 正在建立租赁服连接\n",
		"[INFO] 正在发送登录后初始化数据\n",
		"[INFO] 登录后初始化完成\n",
		"[DONE] Minecraft 连接完成\n",
		"[INFO] 正在处理租赁服挑战校验...\n",
		"[DONE] 租赁服挑战校验完成\n",
		"Message from auth server: hello\n",
		"RaaBel debug output\n",
		"Star" + "Shuttler-main legacy output\n",
	}

	for _, line := range tests {
		t.Run(line, func(t *testing.T) {
			if !shouldHideFlowersOutput(line) {
				t.Fatalf("shouldHideFlowersOutput(%q) = false, want true", line)
			}
		})
	}
}

func TestFlowersBotNamePrefetchUsesFlowersTargetNormalization(t *testing.T) {
	serverCode, serverPasscode := starclient.NormalizeServerTarget("山头:4424D9EE8C6", "ignored")
	if serverCode != "DomainGame:4424D9EE8C6" {
		t.Fatalf("serverCode = %q, want %q", serverCode, "DomainGame:4424D9EE8C6")
	}
	if serverPasscode != "" {
		t.Fatalf("serverPasscode = %q, want empty", serverPasscode)
	}
}

func TestFlowersBotNameAuthTarget(t *testing.T) {
	tests := []struct {
		name         string
		server       string
		passcode     string
		wantServer   string
		wantPasscode string
		wantTanLobby bool
	}{
		{
			name:         "rental server",
			server:       "123456",
			passcode:     "pass",
			wantServer:   "123456",
			wantPasscode: "pass",
			wantTanLobby: false,
		},
		{
			name:         "domain game",
			server:       "山头:4424D9EE8C6",
			passcode:     "ignored",
			wantServer:   "DomainGame:4424D9EE8C6",
			wantPasscode: "",
			wantTanLobby: false,
		},
		{
			name:         "tan lobby",
			server:       "本地联机:712577",
			passcode:     "pass",
			wantServer:   "TanLobby:712577",
			wantPasscode: "pass",
			wantTanLobby: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotServer, gotPasscode, gotTanLobby := flowersBotNameAuthTarget(tt.server, tt.passcode)
			if gotServer != tt.wantServer {
				t.Fatalf("server = %q, want %q", gotServer, tt.wantServer)
			}
			if gotPasscode != tt.wantPasscode {
				t.Fatalf("passcode = %q, want %q", gotPasscode, tt.wantPasscode)
			}
			if gotTanLobby != tt.wantTanLobby {
				t.Fatalf("isTanLobby = %v, want %v", gotTanLobby, tt.wantTanLobby)
			}
		})
	}
}

func TestFlowersServiceTargetSeparatesEntranceTypes(t *testing.T) {
	tests := []struct {
		name         string
		server       string
		passcode     string
		wantServer   string
		wantPasscode string
	}{
		{
			name:         "rental server",
			server:       "123456",
			passcode:     "pass",
			wantServer:   "123456",
			wantPasscode: "pass",
		},
		{
			name:         "domain game",
			server:       "山头:4424D9EE8C6",
			passcode:     "ignored",
			wantServer:   "DomainGame:4424D9EE8C6",
			wantPasscode: "",
		},
		{
			name:         "online lobby",
			server:       "联机大厅:1234567890123456789",
			passcode:     "pass",
			wantServer:   "LobbyGame:1234567890123456789",
			wantPasscode: "pass",
		},
		{
			name:         "online lobby alias",
			server:       "大厅:1234567890123456789",
			passcode:     "pass",
			wantServer:   "LobbyGame:1234567890123456789",
			wantPasscode: "pass",
		},
		{
			name:         "tan lobby",
			server:       "本地联机:712577",
			passcode:     "pass",
			wantServer:   "TanLobby:712577",
			wantPasscode: "pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotServer, gotPasscode := flowersServiceTarget(tt.server, tt.passcode)
			if gotServer != tt.wantServer {
				t.Fatalf("server = %q, want %q", gotServer, tt.wantServer)
			}
			if gotPasscode != tt.wantPasscode {
				t.Fatalf("passcode = %q, want %q", gotPasscode, tt.wantPasscode)
			}
		})
	}
}

func TestShouldAutoReconnectImportForAccessPointErrors(t *testing.T) {
	tests := []string{
		"持续 60s 秒未能从网易租赁服获得数据, 机器人已确认掉线 (机器人在线时间 120s)",
		"node dead: context canceled",
		"租赁服主动断开了与机器人的连接: disconnect.lost",
		"与网易租赁服连接已经断开 -> 网易土豆的常见问题",
		"connection unavailable",
	}

	for _, errText := range tests {
		t.Run(errText, func(t *testing.T) {
			client := &clientpkg.Client{
				Conn:            openTestConn{},
				LastImportError: errText,
			}
			if !shouldAutoReconnectImport(client) {
				t.Fatalf("shouldAutoReconnectImport(%q) = false, want true", errText)
			}
		})
	}
}

func TestImportReconnectWatchdogDrainDelayForReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   time.Duration
	}{
		{
			name:   "closed connection waits for watchdog",
			reason: "conn dead: error reading batch from reader: use of closed network connection",
			want:   importReconnectWatchdogDrainDelay,
		},
		{
			name:   "confirmed watchdog timeout does not wait again",
			reason: "no response after a long time bot is down",
			want:   0,
		},
		{
			name:   "confirmed chinese offline reason does not wait again",
			reason: "持续 60s 秒未能从网易租赁服获得数据, 机器人已确认掉线",
			want:   0,
		},
		{
			name:   "local stall abort does not wait again",
			reason: "机器人无响应倒计时结束，主动中断当前导入以触发断点续导",
			want:   0,
		},
		{
			name:   "access point countdown waits remaining time",
			reason: accessPointNoResponseImportReason(53.15),
			want:   55 * time.Second,
		},
		{
			name:   "expired access point countdown does not wait again",
			reason: accessPointNoResponseImportReason(-0.08),
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := importReconnectWatchdogDrainDelayForReason(tt.reason); got != tt.want {
				t.Fatalf("delay = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWaitImportReconnectWatchdogDrainUsesSleeper(t *testing.T) {
	var slept time.Duration
	waitImportReconnectWatchdogDrain(
		nil,
		true,
		nil,
		"conn dead: error reading batch from reader: use of closed network connection",
		func(d time.Duration) {
			slept = d
		},
	)
	if slept != importReconnectWatchdogDrainDelay {
		t.Fatalf("slept = %v, want %v", slept, importReconnectWatchdogDrainDelay)
	}

	slept = -1
	waitImportReconnectWatchdogDrain(
		nil,
		true,
		nil,
		"no response after a long time bot is down",
		func(d time.Duration) {
			slept = d
		},
	)
	if slept != -1 {
		t.Fatalf("sleeper was called for already expired watchdog: %v", slept)
	}
}

func TestIsTransientOPCommandFailure(t *testing.T) {
	transient := []string{
		"SendCommandWithResponse: context deadline exceeded",
		"write packet: use of closed network connection",
		"conn dead: error reading batch from reader",
		"connection unavailable",
		"i/o timeout",
	}
	for _, detail := range transient {
		t.Run(detail, func(t *testing.T) {
			if !isTransientOPCommandFailure(detail) {
				t.Fatalf("isTransientOPCommandFailure(%q) = false, want true", detail)
			}
		})
	}

	if isTransientOPCommandFailure("commands.generic.noTargetMatch") {
		t.Fatal("noTargetMatch should not be classified as a transient transport failure")
	}
}

func TestIsClientCommandAvailableRejectsClosedConnection(t *testing.T) {
	if isClientCommandAvailable(nil) {
		t.Fatal("nil client should not be available")
	}
	if isClientCommandAvailable(&clientpkg.Client{GameInterface: noopGameInterface{}}) {
		t.Fatal("client without connection should not be available")
	}
	if !isClientCommandAvailable(&clientpkg.Client{GameInterface: noopGameInterface{}, Conn: openTestConn{}}) {
		t.Fatal("open client should be available")
	}
	if isClientCommandAvailable(&clientpkg.Client{GameInterface: noopGameInterface{}, Conn: closedTestConn{}}) {
		t.Fatal("closed client should not be available")
	}
}

func TestImportStallWatchdogClosesStalledClient(t *testing.T) {
	oldDelay := importStallAbortDelay
	oldCheckEvery := importStallCheckEvery
	importStallAbortDelay = 10 * time.Millisecond
	importStallCheckEvery = time.Millisecond
	defer func() {
		importStallAbortDelay = oldDelay
		importStallCheckEvery = oldCheckEvery
	}()

	conn := &closeTrackingConn{}
	client := &clientpkg.Client{Conn: conn}
	var activity atomic.Int64
	activity.Store(time.Now().Add(-time.Second).UnixNano())
	stop := make(chan struct{})
	defer close(stop)

	startImportStallWatchdog(func() *clientpkg.Client { return client }, &activity, stop)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if conn.closed.Load() {
			if client.LastImportError == "" {
				t.Fatal("watchdog closed client without setting LastImportError")
			}
			if _, ok := parseAccessPointNoResponseImportReasonRemaining(client.LastImportError); !ok {
				t.Fatalf("watchdog LastImportError = %q, want no-response countdown reason", client.LastImportError)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("watchdog did not close stalled client")
}

func TestParseAccessPointNoResponseCountdown(t *testing.T) {
	tests := []struct {
		line string
		want float64
		ok   bool
	}{
		{
			line: "[WARN] 机器人无响应，可能正被灌入大量数据, 若无法恢复,将在 -0.95 秒 后重启以摆脱问题",
			want: -0.95,
			ok:   true,
		},
		{
			line: "[WARN] 机器人无响应，可能正被灌入大量数据, 若无法恢复,将在 54.20 秒 后重启以摆脱问题",
			want: 54.20,
			ok:   true,
		},
		{
			line: "[INFO] NBT 处理器服务已完全就绪",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got, ok := parseAccessPointNoResponseCountdown(tt.line)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("countdown = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandleAccessPointOutputRecordsCountdownWithoutEarlyClose(t *testing.T) {
	conn := &closeTrackingConn{}
	client := &clientpkg.Client{Conn: conn}
	setAccessPointOutputClientProvider(func() *clientpkg.Client { return client })
	defer setAccessPointOutputClientProvider(nil)

	handleAccessPointOutput("[WARN] 机器人无响应，可能正被灌入大量数据, 若无法恢复,将在 53.15 秒 后重启以摆脱问题")

	if conn.closed.Load() {
		t.Fatal("access point output countdown closed client too early")
	}
	if client.LastImportError == "" {
		t.Fatal("access point output countdown did not set LastImportError")
	}
	if remaining, ok := parseAccessPointNoResponseImportReasonRemaining(client.LastImportError); !ok || remaining != 53.15 {
		t.Fatalf("LastImportError remaining = %v, %v, want 53.15, true", remaining, ok)
	}
}

func TestHandleAccessPointOutputClosesClientNearCountdownEnd(t *testing.T) {
	conn := &closeTrackingConn{}
	client := &clientpkg.Client{Conn: conn}
	setAccessPointOutputClientProvider(func() *clientpkg.Client { return client })
	defer setAccessPointOutputClientProvider(nil)

	handleAccessPointOutput("[WARN] 机器人无响应，可能正被灌入大量数据, 若无法恢复,将在 2.50 秒 后重启以摆脱问题")

	if !conn.closed.Load() {
		t.Fatal("access point output countdown near end did not close client")
	}
}

func TestStopFlowersOPWatcherWhenStaleDoesNotBlock(t *testing.T) {
	generation := flowersGen.Add(1)
	stopped := make(chan struct{})
	stopFlowersOPWatcherWhenStale(func() {
		close(stopped)
	}, generation)

	select {
	case <-stopped:
		t.Fatal("watcher stopped while generation is still current")
	case <-time.After(20 * time.Millisecond):
	}

	flowersGen.Add(1)
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop after generation became stale")
	}
}
