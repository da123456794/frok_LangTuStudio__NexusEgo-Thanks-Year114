package control

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempWorkingDir(t *testing.T) string {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
	return dir
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveImportFilePathRootFallback(t *testing.T) {
	withTempWorkingDir(t)

	writeTestFile(t, "root.mcworld")

	if got, want := ResolveImportFilePath("root.mcworld"), "root.mcworld"; got != want {
		t.Fatalf("ResolveImportFilePath(root.mcworld) = %q, want %q", got, want)
	}
}

func TestResolveImportFilePathPrefersStorageFile(t *testing.T) {
	withTempWorkingDir(t)

	writeTestFile(t, "same.mcworld")
	writeTestFile(t, filepath.Join(StorageFileDir(), "same.mcworld"))

	want := filepath.Join(StorageFileDir(), "same.mcworld")
	if got := ResolveImportFilePath("same.mcworld"); got != want {
		t.Fatalf("ResolveImportFilePath(same.mcworld) = %q, want %q", got, want)
	}
}

func TestResolveImportFilePathKeepsStorageDefaultWhenMissing(t *testing.T) {
	withTempWorkingDir(t)

	want := filepath.Join(StorageFileDir(), "missing.mcworld")
	if got := ResolveImportFilePath("missing.mcworld"); got != want {
		t.Fatalf("ResolveImportFilePath(missing.mcworld) = %q, want %q", got, want)
	}
}

func TestGetImportFilesInSearchDirsMergesRootCandidates(t *testing.T) {
	withTempWorkingDir(t)

	writeTestFile(t, filepath.Join(StorageFileDir(), "stored.mcworld"))
	writeTestFile(t, filepath.Join(StorageFileDir(), "same.mcworld"))
	writeTestFile(t, "root.mcworld")
	writeTestFile(t, "same.mcworld")
	writeTestFile(t, "README.md")

	files, err := getImportFilesInSearchDirs()
	if err != nil {
		t.Fatal(err)
	}

	counts := map[string]int{}
	for _, name := range files {
		counts[name]++
	}

	if counts["stored.mcworld"] != 1 {
		t.Fatalf("stored.mcworld count = %d, want 1 in %v", counts["stored.mcworld"], files)
	}
	if counts["root.mcworld"] != 1 {
		t.Fatalf("root.mcworld count = %d, want 1 in %v", counts["root.mcworld"], files)
	}
	if counts["same.mcworld"] != 1 {
		t.Fatalf("same.mcworld count = %d, want 1 in %v", counts["same.mcworld"], files)
	}
	if counts["README.md"] != 0 {
		t.Fatalf("README.md count = %d, want 0 in %v", counts["README.md"], files)
	}
}
