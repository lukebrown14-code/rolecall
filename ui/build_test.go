package ui_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

// fakeCapture stands in for the captured process runner: it records the commands
// it was asked to run and can write files as a real build.py would.
type fakeCapture struct {
	mu     sync.Mutex
	calls  [][]string
	block  chan struct{} // when set, the run waits for a close before finishing
	run    func(dir string) error
	output string
}

func (f *fakeCapture) runner(c *exec.Cmd, done func(ui.CapturedResult) tea.Msg) tea.Cmd {
	return func() tea.Msg {
		f.mu.Lock()
		f.calls = append(f.calls, append([]string(nil), c.Args...))
		f.mu.Unlock()
		if f.block != nil {
			<-f.block
		}
		var err error
		if f.run != nil {
			err = f.run(c.Args[2])
		}
		return done(ui.CapturedResult{Stdout: f.output, Err: err})
	}
}

// buildHarness drives the TUI on a temp copy of the fixture Workspace with the
// fake captured runner. A Draft and a Lead are given Written documents so b has
// something to build.
type buildHarness struct {
	t     *testing.T
	root  string
	store *store.Store
	tm    *teatest.TestModel
	fake  *fakeCapture
}

func newBuildHarness(t *testing.T, opts ...ui.Option) *buildHarness {
	t.Helper()
	h := &buildHarness{t: t, root: t.TempDir(), fake: &fakeCapture{}}
	if err := os.CopyFS(h.root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"status-draft", "fern"} {
		writeFile(t, filepath.Join(h.root, "jobs", "SWE", name, "resume.md"), "Draft text")
	}
	s, err := store.Open(h.root)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	base := []ui.Option{ui.WithCapturedRunner(h.fake.runner)}
	m, err := ui.New(s, fixtureClock, append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	h.tm = teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	return h
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writePDFs is what a successful build.py leaves behind.
func writePDFs(dir string) error {
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"Alex Quinn Resume Example Engineer.pdf", "Alex Quinn Cover Letter Example Engineer.pdf"} {
		if err := os.WriteFile(filepath.Join(out, name), []byte("%PDF-1.4\n"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

func (h *buildHarness) wait(texts ...string) {
	h.t.Helper()
	var seen []byte
	teatest.WaitFor(h.t, h.tm.Output(), func(b []byte) bool {
		seen = append(seen, b...)
		for _, text := range texts {
			if !bytes.Contains(seen, []byte(text)) {
				return false
			}
		}
		return true
	}, teatest.WithDuration(3*time.Second))
}

func (h *buildHarness) enter() { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter}) }
func (h *buildHarness) esc()   { h.tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape}) }

func (h *buildHarness) quit() string {
	h.t.Helper()
	h.tm.Type("q")
	return h.tm.FinalModel(h.t, teatest.WithFinalTimeout(3*time.Second)).View().Content
}

func TestBuildOfLeadMovesToDraftAndStaysPut(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	folder := filepath.Join(h.root, "jobs", "SWE", "fern")
	h.fake.run = func(dir string) error {
		if err := writePDFs(dir); err != nil {
			return err
		}
		err := h.store.AppendStatus(dir, "Draft", fixtureClock())
		return err
	}
	h.tm.Type("/fern")
	h.enter()
	h.wait("Platform Engineer")
	h.tm.Type("b")
	h.wait("Built 2 PDFs · moved to Draft")

	// u must not undo the build's own move to Draft.
	h.tm.Type("u")
	h.tm.Type("j")
	if a, err := h.store.Rescan(folder); err != nil || a.Status != "Draft" {
		t.Fatalf("u undid the build: %+v %v", a, err)
	}
	view := h.quit()
	if !strings.Contains(ansi.Strip(view), "1 Leads") {
		t.Fatalf("view left the Leads screen:\n%s", view)
	}
}

func TestBuildFailureShowsOutputPopup(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.output = "Gate: cover letter is 420 words, over the 350-word cap"
	h.fake.run = func(string) error { return errors.New("exit status 1") }
	h.tm.Type("/Engineer")
	h.enter()
	h.tm.Type("2")
	h.wait("Engineer #1")
	h.tm.Type("b")
	h.wait("Build failed", "over the 350-word cap")
	h.esc()
	view := h.quit()
	if strings.Contains(ansi.Strip(view), "over the 350-word cap") {
		t.Fatalf("popup still shown after esc:\n%s", view)
	}
}

func TestSecondBuildWhileRunningIsRefused(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.block = make(chan struct{})
	h.fake.run = writePDFs
	h.tm.Type("/Engineer")
	h.enter()
	h.tm.Type("2")
	h.wait("Engineer #1")
	h.tm.Type("b")
	h.wait("Building Engineer #1")
	h.tm.Type("b")
	h.wait("Build already running")
	h.tm.Type("1") // other keys keep working
	h.wait("1 Leads")
	close(h.fake.block)
	h.wait("Built 2 PDFs")
	h.quit()
}

func TestBuildAvailabilityByStatusAndDocuments(t *testing.T) {
	for _, tc := range []struct {
		name, filter, title, screen string
		want                        bool
	}{
		{"draft written", "/Engineer", "Engineer #1", "2", true},
		{"lead written", "/fern", "Platform Engineer", "1", true},
		{"draft nothing written", "/docs-none", "docs-none", "2", false},
		{"interviewing written", "/docs-written", "docs-written", "3", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newBuildHarness(t, ui.WithPlugin("/plugin"))
			h.tm.Type(tc.filter)
			h.enter()
			h.tm.Type(tc.screen)
			h.wait(tc.title)
			view := h.quit()
			if got := strings.Contains(ansi.Strip(view), "b build"); got != tc.want {
				t.Fatalf("b shown = %v, want %v:\n%s", got, tc.want, view)
			}
		})
	}
}

