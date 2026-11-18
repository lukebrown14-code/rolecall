package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// A dashboard-era SHORTLIST.md is never read, so doctor names it and the migration.
func TestCLIDoctorWarnsAboutLegacyShortlist(t *testing.T) {
	binary := buildRolecall(t)
	workspace := filepath.Join(t.TempDir(), "ws")
	home := t.TempDir()
	initCmd := exec.Command(binary, "init", workspace)
	initCmd.Env = append(os.Environ(), "HOME="+home)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %s %v", out, err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "jobs", "SHORTLIST.md"), []byte("# Shortlist\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, _ := cmd.CombinedOutput()
	for _, want := range []string{"WARNING: jobs/SHORTLIST.md is from an older rolecall", "migrate.py", "--dry-run"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestCLI(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, "..").CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	home := t.TempDir()
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "HOME="+home, "GOCACHE="+filepath.Join(t.TempDir(), "cache"))
	out, err := cmd.CombinedOutput()
	if err == nil || string(out) != "No Workspace yet. Run `rolecall init` to create one, or set workspace in "+filepath.Join(home, ".config/rolecall/config.toml")+"\n" {
		t.Fatalf("%s %v", out, err)
	}
}

// Before init there is no config file: doctor still runs its setup checks, then
// fails with the next step instead of stopping at the missing Workspace.
func TestCLIDoctorWithoutConfigRunsChecksAndPointsToInit(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	cmd := exec.Command(binary, "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "Run `rolecall init` to create one") {
		t.Fatalf("doctor output = %q, err = %v; want a failure pointing to rolecall init", out, err)
	}
	for _, check := range []string{"typst", "Plugin path", "Harness command", "Template in use"} {
		if !strings.Contains(string(out), check) {
			t.Errorf("doctor output missing %s: %q", check, out)
		}
	}
}

// A configured Workspace that is not on disk is reported on one line that names the config file.
func TestCLIMissingWorkspaceNamesConfigPath(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, "..").CombinedOutput(); err != nil {
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
	for _, want := range []string{path, "line 2, column 11", "array is incomplete", "rolecall doctor", "rolecall --dir"} {
		if err == nil || !strings.Contains(message, want) || strings.Contains(message, "private-sentinel") {
			t.Fatalf("want safe error with %q, got %q %v", want, out, err)
		}
	}
	// doctor reports the same problem, then carries on with the other checks and fails.
	cmd = exec.Command(binary, "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err = cmd.CombinedOutput()
	message = string(out)
	for _, want := range []string{"PROBLEM: " + path, "line 2, column 11", "Editor", "config.toml is invalid"} {
		if err == nil || !strings.Contains(message, want) || strings.Contains(message, "private-sentinel") {
			t.Fatalf("doctor: want %q, got %q %v", want, out, err)
		}
	}
}

// doctor applies the Track name rules the app applies, so a tracks list the app refuses
// (one listed twice, a blank one) fails doctor too.
func TestCLIDoctorReportsTrackListTheAppRefuses(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "ws")
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, tracks := range []string{`["SWE","SWE"]`, `["SWE",""]`} {
		if err := os.WriteFile(filepath.Join(workspace, "workspace.json"), []byte(`{"tracks":`+tracks+`}`), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(binary, "doctor", "--dir", workspace)
		cmd.Env = append(os.Environ(), "HOME="+home)
		out, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(out), "PROBLEM: "+filepath.Join(workspace, "workspace.json")) || !strings.Contains(string(out), "tracks:") {
			t.Fatalf("tracks %s: want a PROBLEM, got %q %v", tracks, out, err)
		}
	}
}

// follow_up_days outside 1-90 is clamped, and doctor says so.
func TestCLIDoctorWarnsAboutFollowUpDaysOutOfRange(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	path := filepath.Join(home, ".config/rolecall/config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("follow_up_days = 500\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "WARNING: "+path+": follow_up_days = 500 is outside 1-90; using 90") {
		t.Fatalf("no range warning:\n%s", out)
	}
}

