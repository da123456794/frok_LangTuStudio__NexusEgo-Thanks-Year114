package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"nexus/constants"
)

func SliceAtoi(sa []string) ([]int, error) {
	si := make([]int, 0, len(sa))
	for _, a := range sa {
		i, err := strconv.Atoi(a)
		if err != nil {
			return si, err
		}
		si = append(si, i)
	}
	return si, nil
}

func MergeMaps(mapping ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, value := range mapping {
		maps.Copy(result, value)
	}
	return result
}

func GetHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func GetMD5(i string) string {
	sum := md5.Sum([]byte(i))
	return hex.EncodeToString(sum[:])
}

func CheckUpdate(currentVersion string) (bool, string) {
	version_regexp := regexp.MustCompile(`^v?((\d+).(\d+).(\d+))$`)
	current_version_m := version_regexp.FindAllStringSubmatch(currentVersion, -1)
	if len(current_version_m) == 0 || len(current_version_m[0]) != 5 {
		return false, ""
	}
	current_major_version, _ := strconv.Atoi(current_version_m[0][2])
	current_minor_version, _ := strconv.Atoi(current_version_m[0][3])
	current_patch_version, _ := strconv.Atoi(current_version_m[0][4])

	var (
		content []byte
		err     error
	)
	for _, url := range constants.LatestReleaseAPIURLCandidates() {
		resp, requestErr := http.Get(url)
		if requestErr != nil {
			err = requestErr
			continue
		}
		content, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		err = fmt.Errorf("unexpected status: %d", resp.StatusCode)
		content = nil
	}
	if len(content) == 0 {
		fmt.Printf("Failed to check update!\nPlease check your network status.\n")
		return false, ""
	}
	var json_structure map[string]interface{}
	err = json.Unmarshal(content, &json_structure)
	if err != nil {
		fmt.Printf("Failed to check update due to invalid response received from GitHub.\n")
		return false, ""
	}
	version, found_tag_name_item := json_structure["tag_name"].(string)
	if !found_tag_name_item {
		fmt.Printf("Unknown error occured while checking the update\n")
		return false, ""
	}
	regexp_res := version_regexp.FindAllStringSubmatch(version, -1)
	if len(regexp_res) == 0 || len(regexp_res[0]) != 5 {
		fmt.Printf("Invalid latest version found from Github.\n")
		return false, ""
	}
	latest_major_version, _ := strconv.Atoi(regexp_res[0][2])
	latest_minor_version, _ := strconv.Atoi(regexp_res[0][3])
	latest_patch_version, _ := strconv.Atoi(regexp_res[0][4])
	if current_major_version < latest_major_version {
		return true, regexp_res[0][1]
	} else if current_major_version == latest_major_version {
		if current_minor_version < latest_minor_version {
			return true, regexp_res[0][1]
		} else if current_minor_version == latest_minor_version && current_patch_version < latest_patch_version {
			return true, regexp_res[0][1]
		}
	}
	return false, ""
}

func ReadToken(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
