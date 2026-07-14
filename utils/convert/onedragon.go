package convert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Yeah114/WaterStructure/structure"
	"github.com/bodgit/sevenzip"
	"github.com/mholt/archiver/v3"
	"golang.org/x/text/encoding/simplifiedchinese"
	textunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var ErrNotOneDragonArchive = errors.New("not a one-dragon archive")

const maxOneDragonCoordinateFileSize = 4 << 20

type OneDragonEntry struct {
	SourcePath  string
	DisplayName string
	X           int
	Y           int
	Z           int
}

type OneDragonPackage struct {
	TempDir string
	Entries []OneDragonEntry
	Skipped []string
}

func (p *OneDragonPackage) Cleanup() {
	if p == nil || strings.TrimSpace(p.TempDir) == "" {
		return
	}
	_ = os.RemoveAll(p.TempDir)
}

func IsOneDragonArchivePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".mcworld" || ext == ".nexus" {
		return false
	}
	if isSevenZipPath(path) {
		return true
	}
	if _, ok := archiverByExtension(path); ok {
		return true
	}
	return false
}

func PrepareOneDragonArchive(archivePath string) (*OneDragonPackage, error) {
	if !IsOneDragonArchivePath(archivePath) {
		return nil, ErrNotOneDragonArchive
	}

	tempDir, err := os.MkdirTemp("", "nexusego_onedragon_*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir failed: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tempDir)
		}
	}()

	if err := unarchiveOneDragonPackage(archivePath, tempDir); err != nil {
		return nil, fmt.Errorf("extract one-dragon archive failed: %w", err)
	}
	result, err := InspectOneDragonDirectory(tempDir)
	if err != nil {
		return nil, err
	}
	cleanup = false
	return &OneDragonPackage{TempDir: tempDir, Entries: result.Entries, Skipped: result.Skipped}, nil
}

func ResolveOneDragonDirectory(root string) ([]OneDragonEntry, error) {
	result, err := InspectOneDragonDirectory(root)
	if err != nil {
		return nil, err
	}
	return result.Entries, nil
}

type OneDragonInspectResult struct {
	Entries []OneDragonEntry
	Skipped []string
}

func InspectOneDragonDirectory(root string) (OneDragonInspectResult, error) {
	files, err := walkOneDragonFiles(root)
	if err != nil {
		return OneDragonInspectResult{}, err
	}
	if len(files) == 0 {
		return OneDragonInspectResult{}, ErrNotOneDragonArchive
	}

	groupRoots := findOneDragonGroupRoots(root, files)
	if len(groupRoots) > 0 {
		var result OneDragonInspectResult
		for _, groupRoot := range groupRoots {
			groupFiles := filesForOneDragonGroup(files, groupRoot, groupRoots)
			groupResult, err := inspectOneDragonFileSet(groupFiles)
			if err != nil || len(groupResult.Entries) == 0 {
				continue
			}
			result.Entries = append(result.Entries, groupResult.Entries...)
			for _, skipped := range groupResult.Skipped {
				result.Skipped = append(result.Skipped, formatSkippedOneDragonFile(root, groupRoot, skipped))
			}
		}
		if len(result.Entries) > 0 {
			return result, nil
		}
	}

	return inspectOneDragonFileSet(files)
}

func inspectOneDragonFileSet(files []oneDragonFile) (OneDragonInspectResult, error) {
	if len(files) == 0 {
		return OneDragonInspectResult{}, ErrNotOneDragonArchive
	}

	coordinates, coordinateFiles := collectOneDragonCoordinates(files)
	structures := dedupeOneDragonStructures(collectOneDragonStructures(files, coordinateFiles))
	if len(structures) == 0 {
		return OneDragonInspectResult{}, ErrNotOneDragonArchive
	}

	if len(coordinates) == 0 {
		entries, skipped := entriesFromStructureFilenameCoordinates(nil, structures)
		if len(entries) == 0 {
			return OneDragonInspectResult{}, ErrNotOneDragonArchive
		}
		return OneDragonInspectResult{Entries: entries, Skipped: skipped}, nil
	}

	entries, skipped, err := matchOneDragonEntries(structures, coordinates)
	if err != nil {
		return OneDragonInspectResult{}, err
	}
	if len(entries) == 0 {
		return OneDragonInspectResult{}, ErrNotOneDragonArchive
	}
	return OneDragonInspectResult{Entries: entries, Skipped: skipped}, nil
}

