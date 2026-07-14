package common

import (
	"fmt"
	"os"
	"strings"

	consolepkg "nexus/utils/console"
	"nexus/utils/ui"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"
	"golang.org/x/term"
)

func init() {
	enableANSIColors()
}

func ShowProgramInfo() {
	renderLaunchPanel()
	if os.Getenv("NEXUS_PREVIEW_LAUNCH") == "1" {
		os.Exit(0)
	}
}

type Console = consolepkg.Console_input

func NewConsole() *Console {
	return consolepkg.New()
}

func InfoPrompt(text string) string {
	return fmt.Sprintf("%s %s", pterm.White("[")+pterm.Green("INFO")+pterm.White("]"), pterm.White(text))
}

func renderLaunchPanel() {
	pterm.Println()
	for _, line := range buildLaunchPanel() {
		pterm.Println(line)
	}
	pterm.Println()
}

type styledLine struct {
	raw    string
	styled string
}

func buildLaunchPanel() []string {
	const (
		minPanelWidth = 72
		rowChrome     = 7
	)

	leftWidth := calculateLaunchLeftWidth()
	naturalRightWidth := calculateLaunchRightWidth()
	naturalPanelWidth := leftWidth + naturalRightWidth + rowChrome
	panelWidth := naturalPanelWidth
	if columns := terminalColumns(); columns > 0 {
		panelWidth = clamp(naturalPanelWidth, minPanelWidth, max(minPanelWidth, columns-2))
	}
	rightWidth := max(18, panelWidth-leftWidth-rowChrome)
	panelWidth = leftWidth + rightWidth + rowChrome

	leftLines := buildLaunchLeftLines(leftWidth)
	rightContentColumn := leftWidth + 5
	rightLines := buildLaunchRightLines(rightWidth, rightContentColumn, panelWidth)
	rowCount := max(len(leftLines), len(rightLines))
	lines := make([]string, 0, rowCount+2)
	dividerColumn := leftWidth + 3

	lines = append(lines, colorizeGradientColumns(topBorder(panelWidth, dividerColumn), 0, panelWidth))
	for i := 0; i < rowCount; i++ {
		left := blankStyledLine(leftWidth)
		if i < len(leftLines) {
			left = leftLines[i]
		}
		right := blankStyledLine(rightWidth)
		if i < len(rightLines) {
			right = rightLines[i]
		}
		lines = append(lines, renderLaunchPanelRow(left, right, leftWidth, rightWidth, i))
	}
	lines = append(lines, colorizeGradientColumns(bottomBorder(panelWidth, dividerColumn), 0, panelWidth))
	return lines
}

func buildLaunchLeftLines(width int) []styledLine {
	artLines := []string{
		"███╗   ██╗███████╗",
		"████╗  ██║██╔════╝",
		"██╔██╗ ██║█████╗  ",
		"██║╚██╗██║██╔══╝  ",
		"██║ ╚████║███████╗",
		"╚═╝  ╚═══╝╚══════╝",
	}
	coloredArtLines := colorizeTitleLines(artLines)

	lines := make([]styledLine, 0, 9)
	welcome := "Welcome back!"
	lines = append(lines, blankStyledLine(width))
	lines = append(lines, centerStyledLine(welcome, pterm.Bold.Sprint(welcome), width))
	lines = append(lines, blankStyledLine(width))
	for i, raw := range artLines {
		lines = append(lines, centerArtLine(raw, coloredArtLines[i], width))
	}
	lines = append(lines, blankStyledLine(width))

	metaLines := []string{
		"Studio  - 浪兔工作室 / 星白工作室",
		"Author  - LangTu / xingbaiawa / yuansi",
	}
	metaWidth := maxVisibleWidth(metaLines)
	metaPadding := max(0, (width-metaWidth)/2)
	for _, raw := range metaLines {
		lines = append(lines, leftPaddingStyledLine(raw, mutedText(raw), width, metaPadding))
	}
	return lines
}

