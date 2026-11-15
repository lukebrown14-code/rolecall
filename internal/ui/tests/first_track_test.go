package ui_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func workspaceTracks(t *testing.T, root string) []string {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	tracks, err := s.Tracks()
	if err != nil {
		t.Fatal(err)
	}
	return tracks
}

// A fresh Workspace lists no Tracks, so Add asks for the first one as text, and saving
// adds it to workspace.json along with the Lead: the welcome's first step just works.
func TestFirstLeadOnAFreshWorkspaceAddsItsTrack(t *testing.T) {
	root := freshWorkspace(t)
	if tracks := workspaceTracks(t, root); len(tracks) != 0 {
		t.Fatalf("fresh Workspace lists %q", tracks)
	}
	d := newDriver(t, root, setupNow, 120, 40)
	d.Type("a")
	requireAll(t, d.plain(), "Add a Lead", "Track*", "SWE", "your first Track")
	d.Type("enter") // URL
	d.Type("Platform Engineer")
	d.Type("enter")
	d.Type("Acme")
	d.Type("enter")
	d.Type("ctrl+s")
	requireAll(t, d.plain(), "Track is required")
	d.Type(" Data ")
	d.Type("ctrl+s")
	if got := workspaceTracks(t, root); !slices.Equal(got, []string{"Data"}) {
		t.Fatalf("tracks = %q, want [Data]\n%s", got, d.plain())
	}
	if _, err := os.Stat(filepath.Join(root, "jobs", "Data", "acme-platform-engineer-2026-11-15", "meta.yml")); err != nil {
		t.Fatalf("Lead not created: %v\n%s", err, d.plain())
	}
	requireAll(t, d.plain(), "Added Lead · Platform Engineer")
}

// A typed first Track follows the Track name rules before anything is written.
func TestFirstTrackFollowsTheNameRules(t *testing.T) {
	root := freshWorkspace(t)
	d := newDriver(t, root, setupNow, 120, 40)
	d.Type("a")
	d.Type("enter")
	d.Type("Engineer")
	d.Type("enter")
	d.Type("Acme")
	d.Type("enter")
	d.Type("a/b")
	d.Type("ctrl+s")
	requireAll(t, d.plain(), "Track name cannot contain /")
	if tracks := workspaceTracks(t, root); len(tracks) != 0 {
		t.Fatalf("tracks written: %q", tracks)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "jobs")); len(entries) != 0 {
		t.Fatalf("jobs/ not empty: %v", entries)
	}
}

// A tracks list the app refused is not written over by Add: the error says where to fix it.
func TestAddWithABrokenTrackListPointsToSettings(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		if err := os.WriteFile(filepath.Join(root, "workspace.json"), []byte(`{"tracks": ["SWE", "swe"]}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("a")
	requireAll(t, d.plain(), "6 Settings › Tracking")
	if strings.Contains(d.plain(), "Add a Lead") {
		t.Fatalf("Add opened over a broken tracks list:\n%s", d.plain())
	}
	b, _ := os.ReadFile(filepath.Join(root, "workspace.json"))
	if string(b) != `{"tracks": ["SWE", "swe"]}`+"\n" {
		t.Fatalf("workspace.json rewritten: %s", b)
	}
}

// offListHarness is Settings › Tracking on the fixture with one more Lead on track.
func offListHarness(t *testing.T, track string) (*driver, string) {
	t.Helper()
	root := fixtureCopy(t, func(root string) {
		dir := filepath.Join(root, "jobs", "SWE", "spacetrack")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		meta := "schema: 1\ntitle: Space Track\ncompany: Spco\ntrack: \"" + track + "\"\nstatus_log:\n  - {status: Lead, at: 2026-08-01}\n"
		if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("6")
	d.Type("]")
	d.Type("]")
	return d, root
}

// Spaces around a meta.yml Track are noise: " SWE " counts as SWE, not as a second
// Track that Settings could offer to list.
func TestSpacedTrackFoldsIntoItsTrack(t *testing.T) {
	d, _ := offListHarness(t, " SWE ")
	if n := strings.Count(d.plain(), "not listed"); n != 1 {
		t.Fatalf("%d off-list rows, want only Unknown:\n%s", n, d.plain())
	}
}

// An off-list Track the list could not hold (here a case twin of SWE) is refused, and
// workspace.json is left as a file the next load accepts.
func TestOffListTrackThatBreaksTheRulesIsNotWritten(t *testing.T) {
	d, root := offListHarness(t, "swe")
	requireAll(t, squash(d.plain()), "swe 1 application · not listed")
	before, _ := os.ReadFile(filepath.Join(root, "workspace.json"))
	for range 3 {
		d.Type("j")
	}
	requireAll(t, d.plain(), "▌ swe")
	d.Type("enter")
	requireAll(t, d.plain(), "Can't list swe: Track SWE already exists")
	after, _ := os.ReadFile(filepath.Join(root, "workspace.json"))
	if string(after) != string(before) {
		t.Fatalf("workspace.json rewritten:\n%s", after)
	}
	workspaceTracks(t, root) // still loads
}
