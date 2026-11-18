package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// A document the watch.Watcher saw written reads as being written, with the ready meter beside
// it, until its mark's tick puts it out; a later write renews the mark.
func TestWrittenDocumentShowsWritingUntilItsTick(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 120, 36
	m.harness = "claude {skill} {dir}" // only a Harness writes Documents
	folder := filepath.Join(m.store.Root(), "jobs", "SWE", "docs-none")
	if err := os.WriteFile(filepath.Join(folder, resumeFile), []byte("# Resume\n\nFictional text."), 0o600); err != nil {
		t.Fatal(err)
	}
	key := watch.DocumentKey(folder, resumeFile)
	next, _ := m.Update(watch.Changed{Folders: []string{folder}, Written: []string{key}})
	m = next.(model)
	m.screen = 1 // Drafts, where docs-none is the second row: Engineer #1 has a Fit
	m.cursor = 1
	view := xansi.Strip(m.View().Content)
	if !strings.Contains(lineWith(view, "writing…"), "Resume") || !strings.Contains(view, "1/4") {
		t.Fatalf("no writing mark:\n%s", view)
	}
	first := m.writing[key]
	next, _ = m.Update(watch.Changed{Folders: []string{folder}, Written: []string{key}}) // written again
	m = next.(model)
	next, _ = m.Update(writingDone{key, first}) // the first mark's tick is stale
	m = next.(model)
	if !strings.Contains(xansi.Strip(m.View().Content), "writing…") {
		t.Fatal("a stale tick put out a renewed mark")
	}
	next, _ = m.Update(writingDone{key, m.writing[key]})
	m = next.(model)
	if view := xansi.Strip(m.View().Content); strings.Contains(view, "writing…") || !strings.Contains(view, "✓ Resume") {
		t.Fatalf("mark not put out:\n%s", view)
	}
}

// With no Harness set no agent is writing, so a written file, such as the empty jd.md a new
// Lead gets or a Resume saved in the Owner's Editor, never shows the writing mark.
func TestNoHarnessNeverShowsTheWritingMark(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 120, 36
	m.harness = ""
	folder := filepath.Join(m.store.Root(), "jobs", "SWE", "docs-none")
	written := []string{watch.DocumentKey(folder, jdFile), watch.DocumentKey(folder, resumeFile)}
	next, _ := m.Update(watch.Changed{Folders: []string{folder}, Written: written})
	m = next.(model)
	if len(m.writing) != 0 {
		t.Fatalf("writes marked with no Harness: %v", m.writing)
	}
	m.screen, m.cursor = 1, 1
	if view := xansi.Strip(m.View().Content); strings.Contains(view, "writing…") {
		t.Fatalf("writing mark with no Harness:\n%s", view)
	}
}

// lineWith is the first line of view holding s, or "".
func lineWith(view, s string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, s) {
			return l
		}
	}
	return ""
}
