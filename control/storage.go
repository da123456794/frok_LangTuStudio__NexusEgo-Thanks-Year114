package control

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"nexus/constants"
	"nexus/utils/dimension"
	"nexus/utils/log"
)

func StorageFileDir() string {
	return filepath.Join(constants.StorageRootName, constants.StorageFileDirName)
}

func StorageTaskDir() string {
	return filepath.Join(constants.StorageRootName, constants.StorageTaskDirName)
}

func StorageLogDir() string {
	return filepath.Join(constants.StorageRootName, constants.StorageLogDirName)
}

func StorageTokenFilePath() string {
	return filepath.Join(constants.StorageRootName, constants.StorageTokenFileName)
}

func StorageLastServerFilePath() string {
	return filepath.Join(constants.StorageRootName, "last_server.json")
}

func StorageServerConfigDir() string {
	return filepath.Join(constants.StorageRootName, "server_configs")
}

func EnsureStorageDirs() error {
	if err := os.MkdirAll(StorageFileDir(), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(StorageTaskDir(), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(StorageServerConfigDir(), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(constants.StorageRootName, constants.StorageAuthDirName), 0755); err != nil {
		return err
	}
	return os.MkdirAll(StorageLogDir(), 0755)
}

func LoadLastServerConfig() (LastServerConfig, error) {
	var config LastServerConfig
	data, err := os.ReadFile(StorageLastServerFilePath())
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	return config, nil
}

func SaveLastServerConfig(server, password string) error {
	return SaveLastServerConfigWithName(server, password, "")
}

func SaveLastServerConfigWithName(server, password, configName string) error {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil
	}
	if err := os.MkdirAll(constants.StorageRootName, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(LastServerConfig{
		Server:     server,
		Password:   password,
		UsedAt:     time.Now().Format(time.RFC3339),
		ConfigName: strings.TrimSpace(configName),
	}, "", "  ")
	if err != nil {
		return err
	}
	target := StorageLastServerFilePath()
	temp := target + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, target)
}

func ListServerConfigs() ([]ServerConfigEntry, error) {
	entries, err := os.ReadDir(StorageServerConfigDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	configs := make([]ServerConfigEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		config, err := LoadNamedServerConfig(name)
		if err != nil {
			continue
		}
		configs = append(configs, ServerConfigEntry{
			Name:     name,
			Path:     filepath.Join(StorageServerConfigDir(), entry.Name()),
			Server:   config.Server,
			Password: config.Password,
			UsedAt:   config.UsedAt,
		})
	}
	sort.Slice(configs, func(i, j int) bool {
		return strings.ToLower(configs[i].Name) < strings.ToLower(configs[j].Name)
	})
	return configs, nil
}

func LoadNamedServerConfig(name string) (LastServerConfig, error) {
	var config LastServerConfig
	name = sanitizeServerConfigName(name)
	if name == "" {
		return config, fmt.Errorf("server config name is empty")
	}
	data, err := os.ReadFile(filepath.Join(StorageServerConfigDir(), name+".json"))
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	config.ConfigName = name
	return config, nil
}

func SaveNamedServerConfig(name, server, password string) error {
	name = sanitizeServerConfigName(name)
	server = strings.TrimSpace(server)
	if name == "" {
		return fmt.Errorf("server config name is empty")
	}
	if server == "" {
		return fmt.Errorf("server is empty")
	}
	if err := os.MkdirAll(StorageServerConfigDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(LastServerConfig{
		Server:     server,
		Password:   password,
		UsedAt:     time.Now().Format(time.RFC3339),
		ConfigName: name,
	}, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(StorageServerConfigDir(), name+".json")
	temp := target + ".tmp"
	if err := os.WriteFile(temp, data, 0600); err != nil {
		return err
	}
	return os.Rename(temp, target)
}

func sanitizeServerConfigName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	name = replacer.Replace(name)
	name = strings.Trim(name, " .")
	if name == "" {
		return ""
	}
	return name
}

func StorageFilePath(name string) string {
	if strings.TrimSpace(name) == "" {
		return StorageFileDir()
	}
	return filepath.Join(StorageFileDir(), name)
}

func TaskFilePath(task *Task) string {
	if task == nil {
		return filepath.Join(StorageTaskDir(), "task")
	}
	path := strings.TrimSpace(task.TaskFile)
	if path == "" {
		return filepath.Join(StorageTaskDir(), "task")
	}
	if filepath.IsAbs(path) {
		return path
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	storageTask := strings.ReplaceAll(StorageTaskDir(), "\\", "/")
	if normalized == storageTask || strings.HasPrefix(normalized, storageTask+"/") {
		return filepath.FromSlash(normalized)
	}
	return filepath.Join(StorageTaskDir(), path)
}

func ResolveImportFilePath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		return name
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(normalized, constants.StorageRootName+"/") {
		return filepath.FromSlash(normalized)
	}
	if strings.HasPrefix(normalized, "file/") {
		return filepath.Join(StorageFileDir(), strings.TrimPrefix(normalized, "file/"))
	}
	if !strings.Contains(normalized, "/") {
		storagePath := filepath.Join(StorageFileDir(), normalized)
		if isRegularFile(storagePath) {
			return storagePath
		}
		rootPath := filepath.FromSlash(normalized)
		if isRegularFile(rootPath) {
			return rootPath
		}
		return storagePath
	}
	rootPath := filepath.FromSlash(normalized)
	if isRegularFile(rootPath) {
		return rootPath
	}
	return filepath.Join(constants.StorageRootName, filepath.FromSlash(normalized))
}

func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func ResolveExportFilePath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if filepath.IsAbs(name) {
		return name
	}
	normalized := strings.ReplaceAll(name, "\\", "/")
	if !strings.Contains(normalized, "/") {
		return filepath.Join(StorageFileDir(), normalized)
	}
	if strings.HasPrefix(normalized, constants.StorageRootName+"/") {
		return filepath.FromSlash(normalized)
	}
	if strings.HasPrefix(normalized, "file/") {
		return filepath.Join(StorageFileDir(), strings.TrimPrefix(normalized, "file/"))
	}
	return filepath.Join(constants.StorageRootName, filepath.FromSlash(normalized))
}

func displayFileName(name string) string {
	base := filepath.Base(name)
	ext := filepath.Ext(base)
	base = strings.TrimSuffix(base, ext)
	if idx := strings.Index(base, "@"); idx > 0 {
		base = base[:idx]
	}
	return base
}

func formatDimensionCN(input string) string {
	info, err := dimension.Parse(strings.TrimSpace(input))
	if err != nil {
		if strings.TrimSpace(input) == "" {
			return "主世界"
		}
		return strings.TrimSpace(input)
	}
	switch info.Name {
	case "overworld":
		return "主世界"
	case "nether":
		return "下界"
	case "the_end":
		return "末地"
	default:
		return info.Name
	}
}

func formatEnabledCN(enabled bool) string {
	if enabled {
		return "开启"
	}
	return "关闭"
}

func truncateStringByWidth(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}
	width := 0
	result := ""
	for _, r := range s {
		charWidth := 1
		if r > 127 {
			charWidth = 2
		}
		if width+charWidth > maxWidth-3 {
			return result + "..."
		}
		result += string(r)
		width += charWidth
	}
	return result
}

func formatFileSize(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	sizeMB := float64(info.Size()) / (1024 * 1024)
	return fmt.Sprintf("%.2f MB", sizeMB)
}

func printTaskInfo(task *Task) {
	taskType := strings.ToLower(strings.TrimSpace(task.TaskType))
	if taskType == "" {
		taskType = "import"
	}

	if taskType == "export" {
		log.Log.Info("检测到现有任务")
		log.Log.Info("任务:  导出")
		log.Log.Info("文件:  " + displayFileName(task.ExportFile))
		log.Log.Info("服号:  " + task.Server)
		log.Log.Info("维度:  " + formatDimensionCN(task.Dimension))
		log.Log.Info(fmt.Sprintf("范围:  [%d, %d, %d]~[%d, %d, %d]",
			task.ExportMin[0], task.ExportMin[1], task.ExportMin[2],
			task.ExportMax[0], task.ExportMax[1], task.ExportMax[2],
		))
		fmt.Println()
		return
	}

	log.Log.Info("检测到现有任务")
	log.Log.Info("任务:  导入")
	if len(task.BatchImports) > 0 {
		log.Log.Info(fmt.Sprintf("批量:  %d 个建筑", len(task.BatchImports)))
		log.Log.Info("文件:  " + displayFileName(task.BatchImports[0].FileName))
		log.Log.Info("服号:  " + task.Server)
		log.Log.Info("维度:  " + formatDimensionCN(task.Dimension))
		log.Log.Info(fmt.Sprintf("首个坐标:  %d  %d  %d", task.BatchImports[0].X, task.BatchImports[0].Y, task.BatchImports[0].Z))
		if hasImportResumeProgress(task) {
			log.Log.Info(importResumeSummary(task))
		}
		fmt.Println()
		return
	}
	log.Log.Info("文件:  " + displayFileName(task.FileName))
	log.Log.Info("服号:  " + task.Server)
	log.Log.Info("维度:  " + formatDimensionCN(task.Dimension))
	log.Log.Info(fmt.Sprintf("坐标:  %d  %d  %d", task.X, task.Y, task.Z))
	log.Log.Info("掉落物清理:  " + formatEnabledCN(task.ClearDrops))
	log.Log.Info("拒绝方块:  " + formatEnabledCN(task.AutoPlaceDenyBlock))
	log.Log.Info("边界方块:  " + formatEnabledCN(task.AutoPlaceBorder))
	if hasImportResumeProgress(task) {
		log.Log.Info(importResumeSummary(task))
	} else if task.NZ > 0 {
		log.Log.Info(fmt.Sprintf("进度:  %d%%", task.NZ))
	}
	fmt.Println()
}

func stripFileSuffix(name string) string {
	if idx := strings.Index(name, "@"); idx > 0 {
		name = name[:idx]
	}
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return name
}

func parseTaskIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, "task") {
		return 0, false
	}
	suffix := strings.TrimPrefix(name, "task")
	if suffix == "" {
		return 0, false
	}
	idx, err := strconv.Atoi(suffix)
	if err != nil || idx <= 0 {
		return 0, false
	}
	return idx, true
}

