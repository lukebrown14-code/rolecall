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
	if !strings.Contains(out.String(), "run `rolecall`") {
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
