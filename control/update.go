package control

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"nexus/cmd/common"
	"nexus/constants"
	consolepkg "nexus/utils/console"
	"nexus/utils/log"
	"nexus/utils/netutil"
	"nexus/utils/ui"
)

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Body    string               `json:"body"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (a *App) checkLatestVersion() (*UpdateInfo, error) {
	client := netutil.NewHTTPClient(15 * time.Second)
	body, _, err := fetchFirstSuccessfulURL(client, constants.LatestReleaseAPIURLCandidates())
	if err != nil {
		return nil, err
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}
	version := strings.TrimSpace(release.TagName)
	if version == "" {
		return nil, fmt.Errorf("未获取到版本号")
	}

	info := &UpdateInfo{
		Success:   true,
		Version:   version,
		Changelog: strings.TrimSpace(release.Body),
	}
	if asset, ok := matchReleaseAsset(release.Assets); ok {
		info.DownloadURL = asset.BrowserDownloadURL
		info.Filename = asset.Name
	}
	return info, nil
}

func fetchFirstSuccessfulURL(client *http.Client, urls []string) ([]byte, string, error) {
	var lastErr error
	for _, requestURL := range urls {
		req, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", ui.AppName+"/"+ui.Version)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s 返回 %d", requestURL, resp.StatusCode)
			continue
		}
		return body, requestURL, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的 GitHub 地址")
	}
	return nil, "", lastErr
}

func matchReleaseAsset(assets []githubReleaseAsset) (githubReleaseAsset, bool) {
	expectedName := strings.ToLower(expectedReleaseAssetName())
	appName := strings.ToLower(ui.AppName)

	for _, asset := range assets {
		if asset.BrowserDownloadURL == "" {
			continue
		}
		if strings.EqualFold(asset.Name, expectedName) {
			return asset, true
		}
	}

	for _, asset := range assets {
		if asset.BrowserDownloadURL == "" {
			continue
		}
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, appName) &&
			strings.Contains(name, runtime.GOOS) &&
			strings.Contains(name, runtime.GOARCH) {
			return asset, true
		}
	}

	return githubReleaseAsset{}, false
}

func expectedReleaseAssetName() string {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	return fmt.Sprintf("%s_%s_%s%s", ui.AppName, runtime.GOOS, runtime.GOARCH, ext)
}

func isNewerVersion(latest, current string) bool {
	latest = strings.TrimPrefix(latest, "v")
	current = strings.TrimPrefix(current, "v")
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	maxLen := len(latestParts)
	if len(currentParts) > maxLen {
		maxLen = len(currentParts)
	}

	for i := 0; i < maxLen; i++ {
		var latestValue, currentValue int
		if i < len(latestParts) {
			fmt.Sscanf(latestParts[i], "%d", &latestValue)
		}
		if i < len(currentParts) {
			fmt.Sscanf(currentParts[i], "%d", &currentValue)
		}
		if latestValue > currentValue {
			return true
		}
		if latestValue < currentValue {
			return false
		}
	}
	return false
}

func downloadUpdate(url, filename string) error {
	log.Log.Info("正在下载: " + filename)
	client := netutil.NewHTTPClient(10 * time.Minute)
	var lastErr error

	for _, candidate := range constants.GitHubURLCandidates(url) {
		resp, err := client.Get(candidate)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("下载失败, HTTP %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}

		out, err := os.Create(filename)
		if err != nil {
			resp.Body.Close()
			return err
		}

		_, err = io.Copy(out, resp.Body)
		closeErr := out.Close()
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的下载地址")
	}
	return lastErr
}

func (a *App) checkAndPromptUpdate(console *consolepkg.Console_input) {
	log.Log.Info("正在检查更新...")
	info, err := a.checkLatestVersion()
	if err != nil {
		log.Log.Warn("检查更新失败: " + err.Error())
		return
	}
	if !isNewerVersion(info.Version, ui.Version) {
		log.Log.Info("当前已是最新版本")
		return
	}

	log.Log.Info(fmt.Sprintf("发现新版本: v%s (当前: v%s)", strings.TrimPrefix(info.Version, "v"), ui.Version))
	changelog := strings.TrimSpace(info.Changelog)
	if changelog != "" {
		log.Log.Info("更新日志:")
		for _, line := range strings.Split(changelog, "\n") {
			fmt.Println("  " + line)
		}
	}
	if info.DownloadURL == "" {
		log.Log.Warn("当前平台暂无可用下载，请联系管理员")
		return
	}

	filename := info.Filename
	if filename == "" {
		filename = fmt.Sprintf("%s_%s_%s", ui.AppName, runtime.GOOS, runtime.GOARCH)
	}
	input, _, _ := console.InputInfo(fmt.Sprintf("是否下载更新 (%s)? [y/n, 默认y]: ", filename))
	choice := strings.ToLower(strings.TrimSpace(input))
	if choice == "n" || choice == "no" {
		log.Log.Info("已跳过更新，程序即将退出")
		common.ExitAfterPrompt(console, 0)
	}

	if err := downloadUpdate(info.DownloadURL, filename); err != nil {
		log.Log.Error("下载更新失败: " + err.Error())
		common.ExitAfterPrompt(console, 0)
	}

	log.Log.Success("下载完成: " + filename)
	log.Log.Info("请替换当前程序后重启")
	common.ExitAfterPrompt(console, 0)
}
