package ui_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

// liveHarness drives the whole program with live refresh on, over a temp copy of
// the fixture Workspace. Tests play the agent by writing files under root.
type liveHarness struct {
	t    *testing.T
	root string
	tm   *teatest.TestModel
}

func newLiveHarness(t *testing.T, setup func(root string)) *liveHarness {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(root)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, fixtureClock, ui.WithWatch(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	h := &liveHarness{t: t, root: root, tm: teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))}
	h.wait("Leads 4")
	return h
}

// wait blocks until text appears in the program's output. It polls with a generous
// deadline rather than sleeping for a guessed refresh time.
func (h *liveHarness) wait(text string) {
	h.t.Helper()
	teatest.WaitFor(h.t, h.tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(text)) },
		teatest.WithDuration(10*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
}

func (h *liveHarness) key(code rune) { h.tm.Send(tea.KeyPressMsg{Code: code}) }

// rowLine is the first line of the row whose title starts with title.
func rowLine(view, title string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "  "+title) || strings.Contains(line, "▌ "+title) {
			return line
		}
	}
	return ""
}

// finalView quits and returns the last rendered screen without styling.
func (h *liveHarness) finalView() string {
	h.t.Helper()
	h.tm.Type("q")
	return ansi.Strip(h.tm.FinalModel(h.t, teatest.WithFinalTimeout(5*time.Second)).View().Content)
}

func (h *liveHarness) folder(name string) string { return filepath.Join(h.root, "jobs", "SWE", name) }

func (h *liveHarness) write(path, content string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

// appendMeta plays an agent adding a key to an existing meta.yml.
func (h *liveHarness) addMetaKey(folder, line string) {
	h.t.Helper()
	path := filepath.Join(folder, "meta.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatal(err)
	}
	h.write(path, strings.Replace(string(b), "status_log:", line+"\nstatus_log:", 1))
}

func TestLiveFitWrittenToMetaUpdatesRow(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.addMetaKey(h.folder("fern"), "fit: Strong")
	h.wait("Strong")
	if v := h.finalView(); !strings.Contains(rowLine(v, "Platform Engineer"), "Strong") {
		t.Fatalf("Fern row did not show its new Fit:\n%s", v)
	}
}

func TestLiveDroppedPDFsFlipDocsMarker(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.tm.Type("2")
	h.wait("Draft 2")
	out := filepath.Join(h.folder("docs-none"), "out")
	h.write(filepath.Join(out, "Owner Resume Role.pdf"), "%PDF")
	h.write(filepath.Join(out, "Owner Role Cover Letter.pdf"), "%PDF")
	h.wait("● ready")
	if v := h.finalView(); !strings.Contains(rowLine(v, "docs-none"), "● ready") {
		t.Fatalf("docs-none row lacks the Ready marker:\n%s", v)
	}
}

const newLead = "schema: 1\ntitle: Brand New Role\ncompany: Newco\ntrack: SWE\nstatus_log:\n  - {status: Lead, at: 2026-10-01T01:00:00Z}\n"

func TestLiveNewApplicationAppearsAndIsWatched(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.write(filepath.Join(h.folder("newco"), "meta.yml"), newLead)
	h.wait("Brand New Role")
	h.addMetaKey(h.folder("newco"), "fit: Marginal")
	h.wait("Marginal")
	// A whole folder appearing at once, with out/ already inside it.
	h.write(filepath.Join(h.folder("late"), "meta.yml"), strings.Replace(newLead, "Brand New Role", "Late Arrival", 1))
	h.wait("Late Arrival")
	if v := h.finalView(); !strings.Contains(rowLine(v, "Brand New Role"), "Marginal") || !strings.Contains(v, "Newco") {
		t.Fatalf("new Application row missing or stale:\n%s", v)
	}
}

func TestLiveNewTrackFolderIsWatched(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.write(filepath.Join(h.root, "jobs", "Ops", "ops-role", "meta.yml"), strings.Replace(newLead, "track: SWE", "track: Ops", 1))
	h.wait("Brand New Role")
	h.addMetaKey(filepath.Join(h.root, "jobs", "Ops", "ops-role"), "fit: Poor")
	h.wait("Poor")
}

func TestLiveCursorStaysOnSameFolder(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.tm.Type("j") // second Lead: Empty log
	h.tm.Type("j") // third Lead: Fern
	// A new Lead sorts above Fern (a close date), and another row changes Fit.
	h.write(filepath.Join(h.folder("early"), "meta.yml"), strings.Replace(newLead, "track: SWE\n", "track: SWE\ncloses: 2026-12-01\n", 1))
	h.wait("Brand New Role")
	h.addMetaKey(h.folder("empty"), "fit: Good")
	h.wait("Good")
	h.key(tea.KeyEnter)
	if v := h.finalView(); !strings.Contains(v, "Platform Engineer · Fern Systems") {
		t.Fatalf("cursor left Fern after live updates:\n%s", v)
	}
}

func jdWithItems(changed string) string {
	var b strings.Builder
	for i := 1; i <= 80; i++ {
		fmt.Fprintf(&b, "- item %02d%s\n", i, map[bool]string{true: changed}[i == 10])
	}
	return b.String()
}

func TestLiveDetailViewRerendersAndKeepsScrollPosition(t *testing.T) {
	h := newLiveHarness(t, func(root string) {
		os.WriteFile(filepath.Join(root, "jobs", "SWE", "fern", "jd.md"), []byte(jdWithItems("")), 0o644)
	})
	h.tm.Type("jj") // Fern
	h.key(tea.KeyEnter)
	h.wait("item 01")
	for i := 0; i < 5; i++ {
		h.tm.Type("j")
	}
	h.write(filepath.Join(h.folder("fern"), "jd.md"), jdWithItems(" REWRITTEN"))
	h.wait("REWRITTEN")
	v := h.finalView()
	if !strings.Contains(v, "item 10 REWRITTEN") {
		t.Fatalf("detail view did not show the rewritten JD:\n%s", v)
	}
	if strings.Contains(v, "item 01") || !strings.Contains(v, "item 06") {
		t.Fatalf("scroll position was not kept:\n%s", v)
	}
}
