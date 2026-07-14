package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	ansi "github.com/leaanthony/go-ansi-parser"
	"github.com/natefinch/lumberjack"
	"github.com/pterm/pterm"
)

type Logs struct {
	logs *lumberjack.Logger
}

var DefaultMultiPrinter = pterm.DefaultMultiPrinter
var logFilePath = filepath.Join("NexusEgo_Storage", "log", "logs.log")

// fileLogger 用于写入日志文件
var fileLogger = &lumberjack.Logger{
	Filename:   logFilePath,
	MaxSize:    10240,
	MaxBackups: 3,
	MaxAge:     180,
	Compress:   true,
}

// NexusLogger PhoenixBuilder 风格的日志器
type NexusLogger struct{}

// ArgsFromMap 兼容旧 API，将 map 转为 []pterm.LoggerArgument
func (l NexusLogger) ArgsFromMap(m map[string]any) []pterm.LoggerArgument {
	args := make([]pterm.LoggerArgument, 0, len(m))
	for k, v := range m {
		args = append(args, pterm.LoggerArgument{Key: k, Value: v})
	}
	return args
}

// formatArgs 将参数格式化为字符串
func formatArgs(args [][]pterm.LoggerArgument) string {
	if len(args) == 0 {
		return ""
	}
	var parts []string
	for _, argList := range args {
		if argList == nil {
			continue
		}
		for _, arg := range argList {
			parts = append(parts, fmt.Sprintf("%v", arg.Value))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// writeToFile 写入日志文件（纯文本，无 ANSI）
func writeToFile(level, msg string) {
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("%s [%s] %s\n", ts, level, msg)
	fileLogger.Write([]byte(line))
}

// 日志前缀样式
var (
	wb          = pterm.White("[")
	we          = pterm.White("]")
	infoPrefix  = wb + pterm.Green("INFO") + we
	errorPrefix = wb + pterm.Red("EROR") + we
	warnPrefix  = wb + pterm.Yellow("WARN") + we
	okPrefix    = wb + pterm.Green("DONE") + we
	debugPrefix = wb + pterm.Gray("DEBUG") + we
)

// Info 信息日志
func (l NexusLogger) Info(msg string, args ...[]pterm.LoggerArgument) {
	full := msg + formatArgs(args)
	fmt.Printf("%s %s\n", infoPrefix, full)
	writeToFile("INFO", cleanAnsi(full))
}

// Error 错误日志
func (l NexusLogger) Error(msg string, args ...[]pterm.LoggerArgument) {
	full := msg + formatArgs(args)
	fmt.Printf("%s %s\n", errorPrefix, full)
	writeToFile("EROR", cleanAnsi(full))
}

// Warn 警告日志
func (l NexusLogger) Warn(msg string, args ...[]pterm.LoggerArgument) {
	full := msg + formatArgs(args)
	fmt.Printf("%s %s\n", warnPrefix, full)
	writeToFile("WARN", cleanAnsi(full))
}

// Success 成功日志
func (l NexusLogger) Success(msg string, args ...[]pterm.LoggerArgument) {
	full := msg + formatArgs(args)
	fmt.Printf("%s %s\n", okPrefix, full)
	writeToFile("DONE", cleanAnsi(full))
}

// Trace 跟踪日志
func (l NexusLogger) Trace(msg string, args ...[]pterm.LoggerArgument) {
	full := msg + formatArgs(args)
	fmt.Printf("%s %s\n", debugPrefix, full)
	writeToFile("DEBUG", cleanAnsi(full))
}

// cleanAnsi 清除 ANSI 转义码
func cleanAnsi(s string) string {
	cleaned, err := ansi.Cleanse(s)
	if err != nil {
		return s
	}
	return cleaned
}

// Log 全局日志实例
var Log = NexusLogger{}

func (l *Logs) Write(p []byte) (n int, err error) {
	os.Stdout.Write(p)
	strs := string(p)
	write_log, _ := ansi.Cleanse(strs)
	bytes := []byte(write_log)
	l.logs.Write(bytes)
	return len(p), nil
}

// Close implements io.Closer, and closes the current logfile.
func (l *Logs) Close() error {
	err1 := l.logs.Close()
	err2 := os.Stdout.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func init() {
	_ = os.MkdirAll(filepath.Dir(logFilePath), 0755)
	DefaultMultiPrinter.Writer = &Logs{logs: &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    10240,
		MaxBackups: 3,
		MaxAge:     180,
		Compress:   true,
	}}
}
