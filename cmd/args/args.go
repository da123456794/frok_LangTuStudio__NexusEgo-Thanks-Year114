package args

import (
	"flag"

	"nexus/control"
)

func Parse() control.CLIOptions {
	var opts control.CLIOptions

	flag.StringVar(&opts.Token, "token", "", "API Token (可选，可在启动后输入)")
	flag.StringVar(&opts.APIKey, "apikey", "", "MapBuilder API Key")

	flag.StringVar(&opts.Mode, "mode", "", "运行模式 (import/export)")
	flag.StringVar(&opts.Server, "server", "", "4到8位纯数字租赁服号，或 山头/联机大厅/本地联机:入口")
	flag.StringVar(&opts.Password, "password", "", "租赁服密码")
	flag.StringVar(&opts.Dimension, "dimension", "overworld", "维度 (overworld/nether/the_end/dm:<N>)")
	flag.StringVar(&opts.File, "file", "", "建筑文件名")
	flag.IntVar(&opts.X, "x", 0, "起始X坐标")
	flag.IntVar(&opts.Y, "y", 0, "起始Y坐标")
	flag.IntVar(&opts.Z, "z", 0, "起始Z坐标")
	flag.IntVar(&opts.Speed, "speed", control.DefaultImportSpeed, "导入速度 (命令/秒)")
	flag.BoolVar(&opts.UseFill, "usefill", true, "是否启用增量导入")
	flag.IntVar(&opts.RegionSize, "region", 5, "增量导入边长")
	flag.BoolVar(&opts.ImportNBT, "importnbt", true, "是否导入NBT数据")
	flag.BoolVar(&opts.ImportCommand, "importcmd", true, "是否导入命令方块数据")
	flag.IntVar(&opts.CommandSpeed, "cmdspeed", control.DefaultCommandDataSpeed, "命令方块写入速度")
	flag.BoolVar(&opts.ClearArea, "clear", false, "是否清理导入区域")
	flag.BoolVar(&opts.ClearDrops, "cleardrops", false, "是否清理导入时掉落物")
	flag.BoolVar(&opts.PlaceDenyBlock, "deny", false, "是否自动铺设拒绝方块")
	flag.BoolVar(&opts.PlaceBorder, "border", false, "是否自动铺设边界方块")
	flag.BoolVar(&opts.CloseCommand, "closecmd", true, "是否关闭命令方块启用")
	flag.BoolVar(&opts.EnterRepair, "fix", false, "是否直接进入修补模式")
	flag.IntVar(&opts.StartProgress, "progress", 0, "起始进度百分比 (0-100)")
	flag.StringVar(&opts.Crop, "croparea", "", "裁剪坐标 \"x1 y1 z1 x2 y2 z2\"")

	flag.StringVar(&opts.ExportFile, "exportfile", "", "导出文件名")
	flag.StringVar(&opts.ExportCoords, "exportarea", "", "导出对角坐标 \"x1 y1 z1 x2 y2 z2\"")
	flag.StringVar(&opts.ExportAuthor, "exportauthor", "", "Nexus作者")
	flag.StringVar(&opts.ExportPassword, "exportpassword", "", "Nexus密码")

	flag.Parse()
	opts.HasFlags = flag.NFlag() > 0
	return opts
}
