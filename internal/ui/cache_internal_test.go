package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"github.com/charmbracelet/x/ansi"
)

// pressModel feeds one key straight to Update and returns the model, ignoring commands:
// the detail view's navigation and scroll keys return none.
func pressModel(t *testing.T, m model, k tea.KeyPressMsg) model {
	t.Helper()
	next, _ := m.Update(k)
	return next.(model)
}

// countRenders replaces the one Markdown render call with a counter for the duration of a
// test, and returns a function giving the number of renders so far.
func countRenders(t *testing.T) func() int {
	t.Helper()
	renders := 0
	real := renderText
	renderText = func(r *glamour.TermRenderer, text string) (string, error) {
		renders++
		return real(r, text)
	}
	t.Cleanup(func() { renderText = real })
	return func() int { return renders }
}

// The detail view re-renders a Document on every keystroke today. Once a generation's
// Document has been rendered, scrolling and other keys on it reuse the render instead.
func TestDetailDocumentRendersOncePerGeneration(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 120, 40
	m.screen, m.table, m.text = 2, 0, "docs-written" // Applied, docs-written is Interviewing

	renders := countRenders(t)

	// Open the detail view and move off the empty JD tab to Fit, which has text.
	m = pressModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = pressModel(t, m, tea.KeyPressMsg{Code: tea.KeyRight})
	if got := renders(); got == 0 {
		t.Fatal("opening a tab with a Document did not render it")
	}
	first := renders()

	// Scroll keys and frame draws on the same generation add no renders.
	for i := 0; i < 3; i++ {
		m = pressModel(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
		_ = m.View()
	}
	if got := renders(); got != first {
		t.Fatalf("unchanged frames re-rendered the Document %d times, want 0", got-first)
	}

	// A new generation invalidates the cached render.
	m.gen++
	m = m.syncDetail()
	if got := renders(); got <= first {
		t.Fatalf("a new generation did not re-render: %d renders, want more than %d", got, first)
	}
}

// The body cache must not let a wider Glamour renderer stand in for a narrower one, or a
// resize would keep the old wrap width.
func TestBodyCacheKeepsRendererWidth(t *testing.T) {
	m := fixtureModel(t)
	long := strings.Repeat("wrap-me ", 40)
	wide := strings.Count(m.renderMarkdown(long, 120), "\n")
	narrow := strings.Count(m.md.body(m.gen, 40, "resize", func() string {
		return m.renderMarkdown(long, 40)
	}), "\n")
	if narrow <= wide {
		t.Fatalf("narrow render has %d lines, wide has %d: the renderer kept the old width", narrow, wide)
	}
}

// A Document changed on disk still shows after the watcher's generation moves.
func TestDetailDocumentFollowsChangedFile(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 120, 40
	m.screen, m.table, m.text = 2, 0, "docs-written"

	m = pressModel(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = pressModel(t, m, tea.KeyPressMsg{Code: tea.KeyRight}) // Fit tab
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "fit") {
		t.Fatalf("Fit tab did not show the document:\n%s", v)
	}

	folder := filepath.Join(m.store.Root(), "jobs", "SWE", "docs-written")
	if err := os.WriteFile(filepath.Join(folder, "fit.md"), []byte("A rewritten fictional fit"), 0o600); err != nil {
		t.Fatal(err)
	}
	m = m.refresh([]string{folder})
	if v := ansi.Strip(m.View().Content); !strings.Contains(v, "A rewritten fictional fit") {
		t.Fatalf("changed Document did not appear after a refresh:\n%s", v)
	}
}
