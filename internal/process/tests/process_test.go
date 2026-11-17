package tests

import (
	tea "charm.land/bubbletea/v2"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/process"
)

func TestDetectWSL(t *testing.T) {
	for _, tc := range []struct {
		name, distro, version string
		want                  bool
	}{
		{"distro name set", "Ubuntu", "", true},
		{"microsoft kernel", "", "Linux version 5.15.1-microsoft-standard-WSL2", true},
		{"mixed case", "", "Linux version 4.4.0-Microsoft", true},
		{"plain linux", "", "Linux version 6.1.0-generic", false},
		{"nothing", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := process.DetectWSL(tc.distro, tc.version); got != tc.want {
				t.Fatalf("DetectWSL(%q, %q) = %v, want %v", tc.distro, tc.version, got, tc.want)
			}
		})
	}
}

// fakeBin puts executable shell scripts on an otherwise empty PATH.
func fakeBin(t *testing.T, scripts map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range scripts {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("WAYLAND_DISPLAY", "")
	return dir
}

func TestOnPath(t *testing.T) {
	fakeBin(t, map[string]string{"present-tool": "exit 0"})
	if !process.OnPath("present-tool") || process.OnPath("absent-tool") {
		t.Fatal("OnPath did not tell an installed program from a missing one")
	}
}

func TestCopyToClipboardWithoutAProgramReturnsErrNoClipboard(t *testing.T) {
	fakeBin(t, nil)
	if err := process.CopyToClipboard("x"); !errors.Is(err, process.ErrNoClipboard) {
		t.Fatalf("err = %v, want ErrNoClipboard", err)
	}
}

func TestCopyToClipboardSendsTextToTheFirstProgram(t *testing.T) {
	dir := fakeBin(t, nil)
	out := filepath.Join(dir, "captured")
	// Both X programs exist on Linux and pbcopy on macOS; whichever applies records stdin.
	for _, name := range []string{"pbcopy", "xclip", "xsel"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\ncat > "+out+"\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// The shell script needs cat, which PATH no longer has.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/bin:/usr/bin")
	if err := process.CopyToClipboard("hello clip"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(out)
	if err != nil || string(got) != "hello clip" {
		t.Fatalf("clipboard got %q (%v)", got, err)
	}
}

func TestCopyToClipboardReportsAFailingProgram(t *testing.T) {
	dir := fakeBin(t, nil)
	for _, name := range []string{"pbcopy", "xclip", "xsel"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho no display >&2\nexit 3\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	err := process.CopyToClipboard("x")
	if err == nil || errors.Is(err, process.ErrNoClipboard) || !strings.Contains(err.Error(), "no display") {
		t.Fatalf("err = %v, want the program's own message", err)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("err = %v, want it to wrap the exit error", err)
	}
}

func TestInitGitCreatesARepository(t *testing.T) {
	if !process.OnPath("git") {
		t.Skip("git is not installed")
	}
	root := filepath.Join(t.TempDir(), "ws")
	if err := process.InitGit(root); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, ".git")); err != nil || !info.IsDir() {
		t.Fatalf(".git missing: %v", err)
	}
}

func TestInitGitFailureNamesGit(t *testing.T) {
	fakeBin(t, map[string]string{"git": "echo boom >&2; exit 1"})
	err := process.InitGit(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "git init failed: boom") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitGitWithoutGitFails(t *testing.T) {
	fakeBin(t, nil)
	if err := process.InitGit(t.TempDir()); err == nil {
		t.Fatal("expected an error when git is not installed")
	}
}

func TestRunCapturedReturnsOutputAndError(t *testing.T) {
	fakeBin(t, map[string]string{"prog": "echo out; echo err >&2; exit 2"})
	cmd := process.RunCaptured(process.Command("prog"), func(r process.CapturedResult) tea.Msg { return r })
	got := cmd().(process.CapturedResult)
	if got.Stdout != "out\n" || got.Stderr != "err\n" || got.Err == nil {
		t.Fatalf("got %+v", got)
	}
}
