package ui_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// fakeBuild stands in for the captured process runner: it records the commands it was
// asked to run and can write files as a real build.py would. A blocked run is held as a
// pending callback so the test can inspect the model while the build is in flight.
type fakeBuild struct {
	calls    [][]string
	pending  []func() tea.Msg
	block    bool
	run      func(dir string) error
	output   string
	fetch    string
	fetchErr error
}

func (f *fakeBuild) runner(c *exec.Cmd, done func(ui.CapturedResult) tea.Msg) tea.Cmd {
	f.calls = append(f.calls, append([]string(nil), c.Args...))
	dir := c.Args[2]
	isFetch := dir == "--fetch-only"
	finish := func() tea.Msg {
		if isFetch {
			return done(ui.CapturedResult{Stdout: f.fetch, Err: f.fetchErr})
		}
		var err error
		if f.run != nil {
			err = f.run(dir)
		}
		return done(ui.CapturedResult{Stdout: f.output, Err: err})
	}
	return func() tea.Msg {
		if f.block {
			f.pending = append(f.pending, finish)
			return nil
		}
		return finish()
	}
}

// buildHarness drives the model on a temp copy of the fixture Workspace with the fake
// captured runner. A Draft and a Lead are given Written documents so b has something to
// build.
type buildHarness struct {
	t     *testing.T
	root  string
	store *store.Store
	m     tea.Model
	fake  *fakeBuild
}

func newBuildHarness(t *testing.T, opts ...ui.Option) *buildHarness {
	t.Helper()
	h := &buildHarness{t: t, root: t.TempDir(), fake: &fakeBuild{}}
	if err := os.CopyFS(h.root, os.DirFS("../../../fixtures/workspace")); err != nil {
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
	h.m = driveStart(t, h.root, fixtureClock, 120, 40, append(base, opts...)...)
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

// send feeds one message to the model and runs its commands, dropping spinner ticks:
// they animate the frame but are not behaviour under test, and replaying them would
// spin forever.
func (h *buildHarness) send(msg tea.Msg) {
	if msg == nil {
		return
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			h.sendCmd(c)
		}
		return
	}
	next, cmd := h.m.Update(msg)
	h.m = next
	h.sendCmd(cmd)
}

func (h *buildHarness) sendCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if _, ok := msg.(spinner.TickMsg); ok {
		return
	}
	h.send(msg)
}

func (h *buildHarness) key(k string) {
	for _, msg := range keyMessages(k) {
		h.send(msg)
	}
}

func (h *buildHarness) plain() string { return chipped(h.m.View().Content) }

func (h *buildHarness) wait(texts ...string) {
	h.t.Helper()
	view := h.plain()
	for _, text := range texts {
		if !strings.Contains(view, text) {
			h.t.Fatalf("screen lacks %q:\n%s", text, view)
		}
	}
}

// release completes the one blocked build.
func (h *buildHarness) release() {
	h.t.Helper()
	if len(h.fake.pending) == 0 {
		h.t.Fatal("no build pending")
	}
	finish := h.fake.pending[0]
	h.fake.pending = h.fake.pending[1:]
	h.send(finish())
}

func TestBuildOfLeadMovesToDraftAndStaysPut(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	folder := filepath.Join(h.root, "jobs", "SWE", "fern")
	h.fake.run = func(dir string) error {
		if err := writePDFs(dir); err != nil {
			return err
		}
		return h.store.AppendStatus(dir, "Draft", fixtureClock())
	}
	h.key("/fern")
	h.key("enter")
	h.wait("Platform Engineer")
	h.key("b")
	h.wait("Built 2 PDFs · moved to Draft")

	// u must not undo the build's own move to Draft.
	h.key("u")
	h.key("j")
	if a, err := h.store.Rescan(folder); err != nil || a.Status != "Draft" {
		t.Fatalf("u undid the build: %+v %v", a, err)
	}
	if view := h.plain(); !strings.Contains(view, "1 Leads") {
		t.Fatalf("view left the Leads screen:\n%s", view)
	}
}

func TestBuildFailureShowsOutputPopup(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.output = "Gate: cover letter is 420 words, over the 350-word cap"
	h.fake.run = func(string) error { return errors.New("exit status 1") }
	h.key("/Engineer")
	h.key("enter")
	h.key("2")
	h.wait("Engineer #1")
	h.key("b")
	h.wait("Build failed", "over the 350-word cap")
	h.key("esc")
	if view := h.plain(); strings.Contains(view, "over the 350-word cap") {
		t.Fatalf("popup still shown after esc:\n%s", view)
	}
}

