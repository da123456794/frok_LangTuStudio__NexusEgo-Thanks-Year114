package control

import "testing"

func TestIsRentalServerCode(t *testing.T) {
	tests := []struct {
		name       string
		serverCode string
		want       bool
	}{
		{name: "four digit rental server", serverCode: "1234", want: true},
		{name: "eight digit rental server", serverCode: "12345678", want: true},
		{name: "trimmed domain game target", serverCode: " 山头:ABCDEF ", want: true},
		{name: "full-width colon domain game target", serverCode: "山头：ABCDEF", want: true},
		{name: "domain alias game target", serverCode: "我的山头:ABCDEF", want: true},
		{name: "tan lobby target", serverCode: "本地联机:544895", want: true},
		{name: "menu id without target", serverCode: "2", want: false},
		{name: "empty", serverCode: "", want: false},
		{name: "too short", serverCode: "123", want: false},
		{name: "too long", serverCode: "123456789", want: false},
		{name: "non digit rental server", serverCode: "12a4", want: false},
		{name: "empty domain game target", serverCode: "山头:", want: false},
		{name: "empty tan lobby target", serverCode: "本地联机:", want: false},
		{name: "menu id without target", serverCode: "4", want: false},
		{name: "online lobby target", serverCode: "联机大厅:1234567890123456789", want: true},
		{name: "online lobby shorthand", serverCode: "大厅:1234567890123456789", want: true},
		{name: "online lobby english alias", serverCode: "lobby:1234567890123456789", want: true},
		{name: "unsupported prefix", serverCode: "网络游戏:544895", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRentalServerCode(tt.serverCode); got != tt.want {
				t.Fatalf("IsRentalServerCode(%q) = %v, want %v", tt.serverCode, got, tt.want)
			}
		})
	}
}

func TestIsExportServerCode(t *testing.T) {
	tests := []struct {
		name       string
		serverCode string
		want       bool
	}{
		{name: "four digit rental server", serverCode: "1234", want: true},
		{name: "eight digit rental server", serverCode: "12345678", want: true},
		{name: "domain game target", serverCode: "山头:ABCDEF", want: false},
		{name: "domain alias game target", serverCode: "我的山头:ABCDEF", want: false},
		{name: "local online target", serverCode: "本地联机:544895", want: false},
		{name: "local shorthand target", serverCode: "联机:544895", want: false},
		{name: "too short rental server", serverCode: "123", want: false},
		{name: "non digit rental server", serverCode: "12a4", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsExportServerCode(tt.serverCode); got != tt.want {
				t.Fatalf("IsExportServerCode(%q) = %v, want %v", tt.serverCode, got, tt.want)
			}
		})
	}
}

