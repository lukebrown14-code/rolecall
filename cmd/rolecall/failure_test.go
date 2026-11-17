package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig puts a config.toml under a fresh HOME and returns the HOME.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if body != "" {
		path := filepath.Join(home, ".config", "rolecall", "config.toml")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

// A failing git is reported by name, and no config.toml is written for a Workspace
// that was not fully made.
func TestInitReportsGitFailureAndWritesNoConfig(t *testing.T) {
	home := writeConfig(t, "")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\necho no git today >&2\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	err := initCommand([]string{filepath.Join(t.TempDir(), "ws")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "git init failed: no git today") {
		t.Fatalf("err = %v, want the git failure", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "rolecall", "config.toml")); statErr == nil {
		t.Fatal("config.toml was written although init failed")
	}
}

func TestInitCreatesNothingWhenConfigDoesNotParse(t *testing.T) {
	writeConfig(t, "workspace = \n[[[\n")
	workspace := filepath.Join(t.TempDir(), "ws")
	err := initCommand([]string{workspace}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "init created nothing") {
		t.Fatalf("err = %v, want a refusal that creates nothing", err)
	}
	if _, statErr := os.Stat(workspace); statErr == nil {
		t.Fatal("a Workspace was created despite the broken config")
	}
}

func TestInitRefusesAnExistingWorkspaceWithoutSwitch(t *testing.T) {
	writeConfig(t, "")
	workspace := filepath.Join(t.TempDir(), "ws")
	if err := initCommand([]string{workspace}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	err := initCommand([]string{workspace}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "already a Workspace") || !strings.Contains(err.Error(), "--switch") {
		t.Fatalf("err = %v, want a refusal naming --switch", err)
	}
}

func TestInitFailsWhenTheTargetIsAFile(t *testing.T) {
	writeConfig(t, "")
	target := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := initCommand([]string{target}, io.Discard, io.Discard); err == nil {
		t.Fatal("init over a regular file should fail")
	}
}

func TestInitFailsWhenConfigCannotBeWritten(t *testing.T) {
	home := writeConfig(t, "")
	// A file where the config directory should be makes the config write fail.
	if err := os.WriteFile(filepath.Join(home, ".config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := initCommand([]string{filepath.Join(t.TempDir(), "ws")}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("init should fail when config.toml cannot be written")
	}
}

func TestRunReportsMissingOrBrokenSetupWithoutOpeningTheApp(t *testing.T) {
	for _, tc := range []struct {
		name, config, dir, want string
	}{
		{"no workspace", "", "", "No Workspace yet"},
		{"broken config", "workspace = \n[[[\n", "", "has an error"},
		{"workspace missing", "", filepath.Join(t.TempDir(), "gone"), "does not exist; check --dir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeConfig(t, tc.config)
			err := run(tc.dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestRunNamesTheConfigFileForAMissingConfiguredWorkspace(t *testing.T) {
	writeConfig(t, "workspace = \""+filepath.Join(t.TempDir(), "gone")+"\"\n")
	err := run("")
	if err == nil || !strings.Contains(err.Error(), "workspace in ") {
		t.Fatalf("err = %v", err)
	}
}

func TestDoctorFailsForAMissingWorkspaceAndPassesForAnInitializedOne(t *testing.T) {
	writeConfig(t, "")
	if err := runDoctor(filepath.Join(t.TempDir(), "gone")); err == nil {
		t.Fatal("doctor should fail for a Workspace that does not exist")
	}
	workspace := filepath.Join(t.TempDir(), "ws")
	if err := initCommand([]string{workspace}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := runDoctor(workspace); err != nil {
		t.Fatalf("doctor on a fresh Workspace: %v", err)
	}
}