func buildLaunchRightLines(width, startColumn, panelWidth int) []styledLine {
	const (
		githubTitle   = "GitHub Repository Links"
		repoURL       = "https://github.com/LangTuStudio/Nexusego-Release"
		coreURL       = "https://github.com/LangTuStudio/Nexusego-core"
		flowersURL    = "https://github.com/LangTuStudio/RaaBel"
		thanksTitle   = "Thanks Contributors"
		thanksLine    = "守卫 / 黄桃 / Sword_flute / Conla"
		partnersTitle = "Authorized Panel Providers"
		partnersLine  = "SW面板 / HT面板 / HG面板"
		shopTitle     = "Recommend Shop"
		shopURL       = "https://pioneershop.pw"
	)

	divider := strings.Repeat("─", width)
	return []styledLine{
		styledContentLine(githubTitle, width, func(text string) string { return colorizeMetaLine(text, 0) }),
		styledContentLine(repoURL, width, mutedText),
		styledContentLine(coreURL, width, mutedText),
		styledContentLine(flowersURL, width, mutedText),
		{raw: divider, styled: colorizeGradientColumns(divider, startColumn, panelWidth)},
		styledContentLine(thanksTitle, width, func(text string) string { return colorizeMetaLine(text, 1) }),
		styledContentLine(thanksLine, width, mutedText),
		{raw: divider, styled: colorizeGradientColumns(divider, startColumn, panelWidth)},
		styledContentLine(partnersTitle, width, func(text string) string { return colorizeMetaLine(text, 2) }),
		styledContentLine(partnersLine, width, mutedText),
		{raw: divider, styled: colorizeGradientColumns(divider, startColumn, panelWidth)},
		styledContentLine(shopTitle, width, func(text string) string { return colorizeMetaLine(text, 3) }),
		styledContentLine(shopURL, width, mutedText),
	}
}

func calculateLaunchLeftWidth() int {
	lines := []string{
		"Welcome back!",
		"███╗   ██╗███████╗",
		"████╗  ██║██╔════╝",
		"██╔██╗ ██║█████╗  ",
		"██║╚██╗██║██╔══╝  ",
		"██║ ╚████║███████╗",
		"╚═╝  ╚═══╝╚══════╝",
		"Studio  - 浪兔工作室 / 星白工作室",
		"Author  - LangTu / xingbaiawa / yuansi",
	}
	return min(maxVisibleWidth(lines)+4, 50)
}

func calculateLaunchRightWidth() int {
	lines := []string{
		"GitHub Repository Links",
		"https://github.com/LangTuStudio/Nexusego-Release",
		"https://github.com/LangTuStudio/Nexusego-core",
		"https://github.com/LangTuStudio/RaaBel",
		"Thanks Contributors",
		"守卫 / 黄桃 / Sword_flute / Conla",
		"Authorized Panel Providers",
		"SW面板 / HT面板 / HG面板",
		"Recommend Shop",
		"https://pioneershop.pw",
	}
	return maxVisibleWidth(lines)
}

func renderLaunchPanelRow(left, right styledLine, leftWidth, rightWidth, _ int) string {
	panelWidth := leftWidth + rightWidth + 7
	leftBorder := gradientColorForColumn(0, panelWidth)
	dividerBorder := gradientColorForColumn(leftWidth+3, panelWidth)
	rightBorder := gradientColorForColumn(panelWidth-1, panelWidth)
	var b strings.Builder
	b.WriteString(leftBorder.Sprint("│"))
	b.WriteByte(' ')
	b.WriteString(padRightStyledLine(left, leftWidth).styled)
	b.WriteByte(' ')
	b.WriteString(dividerBorder.Sprint("│"))
	b.WriteByte(' ')
	b.WriteString(padRightStyledLine(right, rightWidth).styled)
	b.WriteByte(' ')
	b.WriteString(rightBorder.Sprint("│"))
	return b.String()
}

