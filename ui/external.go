package ui

import (
	tea "charm.land/bubbletea/v2"
	"os"
	"os/exec"
	"path/filepath"
)

// ExternalRunner substitutes the suspend/run/resume boundary in tests.
type ExternalRunner func(*exec.Cmd, tea.ExecCallback) tea.Cmd

// ResolveEditor follows the Owner's configured editor preference.
func ResolveEditor(configured string) string {
	for _, s := range []string{configured, os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if s != "" {
			return s
		}
	}
	return "vi"
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

// Option configures the external-program boundary.
type Option func(*model)

// WithExternalRunner injects a runner for external programs.
func WithExternalRunner(r ExternalRunner) Option { return func(m *model) { m.external = r } }

// WithEditor sets the configured editor command.
func WithEditor(editor string) Option { return func(m *model) { m.editor = editor } }

// WithFollowUpDays sets the configured follow-up period.
func WithFollowUpDays(days int) Option { return func(m *model) { m.followUpDays = days } }
