package types

type Task struct {
	TaskType            string `json:"task_type"`
	UserID              string `json:"user_id"`
	Server              string `json:"server"`
	Password            string `json:"password"`
	NowOperation        string `json:"now_operation"`
	NowBlock            int    `json:"now_block"`
	MaxBlock            int    `json:"max_block"`
	FilePath            string `json:"file_path"`
	FileID              int    `json:"file_id"`
	XYZ                 [3]int `json:"xyz"`
	Operation_app       int    `json:"operation_app"`
	Operation_max       int    `json:"operation_max"`
	High_import_Setting string `gorm:"high_import_setting" json:"high_import_setting"`
	ImportNBT           bool   `json:"import_nbt"`
	ImportCommandBlock  bool   `json:"importcmd"`
	UseFill             bool   `json:"usefill"`
	VerifySuspicious    bool   `json:"verify_suspicious"`
	RegionSize          int    `json:"region"`
	ClearArea           bool   `json:"clear"`
	ClearDrops          bool   `json:"clear_drops"`
	AutoPlaceDenyBlock  bool   `json:"deny"`
	AutoPlaceBorder     bool   `json:"border"`
	CloseCommandBlock   bool   `json:"closecmd"`
	DefaultSignWax      bool   `json:"default_sign_wax"`
	EnterRepairDirect   bool   `json:"fix"`
	CommandDataSpeed    int    `json:"cmdspeed"`
	CropEnabled         bool   `json:"crop"`
	CropMin             [3]int `json:"crop_min"`
	CropMax             [3]int `json:"crop_max"`
	Dimension           string `json:"world"`
	ResumeProcessed     int    `json:"resume_processed,omitempty"`
	ResumeTotal         int    `json:"resume_total,omitempty"`
}
