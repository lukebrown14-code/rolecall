package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/process"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// ExternalRunner substitutes the suspend/run/resume boundary in tests.
type ExternalRunner = process.ExternalRunner

// CapturedResult is a captured run's outcome: what it printed and how it ended.
type CapturedResult = process.CapturedResult

// CapturedRunner substitutes the background run in tests: it starts a command
// without suspending the TUI and reports its output through done.
type CapturedRunner = process.CapturedRunner

// ResolveEditor follows the Owner's configured editor preference.
func ResolveEditor(configured string) string { return config.ResolveEditor(configured) }

// editorOnPath reports whether the executable of an Editor command can be found: the
// first word, with surrounding quotes honored, looked up on PATH (or used directly
// when it contains a slash). Flags and later arguments are not checked.
func editorOnPath(command string) bool {
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
	toast  string // shown when the run succeeded
}

// runExternal is the sole process launch boundary; completion always rescans.
func (m model) runExternal(folder, command string, args ...string) tea.Cmd {
	runner := m.external
	if runner == nil {
		runner = process.Exec
	}
	return runner(process.Command(command, args...), func(err error) tea.Msg { return externalFinished{folder: folder, err: err} })
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
func (m model) runHandoff(folder, command, toast string) tea.Cmd {
	runner := m.external
	if runner == nil {
		runner = process.Exec
	}
	// The Harness starts in the Workspace, wherever rolecall was launched: the skills
	// read profile/ and write into jobs/, and a Harness treats its start folder as the
	// project it may edit without asking.
	c := process.Command("/bin/sh", "-c", command)
	c.Dir = m.store.Root()
	return runner(c, func(err error) tea.Msg {
		return externalFinished{folder: folder, err: handoffError(err), toast: toast}
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
	return runner(process.Command(command, args...), done)
}

// runCapturedProcess is the default captured runner: no suspend, stdout, stderr
// and the exit status kept.
func runCapturedProcess(c *exec.Cmd, done func(CapturedResult) tea.Msg) tea.Cmd {
	return process.RunCaptured(c, done)
}

// runEditor opens path in the Owner's Editor, at line when line > 0 and the Editor takes
// one. The editor preference may include flags or quoted executable paths, so the
// filename is passed as a shell positional argument and the path stays literal.
func (m model) runEditor(folder, path string, line int) tea.Cmd {
	editor := ResolveEditor(m.editor)
	before, after := editorLineArgs(firstWord(editor), line)
	return m.runExternal(folder, "/bin/sh", "-c", "exec "+editor+before+" \"$1\""+after, "rolecall-editor", path)
}

// editorLineArgs is how an Editor is told to open at a line: +N for the vi and Emacs
// families and nano, path:N for Sublime Text, Zed and Helix, -g path:N for VS Code.
// Any other Editor, or line 0, opens at the top.
func editorLineArgs(editor string, line int) (before, after string) {
	if line <= 0 {
		return "", ""
	}
	switch filepath.Base(editor) {
	case "vi", "vim", "nvim", "view", "nano", "pico", "emacs", "emacsclient", "micro", "kak", "joe", "mg", "ne":
		return fmt.Sprintf(" +%d", line), ""
	case "subl", "zed", "hx", "helix":
		return "", fmt.Sprintf(":%d", line)
	case "code", "codium", "cursor":
		return " -g", fmt.Sprintf(":%d", line)
	}
	return "", ""
}

// editFile opens a file of the Application folder in the Owner's editor.
func (m model) editFile(folder, name string) tea.Cmd {
	return m.runEditor(folder, filepath.Join(folder, name), 0)
}

// editFileAt is editFile at a line, for an Editor known to take one.
func (m model) editFileAt(folder, name string, line int) tea.Cmd {
	return m.runEditor(folder, filepath.Join(folder, name), line)
}

func (m model) editNotes(folder string) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.EnsureNotes(folder); err != nil {
			return externalFinished{folder: folder, err: err}
		}
		return m.editFile(folder, "notes.md")()
	}
}

// editConfig opens config.toml in the Editor, creating it empty first if missing. The
// Settings screen reloads when the Editor exits.
func (m model) editConfig() tea.Cmd {
	if m.configHome == "" {
		return nil
	}
	path, root := config.Path(m.configHome), m.store.Root()
	return func() tea.Msg {
		if err := config.Ensure(path); err != nil {
			return externalFinished{folder: root, err: err}
		}
		return m.runEditor(root, path, 0)()
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
type SettingSources = settings.Sources

// WithSettingSources names where each effective setting came from.
func WithSettingSources(s SettingSources) Option { return func(m *model) { m.sources = s } }

// WithFollowUpDays sets the configured follow-up period.
func WithFollowUpDays(days int) Option { return func(m *model) { m.followUpDays = days } }

// WithWatch turns on live refresh: the Workspace's job folders are watched and
// changes are rescanned after a quiet window of the given length.
func WithWatch(window time.Duration) Option { return func(m *model) { m.watchWindow = window } }

// openPath opens a document or reveals the downloads folder with the desktop launcher.
func (m model) openPath(folder, path string, reveal bool) tea.Cmd {
	command, args := desktopCommand(runtime.GOOS, path, reveal)
	return m.runExternal(folder, command, args...)
}

func desktopCommand(platform, path string, reveal bool) (string, []string) {
	switch platform {
	case "darwin":
		if reveal {
			return "open", []string{"-R", path}
		}
		return "open", []string{path}
	case "windows":
		return "rundll32.exe", []string{"url.dll,FileProtocolHandler", path}
	default:
		return "xdg-open", []string{path}
	}
}