func TestCanExportServerWithHiddenAPIKey(t *testing.T) {
	tests := []struct {
		name string
		app  *App
		code string
		want bool
	}{
		{name: "rental server without key", app: &App{}, code: "12345678", want: true},
		{name: "domain without flags", app: &App{lastAPIKey: hiddenExportAPIKey}, code: "domain:ABCDEF", want: false},
		{name: "domain with wrong key", app: &App{lastHasFlags: true, lastAPIKey: "bad"}, code: "domain:ABCDEF", want: false},
		{name: "domain with hidden key", app: &App{lastHasFlags: true, lastAPIKey: hiddenExportAPIKey}, code: "domain:ABCDEF", want: true},
		{name: "local online with hidden key", app: &App{lastHasFlags: true, lastAPIKey: hiddenExportAPIKey}, code: "local:544895", want: true},
		{name: "unsupported prefix with hidden key", app: &App{lastHasFlags: true, lastAPIKey: hiddenExportAPIKey}, code: "unsupported:544895", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.app.canExportServer(tt.code); got != tt.want {
				t.Fatalf("canExportServer(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

func TestCanExportServerCode(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		allowPrefixed bool
		want          bool
	}{
		{name: "rental server", code: "12345678", want: true},
		{name: "domain without permission", code: "domain:ABCDEF", allowPrefixed: false, want: false},
		{name: "domain with permission", code: "domain:ABCDEF", allowPrefixed: true, want: true},
		{name: "local with permission", code: "local:544895", allowPrefixed: true, want: true},
		{name: "unsupported with permission", code: "unsupported:544895", allowPrefixed: true, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanExportServerCode(tt.code, tt.allowPrefixed); got != tt.want {
				t.Fatalf("CanExportServerCode(%q, %v) = %v, want %v", tt.code, tt.allowPrefixed, got, tt.want)
			}
		})
	}
}

func TestNormalizeRentalServerCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: " 山头：ABCDEF ", want: "山头:ABCDEF"},
		{input: "我的山头:ABCDEF", want: "山头:ABCDEF"},
		{input: "联机:544895", want: "本地联机:544895"},
		{input: "大厅:123456789", want: "联机大厅:123456789"},
		{input: "12345678", want: "12345678"},
	}
	for _, tt := range tests {
		if got := NormalizeRentalServerCode(tt.input); got != tt.want {
			t.Fatalf("NormalizeRentalServerCode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShouldSuppressLocalImportTitle(t *testing.T) {
	no := false
	yes := true
	tests := []struct {
		name string
		task *Task
		want bool
	}{
		{name: "local import defaults to suppress", task: &Task{TaskType: "import", Server: "本地联机:544895"}, want: true},
		{name: "local shorthand defaults to suppress", task: &Task{TaskType: "import", Server: "联机:544895"}, want: true},
		{name: "local explicit no", task: &Task{TaskType: "import", Server: "本地联机:544895", SuppressLocalImportTitle: &no}, want: false},
		{name: "local explicit yes", task: &Task{TaskType: "import", Server: "本地联机:544895", SuppressLocalImportTitle: &yes}, want: true},
		{name: "rental import keeps title", task: &Task{TaskType: "import", Server: "12345678"}, want: false},
		{name: "export keeps title", task: &Task{TaskType: "export", Server: "本地联机:544895"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldSuppressLocalImportTitle(tt.task); got != tt.want {
				t.Fatalf("ShouldSuppressLocalImportTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServerTargetTypeByID(t *testing.T) {
	tests := []struct {
		input      string
		wantOK     bool
		wantPrefix string
	}{
		{input: "1", wantOK: true, wantPrefix: ""},
		{input: "2", wantOK: true, wantPrefix: "山头"},
		{input: "3", wantOK: true, wantPrefix: "联机大厅"},
		{input: "4", wantOK: true, wantPrefix: "本地联机"},
		{input: "9", wantOK: false, wantPrefix: ""},
	}
	for _, tt := range tests {
		got, ok := serverTargetTypeByID(tt.input)
		if ok != tt.wantOK || got.Prefix != tt.wantPrefix {
			t.Fatalf("serverTargetTypeByID(%q) = (%q, %v), want (%q, %v)", tt.input, got.Prefix, ok, tt.wantPrefix, tt.wantOK)
		}
	}
}

func TestDefaultServerConfigName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "12345678", want: "12345678"},
		{input: "山头:ABCDEF", want: "山头_ABCDEF"},
		{input: "本地联机:123/456", want: "本地联机_123_456"},
	}
	for _, tt := range tests {
		if got := defaultServerConfigName(tt.input); got != tt.want {
			t.Fatalf("defaultServerConfigName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeServerConfigName(t *testing.T) {
	if got := sanitizeServerConfigName(`a\b/c:d*e?f"g<h>i|j.`); got != "a_b_c_d_e_f_g_h_i_j" {
		t.Fatalf("sanitizeServerConfigName returned %q", got)
	}
}

func TestImportResumeProgressSingleTask(t *testing.T) {
	task := &Task{TaskType: "import", FileName: "demo.mcworld", NZ: 37, ResumeProcessed: 120, ResumeTotal: 300}
	if !hasImportResumeProgress(task) {
		t.Fatal("expected single import task with NZ progress to be resumable")
	}
	resetImportResumeProgress(task)
	if task.NZ != 0 || task.ResumeProcessed != 0 || task.ResumeTotal != 0 {
		t.Fatalf("resetImportResumeProgress left progress = (%d, %d/%d), want zero", task.NZ, task.ResumeProcessed, task.ResumeTotal)
	}
}

func TestImportResumeProgressSingleTaskWithExactProgressOnly(t *testing.T) {
	task := &Task{TaskType: "import", FileName: "demo.mcworld", ResumeProcessed: 3, ResumeTotal: 100}
	if !hasImportResumeProgress(task) {
		t.Fatal("expected exact chunk progress to be resumable even when percent is zero")
	}
}

func TestImportResumeProgressSingleTaskCompleteIgnoresStaleExactProgress(t *testing.T) {
	task := &Task{TaskType: "import", FileName: "demo.mcworld", NZ: 100, ResumeProcessed: 99, ResumeTotal: 100}
	if hasImportResumeProgress(task) {
		t.Fatal("expected completed single import task to ignore stale exact progress")
	}
}

func TestImportResumeProgressBatchTask(t *testing.T) {
	task := &Task{
		TaskType: "import",
		BatchImports: []BatchImportItem{
			{FileName: "done.mcworld", NZ: 100},
			{FileName: "half.mcworld", NZ: 42, ResumeProcessed: 420, ResumeTotal: 1000},
			{FileName: "todo.mcworld", NZ: 0},
		},
	}
	if !hasImportResumeProgress(task) {
		t.Fatal("expected batch task with saved item progress to be resumable")
	}
	summary := importResumeSummary(task)
	if summary == "" {
		t.Fatal("expected non-empty resume summary")
	}
	resetImportResumeProgress(task)
	for i, item := range task.BatchImports {
		if item.NZ != 0 || item.ResumeProcessed != 0 || item.ResumeTotal != 0 {
			t.Fatalf("batch item %d progress = (%d, %d/%d), want zero", i, item.NZ, item.ResumeProcessed, item.ResumeTotal)
		}
	}
}
