package watch_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// receive bounds a wait through the same interface used by the UI.
func receive(t *testing.T, w *watch.Watcher) watch.Changed {
	t.Helper()
	result := make(chan any, 1)
	go func() { result <- w.Next()() }()
	select {
	case msg := <-result:
		batch, ok := msg.(watch.Changed)
		if !ok {
			t.Fatal("watcher closed before delivering a batch")
		}
		return batch
	case <-time.After(10 * time.Second):
		t.Fatal("no batch arrived")
		return watch.Changed{}
	}
}

func TestWatcherBatchesBurstAndIgnoresArchive(t *testing.T) {
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
	write := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Archive changes, existing and new, come first so any leak would precede the sentinel.
	write(filepath.Join(jobs, "_archive", "old", "meta.yml"))
	write(filepath.Join(jobs, "_archive", "fresh", "meta.yml"))
	write(filepath.Join(jobs, "_archive", "fresh", "out", "a.pdf"))
	fern := filepath.Join(jobs, "SWE", "fern")
	for i := 0; i < 30; i++ {
		write(filepath.Join(fern, "notes.md"))
	}
	var seen [][]string
	for {
		b := receive(t, w).Folders
		seen = append(seen, b)
		if len(b) == 1 && b[0] == fern {
			break
		}
		for _, f := range b {
			if f != fern {
				t.Fatalf("unexpected folder in batch %v", b)
			}
		}
	}
	for _, b := range seen {
		for _, f := range b {
			if strings.Contains(f, "_archive") {
				t.Fatalf("_archive change was reported: %v", b)
			}
		}
	}
}
