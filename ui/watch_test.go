package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The debounce window is closed by hand, so these tests involve no clock.
func TestDebouncerDeliversEachFolderOncePerWindow(t *testing.T) {
	var closeWindow func()
	var batches [][]string
	d := &debouncer{
		schedule: func(f func()) { closeWindow = f },
		deliver:  func(folders []string) { batches = append(batches, folders) },
	}
	for i := 0; i < 50; i++ {
		d.add("/jobs/SWE/b")
		d.add("/jobs/SWE/a")
	}
	if len(batches) != 0 {
		t.Fatalf("delivered before the window closed: %v", batches)
	}
	closeWindow()
	if want := [][]string{{"/jobs/SWE/a", "/jobs/SWE/b"}}; !reflect.DeepEqual(batches, want) {
		t.Fatalf("batches = %v, want %v", batches, want)
	}
	closeWindow = nil
	d.add("/jobs/SWE/a")
	if closeWindow == nil {
		t.Fatal("a change after delivery did not open a new window")
	}
	closeWindow()
	if len(batches) != 2 || !reflect.DeepEqual(batches[1], []string{"/jobs/SWE/a"}) {
		t.Fatalf("second window = %v", batches)
	}
}

func TestDebouncerOpensOneWindowForABurst(t *testing.T) {
	windows := 0
	d := &debouncer{schedule: func(func()) { windows++ }, deliver: func([]string) {}}
	for i := 0; i < 20; i++ {
		d.add("/jobs/SWE/a")
	}
	if windows != 1 {
		t.Fatalf("opened %d windows for one burst", windows)
	}
}

func receive(t *testing.T, w *watcher) []string {
	t.Helper()
	select {
	case b := <-w.batches:
		return b
	case <-time.After(10 * time.Second):
		t.Fatal("no batch arrived")
		return nil
	}
}

func TestWatcherBatchesBurstAndIgnoresArchive(t *testing.T) {
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
		b := receive(t, w)
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
