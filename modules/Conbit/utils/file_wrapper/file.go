package file_wrapper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"crypto/md5"
)

func WriteFile(path string, data []byte, perm os.FileMode) error {
	if err := CreateDirIfNotExists(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func WriteJsonData(path string, data interface{}) error {
	if err := CreateDirIfNotExists(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "\t")
	return encoder.Encode(data)
}

func GetFileData(path string) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func GetJsonData(path string, data interface{}) error {
	content, err := GetFileData(path)
	if err != nil {
		return err
	}
	if len(content) == 0 {
		return nil
	}
	content = bytes.Trim(content, "\xef\xbb\xbf")
	if len(content) == 0 {
		return nil
	}
	return json.Unmarshal(content, data)
}

func CopyDirectory(srcPath string, dstPath string) error {
	entries, err := os.ReadDir(srcPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(srcPath, entry.Name())
		dst := filepath.Join(dstPath, entry.Name())

		info, err := os.Stat(src)
		if err != nil {
			return err
		}

		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			if err := CopySymLink(src, dst); err != nil {
				return err
			}
		case mode.IsDir():
			if err := CreateDirIfNotExists(dst, 0o755); err != nil {
				return err
			}
			if err := CopyDirectory(src, dst); err != nil {
				return err
			}
		default:
			if err := CreateDirIfNotExists(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := CopyFile(src, dst); err != nil {
				return err
			}
		}

		info, err = entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err := os.Chmod(dst, info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func CopyFile(srcPath string, dstPath string) error {
	dstFile, err := os.OpenFile(dstPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	srcFile, err := os.OpenFile(srcPath, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func DirExist(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func CreateDirIfNotExists(path string, perm os.FileMode) error {
	if DirExist(path) {
		return nil
	}
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", path, err)
	}
	return nil
}

func CopySymLink(oldPath string, newPath string) error {
	linkTarget, err := os.Readlink(oldPath)
	if err != nil {
		return err
	}
	if err := os.Symlink(linkTarget, newPath); err != nil {
		if os.IsExist(err) {
			return nil
		}
		return &os.LinkError{
			Op:  "symlink",
			Old: linkTarget,
			New: newPath,
			Err: err,
		}
	}
	return nil
}

func GetFileMD5Str(path string) (string, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
