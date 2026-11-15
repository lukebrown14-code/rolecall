// Package process is rolecall's single boundary for starting external programs.
package process

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type ExternalRunner func(*exec.Cmd, tea.ExecCallback) tea.Cmd

type CapturedResult struct {
	Stdout string
	Stderr string
	Err    error
}

type CapturedRunner func(*exec.Cmd, func(CapturedResult) tea.Msg) tea.Cmd

func Command(name string, args ...string) *exec.Cmd { return exec.Command(name, args...) }

func Exec(c *exec.Cmd, done tea.ExecCallback) tea.Cmd { return tea.ExecProcess(c, done) }

func RunCaptured(c *exec.Cmd, done func(CapturedResult) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		var stdout, stderr bytes.Buffer
		c.Stdout, c.Stderr = &stdout, &stderr
		err := c.Run()
		return done(CapturedResult{Stdout: stdout.String(), Stderr: stderr.String(), Err: err})
	}
}

// ErrNoClipboard means no clipboard program was found for this system.
var ErrNoClipboard = errors.New("no clipboard program found (pbcopy, wl-copy, xclip or xsel)")

// clipboardCommands are the programs that take text on stdin and put it on the system
// clipboard, in the order they are tried. Wayland's comes first where it applies.
func clipboardCommands(goos string, wayland bool) [][]string {
	if goos == "darwin" {
		return [][]string{{"pbcopy"}}
	}
	var cmds [][]string
	if wayland {
		cmds = append(cmds, []string{"wl-copy"})
	}
	return append(cmds, []string{"xclip", "-selection", "clipboard"}, []string{"xsel", "--clipboard", "--input"})
}

// CopyToClipboard puts text on the system clipboard with the first clipboard program
// installed, or returns ErrNoClipboard.
func CopyToClipboard(text string) error {
	for _, argv := range clipboardCommands(runtime.GOOS, os.Getenv("WAYLAND_DISPLAY") != "") {
		if _, err := exec.LookPath(argv[0]); err != nil {
			continue
		}
		c := Command(argv[0], argv[1:]...)
		c.Stdin = strings.NewReader(text)
		if output, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("%s failed: %s: %w", argv[0], strings.TrimSpace(string(output)), err)
		}
		return nil
	}
	return ErrNoClipboard
}

// InitGit creates a local repository for a newly initialized Workspace.
func InitGit(root string) error {
	cmd := Command("git", "init", "--quiet", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}
