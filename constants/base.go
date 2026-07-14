package constants

import (
	"os"
	"strings"
)

const Name = "NexusEgo"

const RepoURL = "https://github.com/LangTuStudio/Nexusego-Release"

const ServerURL = "https://studio.aurelrune.com/"

const DefaultGitHubProxyPrefix = "https://mirror.ghproxy.com/"

const DefaultImportSpeed = 3000

func LatestReleaseAPIURL() string {
	repoPath := strings.TrimPrefix(RepoURL, "https://github.com/")
	repoPath = strings.TrimPrefix(repoPath, "http://github.com/")
	repoPath = strings.Trim(repoPath, "/")
	return "https://api.github.com/repos/" + repoPath + "/releases/latest"
}

func GitHubProxyPrefix() string {
	if value := strings.TrimSpace(os.Getenv("NEXUS_GITHUB_PROXY")); value != "" {
		return normalizeProxyPrefix(value)
	}
	return normalizeProxyPrefix(DefaultGitHubProxyPrefix)
}

func GitHubURLCandidates(rawURL string) []string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}

	candidates := make([]string, 0, 2)
	seen := map[string]struct{}{}
	appendURL := func(url string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		candidates = append(candidates, url)
	}

	if proxyPrefix := GitHubProxyPrefix(); proxyPrefix != "" {
		appendURL(proxyPrefix + rawURL)
	}
	appendURL(rawURL)
	return candidates
}

func LatestReleaseAPIURLCandidates() []string {
	return GitHubURLCandidates(LatestReleaseAPIURL())
}

func normalizeProxyPrefix(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if !strings.HasSuffix(value, "/") {
		value += "/"
	}
	return value
}