func topBorder(width, dividerColumn int) string {
	title := fmt.Sprintf(" NexusEgo v%s ", ui.Version)
	line := make([]rune, width)
	for i := range line {
		line[i] = '─'
	}
	line[0] = '╭'
	line[len(line)-1] = '╮'
	if dividerColumn > 0 && dividerColumn < len(line)-1 {
		line[dividerColumn] = '┬'
	}
	for i, r := range []rune(title) {
		column := 4 + i
		if column >= len(line)-1 {
			break
		}
		line[column] = r
	}
	return string(line)
}

func bottomBorder(width, dividerColumn int) string {
	line := make([]rune, width)
	for i := range line {
		line[i] = '─'
	}
	line[0] = '╰'
	line[len(line)-1] = '╯'
	if dividerColumn > 0 && dividerColumn < len(line)-1 {
		line[dividerColumn] = '┴'
	}
	return string(line)
}

func terminalColumns() int {
	columns, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || columns <= 0 {
		return 100
	}
	return columns
}

func blankStyledLine(width int) styledLine {
	raw := strings.Repeat(" ", width)
	return styledLine{raw: raw, styled: raw}
}

func styledContentLine(raw string, width int, style func(string) string) styledLine {
	fitted := truncateToWidth(raw, width)
	return styledLine{raw: fitted, styled: style(fitted)}
}

func centerStyledLine(raw, styled string, width int) styledLine {
	lineWidth := visibleWidth(raw)
	if lineWidth >= width {
		return styledLine{raw: raw, styled: styled}
	}
	leftPadding := (width - lineWidth) / 2
	rightPadding := width - lineWidth - leftPadding
	rawPaddingLeft := strings.Repeat(" ", leftPadding)
	rawPaddingRight := strings.Repeat(" ", rightPadding)
	return styledLine{
		raw:    rawPaddingLeft + raw + rawPaddingRight,
		styled: rawPaddingLeft + styled + rawPaddingRight,
	}
}

func centerArtLine(raw, styled string, width int) styledLine {
	lineWidth := len([]rune(raw))
	if lineWidth >= width {
		return styledLine{raw: raw, styled: styled}
	}
	leftPadding := (width - lineWidth) / 2
	rightPadding := width - lineWidth - leftPadding
	rawPaddingLeft := strings.Repeat(" ", leftPadding)
	rawPaddingRight := strings.Repeat(" ", rightPadding)
	return styledLine{
		raw:    rawPaddingLeft + raw + rawPaddingRight,
		styled: rawPaddingLeft + styled + rawPaddingRight,
	}
}

func leftPaddingStyledLine(raw, styled string, width, leftPadding int) styledLine {
	lineWidth := visibleWidth(raw)
	if lineWidth >= width {
		return styledLine{raw: raw, styled: styled}
	}
	leftPadding = clamp(leftPadding, 0, width-lineWidth)
	rightPadding := width - lineWidth - leftPadding
	rawPaddingLeft := strings.Repeat(" ", leftPadding)
	rawPaddingRight := strings.Repeat(" ", rightPadding)
	return styledLine{
		raw:    rawPaddingLeft + raw + rawPaddingRight,
		styled: rawPaddingLeft + styled + rawPaddingRight,
	}
}

func padRightStyledLine(line styledLine, width int) styledLine {
	lineWidth := visibleWidth(line.raw)
	if lineWidth >= width {
		return line
	}
	padding := strings.Repeat(" ", width-lineWidth)
	return styledLine{
		raw:    line.raw + padding,
		styled: line.styled + padding,
	}
}

func mutedText(text string) string {
	return pterm.NewRGB(190, 196, 204).Sprint(text)
}

func visibleWidth(text string) int {
	return runewidth.StringWidth(text)
}

func maxVisibleWidth(lines []string) int {
	maxWidth := 0
	for _, line := range lines {
		maxWidth = max(maxWidth, visibleWidth(line))
	}
	return maxWidth
}

func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if visibleWidth(text) <= width {
		return text
	}

	tail := "..."
	tailWidth := visibleWidth(tail)
	if width <= tailWidth {
		return strings.Repeat(".", width)
	}

	var b strings.Builder
	currentWidth := 0
	limit := width - tailWidth
	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > limit {
			break
		}
		b.WriteRune(r)
		currentWidth += runeWidth
	}
	b.WriteString(tail)
	return b.String()
}

