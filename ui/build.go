package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// buildable reports whether b can build an Application: a Lead or Draft whose
// Documents are Written or Ready (Ready is rebuilt).
func buildable(a store.Application) bool {
	if a.Malformed != nil {
		return false
	}
	if a.Status != store.StatusLead && a.Status != store.StatusDraft {
		return false
	}
	return a.Documents == store.DocsWritten || a.Documents == store.DocsReady
}

// buildState is the build in flight. Only one runs at a time.
type buildState struct {
	folder, title, status string
}

// buildFinished is a captured build's outcome, delivered to the model.
type buildFinished struct {
	folder, output string
	err            error
}

// buildFailure is a failed build's output, shown in a pop-up until esc.
type buildFailure struct {
	title, output string
}

// startBuild validates a build request and, when it can, starts it. A missing
// Plugin names the setting; a build already in flight is refused and leaves the
// other keys working.
func (m *model) startBuild() tea.Cmd {
	a, ok := m.current()
	if !ok || !buildable(a) {
		return nil
	}
	if m.plugin == "" {
		m.err = errors.New("Set plugin in ~/.config/rolecall/config.toml")
		return nil
	}
	if m.building != nil {
		m.toast, m.toastHint = "Build already running", ""
		return nil
	}
	m.building = &buildState{folder: a.Folder, title: a.Title, status: a.Status}
	return tea.Batch(m.build(a), m.spinner.Tick)
}

// build runs the Plugin's build.py on the Application in captured mode.
func (m model) build(a store.Application) tea.Cmd {
	args := []string{filepath.Join(m.plugin, "scripts", "build.py"), a.Folder, "--data", m.store.Root()}
	return m.runCaptured("python3", args, func(r CapturedResult) tea.Msg {
		return buildFinished{folder: a.Folder, output: combinedOutput(r), err: r.Err}
	})
}

// combinedOutput is a captured run's stdout then stderr, blank lines trimmed.
func combinedOutput(r CapturedResult) string {
	out := strings.TrimSpace(r.Stdout)
	if e := strings.TrimSpace(r.Stderr); e != "" {
		if out != "" {
			out += "\n"
		}
		out += e
	}
	return out
}

// finishBuild rescans the folder and reports the build: a toast naming the PDFs
// on success, a pop-up with the build's own output on failure. A Lead the build
// moved to Draft says so; that move is never the Owner's, so it is not undoable.
func (m model) finishBuild(done buildFinished) (tea.Model, tea.Cmd) {
	build := m.building
	m.building = nil
	a, err := m.store.Rescan(done.folder)
	if err != nil {
		m.err = err
		return m, nil
	}
	if a != nil {
		for i := range m.apps {
			if m.apps[i].Folder == done.folder {
				m.apps[i] = *a
				break
			}
		}
	}
	if done.err != nil {
		m.overlay = &buildFailMode{failure: &buildFailure{title: buildTitle(build), output: done.output}}
		return m, nil
	}
	n := 0
	if a != nil {
		n = len(a.PDFs)
	}
	msg := fmt.Sprintf("Built %d PDFs", n)
	if build != nil && build.status == store.StatusLead && a != nil && a.Status == store.StatusDraft {
		msg += " · moved to Draft"
	}
	m.toast, m.toastHint = msg, ""
	if rows := m.rows(); m.cursor >= len(rows) {
		m.cursor = max(0, len(rows)-1)
	}
	return m, nil
}

// buildTitle is the Application a build belonged to, for a failure pop-up.
func buildTitle(b *buildState) string {
	if b == nil {
		return ""
	}
	return b.title
}