func TestBuildWithoutPluginShowsHint(t *testing.T) {
	h := newBuildHarness(t) // no plugin configured
	h.tm.Type("/Engineer")
	h.enter()
	h.tm.Type("2")
	h.wait("Engineer #1")
	h.tm.Type("b")
	h.wait("Set plugin in ~/.config/rolecall/config.toml")
	view := h.quit()
	if strings.Contains(ansi.Strip(view), "b build") {
		t.Fatalf("b shown without a plugin:\n%s", view)
	}
}

func TestBuildFromDetailView(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.run = writePDFs
	h.tm.Type("/Engineer")
	h.enter()
	h.tm.Type("2")
	h.wait("Engineer #1")
	h.enter() // open the detail view
	h.wait("JD", "Notes", "Docs")
	h.tm.Type("b")
	h.wait("Built 2 PDFs")
	h.quit()
	folder := filepath.Join(h.root, "jobs", "SWE", "status-draft")
	if a, err := h.store.Rescan(folder); err != nil || a.Documents != "Ready" {
		t.Fatalf("build from detail: %+v %v", a, err)
	}
}

func TestBuildSpinnerFallsBackToASCII(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.block = make(chan struct{})
	h.fake.run = writePDFs
	h.tm.Type("/Engineer")
	h.enter()
	h.tm.Type("2")
	h.wait("Engineer #1")
	h.tm.Type("b")
	var seen []byte
	teatest.WaitFor(t, h.tm.Output(), func(b []byte) bool {
		seen = append(seen, b...)
		return bytes.Contains(seen, []byte("Building Engineer #1"))
	}, teatest.WithDuration(3*time.Second))
	for _, g := range []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"} {
		if bytes.Contains(seen, []byte(g)) {
			t.Fatalf("Braille spinner frame %q survived ROLECALL_ASCII", g)
		}
	}
	close(h.fake.block)
	h.wait("Built 2 PDFs")
	h.quit()
}

func TestBuildReportsPDFsAndRefreshesDocs(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.run = writePDFs
	h.tm.Type("/Engineer")
	h.enter()
	h.tm.Type("2") // the Draft screen
	h.wait("Engineer #1")
	h.tm.Type("b")
	h.wait("Built 2 PDFs")
	view := h.quit()

	folder := filepath.Join(h.root, "jobs", "SWE", "status-draft")
	if a, err := h.store.Rescan(folder); err != nil || a.Documents != "Ready" || len(a.PDFs) != 2 {
		t.Fatalf("after build: %+v %v", a, err)
	}
	if !strings.Contains(ansi.Strip(view), "● ready") {
		t.Fatalf("Docs marker not refreshed:\n%s", view)
	}
	want := "python3 /plugin/scripts/build.py " + folder + " --data " + h.root
	if len(h.fake.calls) != 1 || strings.Join(h.fake.calls[0], " ") != want {
		t.Fatalf("build command = %v, want %q", h.fake.calls, want)
	}
}
