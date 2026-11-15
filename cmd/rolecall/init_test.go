package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err := os.WriteFile(cfg, []byte("[harness]\ncommand = \"x\"\n"), 0o600); err != nil {
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
	key, table := strings.Index(string(got), "workspace = "), strings.Index(string(got), "[harness]")
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
	for _, want := range []string{"Connect a Harness", "Connect your Harness", "In your Harness, run the create-profile skill"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "AI service") {
		t.Errorf("Harness privacy note shown without a Harness:\n%s", got)
	}

	var withHarness bytes.Buffer
	writeInitDone(&withHarness, "/ws", "claude")
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