// The app opens with an invalid workspace.json (TestInvalidWorkspaceFileStillStarts), so
// doctor is where it fails: it names the file and the reason, and never a value.
func TestCLIDoctorReportsMalformedWorkspaceWithoutValues(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, "..").CombinedOutput(); err != nil {
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
	cmd := exec.Command(binary, "doctor")
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	message := string(out)
	if err == nil || !strings.Contains(message, "PROBLEM: "+workspaceConfig) || !strings.Contains(message, "unexpected end of JSON input") || strings.Contains(message, "private-sentinel") {
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
	if err != nil || !strings.Contains(string(out), workspace+" (directory)") || !strings.Contains(string(out), "chmod 700 "+workspace) || !strings.Contains(string(out), "Template in use: Header Band (default)") || !strings.Contains(string(out), "#make-your-documents-yours") {
		t.Fatalf("doctor output = %q, err = %v", out, err)
	}
	for _, check := range []string{"typst", "pandoc", "Python 3", "Plugin path", "Harness command"} {
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
	if !strings.Contains(string(out), "Plugin path") {
		t.Errorf("doctor stopped before its setup checks: %q", out)
	}
}

func buildRolecall(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-o", binary, "..").CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", out, err)
	}
	return binary
}

func TestCLIInitCreatesPrivateWorkspaceAndLocalGitRepo(t *testing.T) {
	binary := buildRolecall(t)
	workspace := filepath.Join(t.TempDir(), "private")
	home := t.TempDir()
	cmd := exec.Command(binary, "init", workspace) // reads no input: no terminal needed
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %s %v", out, err)
	}
	for _, text := range []string{workspace, "Harness", "create-profile", "private", "run `rolecall`"} {
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
	// Contact is asked for by the app's first-run form, not by init.
	profile, err := s.Profile()
	if err != nil || len(profile.Missing) != 2 {
		t.Errorf("initialized Profile = %+v, %v; want Name and Email still missing", profile, err)
	}
	if settings, err := os.ReadFile(filepath.Join(workspace, "workspace.json")); err != nil || !strings.Contains(string(settings), `"tracks": []`) {
		t.Errorf("workspace settings = %q, %v; want no Tracks", settings, err)
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
	out, err := exec.Command(binary, "init", workspace).CombinedOutput()
	if err == nil || !strings.Contains(strings.ToLower(string(out)), "not empty") {
		t.Fatalf("init output = %q, error = %v; want clear non-empty refusal", out, err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("existing file was changed or removed: %v", err)
	}
}

// `rolecall --version` reports the build version injected with ldflags.
func TestCLIVersionFlagReportsBuildVersion(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "rolecall")
	if out, err := exec.Command("go", "build", "-ldflags", "-X main.version=v9.9.9", "-o", binary, "..").CombinedOutput(); err != nil {
		t.Fatalf("build: %s %v", out, err)
	}
	out, err := exec.Command(binary, "--version").CombinedOutput()
	if err != nil || string(out) != "rolecall v9.9.9\n" {
		t.Fatalf("--version output = %q, err = %v; want \"rolecall v9.9.9\\n\"", out, err)
	}
}

// A development build still reports a version, without a Workspace.
func TestCLIVersionFlagDefaultsWithoutInjectingLdflags(t *testing.T) {
	binary := buildRolecall(t)
	cmd := exec.Command(binary, "--version")
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || string(out) != "rolecall dev\n" {
		t.Fatalf("--version output = %q, err = %v; want \"rolecall dev\\n\"", out, err)
	}
}

// A Workspace copied with loose permissions is one summary line, not one per file; the
// Editor fallback and the documentation pointers are plain URLs; the exit status is 0.
func TestCLIDoctorCollapsesPermissionsAndWarnsOnViFallback(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	workspace := filepath.Join(home, "ws")
	for _, d := range []string{"a", "b", "c"} {
		if err := os.MkdirAll(filepath.Join(workspace, "jobs", d), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspace, "jobs", d, "f.md"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// doctor warns on the vi fallback only when vi is installed, so the test brings its own
	// stub vi and a PATH holding only that, whatever the machine has.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "vi"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, "doctor", "--dir", workspace)
	cmd.Env = append(os.Environ(), "HOME="+home, "VISUAL=", "EDITOR=", "PATH="+bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor failed: %v\n%s", err, out)
	}
	text := string(out)
	if strings.Count(text, "permissions 0") != 0 || !strings.Contains(text, "chmod -R go-rwx "+workspace) {
		t.Errorf("permission warnings not collapsed:\n%s", text)
	}
	if strings.Contains(text, "OK: Editor") || !strings.Contains(text, "WARNING: Editor") || !strings.Contains(text, "vi fallback") {
		t.Errorf("Editor fallback not a warning:\n%s", text)
	}
	if strings.Contains(text, "README") || !strings.Contains(text, "https://github.com/lukebrown14-code/rolecall/blob/main/docs/harnesses.md") {
		t.Errorf("README pointers remain:\n%s", text)
	}
}
