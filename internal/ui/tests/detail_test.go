package ui_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// detailHarness drives the whole program on a temp copy of the fixture Workspace.
// External programs are recorded by a fake runner instead of launched.
type detailHarness struct {
	t     *testing.T
	root  string
	tm    *driver
	store *store.Store
	mu    sync.Mutex
	runs  [][]string
}

func newDetailHarness(t *testing.T, setup func(root string), extra ...ui.Option) *detailHarness {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(root)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	h := &detailHarness{t: t, root: root, store: s}
	opts := append([]ui.Option{
		ui.WithEditor("myeditor"),
		ui.WithExternalRunner(func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
			return func() tea.Msg {
				h.mu.Lock()
				h.runs = append(h.runs, c.Args)
				h.mu.Unlock()
				return done(nil)
			}
		})}, extra...)
	h.tm = newDriver(t, root, fixtureClock, 120, 40, opts...)
	h.wait("Leads 7")
	return h
}

func (h *detailHarness) wait(text string) {
	h.t.Helper()
	if view := h.tm.plain(); !strings.Contains(view, text) {
		h.t.Fatalf("screen lacks %q:\n%s", text, view)
	}
}

// show returns the final rendered screen, failing unless it contains each wanted text
// once styling is stripped.
func (h *detailHarness) show(want ...string) string {
	h.t.Helper()
	v := h.tm.plain()
	for _, w := range want {
		if !strings.Contains(v, w) {
			h.t.Fatalf("screen lacks %q:\n%s", w, v)
		}
	}
	return v
}

func (h *detailHarness) key(code rune) { h.tm.key(code) }

// open selects the named Application's row (by text filter, on the given screen and
// table index) and opens its detail view with Enter.
func (h *detailHarness) open(screen string, tabs int, filter string) {
	h.t.Helper()
	h.tm.Type(screen)
	for i := 0; i < tabs; i++ {
		h.key(tea.KeyTab)
	}
	h.tm.Type("/" + filter)
	h.key(tea.KeyEnter)
	h.key(tea.KeyEnter)
}

func (h *detailHarness) finalView() string {
	h.t.Helper()
	return h.tm.View()
}

func (h *detailHarness) lastRun() []string {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.runs) == 0 {
		h.t.Fatal("no external program was run")
	}
	return h.runs[len(h.runs)-1]
}

func (h *detailHarness) folder(name string) string { return filepath.Join(h.root, "jobs", "SWE", name) }

func TestDetailHeaderWithMissingAppliedDate(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("1", 0, "Fern")
	h.show("Platform Engineer · Fern Systems", "Fern Systems · Platform")
}

func TestDetailHeaderApproximateAppliedDate(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("3", 2, "Engineer #1")
	h.show("Engineer #1 · Example", "Example · Unknown", "applied ~1 Jan 2025")
}

func TestDetailHeaderShowsCloseAndAppliedDates(t *testing.T) {
	h := newDetailHarness(t, func(root string) {
		meta := filepath.Join(root, "jobs", "SWE", "orbit", "meta.yml")
		b, _ := os.ReadFile(meta)
		os.WriteFile(meta, bytes.Replace(b, []byte("track: SWE\n"), []byte("track: SWE\ncloses: 2026-12-01\n"), 1), 0o600)
	})
	h.open("3", 1, "Orbit")
	// The card's subtitle is Company · Track; its Status line says the age.
	h.show("Orbit Labs · SWE", "● Applied · 6d in Applied", "applied 25 Sep 2026 · 6d ago")
}

func TestDetailJDRendersMarkdown(t *testing.T) {
	h := newDetailHarness(t, func(root string) {
		os.WriteFile(filepath.Join(root, "jobs", "SWE", "fern", "jd.md"), []byte("# Platform role\n\nBuild **reliable** systems.\n"), 0o600)
	})
	h.open("1", 0, "Fern")
	if v := h.show("Platform role", "Build reliable systems."); strings.Contains(v, "**") {
		t.Fatalf("Markdown not rendered:\n%s", v)
	}
}

