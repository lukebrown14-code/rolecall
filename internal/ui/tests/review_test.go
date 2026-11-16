package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// reviewHarness drives the whole program over a hand-built Workspace holding one Application.
type reviewHarness struct {
	t      *testing.T
	root   string
	folder string
	tm     *driver
}

func entry(status, day string) string {
	return "  - {status: " + status + ", at: 2026-01-" + day + "T00:00:00Z}\n"
}

func newReviewHarness(t *testing.T, log string, files map[string]string) *reviewHarness {
	t.Helper()
	root := t.TempDir()
	folder := filepath.Join(root, "jobs", "SWE", "role")
	files["meta.yml"] = "title: Role\ncompany: Company\ntrack: SWE\nstatus_log:\n" + log
	for name, text := range files {
		path := filepath.Join(folder, name)
		if name == "workspace.json" {
			path = filepath.Join(root, name)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	h := &reviewHarness{t: t, root: root, folder: folder}
	h.start()
	return h
}

func (h *reviewHarness) start() {
	h.t.Helper()
	h.tm = newDriver(h.t, h.root, func() time.Time { return time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC) }, 120, 40)
}

func (h *reviewHarness) waitOutput(text string) {
	h.t.Helper()
	if view := h.tm.plain(); !strings.Contains(view, text) {
		h.t.Fatalf("screen lacks %q:\n%s", text, view)
	}
}

func (h *reviewHarness) meta() string {
	b, err := os.ReadFile(filepath.Join(h.folder, "meta.yml"))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

// waitForLogLength fails unless meta.yml holds n status_log entries.
func (h *reviewHarness) waitForLogLength(n int) {
	h.t.Helper()
	if got := strings.Count(h.meta(), "- {status:"); got != n {
		h.t.Fatalf("meta.yml holds %d status_log entries, want %d:\n%s", got, n, h.meta())
	}
}

func (h *reviewHarness) lastStatus() string {
	lines := strings.Split(strings.TrimSpace(h.meta()), "\n")
	last := lines[len(lines)-1]
	return strings.TrimPrefix(last[:strings.Index(last, ",")], "  - {status: ")
}

// finish returns the last screen with styling stripped.
func (h *reviewHarness) finish() string {
	h.t.Helper()
	return h.tm.plain()
}

// rawFinish quits and returns the final screen with its styling, for checks that read it.
func (h *reviewHarness) rawFinish() string {
	h.t.Helper()
	h.tm.Type("q")
	return h.tm.View()
}

func (h *reviewHarness) key(code rune) { h.tm.key(code) }

// The popup's default for a Dismissed row is the Status before the last Dismissed entry, however
// many Dismissed entries an agent or an undo has stacked at the end of the log.
func TestRestoreDefaultsToStatusBeforeTheLastDismissedEntry(t *testing.T) {
	h := newReviewHarness(t, entry("Draft", "01")+entry("Dismissed", "02")+entry("Dismissed", "03"), map[string]string{})
	h.key(tea.KeyTab) // Dismissed table
	h.tm.Type("m")
	h.waitOutput("Move")
	h.key(tea.KeyEnter)
	h.waitForLogLength(4)
	if got := h.lastStatus(); got != "Draft" {
		t.Fatalf("restored to %s, want Draft:\n%s", got, h.meta())
	}
	h.finish()
}

func TestRestoreAfterDismissRestoreUndoReturnsToEarlierStatus(t *testing.T) {
	h := newReviewHarness(t, entry("Draft", "01"), map[string]string{})
	h.tm.Type("2")
	h.waitOutput("Drafts 1")
	h.tm.Type("x")
	h.waitForLogLength(2)
	h.tm.Type("1")
	h.key(tea.KeyTab)
	h.tm.Type("m")
	h.waitOutput("Move")
	h.key(tea.KeyEnter)
	h.waitForLogLength(3) // restored to Draft
	h.tm.Type("u")        // undo appends Dismissed again
	h.waitForLogLength(4)
	h.tm.Type("m")
	h.waitOutput("Move")
	h.key(tea.KeyEnter)
	h.waitForLogLength(5)
	if got := h.lastStatus(); got != "Draft" {
		t.Fatalf("restored to %s, want Draft:\n%s", got, h.meta())
	}
	h.finish()
}

// When the sent/ copy fails after Applied is appended, the Owner sees the error, not a success
// toast, and u has nothing to undo.
func TestFailedSentCopyShowsErrorWithoutToastOrUndo(t *testing.T) {
	h := newReviewHarness(t, entry("Draft", "01"), map[string]string{"resume.md": "written"})
	if err := os.Chmod(filepath.Join(h.folder, "resume.md"), 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(h.folder, "resume.md"), 0o644) })
	h.tm.Type("2")
	h.waitOutput("Drafts 1")
	h.tm.Type("ma")
	h.waitOutput("Docs not Ready")
	h.tm.Type("y")
	h.waitForLogLength(2)
	h.tm.Type("u")
	view := h.finish()
	if !strings.Contains(view, "permission denied") || strings.Contains(view, "Moved to Applied") {
		t.Fatalf("want the copy error and no toast, got:\n%s", view)
	}
	if got := h.lastStatus(); got != "Applied" {
		t.Fatalf("u undid a move that was never confirmed; last Status %s:\n%s", got, h.meta())
	}
}