func findOneDragonGroupRoots(root string, files []oneDragonFile) []string {
	root = filepath.Clean(root)
	roots := map[string]bool{}
	for _, f := range files {
		if !isPossibleCoordinateFile(f) || len(parseOneDragonCoordinateFile(f.Path, 0)) == 0 {
			continue
		}
		if groupRoot := nearestOneDragonStructureAncestor(root, filepath.Dir(f.Path), files); groupRoot != "" {
			roots[groupRoot] = true
		}
	}

	for _, f := range files {
		if isParsedCoordinateFile(f) || !isOneDragonStructureFile(f.Path) || isUnderAnyOneDragonRoot(f.Path, roots) || directoryHasCoordinateFile(filepath.Dir(f.Path), files) {
			continue
		}
		if _, _, _, _, ok := extractStructureFilenameCoordinate(f.Name); ok {
			roots[filepath.Dir(f.Path)] = true
		}
	}

	groupRoots := make([]string, 0, len(roots))
	for groupRoot := range roots {
		groupRoots = append(groupRoots, groupRoot)
	}
	sort.SliceStable(groupRoots, func(i, j int) bool {
		return strings.ToLower(groupRoots[i]) < strings.ToLower(groupRoots[j])
	})
	return groupRoots
}

func nearestOneDragonStructureAncestor(root, start string, files []oneDragonFile) string {
	root = filepath.Clean(root)
	current := filepath.Clean(start)
	for {
		if directoryHasOneDragonStructure(current, files) {
			return current
		}
		if samePath(current, root) {
			return ""
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func directoryHasOneDragonStructure(dir string, files []oneDragonFile) bool {
	for _, f := range files {
		if isPathWithin(dir, f.Path) && !isParsedCoordinateFile(f) && isOneDragonStructureFile(f.Path) {
			return true
		}
	}
	return false
}

func directoryHasCoordinateFile(dir string, files []oneDragonFile) bool {
	for _, f := range files {
		if isPathWithin(dir, f.Path) && isParsedCoordinateFile(f) {
			return true
		}
	}
	return false
}

func isParsedCoordinateFile(f oneDragonFile) bool {
	return isPossibleCoordinateFile(f) && len(parseOneDragonCoordinateFile(f.Path, 0)) > 0
}

func isUnderAnyOneDragonRoot(path string, roots map[string]bool) bool {
	for root := range roots {
		if isPathWithin(root, path) {
			return true
		}
	}
	return false
}

func filesForOneDragonGroup(files []oneDragonFile, groupRoot string, allGroupRoots []string) []oneDragonFile {
	var result []oneDragonFile
	for _, f := range files {
		if !isPathWithin(groupRoot, f.Path) {
			continue
		}
		inChildGroup := false
		for _, otherRoot := range allGroupRoots {
			if samePath(otherRoot, groupRoot) {
				continue
			}
			if isPathWithin(otherRoot, f.Path) && isPathWithin(groupRoot, otherRoot) {
				inChildGroup = true
				break
			}
		}
		if !inChildGroup {
			result = append(result, f)
		}
	}
	return result
}

func formatSkippedOneDragonFile(root, groupRoot, skipped string) string {
	target := skipped
	if !filepath.IsAbs(target) {
		target = filepath.Join(groupRoot, skipped)
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return skipped
	}
	return rel
}

func unarchiveOneDragonPackage(archivePath, destination string) error {
	if isSevenZipPath(archivePath) {
		return unarchiveSevenZip(archivePath, destination)
	}
	return archiver.Unarchive(archivePath, destination)
}

func isSevenZipPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".7z")
}

func unarchiveSevenZip(archivePath, destination string) error {
	reader, err := sevenzip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	destAbs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		name := filepath.Clean(filepath.FromSlash(entry.Name))
		if name == "." || filepath.IsAbs(name) || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || name == ".." {
			return fmt.Errorf("unsafe 7z path: %s", entry.Name)
		}
		target := filepath.Join(destAbs, name)
		if !isPathWithin(destAbs, target) {
			return fmt.Errorf("unsafe 7z path: %s", entry.Name)
		}
		mode := entry.Mode()
		if mode.IsDir() || strings.HasSuffix(entry.Name, "/") || strings.HasSuffix(entry.Name, "\\") {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := entry.Open()
		if err != nil {
			return err
		}
		err = writeOneDragonExtractedFile(target, rc, mode)
		closeErr := rc.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func writeOneDragonExtractedFile(target string, src io.Reader, mode fs.FileMode) error {
	perm := mode.Perm()
	if perm == 0 {
		perm = 0644
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, src)
	return err
}

func isPathWithin(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func CopyOneDragonMCWorld(sourcePath, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}
	name := filepath.Base(sourcePath)
	target := filepath.Join(outputDir, name)
	if samePath(sourcePath, target) {
		return target, nil
	}
	target = uniquePath(target)

	in, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	return target, nil
}

type oneDragonFile struct {
	Path string
	Name string
	Size int64
}

type oneDragonCoordinate struct {
	Name       string
	X          int
	Y          int
	Z          int
	SourcePath string
	Order      int
}

type oneDragonStructure struct {
	Path string
	Name string
}

func walkOneDragonFiles(root string) ([]oneDragonFile, error) {
	var files []oneDragonFile
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, oneDragonFile{
			Path: path,
			Name: entry.Name(),
			Size: info.Size(),
		})
		return nil
	})
	return files, err
}

