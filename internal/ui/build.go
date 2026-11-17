package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// dataFlag is the flag that names the Workspace to every Plugin script.
const dataFlag = "--data"

// buildable reports whether b can build an Application: a Lead or Draft whose
// Documents are Written or Ready (Ready is rebuilt).
func buildable(a store.Application) bool {
	if a.Malformed != nil {
		return false
	}
	if !a.Unsent() {
		return false
	}
	return a.Documents == store.DocsWritten || a.Documents == store.DocsReady
}

// buildState is the build in flight. Only one runs at a time. cv names a CV build
// (cv/<cv>.md); an Application build leaves it empty.
type buildState struct {
	folder, title, status, cv string
}

// buildFinished is a captured build's outcome, delivered to the model.
type buildFinished struct {
	folder, output, cv string
	err                error
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
	if !ok {
		return nil
	}
	// The missing Plugin hint comes before the buildable check: b on any row should say
	// how to turn the feature on, even when that row has nothing to build.
	if m.plugin == "" {
		m.err = errors.New(unsetMachineSetting(labelPlugin))
		return nil
	}
	if !buildable(a) {
		return nil
	}
	if m.refuseBusy() {
		return nil
	}
	m.capturedRun = true
	m.building = &buildState{folder: a.Folder, title: appName(a.Title), status: a.Status}
	return tea.Batch(m.build(a), m.spinner.Tick)
}

// refuseBusy reports a captured run already in flight, saying which kind; only one
// build or fetch runs at a time.
func (m *model) refuseBusy() bool {
	if !m.capturedRun {
		return false
	}
	m.toast, m.toastHint = "Build already running", ""
	if m.building == nil {
		m.toast = "A fetch is already running"
	}
	return true
}

// startCVBuild builds the selected CV on the Profile screen, with the same guards as an
// Application build: the Plugin must be set and only one build runs at a time.
func (m *model) startCVBuild(cv store.CV) tea.Cmd {
	if m.plugin == "" {
		m.err = errors.New(unsetMachineSetting(labelPlugin))
		return nil
	}
	if m.refuseBusy() {
		return nil
	}
	m.capturedRun = true
	m.building = &buildState{folder: m.store.Root(), title: cv.FileName(), cv: cv.Name}
	args := []string{filepath.Join(m.plugin, "scripts", "build.py"), "--cv", cv.Name, dataFlag, m.store.Root()}
	return tea.Batch(m.runCaptured("python3", args, func(r CapturedResult) tea.Msg {
		return buildFinished{folder: m.store.Root(), cv: cv.Name, output: combinedOutput(r), err: r.Err}
	}), m.spinner.Tick)
}

// finishCVBuild rereads the Profile so the CV shows its new PDF, and reports the build:
// the PDF's name on success, the build's own output in a pop-up on failure.
func (m model) finishCVBuild(done buildFinished) (tea.Model, tea.Cmd) {
	build := m.building
	m.building = nil
	m = m.loadProfile()
	if done.err != nil {
		m.overlay = &buildFailMode{failure: &buildFailure{title: buildTitle(build), output: done.output}}
		return m, nil
	}
	m.toast, m.toastHint = "Built "+done.cv+" CV", ""
	if m.profile != nil {
		for _, cv := range m.profile.profile.CVs {
			if cv.Name == done.cv && cv.PDF != "" {
				m.toast, m.toastHint = "Built "+cv.PDF, "d download"
			}
		}
	}
	return m, nil
}

// build runs the Plugin's build.py on the Application in captured mode.
func (m model) build(a store.Application) tea.Cmd {
	args := []string{filepath.Join(m.plugin, "scripts", "build.py"), a.Folder, dataFlag, m.store.Root()}
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
	if done.cv != "" {
		return m.finishCVBuild(done)
	}
	build := m.building
	m.building = nil
	// Rescan through the one reload helper. A failed scan is reported through m.err, but
	// it must not swallow the build's own outcome.
	m.reload(done.folder)
	var a *store.Application
	for i := range m.apps {
		if m.apps[i].Folder == done.folder {
			a = &m.apps[i]
			break
		}
	}
	if rows := m.rows(); m.cursor >= len(rows) {
		m.cursor = max(0, len(rows)-1)
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
	return m, nil
}

// buildTitle is the Application a build belonged to, for a failure pop-up.
func buildTitle(b *buildState) string {
	if b == nil {
		return ""
	}
	return b.title
}
