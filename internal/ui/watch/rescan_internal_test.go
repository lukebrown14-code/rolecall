package watch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// An fsnotify error (a queue overflow drops events) reports every track and Application
// folder, so the UI rescans the whole Workspace rather than keeping stale rows.
func TestErrorTriggersFullRescan(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	jobs := filepath.Join(root, "jobs")
	w, err := Start(jobs, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	w.fs.Errors <- errors.New("queue overflow")
	got := make(chan Changed, 1)
	go func() {
		for {
			msg, ok := w.Next()().(Changed)
			if !ok {
				return
			}
			if slices.Contains(msg.Folders, filepath.Join(jobs, "SWE")) {
				got <- msg
				return
			}
		}
	}()
	select {
	case b := <-got:
		for _, want := range []string{filepath.Join(jobs, "SWE"), filepath.Join(jobs, "SWE", "fern")} {
			if !slices.Contains(b.Folders, want) {
				t.Fatalf("rescan batch %v lacks %s", b.Folders, want)
			}
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no rescan after an fsnotify error")
	}
}