func collectOneDragonCoordinates(files []oneDragonFile) ([]oneDragonCoordinate, map[string]bool) {
	var coordinates []oneDragonCoordinate
	coordinateFiles := map[string]bool{}
	order := 0
	for _, f := range files {
		if !isPossibleCoordinateFile(f) {
			continue
		}
		parsed := parseOneDragonCoordinateFile(f.Path, order)
		if len(parsed) == 0 {
			continue
		}
		order += len(parsed)
		coordinateFiles[f.Path] = true
		coordinates = append(coordinates, parsed...)
	}
	return coordinates, coordinateFiles
}

func collectOneDragonStructures(files []oneDragonFile, coordinateFiles map[string]bool) []oneDragonStructure {
	var structures []oneDragonStructure
	for _, f := range files {
		if coordinateFiles[f.Path] {
			continue
		}
		if !isOneDragonStructureFile(f.Path) {
			continue
		}
		structures = append(structures, oneDragonStructure{
			Path: f.Path,
			Name: filepath.Base(f.Path),
		})
	}
	sort.SliceStable(structures, func(i, j int) bool {
		return strings.ToLower(structures[i].Name) < strings.ToLower(structures[j].Name)
	})
	return structures
}

func dedupeOneDragonStructures(structures []oneDragonStructure) []oneDragonStructure {
	seen := map[string]bool{}
	result := make([]oneDragonStructure, 0, len(structures))
	for _, structure := range structures {
		key, err := filepath.Abs(structure.Path)
		if err != nil {
			key = filepath.Clean(structure.Path)
		}
		key = strings.ToLower(key)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, structure)
	}
	return result
}

func isPossibleCoordinateFile(f oneDragonFile) bool {
	if f.Size <= 0 || f.Size > maxOneDragonCoordinateFileSize {
		return false
	}
	ext := strings.ToLower(filepath.Ext(f.Name))
	switch ext {
	case ".json", ".txt", ".log", ".md", ".csv", "":
		return true
	default:
		return false
	}
}

func isOneDragonStructureFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".bdx", ".mcworld", ".zip", ".schem", ".schematic", ".litematic", ".mcstructure",
		".kbdx", ".tibi", ".nexus", ".ibi", ".construction", ".bp", ".building",
		".buildingx", ".fhbuild", ".reb", ".bds", ".sibi", ".bcf", ".covstructure",
		".np", ".mcfunction":
		return true
	case ".json":
		return canOpenAsWaterStructure(path)
	default:
		return false
	}
}

func canOpenAsWaterStructure(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	reader, err := structure.StructureFromFile(file)
	if err != nil || reader == nil {
		return false
	}
	_ = reader.Close()
	return true
}

func parseOneDragonCoordinateFile(path string, startOrder int) []oneDragonCoordinate {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := strings.TrimSpace(strings.TrimPrefix(decodeOneDragonText(data), "\ufeff"))
	if text == "" {
		return nil
	}

	var coordinates []oneDragonCoordinate
	if strings.EqualFold(filepath.Ext(path), ".json") || strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		if parsed := parseOneDragonJSONCoordinates([]byte(text), path, startOrder); len(parsed) > 0 {
			coordinates = append(coordinates, parsed...)
			startOrder += len(parsed)
		}
	}
	coordinates = append(coordinates, parseOneDragonTextCoordinates(text, path, startOrder)...)
	return dedupeOneDragonCoordinates(coordinates)
}

func decodeOneDragonText(data []byte) string {
	if len(data) >= 2 {
		switch {
		case data[0] == 0xff && data[1] == 0xfe:
			if decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), textunicode.UTF16(textunicode.LittleEndian, textunicode.UseBOM).NewDecoder())); err == nil {
				return string(decoded)
			}
		case data[0] == 0xfe && data[1] == 0xff:
			if decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), textunicode.UTF16(textunicode.BigEndian, textunicode.UseBOM).NewDecoder())); err == nil {
				return string(decoded)
			}
		}
	}
	if utf8.Valid(data) {
		return string(data)
	}
	if decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), simplifiedchinese.GB18030.NewDecoder())); err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	return string(data)
}

func parseOneDragonJSONCoordinates(data []byte, path string, startOrder int) []oneDragonCoordinate {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil
	}
	var result []oneDragonCoordinate
	var walk func(any, string)
	walk = func(v any, inheritedName string) {
		switch typed := v.(type) {
		case []any:
			if x, y, z, ok := numericArrayCoord(typed); ok && strings.TrimSpace(inheritedName) != "" {
				result = append(result, oneDragonCoordinate{Name: inheritedName, X: x, Y: y, Z: z, SourcePath: path, Order: startOrder + len(result)})
				return
			}
			for _, item := range typed {
				walk(item, inheritedName)
			}
		case map[string]any:
			name := firstStringValue(typed, "name", "file", "filename", "file_name", "path", "bdx", "建筑", "文件", "文件名")
			if name == "" {
				name = inheritedName
			}
			if x, y, z, ok := mapCoord(typed); ok && strings.TrimSpace(name) != "" {
				result = append(result, oneDragonCoordinate{Name: name, X: x, Y: y, Z: z, SourcePath: path, Order: startOrder + len(result)})
			}
			for key, item := range typed {
				walk(item, key)
			}
		}
	}
	walk(value, "")
	return dedupeOneDragonCoordinates(result)
}