func clamp(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func colorizeTitleLines(lines []string) []string {
	if len(lines) == 0 {
		return nil
	}

	runeLines := make([][]rune, len(lines))
	maxWidth := 0
	for i, line := range lines {
		runes := []rune(line)
		runeLines[i] = runes
		if len(runes) > maxWidth {
			maxWidth = len(runes)
		}
	}
	for i := range runeLines {
		if len(runeLines[i]) < maxWidth {
			padding := make([]rune, maxWidth-len(runeLines[i]))
			for j := range padding {
				padding[j] = ' '
			}
			runeLines[i] = append(runeLines[i], padding...)
		}
	}

	visibleColumns := make([]int, 0, maxWidth)
	for col := 0; col < maxWidth; col++ {
		for _, runes := range runeLines {
			if runes[col] != ' ' {
				visibleColumns = append(visibleColumns, col)
				break
			}
		}
	}
	if len(visibleColumns) == 0 {
		return lines
	}

	columnColor := make(map[int]pterm.RGB, len(visibleColumns))
	for i, col := range visibleColumns {
		if len(visibleColumns) <= 1 {
			columnColor[col] = launchGradientColor(0)
			continue
		}
		columnColor[col] = launchGradientColor(float32(i) / float32(len(visibleColumns)-1))
	}

	result := make([]string, 0, len(runeLines))
	for _, runes := range runeLines {
		var b strings.Builder
		for col, r := range runes {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			color, ok := columnColor[col]
			if !ok {
				b.WriteRune(r)
				continue
			}
			b.WriteString(color.Sprint(string(r)))
		}
		result = append(result, b.String())
	}
	return result
}

func colorizeMetaLine(line string, offset int) string {
	runes := []rune(line)

	visibleCount := 0
	for _, r := range runes {
		if r != ' ' {
			visibleCount++
		}
	}
	if visibleCount <= 1 {
		return launchGradientColor(0).Sprint(line)
	}

	var b strings.Builder
	current := 0
	for _, r := range runes {
		if r == ' ' {
			b.WriteRune(r)
			continue
		}
		progress := (float32(current) + float32(offset)*0.45) / float32(visibleCount-1)
		color := launchGradientColor(progress)
		b.WriteString(color.Sprint(string(r)))
		current++
	}
	return b.String()
}

func colorizeGradientColumns(line string, startColumn, totalColumns int) string {
	runes := []rune(line)

	var b strings.Builder
	column := startColumn
	for _, r := range runes {
		runeWidth := visibleWidth(string(r))
		if r == ' ' {
			b.WriteRune(r)
			column += runeWidth
			continue
		}
		color := gradientColorForColumn(column, totalColumns)
		b.WriteString(color.Sprint(string(r)))
		column += runeWidth
	}
	return b.String()
}

func gradientColorForColumn(column, totalColumns int) pterm.RGB {
	if totalColumns <= 1 {
		return launchGradientColor(0)
	}
	const chromeGradientEnd = 0.68
	progress := float32(column) / float32(totalColumns-1)
	return launchGradientColor(progress * chromeGradientEnd)
}

func launchGradientColor(position float32) pterm.RGB {
	position = clampFloat(position, 0, 1)
	stops := []struct {
		position float32
		color    pterm.RGB
	}{
		{position: 0, color: pterm.NewRGB(255, 135, 185)},
		{position: 0.5, color: pterm.NewRGB(255, 216, 120)},
		{position: 1, color: pterm.NewRGB(110, 225, 255)},
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

func interpolateRGB(start, end pterm.RGB, progress float32) pterm.RGB {
	return pterm.NewRGB(
		interpolateChannel(start.R, end.R, progress),
		interpolateChannel(start.G, end.G, progress),
		interpolateChannel(start.B, end.B, progress),
	)
}

func interpolateChannel(start, end uint8, progress float32) uint8 {
	value := float32(start) + (float32(end)-float32(start))*progress
	return uint8(value + 0.5)
}

func clampFloat(value, minValue, maxValue float32) float32 {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
