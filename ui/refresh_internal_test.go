package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/store"
)

// fixtureNowInternal is the clock for internal tests on fixtures/workspace, after every
// status_log date there.
var fixtureNowInternal = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

// fixtureModel opens a temp copy of the fixture Workspace.
func fixtureModel(t *testing.T) model {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(s, func() time.Time { return fixtureNowInternal })
	if err != nil {
		t.Fatal(err)
	}
	return m.(model)
}

// A live refresh that finds an Application gone drops it and keeps the header count right.
func TestRefreshDropsRemovedApplication(t *testing.T) {
	m := fixtureModel(t)
	fern := filepath.Join(m.store.Root(), "jobs", "SWE", "fern")
	if err := os.RemoveAll(fern); err != nil {
		t.Fatal(err)
	}
	next := m.refresh([]string{fern})
	if got := next.screenCount(0); got != 3 {
		t.Fatalf("Leads count is %d, want 3", got)
	}
	for _, a := range next.apps {
		if strings.Contains(a.Company, "Fern") || strings.Contains(a.Title, "Platform Engineer") {
			t.Fatalf("removed Application still present: %+v", a)
		}
	}
}

// A live refresh that moves the row above the cursor out of the table keeps the cursor on
// the same Application.
func TestRefreshKeepsCursorWhenARowAboveLeaves(t *testing.T) {
	m := fixtureModel(t)
	fern := filepath.Join(m.store.Root(), "jobs", "SWE", "fern")
	m.keepCursor(fern)
	if m.cursorFolder() != fern {
		t.Fatalf("cursor not on Fern to start")
	}
	// The agent dismisses a Lead above Fern; Fern must stay selected.
	emptyLog := filepath.Join(m.store.Root(), "jobs", "SWE", "empty-log")
	meta := filepath.Join(emptyLog, "meta.yml")
	b, err := os.ReadFile(meta)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(b), "status_log: []", "status_log:\n  - {status: Dismissed, at: 2026-10-01T00:00:00Z}", 1)
	if err := os.WriteFile(meta, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	next := m.refresh([]string{emptyLog})
	if got := next.cursorFolder(); got != fern {
		t.Fatalf("cursor left Fern after a row above it left the table: on %q", got)
	}
}
