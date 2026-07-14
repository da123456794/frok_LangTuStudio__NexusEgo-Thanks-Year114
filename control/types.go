package control

import consolepkg "nexus/utils/console"

type CLIOptions struct {
	Token  string
	APIKey string

	HasFlags bool

	Mode           string
	Server         string
	Password       string
	Dimension      string
	File           string
	X              int
	Y              int
	Z              int
	Speed          int
	UseFill        bool
	RegionSize     int
	ImportNBT      bool
	ImportCommand  bool
	CommandSpeed   int
	ClearArea      bool
	ClearDrops     bool
	PlaceDenyBlock bool
	PlaceBorder    bool
	CloseCommand   bool
	EnterRepair    bool
	StartProgress  int
	Crop           string
	ExportFile     string
	ExportCoords   string
	ExportAuthor   string
	ExportPassword string
}

type Config struct {
	FBToken             string
	ServerURL           string
	AllowPrefixedExport bool
}

type LastServerConfig struct {
	Server     string `json:"server"`
	Password   string `json:"password"`
	UsedAt     string `json:"used_at"`
	ConfigName string `json:"config_name,omitempty"`
}

type ServerConfigEntry struct {
	Name     string
	Path     string
	Server   string
	Password string
	UsedAt   string
}

func NewConfig(serverURL, fbToken string) *Config {
	return &Config{
		FBToken:   fbToken,
		ServerURL: serverURL,
	}
}

type Task struct {
	TaskType                 string            `json:"task_type"`
	FileName                 string            `json:"file_name"`
	BatchImports             []BatchImportItem `json:"batch_imports,omitempty"`
	Server                   string            `json:"server"`
	Password                 string            `json:"password"`
	X                        int               `json:"x"`
	Y                        int               `json:"y"`
	Z                        int               `json:"z"`
	Dimension                string            `json:"world"`
	NZ                       int               `json:"progress"`
	ResumeProcessed          int               `json:"resume_processed,omitempty"`
	ResumeTotal              int               `json:"resume_total,omitempty"`
	ImportNBT                bool              `json:"import_nbt"`
	ImportCommandBlock       bool              `json:"importcmd"`
	UseFill                  bool              `json:"usefill"`
	ImportSpeed              int               `json:"speed"`
	RegionSize               int               `json:"region"`
	ClearArea                bool              `json:"clear"`
	ClearDrops               bool              `json:"clear_drops"`
	AutoPlaceDenyBlock       bool              `json:"deny"`
	AutoPlaceBorder          bool              `json:"border"`
	CloseCommandBlock        bool              `json:"closecmd"`
	EnterRepairDirect        bool              `json:"fix"`
	DefaultSignWax           bool              `json:"default_sign_wax"`
	CommandDataSpeed         int               `json:"cmdspeed"`
	SuppressLocalImportTitle *bool             `json:"suppress_local_import_title,omitempty"`
	CropEnabled              bool              `json:"crop"`
	CropMin                  [3]int            `json:"crop_min"`
	CropMax                  [3]int            `json:"crop_max"`
	ExportFile               string            `json:"exportfile"`
	ExportAuthor             string            `json:"exportauthor"`
	ExportPassword           string            `json:"exportpassword"`
	ExportMin                [3]int            `json:"export_min"`
	ExportMax                [3]int            `json:"export_max"`
	TaskFile                 string            `json:"-"`
}

type BatchImportItem struct {
	FileName        string `json:"file_name"`
	DisplayName     string `json:"display_name"`
	X               int    `json:"x"`
	Y               int    `json:"y"`
	Z               int    `json:"z"`
	NZ              int    `json:"progress"`
	ResumeProcessed int    `json:"resume_processed,omitempty"`
	ResumeTotal     int    `json:"resume_total,omitempty"`
	CropEnabled     bool   `json:"crop"`
	CropMin         [3]int `json:"crop_min"`
	CropMax         [3]int `json:"crop_max"`
}

type TaskEntry struct {
	Name    string
	Path    string
	Index   int
	Summary string
}

type UpdateInfo struct {
	Success     bool   `json:"success"`
	Version     string `json:"version"`
	Changelog   string `json:"changelog"`
	DownloadURL string `json:"download_url"`
	Filename    string `json:"filename"`
}

type TokenValidator func(serverURL, token string) (bool, string)

type TaskRunner func(console *consolepkg.Console_input, task *Task, config *Config)

type MapBuilderRunner func(console *consolepkg.Console_input, config *Config)

type App struct {
	ServerURL        string
	TokenValidator   TokenValidator
	TaskRunner       TaskRunner
	MapBuilderRunner MapBuilderRunner
	lastConfig       *Config
	lastHasFlags     bool
	lastAPIKey       string
	lastHasTokenArg  bool
}

func NewApp(serverURL string, validator TokenValidator, runner TaskRunner) *App {
	return &App{
		ServerURL:      serverURL,
		TokenValidator: validator,
		TaskRunner:     runner,
	}
}