func longJD(root string) {
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&b, "Requirement number %d\n\n", i)
	}
	os.WriteFile(filepath.Join(root, "jobs", "SWE", "fern", "jd.md"), []byte(b.String()), 0o600)
}

func TestDetailJDIsScrollable(t *testing.T) {
	top := newDetailHarness(t, longJD)
	top.open("1", 0, "Fern")
	if v := top.show("Requirement number 1"); strings.Contains(v, "Requirement number 100") {
		t.Fatalf("whole JD shown without scrolling:\n%s", v)
	}
	bottom := newDetailHarness(t, longJD)
	bottom.open("1", 0, "Fern")
	bottom.tm.Type("G")
	if v := bottom.show("Requirement number 100"); strings.Contains(v, "Requirement number 1\n") {
		t.Fatalf("scrolled view still shows the top:\n%s", v)
	}
	line := newDetailHarness(t, longJD)
	line.open("1", 0, "Fern")
	line.tm.Type("jjjjjjjj")
	if v := line.show("Requirement number"); strings.Contains(v, "Requirement number 1 ") {
		t.Fatalf("j did not scroll:\n%s", v)
	}
}

func TestDetailMissingFilesShowEmptyState(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tabs  int
		wants []string
	}{
		{"jd", 0, []string{"No JD saved · e to open jd.md"}},
		{"fit", 1, []string{"fit not assessed · no reasoning saved"}},
		{"notes", 2, []string{"No notes yet · e to write some"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newDetailHarness(t, nil)
			h.open("1", 0, "Fern")
			for i := 0; i < tc.tabs; i++ {
				h.key(tea.KeyRight)
			}
			h.show(tc.wants...)
		})
	}
}

func TestDetailNotesAndFitTabsRenderTheirFiles(t *testing.T) {
	setup := func(root string) {
		d := filepath.Join(root, "jobs", "SWE", "status-interviewing")
		os.WriteFile(filepath.Join(d, "notes.md"), []byte("Call **Tuesday**\n"), 0o600)
		os.WriteFile(filepath.Join(d, "fit.md"), []byte("Strong *platform* match\n"), 0o600)
	}
	notes := newDetailHarness(t, setup)
	notes.open("3", 2, "Engineer #1")
	notes.key(tea.KeyRight)
	notes.key(tea.KeyRight)
	if v := notes.show("Call Tuesday"); strings.Contains(v, "**") {
		t.Fatalf("notes not rendered:\n%s", v)
	}
	fit := newDetailHarness(t, setup)
	fit.open("3", 2, "Engineer #1")
	fit.key(tea.KeyRight)
	fit.show("fit Good", "Strong platform match")
	back := newDetailHarness(t, setup)
	back.open("3", 2, "Engineer #1")
	back.key(tea.KeyRight)
	back.key(tea.KeyLeft)
	back.key(tea.KeyLeft)
	back.show("Docs") // wraps from JD to the last tab
}

func TestDetailEditOpensCurrentTabFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		tabs int
		file string
	}{{"jd", 0, "jd.md"}, {"fit", 1, "fit.md"}, {"notes", 2, "notes.md"}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newDetailHarness(t, nil)
			h.open("1", 0, "Fern")
			for i := 0; i < tc.tabs; i++ {
				h.key(tea.KeyRight)
			}
			h.tm.Type("e")
			run := h.lastRun()
			if got, want := run[len(run)-1], filepath.Join(h.folder("fern"), tc.file); got != want {
				t.Fatalf("edited %q, want %q (args %q)", got, want, run)
			}
			if !strings.Contains(strings.Join(run, " "), "myeditor") {
				t.Fatalf("not run through the configured editor: %q", run)
			}
			h.finalView()
		})
	}
}

func docsTab(h *detailHarness) {
	h.key(tea.KeyLeft)
}

func TestDetailDocsTabListsDocumentSet(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("3", 4, "docs-ready")
	docsTab(h)
	h.show("○ JD", "✓ Resume", "✓ Cover letter", "○ Gaps", "missing", "gaps.md",
		"Owner Resume Role.pdf", "Owner Role Cover Letter.pdf", "Sent copy")
}