func summarizeTask(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "无法读取任务"
	}
	var task Task
	if json.Unmarshal(data, &task) != nil {
		return "任务格式无效"
	}
	taskType := strings.ToLower(strings.TrimSpace(task.TaskType))
	if taskType == "" {
		taskType = "import"
	}
	if taskType == "export" {
		name := strings.TrimSpace(task.ExportFile)
		if name == "" {
			return "导出 -"
		}
		return "导出 " + stripFileSuffix(filepath.Base(name))
	}
	name := strings.TrimSpace(task.FileName)
	if len(task.BatchImports) > 0 {
		name = task.BatchImports[0].FileName
		return fmt.Sprintf("批量导入 %d 个建筑 - %s", len(task.BatchImports), stripFileSuffix(filepath.Base(name)))
	}
	if name == "" {
		return "导入 -"
	}
	return "导入 " + stripFileSuffix(filepath.Base(name))
}

func listTaskFiles() ([]TaskEntry, error) {
	entries, err := os.ReadDir(StorageTaskDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var tasks []TaskEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		index, ok := parseTaskIndex(entry.Name())
		if !ok {
			continue
		}
		path := filepath.Join(StorageTaskDir(), entry.Name())
		tasks = append(tasks, TaskEntry{
			Name:    entry.Name(),
			Path:    path,
			Index:   index,
			Summary: summarizeTask(path),
		})
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].Index == tasks[j].Index {
			return tasks[i].Name < tasks[j].Name
		}
		return tasks[i].Index < tasks[j].Index
	})
	return tasks, nil
}

func nextTaskFileName(existing []TaskEntry) string {
	used := make(map[int]bool, len(existing))
	for _, entry := range existing {
		if entry.Index > 0 {
			used[entry.Index] = true
		}
	}
	for i := 1; ; i++ {
		if !used[i] {
			return fmt.Sprintf("task%d", i)
		}
	}
}

func nextTaskFilePath() (string, string, error) {
	entries, err := listTaskFiles()
	if err != nil {
		return "", "", err
	}
	name := nextTaskFileName(entries)
	return name, filepath.Join(StorageTaskDir(), name), nil
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
