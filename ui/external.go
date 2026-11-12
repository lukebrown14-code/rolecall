package ui

import (
	"bytes"
	tea "charm.land/bubbletea/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/lukebrown14-code/rolecall/config"
)

// ExternalRunner substitutes the suspend/run/resume boundary in tests.
type ExternalRunner func(*exec.Cmd, tea.ExecCallback) tea.Cmd

// CapturedResult is a captured run's outcome: what it printed and how it ended.
type CapturedResult struct {
	Stdout string
	Stderr string
	Err    error
}

// CapturedRunner substitutes the background run in tests: it starts a command
// without suspending the TUI and reports its output through done.
type CapturedRunner func(*exec.Cmd, func(CapturedResult) tea.Msg) tea.Cmd

// ResolveEditor follows the Owner's configured editor preference.
func ResolveEditor(configured string) string {
	for _, s := range []string{configured, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if s != "" {
			return s
		}
	}
	return "vi"
}

// EditorOnPath reports whether the executable of an Editor command can be found: the
// first word, with surrounding quotes honored, looked up on PATH (or used directly
// when it contains a slash). Flags and later arguments are not checked. It is shared
// with `rolecall doctor`.
func EditorOnPath(command string) bool {
	exe := firstWord(command)
	if exe == "" {
		return false
	}
	_, err := exec.LookPath(exe)
	return err == nil
}

// firstWord returns the first shell word of command, unquoting '...' and "..." and
// treating a backslash as escaping the next character outside single quotes.
func firstWord(command string) string {
	var w []rune
	var quote rune
	started := false
	rs := []rune(command)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			} else {
				w = append(w, c)
			}
		case c == '\\' && i+1 < len(rs):
			i++
			w = append(w, rs[i])
		case quote == '"':
			if c == '"' {
				quote = 0
			} else {
				w = append(w, c)
			}
		case c == '\'' || c == '"':
			quote, started = c, true
		case c == ' ' || c == '\t':
			if started || len(w) > 0 {
				return string(w)
			}
		default:
			w = append(w, c)
			started = true
		}
	}
	return string(w)
}

type externalFinished struct {
	folder string
	err    error
}

// runExternal is the sole process launch boundary; completion always rescans.
func (m model) runExternal(folder, command string, args ...string) tea.Cmd {
	runner := m.external
	if runner == nil {
		runner = tea.ExecProcess
	}
	return runner(exec.Command(command, args...), func(err error) tea.Msg { return externalFinished{folder, err} })
}

// handoffError turns a nonzero Harness exit into the feedback line the Owner reads;
// every other error passes through unchanged.
func handoffError(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Errorf("Harness exited with status %d", exit.ExitCode())
	}
	return err
}

// runHandoff is the interactive boundary for a Handoff: the TUI steps aside while
// /bin/sh runs the Harness command, and the folder is rescanned when it exits. A
// nonzero exit is named for the feedback line.
func (m model) runHandoff(folder, command string) tea.Cmd {
	runner := m.external
	if runner == nil {
		runner = tea.ExecProcess
	}
	return runner(exec.Command("/bin/sh", "-c", command), func(err error) tea.Msg {
		return externalFinished{folder, handoffError(err)}
	})
}

// runCaptured is the other half of the process boundary: it runs a command in the
// background, without suspending, and keeps its output. The caller supplies done,
// which builds the message the model handles when the run ends.
func (m model) runCaptured(command string, args []string, done func(CapturedResult) tea.Msg) tea.Cmd {
	runner := m.captured
	if runner == nil {
		runner = runCapturedProcess
	}
	return runner(exec.Command(command, args...), done)
}

// runCapturedProcess is the default captured runner: no suspend, stdout, stderr
// and the exit status kept.
func runCapturedProcess(c *exec.Cmd, done func(CapturedResult) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		var stdout, stderr bytes.Buffer
		c.Stdout, c.Stderr = &stdout, &stderr
		err := c.Run()
		return done(CapturedResult{Stdout: stdout.String(), Stderr: stderr.String(), Err: err})
	}
}

// editFile opens a file of the Application folder in the Owner's editor. The editor
// preference may include flags or quoted executable paths, so the filename is passed
// as a shell positional argument and Application paths remain literal.
func (m model) editFile(folder, name string) tea.Cmd {
	return m.runExternal(folder, "/bin/sh", "-c", "exec "+ResolveEditor(m.editor)+" \"$1\"", "rolecall-editor", filepath.Join(folder, name))
}

func (m model) editNotes(folder string) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.EnsureNotes(folder); err != nil {
			return externalFinished{folder, err}
		}
		return m.editFile(folder, "notes.md")()
	}
}

const ()

// editConfig opens the config file in the Editor, creating it empty first if missing.
func (m model) editConfig() tea.Cmd {
	if m.configPath == "" {
		return nil
	}
	return func() tea.Msg {
		if err := config.Ensure(m.configPath); err != nil {
			return configEdited{err}
		}
		runner := m.external
		if runner == nil {
			runner = tea.ExecProcess
		}
		c := exec.Command("/bin/sh", "-c", "exec "+ResolveEditor(m.editor)+" \"$1\"", "rolecall-editor", m.configPath)
		return runner(c, func(err error) tea.Msg { return configEdited{err} })()
	}
}

// Option configures the external-program boundary.
type Option func(*model)

// WithExternalRunner injects a runner for external programs.
func WithExternalRunner(r ExternalRunner) Option { return func(m *model) { m.external = r } }

// WithCapturedRunner injects a runner for captured (background) programs.
func WithCapturedRunner(r CapturedRunner) Option { return func(m *model) { m.captured = r } }

// WithPlugin sets the path to the Plugin (rolecall/plugin); unset disables b.
func WithPlugin(path string) Option { return func(m *model) { m.plugin = path } }

// WithHarness sets the Harness command template for Handoff (f/p); unset hides them.
func WithHarness(template string) Option { return func(m *model) { m.harness = template } }

// WithEditor sets the configured editor command.
func WithEditor(editor string) Option { return func(m *model) { m.editor = editor } }

// WithEditorSource names where the effective Editor came from (config.EditorSource),
// for the Settings view.
func WithEditorSource(source string) Option { return func(m *model) { m.editorSource = source } }

// SettingSources records where each of the other effective settings came from, for the
// Settings view. An empty source is left unnamed.
type SettingSources struct{ Workspace, FollowUpDays, Downloads, Plugin, Harness string }

// WithSettingSources names where each effective setting came from.
func WithSettingSources(s SettingSources) Option { return func(m *model) { m.sources = s } }

// WithFollowUpDays sets the configured follow-up period.
func WithFollowUpDays(days int) Option { return func(m *model) { m.followUpDays = days } }

// WithWatch turns on live refresh: the Workspace's job folders are watched and
// changes are rescanned after a quiet window of the given length.
func WithWatch(window time.Duration) Option { return func(m *model) { m.watchWindow = window } }
