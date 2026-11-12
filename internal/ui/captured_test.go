package ui

import (
	"errors"
	"os/exec"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The default captured runner is the production process boundary: it must keep
// stdout, stderr and the exit status without suspending.
func TestRunCapturedProcessKeepsOutputAndExitStatus(t *testing.T) {
	var got CapturedResult
	cmd := runCapturedProcess(exec.Command("/bin/sh", "-c", "printf out; printf err >&2; exit 3"), func(r CapturedResult) tea.Msg {
		got = r
		return r
	})
	cmd()
	if got.Stdout != "out" || got.Stderr != "err" {
		t.Fatalf("stdout/stderr = %q / %q, want %q / %q", got.Stdout, got.Stderr, "out", "err")
	}
	var exit *exec.ExitError
	if !errors.As(got.Err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("exit = %v, want status 3", got.Err)
	}
}
