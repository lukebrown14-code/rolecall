package watch_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// Only the documents' own files count: notes, fit reasoning and stray files do not.
func TestChipFile(t *testing.T) {
	for path, want := range map[string]string{
		"resume.md": "resume.md", "cover-letter.md": "cover-letter.md", "jd.md": "jd.md",
		"out/Owner Resume Role.pdf": watch.PDFDocument, "notes.md": "", "fit.md": "", "out/build.log": "", "sent/resume.md": "",
	} {
		if got := watch.DocumentFile(strings.Split(path, "/")); got != want {
			t.Errorf("%s: got %q, want %q", path, got, want)
		}
	}
}

// The Watcher reports which documents were written with the batch that carries their
// folder; a note is not a document.
func TestWatcherReportsWrittenDocuments(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "jobs")
	w, err := watch.Start(jobs, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	fern := filepath.Join(jobs, "SWE", "fern")
	for _, name := range []string{"notes.md", "cover-letter.md"} {
		if err := os.WriteFile(filepath.Join(fern, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var written []string
	for !slices.Contains(written, watch.DocumentKey(fern, "cover-letter.md")) {
		msg := receive(t, w)
		written = append(written, msg.Written...)
	}
	if slices.Contains(written, watch.DocumentKey(fern, "notes.md")) {
		t.Fatalf("a note counted as a document: %v", written)
	}
}
