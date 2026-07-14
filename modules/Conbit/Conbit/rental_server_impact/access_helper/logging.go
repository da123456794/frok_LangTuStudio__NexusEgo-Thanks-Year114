package access_helper

import (
	"fmt"
	"strings"

	"github.com/LangTuStudio/Conbit/internal/termlog"
)

func logLine(level, msg string) {
	termlog.Logf(level, "%s", strings.TrimRight(msg, "\r\n"))
}

func logf(level, format string, args ...any) {
	logLine(level, fmt.Sprintf(format, args...))
}

func infoLine(msg string) {
	logLine("信息", msg)
}

func infof(format string, args ...any) {
	logf("信息", format, args...)
}

func noticeLine(msg string) {
	logLine("通知", msg)
}

func warnLine(msg string) {
	logLine("警告", msg)
}

func warnf(format string, args ...any) {
	logf("警告", format, args...)
}

func doneLine(msg string) {
	logLine("成功", msg)
}

func errorLine(msg string) {
	logLine("错误", msg)
}

func errorDetailLine(detail error, msg string) {
	termlog.ErrorDetailf(detail, "%s", strings.TrimRight(msg, "\r\n"))
}
