package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A dev build logs each run to the state directory, and `rolecall diagnostics` reads it
// back. Arguments (paths here) stay out of the log.
func TestCLIDiagnosticsLogsARunAndShowsIt(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home, "ROLECALL_DIAGNOSTICS=1")
	missing := filepath.Join(t.TempDir(), "no-such-workspace")
	run := exec.Command(binary, "--dir", missing)
	run.Env = env
	if out, err := run.CombinedOutput(); err == nil {
		t.Fatalf("opening a missing Workspace succeeded: %s", out)
	}
	logPath := filepath.Join(home, ".local", "state", "rolecall", "diagnostics.jsonl")
	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("no log written: %v", err)
	}
	for _, want := range []string{`"msg":"session_start"`, `"command":"tui"`, `"msg":"run_failed"`, `"msg":"session_end","exit":1`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("log missing %s:\n%s", want, raw)
		}
	}

	show := exec.Command(binary, "diagnostics", "-n", "2")
	show.Env = env
	out, err := show.CombinedOutput()
	if err != nil {
		t.Fatalf("diagnostics failed: %s %v", out, err)
	}
	text := string(out)
	if !strings.Contains(text, "Diagnostics on · "+logPath) || !strings.Contains(text, "ERROR run_failed") || !strings.Contains(text, "session_end exit=1") {
		t.Errorf("diagnostics output:\n%s", text)
	}
	if strings.Contains(text, "session_start") {
		t.Errorf("-n 2 showed more than the last two entries:\n%s", text)
	}
}

// With the log off, nothing is written and diagnostics says how to turn it on.
func TestCLIDiagnosticsOffWritesNothing(t *testing.T) {
	binary := buildRolecall(t)
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home, "ROLECALL_DIAGNOSTICS=0")
	run := exec.Command(binary, "--version")
	run.Env = env
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "state")); !os.IsNotExist(err) {
		t.Errorf("state directory created with the log off: %v", err)
	}
	show := exec.Command(binary, "diagnostics")
	show.Env = env
	out, _ := show.CombinedOutput()
	if !strings.Contains(string(out), "off (set ROLECALL_DIAGNOSTICS=1") || !strings.Contains(string(out), "No entries yet.") {
		t.Errorf("diagnostics output:\n%s", out)
	}
}
