package termlog

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	colorReset      = "\033[0m"
	colorGray       = "\033[90m"
	colorCyan       = "\033[36m"
	colorBrightCyan = "\033[96m"
	colorGreen      = "\033[32m"
	colorRed        = "\033[31m"
	colorWhite      = "\033[97m"
	colorYellow     = "\033[33m"
)

var mu sync.Mutex

func Infof(format string, args ...any) {
	Logf("信息", format, args...)
}

func Noticef(format string, args ...any) {
	Logf("通知", format, args...)
}

func Successf(format string, args ...any) {
	Logf("成功", format, args...)
}

func Warnf(format string, args ...any) {
	Logf("警告", format, args...)
}

func Errorf(format string, args ...any) {
	Logf("错误", format, args...)
}

func ErrorDetail(detail error, message string) {
	Detailf("错误", detail, "%s", message)
}

func ErrorDetailf(detail error, format string, args ...any) {
	Detailf("错误", detail, format, args...)
}

func Logf(level string, format string, args ...any) {
	message := strings.TrimRight(fmt.Sprintf(format, args...), "\r\n")
	timestamp := time.Now().Format("15:04:05")
	levelColor := colorForLevel(level)
	marker := markerForLevel(level)
	levelText := fmt.Sprintf("%-4s %s", level, marker)

	mu.Lock()
	defer mu.Unlock()
	if level == "通知" {
		fmt.Printf(
			"%s%s%s %s %s\n",
			colorGray, timestamp, colorReset,
			GradientText(levelText),
			GradientText(message),
		)
		return
	}

	fmt.Printf(
		"%s%s%s %s%-4s %s%s %s%s%s\n",
		colorGray, timestamp, colorReset,
		levelColor, level, marker, colorReset,
		colorWhite, message, colorReset,
	)
}

func Detailf(level string, detail error, format string, args ...any) {
	Logf(level, format, args...)
	if detail == nil {
		return
	}

	mu.Lock()
	defer mu.Unlock()
	fmt.Printf(
		"%s           └─→ %s%s%s\n",
		colorGray,
		colorGray, detail.Error(), colorReset,
	)
}

func colorForLevel(level string) string {
	switch level {
	case "成功":
		return colorGreen
	case "警告":
		return colorYellow
	case "错误":
		return colorRed
	case "信息":
		return colorBrightCyan
	case "通知":
		return colorCyan
	default:
		return colorWhite
	}
}

func markerForLevel(level string) string {
	switch level {
	case "成功":
		return "✓"
	case "警告":
		return "!"
	case "错误":
		return "×"
	case "通知":
		return "»"
	default:
		return "›"
	}
}

type rgbColor struct {
	r uint8
	g uint8
	b uint8
}

func GradientText(text string) string {
	total := utf8.RuneCountInString(text)
	if total <= 0 {
		return text
	}

	var out strings.Builder
	index := 0
	for _, r := range text {
		if r == ' ' {
			out.WriteRune(r)
			index++
			continue
		}
		progress := float32(0)
		if total > 1 {
			progress = float32(index) / float32(total-1)
		}
		color := launchGradientColor(progress * 0.68)
		fmt.Fprintf(&out, "\033[38;2;%d;%d;%dm%s%s", color.r, color.g, color.b, string(r), colorReset)
		index++
	}
	return out.String()
}

func launchGradientColor(position float32) rgbColor {
	position = clampFloat(position, 0, 1)
	stops := []struct {
		position float32
		color    rgbColor
	}{
		{position: 0, color: rgbColor{r: 255, g: 135, b: 185}},
		{position: 0.5, color: rgbColor{r: 255, g: 216, b: 120}},
		{position: 1, color: rgbColor{r: 110, g: 225, b: 255}},
	}

	for i := 1; i < len(stops); i++ {
		if position > stops[i].position {
			continue
		}
		start := stops[i-1]
		end := stops[i]
		span := end.position - start.position
		if span <= 0 {
			return end.color
		}
		progress := (position - start.position) / span
		return interpolateRGB(start.color, end.color, progress)
	}
	return stops[len(stops)-1].color
}

func interpolateRGB(start rgbColor, end rgbColor, progress float32) rgbColor {
	return rgbColor{
		r: interpolateChannel(start.r, end.r, progress),
		g: interpolateChannel(start.g, end.g, progress),
		b: interpolateChannel(start.b, end.b, progress),
	}
}

func interpolateChannel(start uint8, end uint8, progress float32) uint8 {
	value := float32(start) + (float32(end)-float32(start))*progress
	return uint8(value + 0.5)
}

func clampFloat(value float32, minValue float32, maxValue float32) float32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