func TestDetailDocsTabOmitsSentWhenAbsent(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("3", 3, "docs-partial")
	docsTab(h)
	if v := h.show("✓ Resume", "Owner Resume Role.pdf"); strings.Contains(v, "Sent copy") {
		t.Fatalf("sent/ listed although absent:\n%s", v)
	}
}

func TestDetailDocsEditOpensMarkdownInEditor(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("3", 4, "docs-ready")
	docsTab(h)
	h.tm.Type("e")
	run := h.lastRun()
	if got, want := run[len(run)-1], filepath.Join(h.folder("docs-ready"), "jd.md"); got != want || !strings.Contains(strings.Join(run, " "), "myeditor") {
		t.Fatalf("e on .md: %q", run)
	}
	h.finalView()
}

func TestDetailDocsEnterOpensPDFInSystemViewer(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("3", 4, "docs-ready")
	docsTab(h)
	h.tm.Type("jjjj") // JD, Gaps, Resume, Cover letter → the built Resume PDF
	h.key(tea.KeyEnter)
	got := h.lastRun()
	want := expectedOpen(filepath.Join(h.folder("docs-ready"), "out", "Owner Resume Role.pdf"), false)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("Enter on PDF: got %q want %q", got, want)
	}
	h.finalView()
}

// Enter on a document not written yet opens it in the Editor, as e does: writing it is the
// next step, not an error.
func TestDetailDocsEnterOnAbsentMarkdownOpensEditor(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("3", 4, "docs-ready")
	docsTab(h)
	h.tm.Type("j") // JD → gaps.md, an input, is absent
	h.key(tea.KeyEnter)
	if len(h.runs) != 1 || !strings.Contains(strings.Join(h.runs[0], " "), "gaps.md") {
		t.Fatalf("ran %q, want the Editor on gaps.md", h.runs)
	}
	if v := h.tm.plain(); strings.Contains(v, "is missing") {
		t.Fatalf("an error for a normal action:\n%s", v)
	}
}

func TestDetailMoveUpdatesHeader(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("1", 0, "Fern")
	h.tm.Type("m")
	h.wait("Move")
	h.tm.Type("d")
	h.show("✎ Draft", "Moved to Draft")
}

func TestDetailUndoRestoresHeader(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("1", 0, "Fern")
	h.tm.Type("m")
	h.wait("Move")
	h.tm.Type("d")
	h.tm.Type("u")
	h.show("Undone · back to Lead")
	apps, _ := h.store.Scan()
	for _, a := range apps {
		if a.Title == "Platform Engineer" && (a.Status != "Lead" || len(a.StatusLog) != 3) {
			t.Fatalf("history: %+v", a.StatusLog)
		}
	}
}