func parseOneDragonTextCoordinates(text, path string, startOrder int) []oneDragonCoordinate {
	linePattern := regexp.MustCompile(`^\s*(.*?)\s*[:：,，]?\s*[\(\[]?\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*[\)\]]?\s*$`)
	axisPattern := regexp.MustCompile(`(?i)^\s*(.*?)\s+x\s*[:=：]\s*(-?\d+).*?y\s*[:=：]\s*(-?\d+).*?z\s*[:=：]\s*(-?\d+)`)
	coordOnlyPattern := regexp.MustCompile(`^\s*[\(\[]?\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*[\)\]]?\s*$`)
	var coordinates []oneDragonCoordinate
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	pendingName := ""
	appendCoord := func(name, xText, yText, zText string) {
		name = cleanupCoordinateName(name)
		if name == "" {
			name = pendingName
		}
		name = cleanupCoordinateName(name)
		if name == "" {
			return
		}
		x, _ := strconv.Atoi(xText)
		y, _ := strconv.Atoi(yText)
		z, _ := strconv.Atoi(zText)
		coordinates = append(coordinates, oneDragonCoordinate{Name: name, X: x, Y: y, Z: z, SourcePath: path, Order: startOrder + len(coordinates)})
		pendingName = ""
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			pendingName = ""
			continue
		}
		if matches := axisPattern.FindStringSubmatch(line); len(matches) == 5 {
			appendCoord(matches[1], matches[2], matches[3], matches[4])
			continue
		}
		if matches := coordOnlyPattern.FindStringSubmatch(line); len(matches) == 4 {
			appendCoord("", matches[1], matches[2], matches[3])
			continue
		}
		matches := linePattern.FindStringSubmatch(line)
		if len(matches) == 5 {
			appendCoord(matches[1], matches[2], matches[3], matches[4])
			continue
		}
		if isLikelyCoordinateLabel(line) {
			pendingName = cleanupCoordinateName(line)
		} else {
			pendingName = ""
		}
	}
	return dedupeOneDragonCoordinates(coordinates)
}

func entriesFromStructureFilenameCoordinates(existing []OneDragonEntry, structures []oneDragonStructure) ([]OneDragonEntry, []string) {
	used := map[string]bool{}
	entries := make([]OneDragonEntry, 0, len(existing)+len(structures))
	for _, entry := range existing {
		entries = append(entries, entry)
		if abs, err := filepath.Abs(entry.SourcePath); err == nil {
			used[strings.ToLower(abs)] = true
		} else {
			used[strings.ToLower(filepath.Clean(entry.SourcePath))] = true
		}
	}

	for _, structure := range structures {
		key, err := filepath.Abs(structure.Path)
		if err != nil {
			key = filepath.Clean(structure.Path)
		}
		key = strings.ToLower(key)
		if used[key] {
			continue
		}
		displayName, x, y, z, ok := extractStructureFilenameCoordinate(structure.Name)
		if !ok {
			continue
		}
		entries = append(entries, OneDragonEntry{
			SourcePath:  structure.Path,
			DisplayName: displayName,
			X:           x,
			Y:           y,
			Z:           z,
		})
		used[key] = true
	}

	var skipped []string
	for _, structure := range structures {
		key, err := filepath.Abs(structure.Path)
		if err != nil {
			key = filepath.Clean(structure.Path)
		}
		key = strings.ToLower(key)
		if !used[key] {
			skipped = append(skipped, structure.Path)
		}
	}
	return entries, skipped
}

