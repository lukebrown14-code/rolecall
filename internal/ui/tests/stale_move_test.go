package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// staleModel opens a one-Draft Workspace on the Drafts screen, with no watcher, so a
// change made behind the model's back stays unseen until it acts.
func staleModel(t *testing.T) (tea.Model, *store.Store, string) {
	t.Helper()
	root, folder := appliedWorkspace(t, map[string]string{})
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, nil, ui.WithExternalRunner(fakeExternal(t)))
	if err != nil {
		t.Fatal(err)
	}
	m = runMsg(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	return driveModel(t, m, "2"), s, folder
}

// A move to the Status the Application already has on disk writes nothing and leaves
// nothing to undo.
func TestMoveToCurrentStatusRecordsNoUndo(t *testing.T) {
	m, s, folder := staleModel(t)
	if _, err := s.AppendStatus(folder, store.StatusLead, time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	m = driveModel(t, m, "ml")
	if view := m.View().Content; !strings.Contains(view, "Already Lead") {
		t.Fatalf("want the no-op note, got:\n%s", view)
	}
	m = driveModel(t, m, "u")
	if view := m.View().Content; !strings.Contains(view, "Nothing to undo") {
		t.Fatalf("a no-op move left an undo:\n%s", view)
	}
	after, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if string(after) != string(before) {
		t.Fatalf("meta.yml changed:\n%s", after)
	}
}

// An undo is refused once another writer moved the Application on from where the move
// left it, and meta.yml is left as that writer made it.
func TestUndoRefusedAfterExternalMove(t *testing.T) {
	m, s, folder := staleModel(t)
	m = driveModel(t, m, "ml")
	if _, err := s.AppendStatus(folder, store.StatusDismissed, time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	m = driveModel(t, m, "u")
	if view := m.View().Content; !strings.Contains(view, "changed since the move") {
		t.Fatalf("want the refusal, got:\n%s", view)
	}
	after, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if string(after) != string(before) {
		t.Fatalf("undo overwrote the other writer:\n%s", after)
	}
}
