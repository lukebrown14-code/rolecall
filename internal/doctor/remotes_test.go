package doctor

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubVisibilityLookupRequestsOnlyRepositoryIdentity(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Host != "api.github.com" || request.URL.Path != "/repos/owner/repo" {
			t.Fatalf("request = %s %s, want GET /repos/owner/repo on api.github.com", request.Method, request.URL)
		}
		if request.URL.RawQuery != "" || request.URL.User != nil {
			t.Fatalf("request contains extra identity data: %s", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"private":false}`)),
			Header:     make(http.Header),
			Request:    request,
		}, nil
	})}
	visibility, err := lookupGitHubVisibility(client, "owner", "repo")
	if err != nil {
		t.Fatal(err)
	}
	if visibility != VisibilityPublic {
		t.Fatalf("visibility = %q, want public", visibility)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestCheckReportsPublicGitHubRemote(t *testing.T) {
	for _, remote := range []string{
		"git@github.com:owner/repo.git",
		"https://github.com/owner/repo.git",
	} {
		t.Run(remote, func(t *testing.T) {
			root := workspaceWithRemote(t, remote)
			var gotOwner, gotRepo string
			warnings, err := Check(root, func(owner, repo string) (Visibility, error) {
				gotOwner, gotRepo = owner, repo
				return VisibilityPublic, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if gotOwner != "owner" || gotRepo != "repo" {
				t.Fatalf("lookup got %q/%q, want owner/repo", gotOwner, gotRepo)
			}
			if !containsWarning(warnings, "public GitHub repository") || !containsWarning(warnings, "make the repository private") {
				t.Fatalf("warnings = %v, want public-repository privacy fix", warnings)
			}
		})
	}
}

func TestCheckHandlesPrivateOfflineNonGitHubAndMissingRemotes(t *testing.T) {
	tests := []struct {
		name       string
		remote     string
		lookup     func(string, string) (Visibility, error)
		want       string
		lookupCall bool
	}{
		{
			name:       "private GitHub repository",
			remote:     "https://github.com/owner/private.git",
			lookup:     func(string, string) (Visibility, error) { return VisibilityPrivate, nil },
			lookupCall: true,
		},
		{
			name:   "offline GitHub repository",
			remote: "git@github.com:owner/repo.git",
			lookup: func(string, string) (Visibility, error) { return "", errors.New("offline") },
			want:   "skipped (offline)", lookupCall: true,
		},
		{
			name:   "non-GitHub repository",
			remote: "https://gitlab.com/owner/repo.git",
			lookup: func(string, string) (Visibility, error) {
				t.Fatal("lookup called for non-GitHub remote")
				return "", nil
			},
			want: "visibility unknown",
		},
		{
			name:   "no remote",
			lookup: func(string, string) (Visibility, error) { t.Fatal("lookup called without a remote"); return "", nil },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := workspaceWithRemote(t, tt.remote)
			called := false
			lookup := func(owner, repo string) (Visibility, error) {
				called = true
				return tt.lookup(owner, repo)
			}
			warnings, err := Check(root, lookup)
			if err != nil {
				t.Fatal(err)
			}
			if called != tt.lookupCall {
				t.Fatalf("lookup called = %v, want %v", called, tt.lookupCall)
			}
			if tt.want == "" && len(warnings) != 0 {
				t.Fatalf("warnings = %v, want none", warnings)
			}
			if tt.want != "" && !containsWarning(warnings, tt.want) {
				t.Fatalf("warnings = %v, want %q", warnings, tt.want)
			}
		})
	}
}

func workspaceWithRemote(t *testing.T, remote string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	config := "[core]\n\trepositoryformatversion = 0\n"
	if remote != "" {
		config += "[remote \"origin\"]\n\turl = " + remote + "\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n"
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
