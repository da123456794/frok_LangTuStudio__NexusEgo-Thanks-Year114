package convert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOneDragonDirectoryMatchesInstructionKeywords(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "飞蛾最终地铁bdx")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	instruction := "飞蛾最终地铁一条龙\n" +
		"指令99980 -64 99980\n" +
		"主城-1000 0 -1000\n" +
		"仓储-1036 -64 1064\n" +
		"管理员标签 §rop\n"
	if err := os.WriteFile(filepath.Join(root, "使用说明"), []byte(instruction), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"飞蛾最终地铁指令.bdx",
		"飞蛾最终地铁主城.bdx",
		"飞蛾最终地铁仓储.bdx",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("BD"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}
	assertOneDragonEntry(t, entries[0], "飞蛾最终地铁指令.bdx", 99980, -64, 99980)
	assertOneDragonEntry(t, entries[1], "飞蛾最终地铁主城.bdx", -1000, 0, -1000)
	assertOneDragonEntry(t, entries[2], "飞蛾最终地铁仓储.bdx", -1036, -64, 1064)
}

func TestResolveOneDragonDirectoryMatchesJSONCoordinates(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "coords.json"), []byte(`[
		{"name":"主城坐标","x":1,"y":2,"z":3},
		{"file":"黑市区","pos":[0,0,0],"x":-4,"y":5,"z":-6}
	]`), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"项目主城.bdx", "项目黑市.bdx"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("BD"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	assertOneDragonEntry(t, entries[0], "项目主城.bdx", 1, 2, 3)
	assertOneDragonEntry(t, entries[1], "项目黑市.bdx", -4, 5, -6)
}

func TestResolveOneDragonDirectoryFallsBackToOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "坐标.txt"), []byte("第一段 10 20 30\n第二段 40 50 60\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.bdx", "b.bdx"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("BD"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	assertOneDragonEntry(t, entries[0], "a.bdx", 10, 20, 30)
	assertOneDragonEntry(t, entries[1], "b.bdx", 40, 50, 60)
}

func TestResolveOneDragonDirectoryRecursesNestedFolder(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "包名", "建筑")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "说明.txt"), []byte("主城 1 2 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "项目主城.bdx"), []byte("BD"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	assertOneDragonEntry(t, entries[0], "项目主城.bdx", 1, 2, 3)
}

func TestResolveOneDragonDirectoryParsesNameLineThenCoordLine(t *testing.T) {
	dir := t.TempDir()
	text := "星白地铁二代普通版导入坐标\n" +
		"4月29日 凌晨00：57 190字\n\n" +
		"主城\n" +
		"-9 57 -157\n" +
		"指令区\n" +
		"-500 -60 500\n" +
		"天赋加点\n" +
		"6000 100 6000\n" +
		"抽奖\n" +
		"-100000 100 -100000\n" +
		"一图[隐匿处]\n" +
		"-2000 -60 -2000\n" +
		"二图[水上乐园]\n" +
		"-3000 -60 -3000\n" +
		"三图\n" +
		"-4000 -60 -4000\n"
	if err := os.WriteFile(filepath.Join(dir, "星白地铁适配(2).txt"), []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"奥林匹克体育场.bdx",
		"星白地铁主城.bdx",
		"星白地铁天赋.bdx",
		"星白地铁抽奖.bdx",
		"星白地铁指令.bdx",
		"水上乐园.bdx",
		"隐匿处.bdx",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("BD"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 7 {
		t.Fatalf("entry count = %d, want 7", len(entries))
	}
	assertOneDragonEntry(t, entries[0], "星白地铁主城.bdx", -9, 57, -157)
	assertOneDragonEntry(t, entries[1], "星白地铁指令.bdx", -500, -60, 500)
	assertOneDragonEntry(t, entries[2], "星白地铁天赋.bdx", 6000, 100, 6000)
	assertOneDragonEntry(t, entries[3], "星白地铁抽奖.bdx", -100000, 100, -100000)
	assertOneDragonEntry(t, entries[4], "隐匿处.bdx", -2000, -60, -2000)
	assertOneDragonEntry(t, entries[5], "水上乐园.bdx", -3000, -60, -3000)
	assertOneDragonEntry(t, entries[6], "奥林匹克体育场.bdx", -4000, -60, -4000)
}

func TestResolveOneDragonDirectorySeparatesNestedOneDragonGroups(t *testing.T) {
	dir := t.TempDir()
	groupA := filepath.Join(dir, "合集", "甲一条龙")
	groupB := filepath.Join(dir, "合集", "乙一条龙")
	for _, group := range []string{groupA, groupB} {
		if err := os.MkdirAll(group, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(groupA, "坐标.txt"), []byte("主城 1 2 3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groupA, "甲主城.bdx"), []byte("BD"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groupB, "坐标.txt"), []byte("主城 10 20 30\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(groupB, "乙主城.bdx"), []byte("BD"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count = %d, want 2", len(entries))
	}
	assertOneDragonEntryByName(t, entries, "甲主城.bdx", 1, 2, 3)
	assertOneDragonEntryByName(t, entries, "乙主城.bdx", 10, 20, 30)
}

func TestResolveOneDragonDirectoryPrefersCoordinateFileForMCWorld(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "坐标.txt"), []byte("主城 10 20 30\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "主城@[1,2,3]~[4,5,6].mcworld"), []byte("PK"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	assertOneDragonEntry(t, entries[0], "主城@[1,2,3]~[4,5,6].mcworld", 10, 20, 30)
}

func TestResolveOneDragonDirectoryDoesNotFallbackToMCWorldFilenameCoordinate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "主城@[1,2,3]~[4,5,6].mcworld"), []byte("PK"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ResolveOneDragonDirectory(dir)
	if err == nil {
		t.Fatal("ResolveOneDragonDirectory() error = nil, want error")
	}
}

func TestResolveOneDragonDirectoryFallsBackToBDXFilenameCoordinate(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "主城-1_2_3.bdx"), []byte("BD"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ResolveOneDragonDirectory(dir)
	if err != nil {
		t.Fatalf("ResolveOneDragonDirectory() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	assertOneDragonEntry(t, entries[0], "主城-1_2_3.bdx", -1, 2, 3)
}

func assertOneDragonEntryByName(t *testing.T, entries []OneDragonEntry, fileName string, x, y, z int) {
	t.Helper()
	for _, entry := range entries {
		if filepath.Base(entry.SourcePath) == fileName {
			assertOneDragonEntry(t, entry, fileName, x, y, z)
			return
		}
	}
	t.Fatalf("entry %q not found", fileName)
}

func assertOneDragonEntry(t *testing.T, entry OneDragonEntry, fileName string, x, y, z int) {
	t.Helper()
	if filepath.Base(entry.SourcePath) != fileName {
		t.Fatalf("file = %q, want %q", filepath.Base(entry.SourcePath), fileName)
	}
	if entry.X != x || entry.Y != y || entry.Z != z {
		t.Fatalf("coord = %d %d %d, want %d %d %d", entry.X, entry.Y, entry.Z, x, y, z)
	}
}
