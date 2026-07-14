package file

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"nexus/utils/log"
)

func FileIsExisted(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if !os.IsNotExist(err) {
		return false, err
	}

	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		log.Log.Warn(
			"create directory failed",
			log.Log.ArgsFromMap(map[string]any{
				"stage": "plugin",
				"error": err.Error(),
			}),
		)
		return false, err
	}
	return true, nil
}

func Is_File(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func Is_Dir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func Copy_file(path string, new_path string) error {
	originalFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer originalFile.Close()

	newFile, err := os.Create(new_path)
	if err != nil {
		return err
	}
	defer newFile.Close()

	if _, err := io.Copy(newFile, originalFile); err != nil {
		return err
	}
	return newFile.Sync()
}

func GetAllFile(pathname string) ([]string, error) {
	entries, err := os.ReadDir(pathname)
	if err != nil {
		fmt.Println("read dir fail:", err)
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, pathname+"/"+entry.Name())
	}
	return paths, nil
}

func GetSHA256FromFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
