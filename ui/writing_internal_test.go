package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
)

// A document the watcher saw written reads as being written, with the ready meter beside
// it, until its mark's tick puts it out; a later write renews the mark.
func TestWrittenDocumentShowsWritingUntilItsTick(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 120, 36
	folder := filepath.Join(m.store.Root(), "jobs", "SWE", "docs-none")
	if err := os.WriteFile(filepath.Join(folder, resumeFile), []byte("# Resume\n\nFictional text."), 0o600); err != nil {
		t.Fatal(err)
	}
	key := writingKey(folder, resumeFile)
	next, _ := m.Update(foldersChanged{[]string{folder}, []string{key}})
	m = next.(model)
	m.screen = 1 // Drafts, where docs-none is the first row
	view := xansi.Strip(m.View().Content)
	if !strings.Contains(view, "Resume writing…") || !strings.Contains(view, "1 of 4 ready") {
		t.Fatalf("no writing mark:\n%s", view)
	}
	first := m.writing[key]
	next, _ = m.Update(foldersChanged{[]string{folder}, []string{key}}) // written again
	m = next.(model)
	next, _ = m.Update(writingDone{key, first}) // the first mark's tick is stale
	m = next.(model)
	if !strings.Contains(xansi.Strip(m.View().Content), "Resume writing…") {
		t.Fatal("a stale tick put out a renewed mark")
	}
	next, _ = m.Update(writingDone{key, m.writing[key]})
	m = next.(model)
	if view := xansi.Strip(m.View().Content); strings.Contains(view, "writing…") || !strings.Contains(view, "✓ Resume") {
		t.Fatalf("mark not put out:\n%s", view)
	}
}

// Only the documents' own files count: notes, fit reasoning and stray files do not.
func TestChipFile(t *testing.T) {
	for path, want := range map[string]string{
		"resume.md": resumeFile, "cover-letter.md": coverLetterFile, "jd.md": jdFile,
		"out/Owner Resume Role.pdf": pdfChip, "notes.md": "", "fit.md": "", "out/build.log": "", "sent/resume.md": "",
	} {
		if got := chipFile(strings.Split(path, "/")); got != want {
			t.Errorf("%s: got %q, want %q", path, got, want)
		}
	}
}

// The watcher reports which documents were written with the batch that carries their
// folder; a note is not a document.
func TestWatcherReportsWrittenDocuments(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "jobs")
	w, err := startWatcher(jobs, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()
	fern := filepath.Join(jobs, "SWE", "fern")
	for _, name := range []string{notesFile, coverLetterFile} {
		if err := os.WriteFile(filepath.Join(fern, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var written []string
	for !slices.Contains(written, writingKey(fern, coverLetterFile)) {
		msg, ok := w.next()().(foldersChanged)
		if !ok {
			t.Fatal("watcher closed")
		}
		written = append(written, msg.written...)
	}
	if slices.Contains(written, writingKey(fern, notesFile)) {
		t.Fatalf("a note counted as a document: %v", written)
	}
}
