// Package doctor checks privacy-related Workspace permissions.
package doctor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Visibility describes whether a GitHub repository is publicly visible.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// VisibilityLookup returns GitHub repository visibility by owner and name.
type VisibilityLookup func(owner, repo string) (Visibility, error)

const (
	githubRemoteParts   = 2
	githubLookupTimeout = 5 * time.Second
)

// Check reports Workspace paths whose permissions allow broader access than
// rolecall's private-file defaults. Symlinks are not followed.
func Check(root string, lookup VisibilityLookup) ([]string, error) {
	var warnings []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		// Hidden directories are not Workspace data. .git alone holds thousands of files
		// whose modes git manages, and the chmod advice for them would be wrong.
		if path != root && entry.IsDir() && strings.HasPrefix(entry.Name(), ".") {
			return fs.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var limit os.FileMode
		var kind, fix string
		switch {
		case info.IsDir():
			limit, kind, fix = 0o700, "directory", "700"
		case info.Mode().IsRegular():
			limit, kind, fix = 0o600, "file", "600"
		default:
			return nil
		}
		mode := info.Mode().Perm()
		if mode&^limit != 0 {
			warnings = append(warnings, fmt.Sprintf("%s (%s): permissions %04o; fix with chmod %s %s", path, kind, mode, fix, shellQuote(path)))
		}
		return nil
	})
	if err != nil {
		return warnings, err
	}
	remoteWarnings, err := checkRemote(root, lookup)
	if err != nil {
		return warnings, err
	}
	return append(warnings, remoteWarnings...), nil
}

func checkRemote(root string, lookup VisibilityLookup) ([]string, error) {
	urls, err := remoteURLs(filepath.Join(root, ".git", "config"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read Workspace git remotes: %w", err)
	}
	var warnings []string
	for _, remoteURL := range urls {
		owner, repo, github := parseGitHubRemote(remoteURL)
		if !github {
			warnings = append(warnings, fmt.Sprintf("git remote %s: visibility unknown (not a GitHub remote)", remoteURL))
			continue
		}
		if lookup == nil {
			warnings = append(warnings, fmt.Sprintf("git remote %s: visibility skipped (offline)", remoteURL))
			continue
		}
		visibility, err := lookup(owner, repo)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("git remote %s: visibility skipped (offline)", remoteURL))
			continue
		}
		if visibility == VisibilityPublic {
			warnings = append(warnings, fmt.Sprintf("git remote %s: public GitHub repository; make the repository private to protect Workspace data", remoteURL))
		}
	}
	return warnings, nil
}

func remoteURLs(configPath string) ([]string, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var urls []string
	inRemote := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inRemote = strings.HasPrefix(strings.ToLower(line), "[remote ")
			continue
		}
		if !inRemote || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "url") {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
			value = value[1 : len(value)-1]
		}
		if value != "" {
			urls = append(urls, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return urls, nil
}

func parseGitHubRemote(remote string) (owner, repo string, ok bool) {
	var host, path string
	if strings.HasPrefix(remote, "http://") || strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "ssh://") {
		parsed, err := url.Parse(remote)
		if err != nil {
			return "", "", false
		}
		host, path = strings.ToLower(parsed.Hostname()), parsed.Path
	} else {
		left, right, found := strings.Cut(remote, ":")
		if !found || !strings.Contains(left, "@") {
			return "", "", false
		}
		_, host, _ = strings.Cut(left, "@")
		host, path = strings.ToLower(host), right
	}
	if host != "github.com" {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != githubRemoteParts {
		return "", "", false
	}
	owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}

// GitHubVisibilityLookup performs an unauthenticated GitHub API lookup. A 404
// means the repository is private or unavailable to anonymous callers.
func GitHubVisibilityLookup(owner, repo string) (Visibility, error) {
	return lookupGitHubVisibility(&http.Client{Timeout: githubLookupTimeout}, owner, repo)
}

func lookupGitHubVisibility(client *http.Client, owner, repo string) (Visibility, error) {
	endpoint := "https://api.github.com/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return VisibilityPrivate, nil
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub repository lookup returned HTTP %d", response.StatusCode)
	}
	var repository struct {
		Private bool `json:"private"`
	}
	if err := json.NewDecoder(response.Body).Decode(&repository); err != nil {
		return "", err
	}
	if repository.Private {
		return VisibilityPrivate, nil
	}
	return VisibilityPublic, nil
}

func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("/._-", r))
	}) == -1 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
