// Package process is rolecall's single boundary for starting external programs.
package process

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

// RunShell runs command with /bin/sh in dir, attached to the terminal. An empty dir
// keeps the current directory.
func RunShell(dir, command string) error {
	c := Command("/bin/sh", "-c", command)
	c.Dir = dir
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

func Exec(c *exec.Cmd, done tea.ExecCallback) tea.Cmd { return tea.ExecProcess(c, done) }

func RunCaptured(c *exec.Cmd, done func(CapturedResult) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		var stdout, stderr bytes.Buffer
		c.Stdout, c.Stderr = &stdout, &stderr
		err := c.Run()
		return done(CapturedResult{Stdout: stdout.String(), Stderr: stderr.String(), Err: err})
	}
}

// InitGit creates a local repository for a newly initialized Workspace.
func InitGit(root string) error {
	cmd := Command("git", "init", "--quiet", root)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}
