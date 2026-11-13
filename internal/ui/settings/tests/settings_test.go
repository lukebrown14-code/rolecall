package settings_test

import (
	"os"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

func TestNavigationSkipsReadOnlyRowsAndPreservesPreviousState(t *testing.T) {
	original := settings.State[string]{}
	machine := settings.Machine{Editor: "vi", EditorAvailable: true}
	current := original.Navigate("]", machine).Navigate("[", machine).Navigate("j", machine)
	rows := current.Rows(current.Group, machine)
	if rows[current.Row].Label != "Remote roles" {
		t.Fatalf("cursor moved onto read-only rows: %+v", current)
	}
	if original.Group != 0 || original.Row != 0 {
		t.Fatalf("navigation mutated prior state: %+v", original)
	}
	if up := current.Navigate("k", machine).Navigate("k", machine); !up.Header {
		t.Fatalf("k above the first field should rest on the header: %+v", up)
	}
	current = original
	for range 3 {
		current = current.Navigate("]", machine)
	}
	rows = current.Rows(current.Group, machine)
	if rows[current.Row].Label != settings.LabelEditor {
		t.Fatalf("machine cursor did not skip Workspace: %+v", current)
	}
}

func TestPendingEditsAreCopiedNotShared(t *testing.T) {
	a := settings.State[string]{}.With("editor", "nano")
	b := a.With("downloads", "/x")
	if a.Dirty() != 1 || b.Dirty() != 2 {
		t.Fatalf("pending leaked between copies: %d, %d", a.Dirty(), b.Dirty())
	}
	if c := b.Without("editor"); c.Dirty() != 1 || b.Dirty() != 2 {
		t.Fatalf("Without mutated or miscounted: %d, %d", c.Dirty(), b.Dirty())
	}
}

func TestReloadPreservesSelectionAndUndoAndRefreshesTrackCounts(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	undo := "restore previous setting"
	original := settings.State[string]{Group: 2, Row: 1, Undo: &undo}
	loaded, err := original.Load(s)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Group != original.Group || loaded.Row != original.Row || loaded.Undo != original.Undo {
		t.Fatal("reload lost navigation or undo")
	}
	if len(loaded.TrackUse) == 0 {
		t.Fatal("reload did not populate track usage")
	}
	if original.TrackUse != nil {
		t.Fatal("reload mutated previous state")
	}
	for _, track := range loaded.Ws.Tracks {
		want, err := s.TrackUse(track)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.TrackUse[track]; got != want {
			t.Fatalf("%s usage = %d, want %d", track, got, want)
		}
	}
}