// r rereads workspace.json, so a track added while the program runs can be filtered on
// once it has rows (t passes over a Track with none).
func TestRescanReloadsTracks(t *testing.T) {
	h := newReviewHarness(t, entry("Lead", "01"), map[string]string{"workspace.json": `{"tracks":["SWE","Platform"]}`})
	if err := os.WriteFile(filepath.Join(h.root, "workspace.json"), []byte(`{"tracks":["SWE","Platform","Data"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := "title: Role\ncompany: Company\ntrack: Data\nstatus_log:\n" + entry("Lead", "01")
	if err := os.WriteFile(filepath.Join(h.folder, "meta.yml"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	h.tm.Type("r")
	h.tm.Type("t")
	if view := h.rawFinish(); activeTrack(view) != "Data" {
		t.Fatalf("new track not selectable after r:\n%s", view)
	}
}

// A track filter on a track that r finds removed falls back to All rather than indexing past the list.
func TestRescanDropsFilterOnARemovedTrack(t *testing.T) {
	h := newReviewHarness(t, entry("Lead", "01"), map[string]string{"workspace.json": `{"tracks":["SWE","Platform"]}`})
	h.tm.Type("tt") // Platform
	if err := os.WriteFile(filepath.Join(h.root, "workspace.json"), []byte(`{"tracks":["SWE"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	h.tm.Type("r")
	if view := h.rawFinish(); activeTrack(view) != "All" {
		t.Fatalf("filter kept a removed track:\n%s", view)
	}
}

// Keys that do nothing while an overlay owns the keyboard are not offered in the help bar.
// helpLists reports whether the `?` pop-up lists a "key description" pair: the pop-up
// pads the key column, so any run of spaces separates them.
func helpLists(view, pair string) bool {
	k, d, _ := strings.Cut(pair, " ")
	return regexp.MustCompile(`\[?` + regexp.QuoteMeta(k) + `\]?\s+` + regexp.QuoteMeta(d)).MatchString(view)
}

func TestHelpBarOnlyOffersKeysValidInContext(t *testing.T) {
	rowKeys := []string{"e edit", "m move", "u undo", "x dismiss", "n notes"}
	assertHidden := func(t *testing.T, context, view string) {
		t.Helper()
		for _, k := range rowKeys {
			if strings.Contains(view, k) {
				t.Errorf("%s: help offers %q:\n%s", context, k, view)
			}
		}
	}

	t.Run("move popup", func(t *testing.T) {
		h := newReviewHarness(t, entry("Lead", "01"), map[string]string{})
		h.tm.Type("m")
		h.waitOutput("Move")
		assertHidden(t, "move popup", h.finish())
	})
	t.Run("add form", func(t *testing.T) {
		h := newReviewHarness(t, entry("Lead", "01"), map[string]string{"workspace.json": `{"tracks":["SWE"]}`})
		h.tm.Type("a")
		h.waitOutput("Title")
		assertHidden(t, "add form", h.finish())
	})
	t.Run("not ready prompt", func(t *testing.T) {
		h := newReviewHarness(t, entry("Draft", "01"), map[string]string{})
		h.tm.Type("2")
		h.waitOutput("Drafts 1")
		h.tm.Type("ma")
		h.waitOutput("Docs not Ready")
		assertHidden(t, "prompt", h.finish())
	})
	t.Run("filter input", func(t *testing.T) {
		h := newReviewHarness(t, entry("Lead", "01"), map[string]string{})
		h.tm.Type("/Ro")
		assertHidden(t, "filter", h.finish())
	})
	t.Run("detail move popup", func(t *testing.T) {
		h := newReviewHarness(t, entry("Lead", "01"), map[string]string{})
		h.key(tea.KeyEnter)
		h.tm.Type("m")
		h.waitOutput("Move")
		assertHidden(t, "detail popup", h.finish())
	})
	t.Run("full help lists the same context keys", func(t *testing.T) {
		h := newReviewHarness(t, entry("Lead", "01"), map[string]string{})
		h.tm.Type("?")
		view := h.finish()
		for _, want := range []string{"Keys", "e edit", "m move", "x dismiss", "quit"} {
			if !helpLists(view, want) {
				t.Errorf("full help lacks %q:\n%s", want, view)
			}
		}
	})
}
