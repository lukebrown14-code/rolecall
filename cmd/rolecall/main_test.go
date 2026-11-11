package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/store"
)

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	home := t.TempDir()
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home, "GOCACHE="+filepath.Join(t.TempDir(), "cache"))
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.HasPrefix(string(out), "no Workspace configured in "+filepath.Join(home, ".config/rolecall/config.toml")+"\n") {
		t.Fatalf("%s %v", out, err)
	}
}

// A configured Workspace that is not on disk is reported on one line that names the config file.
func TestCLIMissingWorkspaceNamesConfigPath(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	home := t.TempDir()
	config := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(home, "nowhere")
	if err := os.WriteFile(config, []byte("workspace = \""+missing+"\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || strings.Count(string(out), "\n") != 1 || !strings.Contains(string(out), config) || !strings.Contains(string(out), missing) {
		t.Fatalf("want one line naming %s and %s, got %q %v", config, missing, out, err)
	}
}

func TestCLIMalformedWorkspaceNamesPathWithoutValues(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %s %v", out, err)
	}
	home := t.TempDir()
	config := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(home, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("workspace = \""+workspace+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspaceConfig := filepath.Join(workspace, "workspace.json")
	if err := os.WriteFile(workspaceConfig, []byte(`{"tracks":"private-sentinel"`), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	message := string(out)
	if err == nil || !strings.Contains(message, workspaceConfig) || !strings.Contains(message, "invalid Workspace settings") || strings.Contains(message, "private-sentinel") {
		t.Fatalf("want safe error naming %s, got %q %v", workspaceConfig, out, err)
	}
}

func TestCLIDoctorUsesConfiguredWorkspaceAndIgnoresDownloads(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	workspace := filepath.Join(home, "private-workspace")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(home, "Downloads")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "loose.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("workspace = \""+workspace+"\"\ndownloads = \""+outside+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Workspace permissions are private.") || strings.Contains(string(out), outside) {
		t.Fatalf("doctor output = %q, err = %v", out, err)
	}
}

func TestCLIDoctorDirWarnsButExitsSuccessfully(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	workspace := filepath.Join(home, "selected")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("workspace = \"/a/different/workspace\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "doctor", "--dir", workspace)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), workspace+" (directory)") || !strings.Contains(string(out), "chmod 700 "+workspace) {
		t.Fatalf("doctor output = %q, err = %v", out, err)
	}
}

func buildRolecall(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", out, err)
	}
	return binary
}

func TestCLIInitCreatesPrivateWorkspaceAndLocalGitRepo(t *testing.T) {
	binary := buildRolecall(t)
	workspace := filepath.Join(t.TempDir(), "private")
	out, err := exec.Command(binary, "init", workspace).CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %s %v", out, err)
	}
	for _, text := range []string{"Harness", "AI service", "private"} {
		if !strings.Contains(strings.ToLower(string(out)), strings.ToLower(text)) {
			t.Errorf("init output %q does not mention %q", out, text)
		}
	}
	for _, path := range []string{"workspace.json", "profile/master-profile.md", "profile/phrasing-bank.md", "profile/projects.md", "profile/writing-preferences.md", "cv", "jobs"} {
		if _, err := os.Stat(filepath.Join(workspace, path)); err != nil {
			t.Errorf("missing %s: %v", path, err)
		}
	}
	for _, path := range []string{"", "profile", "cv", "jobs"} {
		info, err := os.Stat(filepath.Join(workspace, path))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("%s mode = %04o, want 0700", path, got)
		}
	}
	for _, path := range []string{"workspace.json", "profile/master-profile.md", "profile/phrasing-bank.md", "profile/projects.md", "profile/writing-preferences.md"} {
		info, err := os.Stat(filepath.Join(workspace, path))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s mode = %04o, want 0600", path, got)
		}
	}
	if out, err := exec.Command("git", "-C", workspace, "remote").CombinedOutput(); err != nil || strings.TrimSpace(string(out)) != "" {
		t.Errorf("git remotes = %q, %v; want none", out, err)
	}
	s, err := store.Open(workspace)
	if err != nil {
		t.Fatalf("initialized Workspace cannot be opened: %v", err)
	}
	if apps, err := s.Scan(); err != nil || len(apps) != 0 {
		t.Errorf("scan initialized Workspace = %v, %v; want no Applications", apps, err)
	}
}

func TestCLIInitRefusesNonEmptyDirectory(t *testing.T) {
	binary := buildRolecall(t)
	workspace := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(workspace, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(binary, "init", workspace).CombinedOutput()
	if err == nil || !strings.Contains(strings.ToLower(string(out)), "not empty") {
		t.Fatalf("init output = %q, error = %v; want clear non-empty refusal", out, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("existing file was changed or removed: %v", err)
	}
}
