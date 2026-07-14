package common

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"nexus/utils/ui"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestBuildLaunchPanelContent(t *testing.T) {
	lines := buildLaunchPanel()
	if len(lines) < 3 {
		t.Fatalf("expected launch panel lines, got %d", len(lines))
	}

	plainLines := make([]string, 0, len(lines))
	for _, line := range lines {
		plainLines = append(plainLines, ansiEscapePattern.ReplaceAllString(line, ""))
	}
	if os.Getenv("NEXUS_PREVIEW_LAUNCH") == "1" {
		for i, line := range plainLines {
			t.Logf("%02d width=%03d dividers=%v %s", i, visibleWidth(line), visibleRuneColumns(line, '│'), line)
		}
	}
	plainPanel := strings.Join(plainLines, "\n")

	expectedTexts := []string{
		"NexusEgo v" + ui.Version,
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
		"Studio  - 浪兔工作室 / 星白工作室",
		"Author  - LangTu / xingbaiawa / yuansi",
	}
	for _, text := range expectedTexts {
		if !strings.Contains(plainPanel, text) {
			t.Fatalf("launch panel missing %q:\n%s", text, plainPanel)
		}
	}

	legacyTexts := []string{
		"Claude Code",
		"Tips for getting started",
		"What's new",
		"梅州浪兔工作室",
		"NearKai",
		"小六神",
		"可白",
		"南Nan",
		"秋凉",
	}
	for _, text := range legacyTexts {
		if strings.Contains(plainPanel, text) {
			t.Fatalf("launch panel should not contain legacy text %q:\n%s", text, plainPanel)
		}
	}

	expectedWidth := visibleWidth(plainLines[0])
	for _, line := range plainLines[1:] {
		if width := visibleWidth(line); width != expectedWidth {
			t.Fatalf("launch panel width mismatch: got %d, want %d in %q", width, expectedWidth, line)
		}
	}

	var dividerColumns []int
	for _, line := range plainLines[1 : len(plainLines)-1] {
		columns := visibleRuneColumns(line, '│')
		if len(columns) != 3 {
			t.Fatalf("launch panel row should have 3 vertical dividers, got %v in %q", columns, line)
		}
		if dividerColumns == nil {
			dividerColumns = columns
			continue
		}
		if !slices.Equal(columns, dividerColumns) {
			t.Fatalf("launch panel divider mismatch: got %v, want %v in %q", columns, dividerColumns, line)
		}
	}
}

func visibleRuneColumns(line string, target rune) []int {
	columns := make([]int, 0, 3)
	column := 0
	for _, r := range line {
		if r == target {
			columns = append(columns, column)
		}
		column += visibleWidth(string(r))
	}
	return columns
}
