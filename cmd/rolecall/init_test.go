package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// init asks nothing: it reads no input at all, so it can never hang on a pipe or
// a missing terminal, and with no argument it creates ./rolecall.
func TestInitAsksNothingAndDefaultsToRolecallHere(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	t.Chdir(dir)
	var out bytes.Buffer
	if err := initCommand(nil, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(cwd, "rolecall", "workspace.json")); err != nil {
		t.Fatalf("no Workspace in ./rolecall: %v", err)
	}
	if !strings.Contains(out.String(), "Run `rolecall`") {
		t.Fatalf("init output does not point to rolecall:\n%s", out.String())
	}
}

func TestInitRejectsMoreThanOnePath(t *testing.T) {
	if err := initCommand([]string{"a", "b"}, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("init a b error = %v, want usage", err)
	}
}

// init writes the workspace setting through the same patcher the Settings screen uses,
// so a config with a table keeps the key above it instead of appending it inside.
func TestInitWritesWorkspaceSettingBeforeTables(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".config", "rolecall", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("[sweep]\ncommand = \"x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := initCommand([]string{workspace}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	key, table := strings.Index(string(got), "workspace = "), strings.Index(string(got), "[sweep]")
	if key < 0 || table < 0 || key > table {
		t.Fatalf("workspace key not written above the table:\n%s", got)
	}
	if !strings.Contains(string(got), "workspace = \""+workspace+"\"") {
		t.Fatalf("workspace value missing:\n%s", got)
	}
}

func TestWorkspacePathDefaultsBesideTheCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := defaultWorkspacePath(), filepath.Join(cwd, "rolecall"); got != want {
		t.Fatalf("default Workspace path = %q, want %q", got, want)
	}
}

func TestExpandHomeTurnsTildeIntoTheHomeDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	for in, want := range map[string]string{
		"~":           home,
		"~/rolecall":  filepath.Join(home, "rolecall"),
		"/abs/path":   "/abs/path",
		"~other/path": "~other/path",
		"rel/path":    "rel/path",
	} {
		if got := expandHome(in); got != want {
			t.Errorf("expandHome(%q) = %q, want %q", in, got, want)
		}
	}
}

// init's closing output leads with the next steps, ending at a Profile, and keeps the
// Harness privacy note for when a Harness is set.
func TestInitOutputListsNextStepsAndShowsHarnessNoteOnlyWithAHarness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	if err := initCommand([]string{filepath.Join(t.TempDir(), "ws")}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	next, private := strings.Index(got, "Next:"), strings.Index(got, "Keep this Workspace private")
	if next < 0 || private < 0 || next > private {
		t.Fatalf("Next steps do not come before the privacy note:\n%s", got)
	}
	for _, want := range []string{"Connect a Harness", "docs/harnesses/claude-code.md", "In your Harness, run the create-profile skill"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "AI service") {
		t.Errorf("Harness privacy note shown without a Harness:\n%s", got)
	}

	var withHarness bytes.Buffer
	writeInitDone(&withHarness, "/ws", "claude", "rolecall")
	if !strings.Contains(withHarness.String(), "AI service") || strings.Contains(withHarness.String(), "Connect a Harness") {
		t.Errorf("with a Harness, want the privacy note and no connect step:\n%s", withHarness.String())
	}
}

// A second init on the same folder says it is already a Workspace and how to open it.
func TestInitOnExistingWorkspaceSaysHowToOpenIt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := filepath.Join(t.TempDir(), "ws")
	if err := initCommand([]string{ws}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	err := initCommand([]string{ws}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "already a Workspace") || !strings.Contains(err.Error(), "rolecall --dir") {
		t.Fatalf("second init error = %v", err)
	}
}

// configText is config.toml under home.
func configText(t *testing.T, home string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(home, ".config", "rolecall", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A second init elsewhere creates the Workspace but keeps config.toml on the first one, which
// still holds the Owner's Applications, and says how to open or switch to the new one.
func TestInitElsewhereKeepsTheConfiguredWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	first, second := filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")
	if err := initCommand([]string{first}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := initCommand([]string{second}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(second, "workspace.json")); err != nil {
		t.Fatalf("second Workspace not created: %v", err)
	}
	if cfg := configText(t, home); !strings.Contains(cfg, first) || strings.Contains(cfg, second) {
		t.Fatalf("config.toml switched away from the first Workspace:\n%s", cfg)
	}
	q := ui.QuoteShellArgument(second)
	for _, want := range []string{"still opens your existing Workspace, " + first, "rolecall --dir " + q,
		"rolecall init --switch " + q, "Run `rolecall --dir " + q + "`"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output missing %q:\n%s", want, out.String())
		}
	}
}

// --switch makes the Workspace the default, before or after DIR, and works on an existing
// Workspace too.
func TestInitSwitchMakesTheWorkspaceTheDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	first, second := filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")
	if err := initCommand([]string{first}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := initCommand([]string{second, "--switch"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if cfg := configText(t, home); !strings.Contains(cfg, second) || strings.Contains(cfg, first) {
		t.Fatalf("--switch did not repoint config.toml:\n%s", cfg)
	}
	if !strings.Contains(out.String(), "previous Workspace is still at "+first) {
		t.Errorf("output does not name the previous Workspace:\n%s", out.String())
	}
	out.Reset()
	if err := initCommand([]string{"--switch", first}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if cfg := configText(t, home); !strings.Contains(cfg, first) {
		t.Fatalf("--switch to an existing Workspace did not repoint config.toml:\n%s", cfg)
	}
	if !strings.Contains(out.String(), "now opens "+first+" by default") {
		t.Errorf("output:\n%s", out.String())
	}
}

// A config.toml that does not parse is never patched: init says where it is broken and
// creates nothing.
func TestInitRefusesToPatchABrokenConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cfg := filepath.Join(home, ".config", "rolecall", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	broken := "editor = \"nvim\n"
	if err := os.WriteFile(cfg, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(t.TempDir(), "ws")
	err := initCommand([]string{ws}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), cfg) || !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error = %v", err)
	}
	if got := configText(t, home); got != broken {
		t.Fatalf("broken config.toml was changed:\n%s", got)
	}
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Fatalf("Workspace created despite the broken config: %v", err)
	}
}

// init writes the Profile skeleton: the Contact table and each expected heading with a
// how-to comment, so a Profile can be written by hand without a Harness.
func TestInitWritesProfileSkeleton(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ws")
	if err := initWorkspaceFiles(root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "profile", "master-profile.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Contact", "| Field | Value |", "## Headlines (approved per track)", "## Experience", "## Key Skills keyword pool", "## Summary", "## Facts that are NOT available", "<!-- "} {
		if !strings.Contains(string(b), want) {
			t.Errorf("master-profile.md lacks %q:\n%s", want, b)
		}
	}
}
