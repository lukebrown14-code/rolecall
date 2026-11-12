package main_test

import (
	"encoding/json"
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

func TestCLIMalformedConfigNamesPathWithoutValues(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	path := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("workspace = \"private-sentinel\"\neditor = [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	message := string(out)
	if err == nil || !strings.Contains(message, path) || !strings.Contains(message, "invalid TOML") || strings.Contains(message, "private-sentinel") {
		t.Fatalf("want safe error naming %s, got %q %v", path, out, err)
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
	if err != nil || !strings.Contains(string(out), workspace+" (directory)") || !strings.Contains(string(out), "chmod 700 "+workspace) || !strings.Contains(string(out), "Template in use: Header Band (default)") || !strings.Contains(string(out), "Make your documents yours") {
		t.Fatalf("doctor output = %q, err = %v", out, err)
	}
	for _, check := range []string{"typst", "pandoc", "Python 3", "Plugin path", "Template fonts", "Harness command"} {
		if !strings.Contains(string(out), "WARNING: "+check) && !strings.Contains(string(out), "OK: "+check) {
			t.Errorf("doctor output missing %s result: %q", check, out)
		}
	}
}

func TestCLIDoctorRejectsWorkspaceThatCannotBeOpened(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	workspaceFile := filepath.Join(home, "not-a-workspace")
	if err := os.WriteFile(workspaceFile, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "doctor", "--dir", workspaceFile)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "open Workspace") {
		t.Fatalf("doctor output = %q, err = %v; want Workspace open failure", out, err)
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
	home := t.TempDir()
	cmd := exec.Command(binary, "init", "--workspace", workspace, "--name", "Alex Quinn", "--email", "alex@example.test", "--phone", "+61 400", "--location", "Melbourne", "--linkedin", "linkedin.com/in/alex", "--tracks", "SWE, Platform", "--search-places", "Australia, Melbourne", "--search-remote", "anywhere", "--profile", "later")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %s %v", out, err)
	}
	for _, text := range []string{"Harness", "AI service", "Profile", "CV", "job ads", "private"} {
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
	profile, err := s.Profile()
	if err != nil || profile.Contact["Name"] != "Alex Quinn" || profile.Contact["Email"] != "alex@example.test" || len(profile.Missing) != 0 {
		t.Errorf("initialized Contact = %+v, %v; want required fields read by Store.Profile", profile, err)
	}
	contact, err := os.ReadFile(filepath.Join(workspace, "profile/master-profile.md"))
	if err != nil || !strings.Contains(string(contact), "| Email | alex@example.test |") || !strings.Contains(string(contact), "| LinkedIn | linkedin.com/in/alex |") {
		t.Errorf("Contact table = %q, %v", contact, err)
	}
	settings, err := os.ReadFile(filepath.Join(workspace, "workspace.json"))
	var decoded struct {
		Tracks     []string `json:"tracks"`
		SearchArea struct {
			Places []string `json:"places"`
			Remote string   `json:"remote"`
		} `json:"search_area"`
	}
	if err == nil {
		err = json.Unmarshal(settings, &decoded)
	}
	if err != nil || strings.Join(decoded.Tracks, ",") != "SWE,Platform" || strings.Join(decoded.SearchArea.Places, ",") != "Australia,Melbourne" || decoded.SearchArea.Remote != "anywhere" {
		t.Errorf("workspace settings = %q (%+v), %v", settings, decoded, err)
	}
	if config, err := os.ReadFile(filepath.Join(home, ".config/rolecall/config.toml")); err != nil || !strings.Contains(string(config), "workspace = \""+workspace+"\"") {
		t.Errorf("machine config = %q, %v", config, err)
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
	out, err := exec.Command(binary, "init", "--workspace", workspace, "--name", "Alex Quinn", "--email", "alex@example.test", "--profile", "later").CombinedOutput()
	if err == nil || !strings.Contains(strings.ToLower(string(out)), "not empty") {
		t.Fatalf("init output = %q, error = %v; want clear non-empty refusal", out, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("existing file was changed or removed: %v", err)
	}
}

func TestCLIInitAccessiblePromptsWorkWithoutTTY(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "accessible workspace")
	cmd := exec.Command(binary, "init", "--accessible")
	cmd.Env = append(os.Environ(), "HOME="+home)
	cmd.Stdin = strings.NewReader(strings.Join([]string{
		"Alex Quinn", "alex@example.test", "", "Melbourne", "linkedin.com/in/alex",
		"SWE,Platform", "Australia", "2", workspace, "3",
	}, "\n") + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("accessible init failed: %s %v", out, err)
	}
	settings, err := os.ReadFile(filepath.Join(workspace, "workspace.json"))
	if err != nil || !strings.Contains(string(settings), `"remote": "in-places"`) {
		t.Fatalf("accessible settings = %q, %v", settings, err)
	}
}