func TestDetailNotesKeyEditsNotes(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("1", 0, "Fern")
	h.tm.Type("n")
	run := h.lastRun()
	if got, want := run[len(run)-1], filepath.Join(h.folder("fern"), "notes.md"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	h.finalView()
	if _, err := os.Stat(filepath.Join(h.folder("fern"), "notes.md")); err != nil {
		t.Fatalf("notes.md not created: %v", err)
	}
}

func TestDetailMalformedShowsPathAndError(t *testing.T) {
	h := newDetailHarness(t, nil)
	apps, _ := h.store.Scan()
	var bad store.Application
	for _, a := range apps {
		if a.Malformed != nil {
			bad = a
		}
	}
	h.tm.Type("G")
	h.key(tea.KeyEnter)
	// The pane says where and how to fix it, and the file's Workspace path.
	h.show("Can't read SWE/malformed", "meta.yml line 4: indented lines must be status_log entries",
		filepath.Join("jobs", "SWE", filepath.Base(bad.Folder), "meta.yml"), "e fix in editor")
}

func TestDetailEscReturnsToSameRow(t *testing.T) {
	h := newDetailHarness(t, nil)
	apps, err := h.store.Scan()
	if err != nil {
		t.Fatal(err)
	}
	var leads []store.Application
	for _, a := range apps {
		if a.Status == "Lead" {
			leads = append(leads, a)
		}
	}
	ui.SortApplications(leads, "Leads")
	h.tm.Type("jj")
	h.key(tea.KeyEnter)
	h.key(tea.KeyEscape)
	h.tm.Type("n")
	run := h.lastRun()
	if got, want := run[len(run)-1], filepath.Join(leads[2].Folder, "notes.md"); got != want {
		t.Fatalf("acted on %q, want third row %q", got, want)
	}
	h.show("Leads 7")
}

func TestDetailKeepsScrollPositionWhenEditorReturns(t *testing.T) {
	h := newDetailHarness(t, longJD)
	h.open("1", 0, "Fern")
	h.tm.Type("G")
	h.tm.Type("e")
	h.lastRun()
	h.tm.Type("x") // consumed by the detail view; also ensures the rescan message was handled
	if v := h.show("Requirement number 100"); strings.Contains(v, "Requirement number 1\n") {
		t.Fatalf("scroll position lost after the editor returned:\n%s", v)
	}
}

func TestDetailMoveToAppliedAsksWhenDocsNotReady(t *testing.T) {
	ask := newDetailHarness(t, nil)
	ask.open("2", 0, "docs-none")
	ask.tm.Type("m")
	ask.wait("Move")
	ask.tm.Type("a")
	ask.show("Docs not Ready")

	yes := newDetailHarness(t, nil)
	yes.open("2", 0, "docs-none")
	yes.tm.Type("m")
	yes.wait("Move")
	yes.tm.Type("ay")
	yes.show("Moved to Applied")
}

func TestResumeReader(t *testing.T) {
	h := newDetailHarness(t, nil)
	h.open("3", 0, "docs-written")
	h.show("[JD]")
	h.key(tea.KeyRight)
	h.key(tea.KeyRight)
	h.show("[Resume]", "Morgan Example")
	h.tm.Type("e")
	if got := h.lastRun(); !strings.Contains(strings.Join(got, " "), "myeditor") || !strings.HasSuffix(got[len(got)-1], "resume.md") {
		t.Fatalf("editor command: %v", got)
	}
	h.key(tea.KeyRight)
	h.show("[Cover letter]")
	h.key(tea.KeyRight)
	h.show("[Gaps]")
	h.key(tea.KeyRight)
	h.show("[Notes]")
	h.key(tea.KeyRight)
	h.show("[Docs]")
	h.key(tea.KeyRight)
	h.show("[JD]")
	h.key(tea.KeyLeft)
	h.show("[Docs]")
}

func TestResumeReaderScrollAndRefresh(t *testing.T) {
	h := newDetailHarness(t, func(root string) {
		text := "# Resume start\n\n" + strings.Repeat("Experience on fictional services.\n\n", 50) + "Resume end\n"
		if err := os.WriteFile(filepath.Join(root, "jobs", "SWE", "docs-written", "resume.md"), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	h.open("3", 0, "docs-written")
	h.key(tea.KeyRight)
	h.key(tea.KeyRight)
	h.show("Resume start", "↓ ")
	h.tm.Type("G")
	h.show("Resume end", "↑ ")
	if err := os.WriteFile(filepath.Join(h.folder("docs-written"), "resume.md"), []byte("# New start\n\n"+strings.Repeat("Updated fictional experience.\n\n", 50)+"Updated end\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.tm.Type("e")
	h.show("[Resume]", "Updated end")
	h.tm.Type("g")
	h.show("New start")
	// The next detail opens on the tab last chosen, at its top.
	h.key(tea.KeyEscape)
	h.key(tea.KeyEnter)
	h.show("[Resume]", "New start")
}

func TestResumeReaderHiddenWithoutText(t *testing.T) {
	for _, name := range []string{"docs-none", "fern"} {
		t.Run(name, func(t *testing.T) {
			h := newDetailHarness(t, nil)
			screen := "2"
			if name == "fern" {
				screen = "1"
			}
			h.open(screen, 0, name)
			h.key(tea.KeyRight)
			h.key(tea.KeyRight)
			h.show("[Notes]")
			h.key(tea.KeyRight)
			h.show("[Docs]")
			h.key(tea.KeyRight)
			h.show("[JD]")
		})
	}
}
