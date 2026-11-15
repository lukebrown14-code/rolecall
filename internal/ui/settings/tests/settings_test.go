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

func TestArrowsSwitchSectionsAndGGoesToTheEnds(t *testing.T) {
	machine := settings.Machine{Editor: "vi", EditorAvailable: true}
	st := settings.State[string]{}.Navigate("right", machine)
	if st.Group != settings.GroupDocuments {
		t.Fatalf("right did not switch section: %+v", st)
	}
	if st = st.Navigate("left", machine).Navigate("left", machine); st.Group != settings.GroupMachine {
		t.Fatalf("left did not wrap to the last section: %+v", st)
	}
	st = st.Navigate("G", machine)
	if rows := st.Rows(st.Group, machine); rows[st.Row].Label != settings.LabelInherit {
		t.Fatalf("G did not reach the last row: %+v", st)
	}
	if st = st.Navigate("g", machine); st.Rows(st.Group, machine)[st.Row].Label != settings.LabelEditor {
		t.Fatalf("g did not reach the first editable row: %+v", st)
	}
}

// A Track Applications use but workspace.json does not list gets its own row, so it can
// be added and renamed; an Application with no Track at all does not.
func TestOffListTracksAreListedAfterTheTracks(t *testing.T) {
	st := settings.State[string]{TrackUse: map[string]int{"SWE": 3, "Unknown": 2, "": 1, "Gone": 0}}
	st.Ws.Tracks = []string{"SWE"}
	if got := st.OffList(); len(got) != 1 || got[0] != "Unknown" {
		t.Fatalf("OffList = %v", got)
	}
	rows := st.Rows(settings.GroupTracking, settings.Machine{})
	if rows[1].Label != "Unknown" || !rows[1].OffList || rows[2].Label != settings.LabelAddTrack {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestEmptyPlacesMeanNoSearchArea(t *testing.T) {
	if got := settings.SplitPlaces(" , "); len(got) != 0 {
		t.Fatalf("SplitPlaces = %v", got)
	}
	if got := settings.SplitPlaces("Australia, APAC"); len(got) != 2 {
		t.Fatalf("SplitPlaces = %v", got)
	}
}

func TestEveryEditableRowSaysWhatItIs(t *testing.T) {
	machine := settings.Machine{Editor: "vi", EditorAvailable: true}
	for g := range settings.Groups {
		for _, r := range (settings.State[string]{}).Rows(g, machine) {
			if r.About == "" && r.Label != settings.LabelWorkspace {
				t.Errorf("%s has no about text", r.Label)
			}
		}
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
