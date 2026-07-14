package control

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nexus/cmd/common"
	"nexus/constants"
	consolepkg "nexus/utils/console"
	convertpkg "nexus/utils/convert"
	"nexus/utils/file"
	"nexus/utils/log"
	"nexus/utils/ui"

	wsstructure "github.com/Yeah114/WaterStructure/structure"
	"github.com/pterm/pterm"
)

const (
	RentalServerCodePrompt  = "请选择服务器类型或直接输入完整服务器号"
	DefaultImportSpeed      = constants.DefaultImportSpeed
	DefaultCommandDataSpeed = 11
	hiddenExportAPIKey      = "ys6610888"

	rentalServerCodeRequirement = "服务器号必须是4到8位纯数字，或通过编号选择山头/联机大厅/本地联机后输入入口"
	ExportServerCodeRequirement = "导出仅支持4到8位纯数字租赁服号，不支持山头、联机大厅或本地联机"
)

type serverTargetType struct {
	ID          string
	Label       string
	Prefix      string
	Prompt      string
	Description string
}

func (a *App) RunWithConsole(console *consolepkg.Console_input, opts CLIOptions) {
	if a.TaskRunner == nil {
		log.Log.Error("task runner is not initialized")
		return
	}

	_ = os.RemoveAll(StorageLogDir())
	_ = os.RemoveAll("cache")
	_ = os.RemoveAll("replace_file")
	if err := EnsureStorageDirs(); err != nil {
		log.Log.Error("无法创建存储目录: " + err.Error())
		common.WaitForExit(nil)
		return
	}

	token, err := a.resolveToken(console, opts.Token)
	if err != nil {
		log.Log.Error("resolve token failed: " + err.Error())
		common.WaitForExit(nil)
		return
	}

	config := NewConfig(a.ServerURL, token)
	a.lastConfig = config
	a.lastHasFlags = opts.HasFlags
	a.lastAPIKey = strings.TrimSpace(opts.APIKey)
	a.lastHasTokenArg = strings.TrimSpace(opts.Token) != ""
	config.AllowPrefixedExport = a.hasHiddenExportAccess()
	if !opts.HasFlags {
		a.checkAndPromptUpdate(console)
	}

	cliTask := a.buildTaskFromArgs(opts)
	if cliTask != nil {
		a.TaskRunner(console, cliTask, config)
		return
	}

	for {
		taskEntries, err := listTaskFiles()
		if err != nil {
			log.Log.Error("读取任务列表失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		}
		if len(taskEntries) == 0 {
			task := a.loadNewTask(console)
			if a.confirmAndMaybeRunTask(console, task, config) {
				return
			}
			continue
		}

		selected, createNew := promptTaskSelection(console, taskEntries)
		if createNew {
			task := a.loadNewTask(console)
			if a.confirmAndMaybeRunTask(console, task, config) {
				return
			}
			continue
		}
		if selected == nil {
			continue
		}

		task := a.loadExistingTask(selected.Path)
		if task == nil {
			continue
		}
		if a.confirmAndMaybeRunTask(console, task, config) {
			return
		}
	}
}

func (a *App) resolveToken(console *consolepkg.Console_input, cliToken string) (string, error) {
	token := strings.TrimSpace(cliToken)
	if token != "" {
		return token, nil
	}

	storedToken, err := LoadTokenFromFSM()
	if err == nil {
		log.Log.Info("正在验证已保存的 Token...")
		if a.canUseStoredToken(storedToken) {
			log.Log.Info("已从 token.fsm 解密读取 Token")
			return storedToken, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Log.Warn("读取 token.fsm 失败，将重新输入 Token", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		if removeErr := RemoveTokenFSM(); removeErr != nil {
			log.Log.Warn("清理无效 token.fsm 失败", log.Log.ArgsFromMap(map[string]any{"error": removeErr.Error()}))
		}
	}

	token = a.promptToken(console)
	if err := SaveTokenToFSM(token); err != nil {
		log.Log.Warn("保存 token.fsm 失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
	}
	return token, nil
}

func (a *App) canUseStoredToken(token string) bool {
	if a.TokenValidator == nil {
		return true
	}

	valid, message := a.TokenValidator(a.ServerURL, token)
	if valid {
		log.Log.Success(message)
		return true
	}
	if shouldResetStoredToken(message) {
		log.Log.Warn("已保存的 Token 无效，将重新输入")
		if err := RemoveTokenFSM(); err != nil {
			log.Log.Warn("删除无效 token.fsm 失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		}
		return false
	}

	log.Log.Warn("无法确认已保存 Token 是否有效，将继续尝试使用已保存 Token", log.Log.ArgsFromMap(map[string]any{"message": message}))
	return true
}

func shouldResetStoredToken(message string) bool {
	lowerMessage := strings.ToLower(strings.TrimSpace(message))
	if lowerMessage == "" {
		return false
	}
	return strings.Contains(lowerMessage, "invalid") ||
		strings.Contains(lowerMessage, "expired") ||
		strings.Contains(message, "无效") ||
		strings.Contains(message, "过期")
}

func (a *App) promptToken(console *consolepkg.Console_input) string {
	for {
		fmt.Print(common.InfoPrompt("请输入 API Token: "))
		input, _, err := console.InputNoPrefix("")
		if err != nil {
			log.Log.Error("读取输入失败: " + err.Error())
			continue
		}
		token := strings.TrimSpace(input)
		if token == "" {
			log.Log.Error("必须提供 Token")
			continue
		}
		if a.TokenValidator == nil {
			return token
		}
		if runtime.GOOS == "linux" {
			fmt.Println()
		}
		log.Log.Info("正在验证 Token...")
		valid, message := a.TokenValidator(a.ServerURL, token)
		if valid {
			log.Log.Success(message)
			return token
		}
		log.Log.Error(message)
	}
}

func promptDimension(console *consolepkg.Console_input) string {
	for {
		fmt.Println("请选择目标维度:")
		fmt.Println("  [1] 主世界 (Overworld)")
		fmt.Println("  [2] 下界 (Nether)")
		fmt.Println("  [3] 末地 (The End)")
		fmt.Println("  [4] 自定义维度 (dm3 ~ dm20)")
		input, _, _ := console.InputNoPrefix("  (ID) [默认: 1]: ")
		input = strings.TrimSpace(input)
		if input == "" {
			input = "1"
		}
		switch input {
		case "1":
			return "overworld"
		case "2":
			return "nether"
		case "3":
			return "the_end"
		case "4":
			for {
				customInput, _, _ := console.InputInfo("请输入自定义维度 ID (3-20): ")
				customInput = strings.TrimSpace(customInput)
				id, err := strconv.Atoi(customInput)
				if err != nil || id < 3 || id > 20 {
					log.Log.Error("维度输入错误: 请输入 3-20 的数字")
					continue
				}
				return fmt.Sprintf("dm%d:%d", id, id)
			}
		default:
			log.Log.Error("维度输入错误: 请输入 1-4")
		}
	}
}

func promptTaskSelection(console *consolepkg.Console_input, entries []TaskEntry) (*TaskEntry, bool) {
	for {
		fmt.Println("任务列表:")
		for i, entry := range entries {
			fmt.Printf("  [%d] %s\n", i+1, entry.Summary)
		}
		fmt.Printf("  [n] %s\n", pterm.Gray("新建任务"))
		input, _, _ := console.InputNoPrefix("选择任务编号或输入 n 新建: ")
		choice := strings.ToLower(strings.TrimSpace(input))
		if choice == "n" || choice == "new" {
			return nil, true
		}
		if strings.HasPrefix(choice, "task") {
			for i := range entries {
				if entries[i].Name == choice {
					return &entries[i], false
				}
			}
		}
		if idx, err := strconv.Atoi(choice); err == nil && idx > 0 && idx <= len(entries) {
			return &entries[idx-1], false
		}
		log.Log.Warn("无效选择，请重新输入")
	}
}

func getOriginalFilesInDirectory(dir string) ([]string, error) {
	if !file.Is_Dir(dir) {
		return []string{}, nil
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~") || strings.HasSuffix(name, ".tmp") || strings.HasSuffix(name, ".swp") {
			continue
		}
		fileNames = append(fileNames, name)
	}
	return fileNames, nil
}

func getImportFilesInSearchDirs() ([]string, error) {
	files, err := getOriginalFilesInDirectory(StorageFileDir())
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(files))
	for _, name := range files {
		seen[strings.ToLower(name)] = true
	}

	rootFiles, err := getOriginalFilesInDirectory(".")
	if err != nil {
		return nil, err
	}
	for _, name := range rootFiles {
		key := strings.ToLower(name)
		if seen[key] || !convertpkg.IsImportCandidatePath(name) {
			continue
		}
		files = append(files, name)
		seen[key] = true
	}
	return files, nil
}

func selectFileFromList(console *consolepkg.Console_input) string {
	return selectFileFromListWithPrompt(
		console,
		constants.StorageRootName+"/file 或程序根目录中没有找到任何结构文件",
		"手动输入文件名（支持 WaterStructure 支持的结构格式）",
		"请选择文件编号或输入文件名: ",
		nil,
	)
}

func selectFileFromListWithPrompt(console *consolepkg.Console_input, emptyMessage, manualLabel, inputPrompt string, isAllowed func(string) bool) string {
	files, err := getImportFilesInSearchDirs()
	if err != nil {
		log.Log.Error("读取文件列表失败")
		return ""
	}
	if isAllowed != nil {
		filtered := make([]string, 0, len(files))
		for _, name := range files {
			if isAllowed(name) {
				filtered = append(filtered, name)
			}
		}
		files = filtered
	}
	if len(files) == 0 {
		log.Log.Error(emptyMessage)
		return ""
	}
	printFileList := func(list []string) {
		fmt.Println("文件列表:")
		for i, f := range list {
			name := truncateStringByWidth(f, 50)
			sizeStr := formatFileSize(ResolveImportFilePath(f))
			fmt.Printf("  [%d] %s  %s\n", i+1, name, pterm.Gray(sizeStr))
		}
		fmt.Printf("  [0] %s\n", pterm.Gray(manualLabel))
		fmt.Printf("  [r] %s\n", pterm.Gray("刷新列表"))
	}
	printFileList(files)
	for {
		choice, _, _ := console.InputNoPrefix(inputPrompt)
		choice = strings.TrimSpace(choice)
		if strings.EqualFold(choice, "r") {
			files, err = getImportFilesInSearchDirs()
			if err != nil {
				log.Log.Error("刷新文件列表失败")
				continue
			}
			if isAllowed != nil {
				filtered := make([]string, 0, len(files))
				for _, name := range files {
					if isAllowed(name) {
						filtered = append(filtered, name)
					}
				}
				files = filtered
			}
			if len(files) == 0 {
				log.Log.Info(emptyMessage)
				continue
			}
			printFileList(files)
			continue
		}
		if idx, err := strconv.Atoi(choice); err == nil {
			if idx == 0 {
				fileName, _, _ := console.InputInfo("请输入文件名: ")
				fileName = strings.TrimSpace(fileName)
				if file.Is_File(ResolveImportFilePath(fileName)) && (isAllowed == nil || isAllowed(fileName)) {
					return fileName
				}
				log.Log.Error("文件不存在或格式不支持，请重新选择")
				continue
			}
			if idx > 0 && idx <= len(files) {
				selectedFile := files[idx-1]
				log.Log.Info("已选择: " + selectedFile)
				return selectedFile
			}
		}
		if file.Is_File(ResolveImportFilePath(choice)) && (isAllowed == nil || isAllowed(choice)) {
			return choice
		}
		if choice != "" {
			matches := []string{}
			for _, f := range files {
				if strings.HasPrefix(f, choice) || strings.Contains(f, choice) {
					matches = append(matches, f)
				}
			}
			if len(matches) == 1 {
				log.Log.Info("自动补全: " + matches[0])
				return matches[0]
			}
			if len(matches) > 1 {
				log.Log.Info("匹配到多个文件:")
				for i, match := range matches {
					fmt.Printf("  [%d] %s\n", i+1, match)
				}
				subChoice, _, _ := console.InputNoPrefix("请选择文件编号: ")
				if subIdx, err := strconv.Atoi(subChoice); err == nil && subIdx > 0 && subIdx <= len(matches) {
					return matches[subIdx-1]
				}
			}
		}
		log.Log.Error("无效的选择或文件不存在，请重新选择")
	}
}

func (a *App) loadExistingTask(taskPath string) *Task {
	task := &Task{CloseCommandBlock: true, EnterRepairDirect: false, DefaultSignWax: true, TaskFile: taskPath}
	filesData, err := os.ReadFile(taskPath)
	if err != nil {
		log.Log.Error("读取任务失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	if err := json.Unmarshal(filesData, task); err != nil {
		log.Log.Error("读取任务文件失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	if strings.TrimSpace(task.TaskType) == "" {
		task.TaskType = "import"
	}
	if strings.EqualFold(task.TaskType, "export") {
		task.ExportFile = ResolveExportFilePath(task.ExportFile)
		if !a.canExportServer(task.Server) {
			log.Log.Error(ExportServerCodeRequirement)
			common.ExitAfterPrompt(nil, 0)
		}
		if strings.TrimSpace(task.ExportFile) == "" {
			log.Log.Error("导出任务缺少导出文件路径")
			common.ExitAfterPrompt(nil, 0)
		}
		printTaskInfo(task)
		return task
	}
	if len(task.BatchImports) > 0 {
		for i := range task.BatchImports {
			item := &task.BatchImports[i]
			item.FileName = filepath.Base(item.FileName)
			if !strings.HasSuffix(strings.ToLower(item.FileName), ".mcworld") {
				log.Log.Error("批量导入只支持 .mcworld 文件")
				common.ExitAfterPrompt(nil, 0)
			}
			if !file.Is_File(ResolveImportFilePath(item.FileName)) {
				log.Log.Error(fmt.Sprintf("批量导入文件不存在: %s", item.FileName))
				common.ExitAfterPrompt(nil, 0)
			}
		}
		if strings.TrimSpace(task.FileName) == "" {
			task.FileName = task.BatchImports[0].FileName
		}
	} else {
		task.FileName = filepath.Base(task.FileName)
		if !strings.HasSuffix(strings.ToLower(task.FileName), ".mcworld") {
			log.Log.Error("只支持 .mcworld 文件")
			common.ExitAfterPrompt(nil, 0)
		}
		if !file.Is_File(ResolveImportFilePath(task.FileName)) {
			log.Log.Error(fmt.Sprintf("文件不存在: %s", task.FileName))
			common.ExitAfterPrompt(nil, 0)
		}
	}
	printTaskInfo(task)
	return task
}

func (a *App) loadNewTask(console *consolepkg.Console_input) *Task {
	_, taskPath, err := nextTaskFilePath()
	if err != nil {
		log.Log.Error("create task file failed", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		common.ExitAfterPrompt(nil, 0)
	}
	for {
		mapBuilderEnabled := a.lastHasFlags &&
			a.lastConfig != nil &&
			a.lastHasTokenArg &&
			a.lastAPIKey == hiddenExportAPIKey &&
			a.MapBuilderRunner != nil

		fmt.Println("请选择操作:")
		fmt.Println("  [1] 开始导入")
		fmt.Println("  [2] 开始导出")
		fmt.Println("  [3] 查看更新内容")
		fmt.Println("  [4] 导入雕塑（使用皮肤展开图）")
		if mapBuilderEnabled {
			fmt.Println("  [5] 启动 NexusEgo-MapBuilder")
		}
		modeInput, _, _ := console.InputNoPrefix("  (ID): ")
		modeInput = strings.TrimSpace(strings.ToLower(modeInput))
		switch modeInput {
		case "", "1":
			return a.loadNewImportTaskSeries(console, taskPath)
		case "2":
			return a.loadNewExportTask(console, taskPath)
		case "3":
			a.showNotice(console)
		case "4":
			return a.loadNewSkinBuilderTask(console, taskPath)
		case "5":
			if !mapBuilderEnabled {
				log.Log.Error("输入错误，请重新选择")
				continue
			}
			a.MapBuilderRunner(console, a.lastConfig)
			os.Exit(0)
			return nil
		default:
			log.Log.Error("输入错误，请重新选择")
		}
	}
}

func (a *App) loadNewImportTaskSeries(console *consolepkg.Console_input, firstTaskPath string) *Task {
	first := a.loadNewImportTask(console, firstTaskPath)
	if first == nil {
		return nil
	}
	if first.EnterRepairDirect {
		return first
	}
	created := 1
	for promptYesNo(console, "是否继续配置下一个导入任务? [y/n, 默认n]: ", false) {
		item, ok := a.loadAdditionalBatchImportItem(console)
		if !ok {
			break
		}
		if len(first.BatchImports) == 0 {
			first.BatchImports = append(first.BatchImports, batchImportItemFromTask(first))
		}
		first.BatchImports = append(first.BatchImports, item)
		first.EnterRepairDirect = false
		first.NZ = 0
		first.ResumeProcessed = 0
		first.ResumeTotal = 0
		first.FileName = first.BatchImports[0].FileName
		first.X = first.BatchImports[0].X
		first.Y = first.BatchImports[0].Y
		first.Z = first.BatchImports[0].Z
		first.CropEnabled = first.BatchImports[0].CropEnabled
		first.CropMin = first.BatchImports[0].CropMin
		first.CropMax = first.BatchImports[0].CropMax
		if err := saveTaskDefinition(first); err != nil {
			log.Log.Error("保存连续导入任务失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
			break
		}
		created++
	}
	if created > 1 {
		log.Log.Info(fmt.Sprintf("已连续配置 %d 个导入任务，将按顺序连续导入", created))
	}
	return first
}

func saveTaskDefinition(task *Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return os.WriteFile(TaskFilePath(task), data, 0655)
}

func batchImportItemFromTask(task *Task) BatchImportItem {
	if task == nil {
		return BatchImportItem{}
	}
	return BatchImportItem{
		FileName:        task.FileName,
		DisplayName:     displayFileName(task.FileName),
		X:               task.X,
		Y:               task.Y,
		Z:               task.Z,
		NZ:              task.NZ,
		ResumeProcessed: task.ResumeProcessed,
		ResumeTotal:     task.ResumeTotal,
		CropEnabled:     task.CropEnabled,
		CropMin:         task.CropMin,
		CropMax:         task.CropMax,
	}
}

func (a *App) loadAdditionalBatchImportItem(console *consolepkg.Console_input) (BatchImportItem, bool) {
	_ = a
	log.Log.Info("开始配置下一个导入任务")
	selectedFile := selectFileFromList(console)
	if selectedFile == "" {
		for {
			fileName, _, _ := console.InputInfo("请输入文件名: ")
			if !file.Is_File(ResolveImportFilePath(fileName)) {
				log.Log.Error("文件不存在，请重新输入，文件请上传到 file 文件夹或程序根目录")
				continue
			}
			selectedFile = fileName
			break
		}
	}

	sourcePath := ResolveImportFilePath(selectedFile)
	fileName := filepath.Base(selectedFile)
	if convertpkg.IsOneDragonArchivePath(sourcePath) {
		log.Log.Error("连续配置的追加任务不支持嵌套一条龙压缩包，请单独新建一条龙任务")
		return BatchImportItem{}, false
	}

	convertedFromSource := false
	if ext := strings.ToLower(filepath.Ext(fileName)); ext != ".mcworld" {
		log.Log.Info("检测到非 mcworld 文件，正在转换为 mcworld 格式...")
		converted, err := convertAdditionalImportFile(console, sourcePath, ext)
		if err != nil {
			log.Log.Error("转换为 mcworld 失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
			common.WaitForExit(nil)
			return BatchImportItem{}, false
		}
		fileName = filepath.Base(converted)
		convertedFromSource = true
	}

	item := BatchImportItem{
		FileName:    fileName,
		DisplayName: displayFileName(fileName),
	}
	promptItemCropBounds := func() {
		for {
			coordInput, _, _ := console.InputInfo("请输入裁剪对角坐标(x1 y1 z1 x2 y2 z2): ")
			coordSlice := strings.Fields(coordInput)
			if len(coordSlice) != 6 {
				log.Log.Error("坐标填写错误，请重新输入")
				continue
			}
			vals := make([]int, 6)
			ok := true
			for i, value := range coordSlice {
				val, err := strconv.Atoi(value)
				if err != nil {
					ok = false
					break
				}
				vals[i] = val
			}
			if !ok {
				log.Log.Error("坐标需为数字，请重新输入")
				continue
			}
			item.CropEnabled = true
			item.CropMin = [3]int{minInt(vals[0], vals[3]), minInt(vals[1], vals[4]), minInt(vals[2], vals[5])}
			item.CropMax = [3]int{maxInt(vals[0], vals[3]), maxInt(vals[1], vals[4]), maxInt(vals[2], vals[5])}
			return
		}
	}
	if convertedFromSource {
		if minPos, maxPos, ok := parseMCWorldBoundsFromName(item.FileName); ok {
			item.CropEnabled = true
			item.CropMin = minPos
			item.CropMax = maxPos
		} else {
			log.Log.Error("转换后的 mcworld 文件名中未包含裁剪信息，请输入对角坐标")
			promptItemCropBounds()
		}
	} else {
		cropInput, _, _ := console.InputInfo("是否采用文件名提取的信息裁剪? [y/n, 默认y]: ")
		cropInput = strings.ToLower(strings.TrimSpace(cropInput))
		if cropInput == "" || cropInput == "y" || cropInput == "yes" {
			if minPos, maxPos, ok := parseMCWorldBoundsFromName(item.FileName); ok {
				item.CropEnabled = true
				item.CropMin = minPos
				item.CropMax = maxPos
			} else {
				log.Log.Error("文件名中未包含裁剪信息，请输入对角坐标")
				promptItemCropBounds()
			}
		} else {
			promptItemCropBounds()
		}
	}

	for {
		coordInput, _, _ := console.InputInfo("请输入起始坐标: ")
		coordSlice := strings.Fields(coordInput)
		if len(coordSlice) != 3 {
			log.Log.Error("坐标填写错误，请重新输入")
			continue
		}
		x, errX := strconv.Atoi(coordSlice[0])
		y, errY := strconv.Atoi(coordSlice[1])
		z, errZ := strconv.Atoi(coordSlice[2])
		if errX != nil || errY != nil || errZ != nil {
			log.Log.Error("坐标需为数字，请重新输入")
			continue
		}
		item.X, item.Y, item.Z = x, y, z
		return item, true
	}
}

func convertAdditionalImportFile(console *consolepkg.Console_input, inputPath, ext string) (string, error) {
	if ext == ".nexus" {
		for {
			pwInput, _, _ := console.InputInfo("Nexus 密码（选填，无密码请直接回车）：")
			converted, err := convertpkg.ConvertToMCWorld(inputPath, StorageFileDir(), strings.TrimSpace(pwInput))
			if err == nil {
				return converted, nil
			}
			if errors.Is(err, wsstructure.ErrNexusPasswordRequired) {
				log.Log.Error("Nexus 密码为必填项")
				continue
			}
			if errors.Is(err, wsstructure.ErrNexusPasswordInvalid) {
				log.Log.Error("Nexus 密码错误")
				continue
			}
			return "", err
		}
	}
	return convertpkg.ConvertToMCWorld(inputPath, StorageFileDir(), "")
}

func (a *App) showNotice(console *consolepkg.Console_input) {
	fmt.Println("更新公告:")
	info, err := a.checkLatestVersion()
	if err != nil {
		fmt.Println("v" + ui.Version)
		fmt.Println("获取更新公告失败: " + err.Error())
	} else {
		changelog := strings.TrimSpace(info.Changelog)
		if changelog == "" {
			changelog = "暂无更新日志"
		}
		ver := strings.TrimPrefix(info.Version, "v")
		fmt.Println("v" + ver)
		fmt.Println(changelog)
	}
	fmt.Println()
	console.InputNoPrefix("按回车键返回...")
}

func (a *App) loadNewImportTask(console *consolepkg.Console_input, taskPath string) *Task {
	log.Log.Info("读取建筑文件中，请将建筑文件上传到 file 目录或程序根目录中")
	if !file.Is_Dir(StorageFileDir()) {
		_ = os.MkdirAll(StorageFileDir(), 0755)
	}
	task := &Task{TaskType: "import", CloseCommandBlock: true, EnterRepairDirect: false, CommandDataSpeed: 0, DefaultSignWax: true, AutoPlaceDenyBlock: false, AutoPlaceBorder: false, TaskFile: taskPath}
	log.Log.Info("检测到你没创建任务，现在开始创建")
	selectedFile := selectFileFromList(console)
	if selectedFile == "" {
		for {
			fileName, _, _ := console.InputInfo("请输入文件名: ")
			if !file.Is_File(ResolveImportFilePath(fileName)) {
				log.Log.Error("文件不存在，请重新输入，文件请上传到 file 文件夹或程序根目录")
				continue
			}
			task.FileName = fileName
			break
		}
	} else {
		task.FileName = selectedFile
	}
	sourcePath := ResolveImportFilePath(task.FileName)
	task.FileName = filepath.Base(task.FileName)
	oneDragonPrepared := false
	if convertpkg.IsOneDragonArchivePath(sourcePath) {
		prepared, handled := a.prepareOneDragonImportTask(console, task, sourcePath)
		if handled {
			if prepared == nil {
				return nil
			}
			task = prepared
			oneDragonPrepared = true
		}
	}
	convertedFromSource := false
	if !oneDragonPrepared {
		if ext := strings.ToLower(filepath.Ext(task.FileName)); ext != ".mcworld" {
			log.Log.Info("检测到非 mcworld 文件，正在转换为 mcworld 格式...")
			inputPath := sourcePath
			var converted string
			var err error
			if ext == ".nexus" {
				for {
					pwInput, _, _ := console.InputInfo("Nexus 密码（选填，无密码请直接回车）：")
					pwInput = strings.TrimSpace(pwInput)
					converted, err = convertpkg.ConvertToMCWorld(inputPath, StorageFileDir(), pwInput)
					if err == nil {
						break
					}
					if errors.Is(err, wsstructure.ErrNexusPasswordRequired) {
						log.Log.Error("Nexus 密码为必填项")
						continue
					}
					if errors.Is(err, wsstructure.ErrNexusPasswordInvalid) {
						log.Log.Error("Nexus 密码错误")
						continue
					}
					log.Log.Error("转换为 mcworld 失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
					common.WaitForExit(nil)
					return nil
				}
			} else {
				converted, err = convertpkg.ConvertToMCWorld(inputPath, StorageFileDir(), "")
				if err != nil {
					log.Log.Error("转换为 mcworld 失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
					common.WaitForExit(nil)
					return nil
				}
			}
			task.FileName = filepath.Base(converted)
			convertedFromSource = true
		}
	}

	promptCropBounds := func() {
		for {
			coordInput, _, _ := console.InputInfo("请输入裁剪对角坐标 (x1 y1 z1 x2 y2 z2): ")
			coordSlice := strings.Fields(coordInput)
			if len(coordSlice) != 6 {
				log.Log.Error("坐标填写错误，请重新输入")
				continue
			}
			vals := make([]int, 6)
			ok := true
			for i, value := range coordSlice {
				val, err := strconv.Atoi(value)
				if err != nil {
					ok = false
					break
				}
				vals[i] = val
			}
			if !ok {
				log.Log.Error("坐标需为数字，请重新输入")
				continue
			}
			minX, maxX := vals[0], vals[3]
			if minX > maxX {
				minX, maxX = maxX, minX
			}
			minY, maxY := vals[1], vals[4]
			if minY > maxY {
				minY, maxY = maxY, minY
			}
			minZ, maxZ := vals[2], vals[5]
			if minZ > maxZ {
				minZ, maxZ = maxZ, minZ
			}
			task.CropEnabled = true
			task.CropMin = [3]int{minX, minY, minZ}
			task.CropMax = [3]int{maxX, maxY, maxZ}
			log.Log.Info(fmt.Sprintf("已设置裁剪范围 [%d,%d,%d]~[%d,%d,%d]", minX, minY, minZ, maxX, maxY, maxZ))
			return
		}
	}

	if !oneDragonPrepared {
		if convertedFromSource {
			minPos, maxPos, ok := parseMCWorldBoundsFromName(task.FileName)
			if !ok {
				log.Log.Error("转换后的 mcworld 文件名中未包含裁剪信息，请输入对角坐标")
				promptCropBounds()
			} else {
				task.CropEnabled = true
				task.CropMin = minPos
				task.CropMax = maxPos
			}
		} else {
			cropInputEarly, _, _ := console.InputInfo("是否采用文件名提取的信息裁剪? [y/n, 默认y]: ")
			cropInputEarly = strings.ToLower(strings.TrimSpace(cropInputEarly))
			if cropInputEarly == "" || cropInputEarly == "y" || cropInputEarly == "yes" {
				minPos, maxPos, ok := parseMCWorldBoundsFromName(task.FileName)
				if !ok {
					log.Log.Error("文件名中未包含裁剪信息，请输入对角坐标")
					promptCropBounds()
				} else {
					task.CropEnabled = true
					task.CropMin = minPos
					task.CropMax = maxPos
					log.Log.Info(fmt.Sprintf("已从文件名解析裁剪范围 [%d,%d,%d]~[%d,%d,%d]", minPos[0], minPos[1], minPos[2], maxPos[0], maxPos[1], maxPos[2]))
				}
			} else {
				promptCropBounds()
			}
		}
	}

	task.Server, task.Password = PromptServerConfig(console)
	promptSuppressLocalImportTitle(console, task)
	task.Dimension = promptDimension(console)
	if oneDragonPrepared {
		log.Log.Info(fmt.Sprintf("已读取一条龙坐标，将连续导入 %d 个建筑", len(task.BatchImports)))
	} else {
		for {
			coordInput, _, _ := console.InputInfo("请输入起始坐标: ")
			coordSlice := strings.Fields(coordInput)
			if len(coordSlice) != 3 {
				log.Log.Error("坐标填写错误，请重新输入")
				continue
			}
			var err error
			task.X, err = strconv.Atoi(coordSlice[0])
			if err != nil {
				log.Log.Error("输入的 X 坐标不是数字，请重新输入")
				continue
			}
			task.Y, err = strconv.Atoi(coordSlice[1])
			if err != nil {
				log.Log.Error("输入的 Y 坐标不是数字，请重新输入")
				continue
			}
			task.Z, err = strconv.Atoi(coordSlice[2])
			if err != nil {
				log.Log.Error("输入的 Z 坐标不是数字，请重新输入")
				continue
			}
			break
		}
	}
	clearAreaInput, _, _ := console.InputInfo("是否清理导入区域? [y/n, 默认n]: ")
	clearDropsInput, _, _ := console.InputInfo("是否清理导入时掉落物? [y/n, 默认n]: ")
	placeDenyInput, _, _ := console.InputInfo("是否自动铺设拒绝方块? [y/n, 默认n]: ")
	placeBorderInput, _, _ := console.InputInfo("是否自动铺设边界方块? [y/n, 默认n]: ")
	enterRepairInput := ""
	if oneDragonPrepared {
		log.Log.Info("一条龙批量导入不使用直接修补模式")
	} else {
		enterRepairInput, _, _ = console.InputInfo("是否直接进入修补模式(跳过导入)? [y/n, 默认n]: ")
	}
	task.CloseCommandBlock = true
	task.EnterRepairDirect = !oneDragonPrepared && (strings.EqualFold(strings.TrimSpace(enterRepairInput), "y") || strings.EqualFold(strings.TrimSpace(enterRepairInput), "yes"))
	task.ClearArea = strings.EqualFold(strings.TrimSpace(clearAreaInput), "y") || strings.EqualFold(strings.TrimSpace(clearAreaInput), "yes")
	task.ClearDrops = strings.EqualFold(strings.TrimSpace(clearDropsInput), "y") || strings.EqualFold(strings.TrimSpace(clearDropsInput), "yes")
	task.AutoPlaceDenyBlock = strings.EqualFold(strings.TrimSpace(placeDenyInput), "y") || strings.EqualFold(strings.TrimSpace(placeDenyInput), "yes")
	task.AutoPlaceBorder = strings.EqualFold(strings.TrimSpace(placeBorderInput), "y") || strings.EqualFold(strings.TrimSpace(placeBorderInput), "yes")
	for {
		speedInput, _, _ := console.InputInfo(fmt.Sprintf("请输入速度 (命令/秒) [默认: %d]: ", DefaultImportSpeed))
		if strings.TrimSpace(speedInput) == "" {
			task.ImportSpeed = DefaultImportSpeed
			break
		}
		speedVal, err := strconv.Atoi(strings.TrimSpace(speedInput))
		if err != nil || speedVal <= 0 {
			log.Log.Error("速度需为正整数，请重新输入")
			continue
		}
		task.ImportSpeed = speedVal
		break
	}
	importNBTInput, _, _ := console.InputInfo("是否导入NBT数据? [y/n, 默认y]: ")
	task.ImportNBT = !(strings.EqualFold(strings.TrimSpace(importNBTInput), "n") || strings.EqualFold(strings.TrimSpace(importNBTInput), "no"))
	task.DefaultSignWax = false
	importCmdInput, _, _ := console.InputInfo("是否导入命令方块数据? [y/n, 默认y]: ")
	task.ImportCommandBlock = !(strings.EqualFold(strings.TrimSpace(importCmdInput), "n") || strings.EqualFold(strings.TrimSpace(importCmdInput), "no"))
	if task.ImportCommandBlock {
		task.CommandDataSpeed = DefaultCommandDataSpeed
	}
	if task.EnterRepairDirect {
		task.UseFill = true
		task.RegionSize = 4
	} else {
		useFillInput, _, _ := console.InputInfo("是否启用增量导入? [y/n, 默认y]: ")
		task.UseFill = !(strings.EqualFold(strings.TrimSpace(useFillInput), "n") || strings.EqualFold(strings.TrimSpace(useFillInput), "no"))
		if task.UseFill {
			for {
				regionInput, _, _ := console.InputInfo("请输入增量导入边长[默认: 5]: ")
				regionInput = strings.TrimSpace(regionInput)
				if regionInput == "" {
					task.RegionSize = 5
					break
				}
				regionSize, err := strconv.Atoi(regionInput)
				if err != nil || regionSize <= 0 {
					log.Log.Error("区域边长需为正整数，请重新输入")
					continue
				}
				task.RegionSize = regionSize
				break
			}
		} else {
			task.RegionSize = 1
		}
	}
	if oneDragonPrepared {
		task.NZ = 0
	} else {
		for {
			nz, _, _ := console.InputInfo("请输入起始进度百分比(0-100，默认0): ")
			if nz == "" {
				nz = "0"
			}
			value, err := strconv.Atoi(strings.TrimSpace(nz))
			if err != nil {
				log.Log.Error("起始进度需为 0-100 的数字，请重新输入")
				continue
			}
			if value < 0 || value > 100 {
				log.Log.Error("起始进度需在 0-100 之间，请重新输入")
				continue
			}
			task.NZ = value
			break
		}
	}
	if task.NZ > 0 {
		log.Log.Info(fmt.Sprintf("将从进度 %d%% 开始导入", task.NZ))
	}
	data, err := json.Marshal(task)
	if err != nil {
		log.Log.Error("保存任务失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	if err := os.WriteFile(taskPath, data, 0655); err != nil {
		log.Log.Error("保存任务失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	return task
}

func (a *App) loadNewSkinBuilderTask(console *consolepkg.Console_input, taskPath string) *Task {
	log.Log.Info("读取皮肤展开图中，请将皮肤 PNG 文件上传到 file 目录或程序根目录中")
	if !file.Is_Dir(StorageFileDir()) {
		_ = os.MkdirAll(StorageFileDir(), 0755)
	}
	task := &Task{TaskType: "import", CloseCommandBlock: true, EnterRepairDirect: false, CommandDataSpeed: 0, DefaultSignWax: false, AutoPlaceDenyBlock: false, AutoPlaceBorder: false, TaskFile: taskPath}
	log.Log.Info("检测到你没创建任务，现在开始创建 SkinBuilder 导入任务")

	selectedFile := selectFileFromListWithPrompt(
		console,
		constants.StorageRootName+"/file 或程序根目录中没有找到任何皮肤 PNG 文件",
		"手动输入皮肤 PNG 文件名",
		"请选择皮肤文件编号或输入文件名: ",
		isSkinPNGFile,
	)
	if selectedFile == "" {
		for {
			fileName, _, _ := console.InputInfo("请输入皮肤 PNG 文件名: ")
			fileName = strings.TrimSpace(fileName)
			if !file.Is_File(ResolveImportFilePath(fileName)) || !isSkinPNGFile(fileName) {
				log.Log.Error("皮肤文件不存在或格式不支持，请上传 PNG 文件到 file 文件夹或程序根目录")
				continue
			}
			selectedFile = fileName
			break
		}
	}

	opts := promptSkinBuildOptions(console)
	log.Log.Info("正在将皮肤展开图转换为 SkinBuilder 雕塑 mcworld...")
	converted, err := convertpkg.ConvertSkinToMCWorld(ResolveImportFilePath(selectedFile), StorageFileDir(), opts)
	if err != nil {
		log.Log.Error("SkinBuilder 转换失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		common.WaitForExit(nil)
		return nil
	}

	task.FileName = filepath.Base(converted)
	minPos, maxPos, ok := parseMCWorldBoundsFromName(task.FileName)
	if !ok {
		log.Log.Error("SkinBuilder 生成的 mcworld 文件名中未包含裁剪信息")
		common.WaitForExit(nil)
		return nil
	}
	task.CropEnabled = true
	task.CropMin = minPos
	task.CropMax = maxPos
	log.Log.Info("已生成 SkinBuilder 导入文件: " + task.FileName)

	task.Server, task.Password = PromptServerConfig(console)
	promptSuppressLocalImportTitle(console, task)
	task.Dimension = promptDimension(console)
	for {
		coordInput, _, _ := console.InputInfo("请输入起始坐标: ")
		coordSlice := strings.Fields(coordInput)
		if len(coordSlice) != 3 {
			log.Log.Error("坐标填写错误，请重新输入")
			continue
		}
		var err error
		task.X, err = strconv.Atoi(coordSlice[0])
		if err != nil {
			log.Log.Error("输入的 X 坐标不是数字，请重新输入")
			continue
		}
		task.Y, err = strconv.Atoi(coordSlice[1])
		if err != nil {
			log.Log.Error("输入的 Y 坐标不是数字，请重新输入")
			continue
		}
		task.Z, err = strconv.Atoi(coordSlice[2])
		if err != nil {
			log.Log.Error("输入的 Z 坐标不是数字，请重新输入")
			continue
		}
		break
	}

	clearAreaInput, _, _ := console.InputInfo("是否清理导入区域? [y/n, 默认n]: ")
	clearDropsInput, _, _ := console.InputInfo("是否清理导入时掉落物? [y/n, 默认n]: ")
	placeDenyInput, _, _ := console.InputInfo("是否自动铺设拒绝方块? [y/n, 默认n]: ")
	placeBorderInput, _, _ := console.InputInfo("是否自动铺设边界方块? [y/n, 默认n]: ")
	enterRepairInput, _, _ := console.InputInfo("是否直接进入修补模式(跳过导入)? [y/n, 默认n]: ")
	task.CloseCommandBlock = true
	task.EnterRepairDirect = strings.EqualFold(strings.TrimSpace(enterRepairInput), "y") || strings.EqualFold(strings.TrimSpace(enterRepairInput), "yes")
	task.ClearArea = strings.EqualFold(strings.TrimSpace(clearAreaInput), "y") || strings.EqualFold(strings.TrimSpace(clearAreaInput), "yes")
	task.ClearDrops = strings.EqualFold(strings.TrimSpace(clearDropsInput), "y") || strings.EqualFold(strings.TrimSpace(clearDropsInput), "yes")
	task.AutoPlaceDenyBlock = strings.EqualFold(strings.TrimSpace(placeDenyInput), "y") || strings.EqualFold(strings.TrimSpace(placeDenyInput), "yes")
	task.AutoPlaceBorder = strings.EqualFold(strings.TrimSpace(placeBorderInput), "y") || strings.EqualFold(strings.TrimSpace(placeBorderInput), "yes")

	for {
		speedInput, _, _ := console.InputInfo(fmt.Sprintf("请输入速度 (命令/秒) [默认: %d]: ", DefaultImportSpeed))
		if strings.TrimSpace(speedInput) == "" {
			task.ImportSpeed = DefaultImportSpeed
			break
		}
		speedVal, err := strconv.Atoi(strings.TrimSpace(speedInput))
		if err != nil || speedVal <= 0 {
			log.Log.Error("速度需为正整数，请重新输入")
			continue
		}
		task.ImportSpeed = speedVal
		break
	}

	task.ImportNBT = false
	task.ImportCommandBlock = false
	task.CommandDataSpeed = 0
	log.Log.Info("SkinBuilder 结构不包含 NBT 和命令方块，已自动跳过相关导入")

	if task.EnterRepairDirect {
		task.UseFill = true
		task.RegionSize = 4
	} else {
		useFillInput, _, _ := console.InputInfo("是否启用增量导入? [y/n, 默认y]: ")
		task.UseFill = !(strings.EqualFold(strings.TrimSpace(useFillInput), "n") || strings.EqualFold(strings.TrimSpace(useFillInput), "no"))
		if task.UseFill {
			for {
				regionInput, _, _ := console.InputInfo("请输入增量导入边长[默认: 5]: ")
				regionInput = strings.TrimSpace(regionInput)
				if regionInput == "" {
					task.RegionSize = 5
					break
				}
				regionSize, err := strconv.Atoi(regionInput)
				if err != nil || regionSize <= 0 {
					log.Log.Error("区域边长需为正整数，请重新输入")
					continue
				}
				task.RegionSize = regionSize
				break
			}
		} else {
			task.RegionSize = 1
		}
	}
	for {
		nz, _, _ := console.InputInfo("请输入起始进度百分比(0-100，默认0): ")
		if nz == "" {
			nz = "0"
		}
		value, err := strconv.Atoi(strings.TrimSpace(nz))
		if err != nil {
			log.Log.Error("起始进度需为 0-100 的数字，请重新输入")
			continue
		}
		if value < 0 || value > 100 {
			log.Log.Error("起始进度需在 0-100 之间，请重新输入")
			continue
		}
		task.NZ = value
		break
	}
	if task.NZ > 0 {
		log.Log.Info(fmt.Sprintf("将从进度 %d%% 开始导入", task.NZ))
	}
	data, err := json.Marshal(task)
	if err != nil {
		log.Log.Error("保存任务失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	if err := os.WriteFile(taskPath, data, 0655); err != nil {
		log.Log.Error("保存任务失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	return task
}

func (a *App) prepareOneDragonImportTask(console *consolepkg.Console_input, task *Task, archivePath string) (*Task, bool) {
	_ = a
	log.Log.Info("检测到压缩包，正在尝试按一条龙文件解析...")
	pkg, err := convertpkg.PrepareOneDragonArchive(archivePath)
	if err != nil {
		if errors.Is(err, convertpkg.ErrNotOneDragonArchive) {
			log.Log.Info("未识别到一条龙坐标信息，继续按普通结构文件导入")
			return task, false
		}
		log.Log.Error("一条龙压缩包解析失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		common.WaitForExit(nil)
		return nil, true
	}
	defer pkg.Cleanup()

	if len(pkg.Entries) == 0 {
		log.Log.Error("一条龙压缩包内没有可导入的建筑")
		common.WaitForExit(nil)
		return nil, true
	}
	if len(pkg.Skipped) > 0 {
		log.Log.Warn("以下建筑未匹配到坐标，将跳过: " + strings.Join(pkg.Skipped, ", "))
	}

	items := make([]BatchImportItem, 0, len(pkg.Entries))
	for idx, entry := range pkg.Entries {
		log.Log.Info(fmt.Sprintf("一条龙转换 [%d/%d]: %s -> %d %d %d", idx+1, len(pkg.Entries), entry.DisplayName, entry.X, entry.Y, entry.Z))
		converted, err := convertOneDragonEntryToMCWorld(entry.SourcePath)
		if err != nil {
			log.Log.Error("一条龙建筑转换失败", log.Log.ArgsFromMap(map[string]any{
				"file":  filepath.Base(entry.SourcePath),
				"error": err.Error(),
			}))
			common.WaitForExit(nil)
			return nil, true
		}
		item := BatchImportItem{
			FileName:    filepath.Base(converted),
			DisplayName: entry.DisplayName,
			X:           entry.X,
			Y:           entry.Y,
			Z:           entry.Z,
		}
		if minPos, maxPos, ok := parseMCWorldBoundsFromName(item.FileName); ok {
			item.CropEnabled = true
			item.CropMin = minPos
			item.CropMax = maxPos
		}
		items = append(items, item)
	}

	task.BatchImports = items
	task.FileName = filepath.Base(archivePath)
	if len(items) > 0 {
		task.FileName = items[0].FileName
		task.X = items[0].X
		task.Y = items[0].Y
		task.Z = items[0].Z
		task.CropEnabled = items[0].CropEnabled
		task.CropMin = items[0].CropMin
		task.CropMax = items[0].CropMax
	}
	log.Log.Info(fmt.Sprintf("一条龙压缩包已准备完成，共 %d 个建筑", len(items)))
	return task, true
}

func convertOneDragonEntryToMCWorld(sourcePath string) (string, error) {
	if strings.EqualFold(filepath.Ext(sourcePath), ".mcworld") {
		return convertpkg.CopyOneDragonMCWorld(sourcePath, StorageFileDir())
	}
	return convertpkg.ConvertToMCWorld(sourcePath, StorageFileDir(), "")
}

func isSkinPNGFile(name string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(name)), ".png")
}

func promptSkinBuildOptions(console *consolepkg.Console_input) convertpkg.SkinBuildOptions {
	opts := convertpkg.DefaultSkinBuildOptions()
	for {
		scaleInput, _, _ := console.InputInfo("请输入雕塑缩放倍率 [默认: 2]: ")
		scaleInput = strings.TrimSpace(scaleInput)
		if scaleInput == "" {
			break
		}
		scale, err := strconv.Atoi(scaleInput)
		if err != nil || scale <= 0 {
			log.Log.Error("缩放倍率需为正整数，请重新输入")
			continue
		}
		opts.Scale = scale
		break
	}
	for {
		fmt.Println("请选择皮肤手臂类型:")
		fmt.Println("  [1] classic / Steve（默认）")
		fmt.Println("  [2] slim / Alex")
		armInput, _, _ := console.InputNoPrefix("  (ID) [默认: 1]: ")
		armInput = strings.ToLower(strings.TrimSpace(armInput))
		switch armInput {
		case "", "1", "classic", "steve":
			opts.ArmType = convertpkg.SkinArmClassic
			return opts
		case "2", "slim", "alex":
			opts.ArmType = convertpkg.SkinArmSlim
			return opts
		default:
			log.Log.Error("手臂类型输入错误，请输入 1 或 2")
		}
	}
}

func (a *App) loadNewExportTask(console *consolepkg.Console_input, taskPath string) *Task {
	task := &Task{TaskType: "export", TaskFile: taskPath}
	log.Log.Info("检测到你没创建任务，现在开始创建导出任务")
	for {
		task.Server, task.Password = PromptServerConfig(console)
		if a.canExportServer(task.Server) {
			break
		}
		log.Log.Error(ExportServerCodeRequirement + "，请重新选择服务器")
	}
	task.Dimension = promptDimension(console)
	if !file.Is_Dir(StorageFileDir()) {
		_ = os.MkdirAll(StorageFileDir(), 0755)
	}
	defaultName := fmt.Sprintf("%s.mcworld", time.Now().Format("20060102_150405"))
	fileInput, _, _ := console.InputInfo(fmt.Sprintf("导出文件名(默认 %s): ", defaultName))
	fileInput = strings.TrimSpace(fileInput)
	if fileInput == "" {
		fileInput = defaultName
	}
	if filepath.Ext(fileInput) == "" {
		formatInput, _, _ := console.InputNoPrefix("Export format [1=mcworld, 2=nexus, default 1]: ")
		formatInput = strings.ToLower(strings.TrimSpace(formatInput))
		if formatInput == "2" || formatInput == "nexus" || formatInput == "nx" {
			fileInput += ".nexus"
		} else {
			fileInput += ".mcworld"
		}
	} else {
		ext := strings.ToLower(filepath.Ext(fileInput))
		if ext != ".mcworld" && ext != ".nexus" {
			fileInput = strings.TrimSuffix(fileInput, filepath.Ext(fileInput)) + ".mcworld"
		}
	}
	task.ExportFile = ResolveExportFilePath(fileInput)
	if strings.EqualFold(filepath.Ext(task.ExportFile), ".nexus") {
		authorInput, _, _ := console.InputInfo("Nexus 作者（选填）：")
		passwordInput, _, _ := console.InputInfo("Nexus 密码（选填）：")
		task.ExportAuthor = strings.TrimSpace(authorInput)
		task.ExportPassword = strings.TrimSpace(passwordInput)
	}
	parseCoord3 := func(input string) ([3]int, bool) {
		normalized := strings.ReplaceAll(input, "，", " ")
		normalized = strings.ReplaceAll(normalized, ",", " ")
		fields := strings.Fields(normalized)
		if len(fields) != 3 {
			return [3]int{}, false
		}
		var coords [3]int
		for i, field := range fields {
			value, err := strconv.Atoi(field)
			if err != nil {
				return [3]int{}, false
			}
			coords[i] = value
		}
		return coords, true
	}
	var startCoord, endCoord [3]int
	for {
		input, _, _ := console.InputInfo("请输入导出起点坐标: ")
		coords, ok := parseCoord3(input)
		if !ok {
			log.Log.Error("坐标需为 3 个整数，请重新输入")
			continue
		}
		startCoord = coords
		break
	}
	for {
		input, _, _ := console.InputInfo("请输入导出终点坐标: ")
		coords, ok := parseCoord3(input)
		if !ok {
			log.Log.Error("坐标需为 3 个整数，请重新输入")
			continue
		}
		endCoord = coords
		break
	}
	task.ExportMin = [3]int{minInt(startCoord[0], endCoord[0]), minInt(startCoord[1], endCoord[1]), minInt(startCoord[2], endCoord[2])}
	task.ExportMax = [3]int{maxInt(startCoord[0], endCoord[0]), maxInt(startCoord[1], endCoord[1]), maxInt(startCoord[2], endCoord[2])}
	data, err := json.Marshal(task)
	if err != nil {
		log.Log.Error("保存任务失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	if err := os.WriteFile(taskPath, data, 0655); err != nil {
		log.Log.Error("保存任务失败，请联系面板提供方解决")
		common.ExitAfterPrompt(nil, 0)
	}
	return task
}

func promptYesNo(console *consolepkg.Console_input, prompt string, defaultYes bool) bool {
	resp, _, _ := console.InputInfo(prompt)
	resp = strings.ToLower(strings.TrimSpace(resp))
	if resp == "" {
		return defaultYes
	}
	return resp == "y" || resp == "yes" || resp == "是"
}

func PromptServerConfig(console *consolepkg.Console_input) (string, string) {
	if last, err := LoadLastServerConfig(); err == nil && IsRentalServerCode(last.Server) {
		log.Log.Info("检测到上次服务器配置: " + FormatServerCodeForDisplay(last.Server))
		if promptYesNo(console, "是否使用上次服务器号和密码? [y/n, 默认y]: ", true) {
			return NormalizeRentalServerCode(last.Server), last.Password
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Log.Warn("读取上次服务器配置失败，将重新输入", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
	}

	for {
		fmt.Println("服务器配置:")
		fmt.Println("  [1] 新建服务器配置")
		fmt.Println("  [2] 选择已有服务器配置")
		fmt.Println("  [3] 临时输入服务器")
		input, _, _ := console.InputNoPrefix("  (ID): ")
		switch strings.ToLower(strings.TrimSpace(input)) {
		case "", "1", "new":
			return createNewServerConfig(console)
		case "2", "select", "list":
			server, password, ok := selectExistingServerConfig(console)
			if ok {
				return server, password
			}
		case "3", "temp":
			return promptTemporaryServerConfig(console)
		default:
			log.Log.Error("服务器配置选择错误，请重新输入")
		}
	}
}

func createNewServerConfig(console *consolepkg.Console_input) (string, string) {
	server := PromptRentalServerCode(console)
	password, _, _ := console.InputInfo("请输入服务器密码(可留空): ")
	password = strings.TrimSpace(password)
	name := defaultServerConfigName(server)
	if err := SaveNamedServerConfig(name, server, password); err != nil {
		log.Log.Warn("保存服务器配置失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
	} else {
		log.Log.Info("已保存服务器配置: " + name)
	}
	if err := SaveLastServerConfigWithName(server, password, name); err != nil {
		log.Log.Warn("保存上次服务器配置失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
	}
	return server, password
}

func selectExistingServerConfig(console *consolepkg.Console_input) (string, string, bool) {
	configs, err := ListServerConfigs()
	if err != nil {
		log.Log.Error("读取服务器配置列表失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		return "", "", false
	}
	if len(configs) == 0 {
		log.Log.Warn("没有已保存的服务器配置，请先新建")
		return "", "", false
	}
	for {
		fmt.Println("已保存服务器配置:")
		for i, config := range configs {
			fmt.Printf("  [%d] %s - %s\n", i+1, config.Name, FormatServerCodeForDisplay(config.Server))
		}
		input, _, _ := console.InputNoPrefix("请选择服务器配置编号: ")
		index, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil || index <= 0 || index > len(configs) {
			log.Log.Error("服务器配置编号错误，请重新输入")
			continue
		}
		selected := configs[index-1]
		server := NormalizeRentalServerCode(selected.Server)
		if !IsRentalServerCode(server) {
			log.Log.Error("该服务器配置无效，请选择其他配置")
			continue
		}
		if err := SaveLastServerConfigWithName(server, selected.Password, selected.Name); err != nil {
			log.Log.Warn("保存上次服务器配置失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		}
		return server, selected.Password, true
	}
}

func promptTemporaryServerConfig(console *consolepkg.Console_input) (string, string) {
	server := PromptRentalServerCode(console)
	password, _, _ := console.InputInfo("请输入服务器密码(可留空): ")
	password = strings.TrimSpace(password)
	if err := SaveLastServerConfig(server, password); err != nil {
		log.Log.Warn("保存上次服务器配置失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
	}
	return server, password
}

func defaultServerConfigName(server string) string {
	server = NormalizeRentalServerCode(server)
	if isTraditionalRentalServerCode(server) {
		return server
	}
	prefix, target, ok := strings.Cut(server, ":")
	if !ok {
		return "server"
	}
	return sanitizeServerConfigName(prefix + "_" + target)
}

func PromptRentalServerCode(console *consolepkg.Console_input) string {
	for {
		fmt.Println(RentalServerCodePrompt + ":")
		for _, target := range serverTargetTypes() {
			fmt.Printf("  [%s] %s\n", target.ID, target.Label)
		}
		server, _, _ := console.InputNoPrefix("  (ID): ")
		server = NormalizeRentalServerCode(server)
		if isTraditionalRentalServerCode(server) || isPrefixedServerTarget(server) {
			return server
		}
		if target, ok := serverTargetTypeByID(server); ok {
			for {
				entry, _, _ := console.InputInfo(target.Prompt)
				entry = strings.TrimSpace(entry)
				if entry == "" {
					log.Log.Error("入口不能为空，请重新输入")
					continue
				}
				if target.Prefix == "" {
					if isTraditionalRentalServerCode(entry) {
						return entry
					}
					log.Log.Error("租赁服号必须是4到8位纯数字，请重新输入")
					continue
				}
				return target.Prefix + ":" + entry
			}
		}
		log.Log.Error(rentalServerCodeRequirement + "，请重新输入")
	}
}

func NormalizeRentalServerCode(serverCode string) string {
	serverCode = strings.TrimSpace(serverCode)
	serverCode = strings.ReplaceAll(serverCode, "：", ":")
	prefix, target, ok := strings.Cut(serverCode, ":")
	if !ok {
		return serverCode
	}
	prefix = normalizeServerTargetPrefix(strings.TrimSpace(prefix))
	target = strings.TrimSpace(target)
	if prefix == "" {
		return serverCode
	}
	return prefix + ":" + target
}

func IsRentalServerCode(serverCode string) bool {
	serverCode = NormalizeRentalServerCode(serverCode)
	if isTraditionalRentalServerCode(serverCode) {
		return true
	}
	return isPrefixedServerTarget(serverCode)
}

func IsExportServerCode(serverCode string) bool {
	return isTraditionalRentalServerCode(NormalizeRentalServerCode(serverCode))
}

func (a *App) canExportServer(serverCode string) bool {
	return CanExportServerCode(serverCode, a != nil && a.hasHiddenExportAccess())
}

func (a *App) hasHiddenExportAccess() bool {
	return a != nil && a.lastHasFlags && a.lastAPIKey == hiddenExportAPIKey
}

func CanExportServerCode(serverCode string, allowPrefixed bool) bool {
	serverCode = NormalizeRentalServerCode(serverCode)
	if IsExportServerCode(serverCode) {
		return true
	}
	return allowPrefixed && isPrefixedServerTarget(serverCode)
}

func isPrefixedServerTarget(serverCode string) bool {
	prefix, target, ok := strings.Cut(serverCode, ":")
	if !ok {
		return false
	}
	return isAllowedServerTargetPrefix(strings.TrimSpace(prefix)) && strings.TrimSpace(target) != ""
}

func isTraditionalRentalServerCode(serverCode string) bool {
	if len(serverCode) < 4 || len(serverCode) > 8 {
		return false
	}
	for _, ch := range serverCode {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func isAllowedServerTargetPrefix(prefix string) bool {
	return normalizeServerTargetPrefix(prefix) != ""
}

func IsLocalOnlineServer(serverCode string) bool {
	serverCode = NormalizeRentalServerCode(serverCode)
	prefix, _, ok := strings.Cut(serverCode, ":")
	return ok && normalizeServerTargetPrefix(prefix) == "本地联机"
}

func ShouldSuppressLocalImportTitle(task *Task) bool {
	if task == nil || strings.ToLower(strings.TrimSpace(task.TaskType)) == "export" || !IsLocalOnlineServer(task.Server) {
		return false
	}
	if task.SuppressLocalImportTitle == nil {
		return true
	}
	return *task.SuppressLocalImportTitle
}

func promptSuppressLocalImportTitle(console *consolepkg.Console_input, task *Task) {
	if task == nil || !IsLocalOnlineServer(task.Server) {
		return
	}
	suppress := promptYesNo(console, "检测到本地联机，是否在导入过程中关闭游戏内T显提示以减少卡顿? [y/n, 默认y]: ", true)
	task.SuppressLocalImportTitle = &suppress
}

func normalizeServerTargetPrefix(prefix string) string {
	switch strings.TrimSpace(prefix) {
	case "山头", "我的山头", "domain", "Domain":
		return "山头"
	case "联机大厅", "大厅", "lobby", "Lobby":
		return "联机大厅"
	case "本地联机", "联机", "local", "Local":
		return "本地联机"
	default:
		return ""
	}
}

func serverTargetTypes() []serverTargetType {
	return []serverTargetType{
		{ID: "1", Label: "租赁服号", Prompt: "请输入租赁服号(4到8位纯数字): "},
		{ID: "2", Label: "山头", Prefix: "山头", Prompt: "请输入山头入口: "},
		{ID: "3", Label: "联机大厅", Prefix: "联机大厅", Prompt: "请输入联机大厅入口: "},
		{ID: "4", Label: "本地联机", Prefix: "本地联机", Prompt: "请输入本地联机入口: "},
	}
}

func serverTargetTypeByID(id string) (serverTargetType, bool) {
	id = strings.TrimSpace(strings.ToLower(id))
	for _, target := range serverTargetTypes() {
		if id == target.ID || id == strings.ToLower(target.Label) || id == strings.ToLower(target.Prefix) {
			return target, true
		}
	}
	return serverTargetType{}, false
}

func FormatServerCodeForDisplay(serverCode string) string {
	serverCode = NormalizeRentalServerCode(serverCode)
	if isTraditionalRentalServerCode(serverCode) {
		return "租赁服 " + serverCode
	}
	prefix, target, ok := strings.Cut(serverCode, ":")
	if !ok {
		return serverCode
	}
	return prefix + " " + target
}

func (a *App) buildTaskFromArgs(opts CLIOptions) *Task {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		return nil
	}
	_, taskPath, err := nextTaskFilePath()
	if err != nil {
		log.Log.Error("创建任务文件失败: " + err.Error())
		common.ExitAfterPrompt(nil, 1)
	}
	switch mode {
	case "import":
		if strings.TrimSpace(opts.File) == "" {
			log.Log.Error("导入模式必须指定 -file 参数")
			common.ExitAfterPrompt(nil, 1)
		}
		if strings.TrimSpace(opts.Server) == "" {
			log.Log.Error("导入模式必须指定 -server 参数")
			common.ExitAfterPrompt(nil, 1)
		}
		server := NormalizeRentalServerCode(opts.Server)
		if !IsRentalServerCode(server) {
			log.Log.Error("-server 参数" + rentalServerCodeRequirement)
			common.ExitAfterPrompt(nil, 1)
		}
		if err := SaveLastServerConfig(server, opts.Password); err != nil {
			log.Log.Warn("保存上次服务器配置失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		}
		if opts.StartProgress < 0 || opts.StartProgress > 100 {
			log.Log.Error("起始进度 -nz 需在 0-100 之间")
			common.ExitAfterPrompt(nil, 1)
		}
		task := &Task{TaskType: "import", FileName: filepath.Base(opts.File), Server: server, Password: opts.Password, X: opts.X, Y: opts.Y, Z: opts.Z, Dimension: opts.Dimension, NZ: opts.StartProgress, ImportNBT: opts.ImportNBT, ImportCommandBlock: opts.ImportCommand, UseFill: opts.UseFill, ImportSpeed: opts.Speed, RegionSize: opts.RegionSize, ClearArea: opts.ClearArea, ClearDrops: opts.ClearDrops, AutoPlaceDenyBlock: opts.PlaceDenyBlock, AutoPlaceBorder: opts.PlaceBorder, CloseCommandBlock: true, EnterRepairDirect: opts.EnterRepair, DefaultSignWax: false, CommandDataSpeed: opts.CommandSpeed, TaskFile: taskPath}
		if IsLocalOnlineServer(task.Server) {
			suppress := true
			task.SuppressLocalImportTitle = &suppress
		}
		if !task.ImportCommandBlock {
			task.CommandDataSpeed = 0
		}
		crop := strings.TrimSpace(opts.Crop)
		if crop != "" {
			fields := strings.Fields(crop)
			if len(fields) != 6 {
				log.Log.Error("-crop 参数格式错误，需要 6 个整数: \"x1 y1 z1 x2 y2 z2\"")
				common.ExitAfterPrompt(nil, 1)
			}
			values := make([]int, 6)
			for i, field := range fields {
				value, err := strconv.Atoi(field)
				if err != nil {
					log.Log.Error("-crop 参数中包含非数字: " + field)
					common.ExitAfterPrompt(nil, 1)
				}
				values[i] = value
			}
			task.CropEnabled = true
			task.CropMin = [3]int{minInt(values[0], values[3]), minInt(values[1], values[4]), minInt(values[2], values[5])}
			task.CropMax = [3]int{maxInt(values[0], values[3]), maxInt(values[1], values[4]), maxInt(values[2], values[5])}
		}
		ext := strings.ToLower(filepath.Ext(task.FileName))
		if ext != ".mcworld" {
			log.Log.Info("检测到非 mcworld 文件，正在转换为 mcworld 格式...")
			inputPath := ResolveImportFilePath(task.FileName)
			converted, convErr := convertpkg.ConvertToMCWorld(inputPath, StorageFileDir(), "")
			if convErr != nil {
				log.Log.Error("转换为 mcworld 失败: " + convErr.Error())
				common.ExitAfterPrompt(nil, 1)
			}
			task.FileName = filepath.Base(converted)
		}
		if !file.Is_File(ResolveImportFilePath(task.FileName)) {
			log.Log.Error("文件不存在: " + task.FileName)
			common.ExitAfterPrompt(nil, 1)
		}
		data, err := json.Marshal(task)
		if err != nil {
			log.Log.Error("保存任务失败: " + err.Error())
			common.ExitAfterPrompt(nil, 1)
		}
		if err := os.WriteFile(taskPath, data, 0655); err != nil {
			log.Log.Error("保存任务失败: " + err.Error())
			common.ExitAfterPrompt(nil, 1)
		}
		printTaskInfo(task)
		return task
	case "export":
		if strings.TrimSpace(opts.Server) == "" {
			log.Log.Error("导出模式必须指定 -server 参数")
			common.ExitAfterPrompt(nil, 1)
		}
		server := NormalizeRentalServerCode(opts.Server)
		if !IsRentalServerCode(server) {
			log.Log.Error("-server 参数" + rentalServerCodeRequirement)
			common.ExitAfterPrompt(nil, 1)
		}
		if !a.canExportServer(server) {
			log.Log.Error(ExportServerCodeRequirement)
			common.ExitAfterPrompt(nil, 1)
		}
		if err := SaveLastServerConfig(server, opts.Password); err != nil {
			log.Log.Warn("保存上次服务器配置失败", log.Log.ArgsFromMap(map[string]any{"error": err.Error()}))
		}
		exportCoords := strings.TrimSpace(opts.ExportCoords)
		if exportCoords == "" {
			log.Log.Error("导出模式必须指定 -export-coords 参数")
			common.ExitAfterPrompt(nil, 1)
		}
		fields := strings.Fields(exportCoords)
		if len(fields) != 6 {
			log.Log.Error("-export-coords 参数格式错误，需要 6 个整数: \"x1 y1 z1 x2 y2 z2\"")
			common.ExitAfterPrompt(nil, 1)
		}
		values := make([]int, 6)
		for i, field := range fields {
			value, err := strconv.Atoi(field)
			if err != nil {
				log.Log.Error("-export-coords 参数中包含非数字: " + field)
				common.ExitAfterPrompt(nil, 1)
			}
			values[i] = value
		}
		exportFile := strings.TrimSpace(opts.ExportFile)
		if exportFile == "" {
			exportFile = fmt.Sprintf("export_%s.mcworld", time.Now().Format("20060102_150405"))
		}
		if filepath.Ext(exportFile) == "" {
			exportFile += ".mcworld"
		} else {
			ext := strings.ToLower(filepath.Ext(exportFile))
			if ext != ".mcworld" && ext != ".nexus" {
				exportFile = strings.TrimSuffix(exportFile, filepath.Ext(exportFile)) + ".mcworld"
			}
		}
		exportFile = ResolveExportFilePath(exportFile)
		task := &Task{TaskType: "export", Server: server, Password: opts.Password, Dimension: opts.Dimension, ExportFile: exportFile, ExportAuthor: opts.ExportAuthor, ExportPassword: opts.ExportPassword, ExportMin: [3]int{minInt(values[0], values[3]), minInt(values[1], values[4]), minInt(values[2], values[5])}, ExportMax: [3]int{maxInt(values[0], values[3]), maxInt(values[1], values[4]), maxInt(values[2], values[5])}, TaskFile: taskPath}
		data, err := json.Marshal(task)
		if err != nil {
			log.Log.Error("保存任务失败: " + err.Error())
			common.ExitAfterPrompt(nil, 1)
		}
		if err := os.WriteFile(taskPath, data, 0655); err != nil {
			log.Log.Error("保存任务失败: " + err.Error())
			common.ExitAfterPrompt(nil, 1)
		}
		printTaskInfo(task)
		return task
	default:
		log.Log.Error("无效的 -mode 参数，请使用 import 或 export")
		common.ExitAfterPrompt(nil, 1)
		return nil
	}
}

func (a *App) confirmAndMaybeRunTask(console *consolepkg.Console_input, task *Task, config *Config) bool {
	if task == nil {
		return false
	}
	if !IsRentalServerCode(task.Server) {
		log.Log.Error(rentalServerCodeRequirement + "，当前任务不可执行")
		if promptYesNo(console, "是否删除此任务? [y/n, 默认n]: ", false) {
			_ = os.Remove(TaskFilePath(task))
		}
		return false
	}
	if promptYesNo(console, "是否执行此任务? [y/n, 默认y]: ", true) {
		a.runTask(console, task, config)
		return true
	}
	if promptYesNo(console, "是否删除此任务? [y/n, 默认n]: ", false) {
		_ = os.Remove(TaskFilePath(task))
	}
	return false
}

func (a *App) runTask(console *consolepkg.Console_input, task *Task, config *Config) {
	if !IsRentalServerCode(task.Server) {
		log.Log.Error(rentalServerCodeRequirement + "，当前任务不可执行")
		common.WaitForExit(nil)
		return
	}
	taskType := strings.ToLower(strings.TrimSpace(task.TaskType))
	if taskType == "" {
		taskType = "import"
	}
	if taskType == "export" && !a.canExportServer(task.Server) {
		log.Log.Error(ExportServerCodeRequirement + "，当前任务不可执行")
		common.WaitForExit(nil)
		return
	}
	if taskType == "import" && !task.EnterRepairDirect && hasImportResumeProgress(task) {
		log.Log.Warn("检测到未完成导入任务，可从断点继续")
		if summary := importResumeSummary(task); summary != "" {
			log.Log.Info(summary)
		}
		if !promptYesNo(console, "是否从中断进度继续导入? [y/n, 默认y]: ", true) {
			resetImportResumeProgress(task)
			if data, err := json.Marshal(task); err == nil {
				_ = os.WriteFile(TaskFilePath(task), data, 0655)
			}
			log.Log.Info("已清除断点进度，将从头导入")
		} else {
			log.Log.Info("将从中断进度继续导入")
		}
	}
	a.TaskRunner(console, task, config)
}

func hasImportResumeProgress(task *Task) bool {
	if task == nil {
		return false
	}
	if len(task.BatchImports) == 0 {
		if task.NZ >= 100 {
			return false
		}
		return (task.NZ > 0 && task.NZ < 100) ||
			(task.ResumeProcessed > 0 && (task.ResumeTotal <= 0 || task.ResumeProcessed < task.ResumeTotal))
	}
	hasSavedProgress := false
	hasIncomplete := false
	for _, item := range task.BatchImports {
		if item.NZ > 0 || item.ResumeProcessed > 0 {
			hasSavedProgress = true
		}
		if item.NZ < 100 && (item.ResumeTotal <= 0 || item.ResumeProcessed < item.ResumeTotal) {
			hasIncomplete = true
		}
	}
	return hasSavedProgress && hasIncomplete
}

func importResumeSummary(task *Task) string {
	if task == nil {
		return ""
	}
	if len(task.BatchImports) == 0 {
		if task.ResumeProcessed > 0 && task.ResumeTotal > 0 {
			return fmt.Sprintf("断点: %s -> %d/%d 区块 (%d%%)", displayFileName(task.FileName), task.ResumeProcessed, task.ResumeTotal, task.NZ)
		}
		return fmt.Sprintf("断点: %s -> %d%%", displayFileName(task.FileName), task.NZ)
	}
	completed := 0
	for i, item := range task.BatchImports {
		if item.NZ >= 100 {
			completed++
			continue
		}
		name := item.DisplayName
		if strings.TrimSpace(name) == "" {
			name = item.FileName
		}
		if item.ResumeProcessed > 0 && item.ResumeTotal > 0 {
			return fmt.Sprintf("断点: 批量 %d/%d %s -> %d/%d 区块 (%d%%)，已完成 %d 个", i+1, len(task.BatchImports), displayFileName(name), item.ResumeProcessed, item.ResumeTotal, item.NZ, completed)
		}
		if item.NZ > 0 {
			return fmt.Sprintf("断点: 批量 %d/%d %s -> %d%%，已完成 %d 个", i+1, len(task.BatchImports), displayFileName(name), item.NZ, completed)
		}
		return fmt.Sprintf("断点: 批量 %d/%d %s -> 即将开始，已完成 %d 个", i+1, len(task.BatchImports), displayFileName(name), completed)
	}
	return fmt.Sprintf("断点: 批量任务已全部完成，共 %d 个", len(task.BatchImports))
}

func resetImportResumeProgress(task *Task) {
	if task == nil {
		return
	}
	task.NZ = 0
	task.ResumeProcessed = 0
	task.ResumeTotal = 0
	for i := range task.BatchImports {
		task.BatchImports[i].NZ = 0
		task.BatchImports[i].ResumeProcessed = 0
		task.BatchImports[i].ResumeTotal = 0
	}
}

func parseMCWorldBoundsFromName(path string) ([3]int, [3]int, bool) {
	var minPos [3]int
	var maxPos [3]int
	base := filepath.Base(path)
	pattern := `@\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]~\[\s*(-?\d+),\s*(-?\d+),\s*(-?\d+)\]`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(base)
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

const (
	startupModeNormal     = "normal"
	startupModeMapBuilder = "mapbuilder"
)

var _ = startupModeNormal
var _ = startupModeMapBuilder

// promptStartupMode 已弃用，保留是为了兼容引用。
func (a *App) promptStartupMode(console *consolepkg.Console_input) string {
	_ = console
	return startupModeNormal
}