func extractStructureFilenameCoordinate(name string) (string, int, int, int, bool) {
	if strings.EqualFold(filepath.Ext(name), ".mcworld") {
		return "", 0, 0, 0, false
	}
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^(.*?)@\[\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*\]`),
		regexp.MustCompile(`^(.*?)\[\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*[,，\s]+\s*(-?\d+)\s*\]`),
		regexp.MustCompile(`^(.*?)(?:\(|（)?\s*(-?\d+)[_,，\s]+(-?\d+)[_,，\s]+(-?\d+)\s*(?:\)|）)?$`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(base)
		if len(matches) != 5 {
			continue
		}
		displayName := cleanupCoordinateName(matches[1])
		if displayName == "" {
			displayName = base
		}
		x, errX := strconv.Atoi(matches[2])
		y, errY := strconv.Atoi(matches[3])
		z, errZ := strconv.Atoi(matches[4])
		if errX == nil && errY == nil && errZ == nil {
			return displayName, x, y, z, true
		}
	}
	return "", 0, 0, 0, false
}

func isLikelyCoordinateLabel(line string) bool {
	line = cleanupCoordinateName(line)
	if line == "" {
		return false
	}
	if len([]rune(line)) > 60 {
		return false
	}
	lower := strings.ToLower(line)
	for _, word := range []string{"导入坐标", "使用说明", "说明", "作者", "拥有者", "owner", "readme"} {
		if strings.Contains(lower, word) {
			return false
		}
	}
	numberPattern := regexp.MustCompile(`-?\d+`)
	if len(numberPattern.FindAllString(line, -1)) >= 2 {
		return false
	}
	return true
}

func cleanupCoordinateName(name string) string {
	name = strings.TrimSpace(name)
	name = regexp.MustCompile(`^\s*(?:第?\d+|[一二三四五六七八九十百]+)\s*[\.、)\]）:：-]\s*`).ReplaceAllString(name, "")
	name = strings.Trim(name, "-_—:：,，;；|/\\[]()（）【】")
	name = strings.TrimSpace(name)
	for _, prefix := range []string{"坐标", "建筑", "结构", "文件", "文件名", "名称", "name", "file"} {
		if strings.EqualFold(name, prefix) {
			return ""
		}
	}
	return name
}

func numericArrayCoord(values []any) (int, int, int, bool) {
	if len(values) < 3 {
		return 0, 0, 0, false
	}
	x, okX := numericValue(values[0])
	y, okY := numericValue(values[1])
	z, okZ := numericValue(values[2])
	return x, y, z, okX && okY && okZ
}

func mapCoord(values map[string]any) (int, int, int, bool) {
	x, okX := lookupNumericValue(values, "x", "X")
	y, okY := lookupNumericValue(values, "y", "Y")
	z, okZ := lookupNumericValue(values, "z", "Z")
	if okX && okY && okZ {
		return x, y, z, true
	}
	for key, value := range values {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "pos", "position", "coord", "coords", "coordinate", "coordinates", "xyz", "坐标", "位置":
			if list, ok := value.([]any); ok {
				return numericArrayCoord(list)
			}
		}
	}
	return 0, 0, 0, false
}

func lookupNumericValue(values map[string]any, keys ...string) (int, bool) {
	for key, value := range values {
		for _, want := range keys {
			if strings.EqualFold(key, want) {
				return numericValue(value)
			}
		}
	}
	return 0, false
}

func numericValue(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		return int(typed), typed == float64(int(typed))
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case json.Number:
		v, err := typed.Int64()
		return int(v), err == nil
	case string:
		v, err := strconv.Atoi(strings.TrimSpace(typed))
		return v, err == nil
	default:
		return 0, false
	}
}

func firstStringValue(values map[string]any, keys ...string) string {
	for _, want := range keys {
		for key, value := range values {
			if !strings.EqualFold(key, want) {
				continue
			}
			if text, ok := value.(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func dedupeOneDragonCoordinates(coordinates []oneDragonCoordinate) []oneDragonCoordinate {
	seen := map[string]bool{}
	result := make([]oneDragonCoordinate, 0, len(coordinates))
	for _, coord := range coordinates {
		key := fmt.Sprintf("%s|%d|%d|%d", normalizeMatchText(coord.Name), coord.X, coord.Y, coord.Z)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, coord)
	}
	return result
}

func matchOneDragonEntries(structures []oneDragonStructure, coordinates []oneDragonCoordinate) ([]OneDragonEntry, []string, error) {
	commonPrefix := commonStructurePrefix(structures)
	type pair struct {
		structIndex int
		coordIndex  int
		score       int
		priority    int
	}
	var pairs []pair
	for si, structure := range structures {
		for ci, coord := range coordinates {
			score := oneDragonMatchScore(structure.Name, coord.Name, commonPrefix)
			if score > 0 {
				pairs = append(pairs, pair{structIndex: si, coordIndex: ci, score: score, priority: oneDragonStructureMatchPriority(structure.Name)})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].score != pairs[j].score {
			return pairs[i].score > pairs[j].score
		}
		if coordinates[pairs[i].coordIndex].Order != coordinates[pairs[j].coordIndex].Order {
			return coordinates[pairs[i].coordIndex].Order < coordinates[pairs[j].coordIndex].Order
		}
		return pairs[i].priority < pairs[j].priority
	})

	usedStructures := map[int]bool{}
	usedCoordinates := map[int]bool{}
	matched := map[int]int{}
	for _, p := range pairs {
		if usedStructures[p.structIndex] || usedCoordinates[p.coordIndex] {
			continue
		}
		usedStructures[p.structIndex] = true
		usedCoordinates[p.coordIndex] = true
		matched[p.coordIndex] = p.structIndex
	}

	coordOrder := coordinateOrder(coordinates)
	if len(matched) == 0 {
		for i, si := range structureOrderByName(structures) {
			if i >= len(coordOrder) {
				break
			}
			matched[coordOrder[i]] = si
		}
	} else if len(coordinates) == len(structures) {
		remainingStructures := remainingStructureOrderByName(structures, usedStructures)
		remainingCoords := remainingCoordinateOrder(coordinates, usedCoordinates)
		for i := 0; i < len(remainingCoords) && i < len(remainingStructures); i++ {
			matched[remainingCoords[i]] = remainingStructures[i]
		}
	}

	if len(matched) == 0 {
		return nil, nil, fmt.Errorf("one-dragon archive has structures and coordinates, but no names could be matched")
	}

	entries := make([]OneDragonEntry, 0, len(matched))
	usedFinalStructures := map[int]bool{}
	for _, ci := range coordOrder {
		coord := coordinates[ci]
		si, ok := matched[ci]
		if !ok {
			continue
		}
		usedFinalStructures[si] = true
		structure := structures[si]
		entries = append(entries, OneDragonEntry{
			SourcePath:  structure.Path,
			DisplayName: strings.TrimSuffix(structure.Name, filepath.Ext(structure.Name)),
			X:           coord.X,
			Y:           coord.Y,
			Z:           coord.Z,
		})
	}
	var skipped []string
	for i, structure := range structures {
		if !usedFinalStructures[i] {
			skipped = append(skipped, structure.Path)
		}
	}
	return entries, skipped, nil
}

func coordinateOrder(coordinates []oneDragonCoordinate) []int {
	coordOrder := make([]int, 0, len(coordinates))
	for i := range coordinates {
		coordOrder = append(coordOrder, i)
	}
	sort.SliceStable(coordOrder, func(i, j int) bool {
		return coordinates[coordOrder[i]].Order < coordinates[coordOrder[j]].Order
	})
	return coordOrder
}

func structureOrderByName(structures []oneDragonStructure) []int {
	structureOrder := make([]int, 0, len(structures))
	for i := range structures {
		structureOrder = append(structureOrder, i)
	}
	sort.SliceStable(structureOrder, func(i, j int) bool {
		return strings.ToLower(structures[structureOrder[i]].Name) < strings.ToLower(structures[structureOrder[j]].Name)
	})
	return structureOrder
}

func remainingCoordinateOrder(coordinates []oneDragonCoordinate, used map[int]bool) []int {
	var result []int
	for _, index := range coordinateOrder(coordinates) {
		if !used[index] {
			result = append(result, index)
		}
	}
	return result
}

func remainingStructureOrderByName(structures []oneDragonStructure, used map[int]bool) []int {
	var result []int
	for _, index := range structureOrderByName(structures) {
		if !used[index] {
			result = append(result, index)
		}
	}
	return result
}

func oneDragonStructureMatchPriority(name string) int {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".bdx":
		return 0
	case ".mcworld":
		return 20
	default:
		return 10
	}
}

func oneDragonMatchScore(fileName, coordName, commonPrefix string) int {
	fileNorm := normalizeMatchText(normalizeStructureMatchName(fileName))
	coordNorm := normalizeMatchText(coordName)
	fileSimple := strings.TrimPrefix(fileNorm, commonPrefix)
	coordSimple := strings.TrimPrefix(coordNorm, commonPrefix)
	fileSimple = trimGenericMatchWords(fileSimple)
	coordSimple = trimGenericMatchWords(coordSimple)

	if fileNorm == "" || coordNorm == "" || fileSimple == "" || coordSimple == "" {
		return 0
	}
	switch {
	case fileNorm == coordNorm:
		return 1000 + len([]rune(fileNorm))
	case fileSimple == coordSimple:
		return 950 + len([]rune(fileSimple))
	case strings.Contains(fileSimple, coordSimple):
		return 800 + len([]rune(coordSimple))
	case strings.Contains(coordSimple, fileSimple):
		return 760 + len([]rune(fileSimple))
	case strings.Contains(fileNorm, coordNorm):
		return 700 + len([]rune(coordNorm))
	case strings.Contains(coordNorm, fileNorm):
		return 650 + len([]rune(fileNorm))
	default:
		return fuzzyRuneOverlapScore(fileSimple, coordSimple)
	}
}

func normalizeStructureMatchName(name string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	base = regexp.MustCompile(`@\[[^\]]+\](?:~\[[^\]]+\])?`).ReplaceAllString(base, "")
	base = regexp.MustCompile(`\[[^\]]+\]`).ReplaceAllString(base, "")
	base = regexp.MustCompile(`(?:\(|（)?\s*-?\d+[_,，\s]+-?\d+[_,，\s]+-?\d+\s*(?:\)|）)?\s*$`).ReplaceAllString(base, "")
	return cleanupCoordinateName(base)
}

func normalizeMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, strings.ToLower(filepath.Ext(value)))
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r > 127 {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func trimGenericMatchWords(value string) string {
	replacer := strings.NewReplacer(
		"坐标", "",
		"建筑", "",
		"结构", "",
		"文件", "",
		"导入", "",
		"一条龙", "",
		"bdx", "",
		"mcworld", "",
	)
	return replacer.Replace(value)
}

func commonStructurePrefix(structures []oneDragonStructure) string {
	if len(structures) < 2 {
		return ""
	}
	prefix := normalizeMatchText(strings.TrimSuffix(structures[0].Name, filepath.Ext(structures[0].Name)))
	for _, structure := range structures[1:] {
		value := normalizeMatchText(strings.TrimSuffix(structure.Name, filepath.Ext(structure.Name)))
		prefix = commonRunePrefix(prefix, value)
		if prefix == "" {
			return ""
		}
	}
	if len([]rune(prefix)) < 2 {
		return ""
	}
	return prefix
}

func commonRunePrefix(a, b string) string {
	ar := []rune(a)
	br := []rune(b)
	n := len(ar)
	if len(br) < n {
		n = len(br)
	}
	i := 0
	for i < n && ar[i] == br[i] {
		i++
	}
	return string(ar[:i])
}

func fuzzyRuneOverlapScore(a, b string) int {
	ar := []rune(a)
	br := []rune(b)
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	set := map[rune]bool{}
	for _, r := range ar {
		set[r] = true
	}
	overlap := 0
	for _, r := range br {
		if set[r] {
			overlap++
		}
	}
	shorter := len(ar)
	if len(br) < shorter {
		shorter = len(br)
	}
	if shorter == 0 || overlap*2 < shorter {
		return 0
	}
	return 300 + overlap
}

func archiverByExtension(path string) (archiver.Unarchiver, bool) {
	value, err := archiver.ByExtension(path)
	if err != nil {
		return nil, false
	}
	unarchiver, ok := value.(archiver.Unarchiver)
	return unarchiver, ok
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return strings.EqualFold(absA, absB)
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func uniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