func TestSecondBuildWhileRunningIsRefused(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.block = true
	h.fake.run = writePDFs
	h.key("/Engineer")
	h.key("enter")
	h.key("2")
	h.wait("Engineer #1")
	h.key("b")
	h.wait("Building Engineer #1")
	h.key("b")
	h.wait("Build already running")
	h.key("1") // other keys keep working
	h.wait("1 Leads")
	h.release()
	h.wait("Built 2 PDFs")
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
			h.key(tc.filter)
			h.key("enter")
			h.key(tc.screen)
			h.wait(tc.title)
			if got := strings.Contains(h.plain(), "b build"); got != tc.want {
				t.Fatalf("b shown = %v, want %v:\n%s", got, tc.want, h.plain())
			}
		})
	}
}

func TestBuildWithoutPluginShowsHint(t *testing.T) {
	h := newBuildHarness(t) // no plugin configured
	h.key("/docs-none")
	h.key("enter")
	h.key("2")
	h.wait("docs-none")
	h.key("b")
	h.wait("No Plugin set · 6 Settings › This machine › Plugin")
	if view := h.plain(); strings.Contains(view, "b build") {
		t.Fatalf("b shown without a plugin:\n%s", view)
	}
}

func TestBuildFromDetailView(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.run = writePDFs
	h.key("/Engineer")
	h.key("enter")
	h.key("2")
	h.wait("Engineer #1")
	h.key("enter") // open the detail view
	h.wait("JD", "Notes", "Docs")
	h.key("b")
	h.wait("Built 2 PDFs")
	folder := filepath.Join(h.root, "jobs", "SWE", "status-draft")
	if a, err := h.store.Rescan(folder); err != nil || a.Documents != "Ready" {
		t.Fatalf("build from detail: %+v %v", a, err)
	}
}

func TestBuildSpinnerFallsBackToASCII(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.block = true
	h.fake.run = writePDFs
	h.key("/Engineer")
	h.key("enter")
	h.key("2")
	h.wait("Engineer #1")
	h.key("b")
	view := h.plain()
	if !strings.Contains(view, "Building Engineer #1") {
		t.Fatalf("build spinner missing:\n%s", view)
	}
	for _, g := range []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"} {
		if strings.Contains(view, g) {
			t.Fatalf("Braille spinner frame %q survived ROLECALL_ASCII:\n%s", g, view)
		}
	}
	h.release()
	h.wait("Built 2 PDFs")
}

func TestBuildReportsPDFsAndRefreshesDocs(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.run = writePDFs
	h.key("/Engineer")
	h.key("enter")
	h.key("2") // the Draft screen
	h.wait("Engineer #1")
	h.key("b")
	h.wait("Built 2 PDFs")
	view := h.plain()

	folder := filepath.Join(h.root, "jobs", "SWE", "status-draft")
	if a, err := h.store.Rescan(folder); err != nil || a.Documents != "Ready" || len(a.PDFs) != 2 {
		t.Fatalf("after build: %+v %v", a, err)
	}
	if !strings.Contains(view, "✓ ready") {
		t.Fatalf("Docs marker not refreshed:\n%s", view)
	}
	want := "python3 /plugin/scripts/build.py " + folder + " --data " + h.root
	if len(h.fake.calls) != 1 || strings.Join(h.fake.calls[0], " ") != want {
		t.Fatalf("build command = %v, want %q", h.fake.calls, want)
	}
}

// One captured run at a time is global: a build waits for a fetch, even after the Add
// form that started the fetch has been closed.
func TestBuildRefusedWhileFetchRuns(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.block = true
	h.fake.run = writePDFs
	h.fake.fetch = `{"jd":"Fetched text"}`
	h.key("a")
	h.wait("Add a Lead")
	h.key("https://example.test/role")
	h.key("enter")
	h.wait("Fetching")
	h.key("esc") // close the form while the fetch is still running
	h.wait("Discard changes?")
	h.key("y")
	h.key("/Engineer")
	h.key("enter")
	h.key("2")
	h.wait("Engineer #1")
	h.key("b")
	h.wait("A fetch is already running")
	h.release() // the closed form drops the fetch result, but the run is over
	h.key("b")
	h.wait("Building Engineer #1")
	h.release()
	h.wait("Built 2 PDFs")
}

// The same guard covers the other direction: Enter on the Add form's URL is refused
// while a build is in flight, and no fetch starts.
func TestFetchRefusedWhileBuildRuns(t *testing.T) {
	h := newBuildHarness(t, ui.WithPlugin("/plugin"))
	h.fake.block = true
	h.fake.run = writePDFs
	h.fake.fetch = `{"jd":"Fetched text"}`
	h.key("/Engineer")
	h.key("enter")
	h.key("2")
	h.wait("Engineer #1")
	h.key("b")
	h.wait("Building Engineer #1")
	h.key("a")
	h.wait("Add a Lead")
	h.key("https://example.test/role")
	h.key("enter")
	h.wait("A build is already running")
	if len(h.fake.calls) != 1 {
		t.Fatalf("fetch started during a build: %v", h.fake.calls)
	}
	h.release()
	h.wait("Built 2 PDFs")
}
