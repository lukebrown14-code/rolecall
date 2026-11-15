package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func readWorkspaceJSON(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSetTracksKeepsKeyOrderAndUnknownKeys(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"sources":[{"name":"a"}],"tracks":["SWE"],"blockers":["x"],"future":{"k":1}}`)
	if err := s.SetTracks([]string{"Platform", "SWE"}); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"sources\": [\n    {\n      \"name\": \"a\"\n    }\n  ],\n  \"tracks\": [\n    \"Platform\",\n    \"SWE\"\n  ],\n" +
		"  \"blockers\": [\n    \"x\"\n  ],\n  \"future\": {\n    \"k\": 1\n  }\n}\n"
	if got := readWorkspaceJSON(t, s.Root()); got != want {
		t.Fatalf("file =\n%s\nwant\n%s", got, want)
	}
	got, _ := s.Tracks()
	if !reflect.DeepEqual(got, []string{"Platform", "SWE"}) {
		t.Fatalf("tracks = %v", got)
	}
}

func TestSetTracksAppendsKeyWhenMissingAndCreatesFile(t *testing.T) {
	s := fixtureProfileStore(t)
	writeWorkspaceJSON(t, s.Root(), `{"sources":[]}`)
	if err := s.SetTracks([]string{"SWE"}); err != nil {
		t.Fatal(err)
	}
	if got := readWorkspaceJSON(t, s.Root()); !strings.HasPrefix(got, "{\n  \"sources\": [],\n  \"tracks\": [\n    \"SWE\"") {
		t.Fatalf("file = %s", got)
	}
	if err := os.Remove(filepath.Join(s.Root(), "workspace.json")); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTracks([]string{"A"}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Tracks(); !reflect.DeepEqual(got, []string{"A"}) {
		t.Fatalf("tracks = %v", got)
	}
}

func TestSetTracksEmptyWritesEmptyArray(t *testing.T) {
	s := fixtureProfileStore(t)
	if err := s.SetTracks(nil); err != nil {
		t.Fatal(err)
	}
	if got := readWorkspaceJSON(t, s.Root()); !strings.Contains(got, `"tracks": []`) {
		t.Fatalf("file = %s", got)
	}
}

func TestSetTracksNeverOverwritesInvalidJSON(t *testing.T) {
	s := fixtureProfileStore(t)
	const bad = `{"tracks": ["SWE",`
	writeWorkspaceJSON(t, s.Root(), bad)
	if err := s.SetTracks([]string{"X"}); err == nil {
		t.Fatal("expected an error")
	}
	if got := readWorkspaceJSON(t, s.Root()); got != bad {
		t.Fatalf("file changed: %s", got)
	}
}

// A hand-edited tracks list is checked on load with the rules Settings applies, so the
// filter never shows a blank or doubled Track; the error says which one and why.
func TestTracksRejectsBlankDuplicateAndUntrimmedNames(t *testing.T) {
	for list, want := range map[string]string{
		`["SWE","SWE"]`:    "Track SWE is listed twice",
		`["SWE","swe"]`:    "Track swe is listed twice",
		`["SWE",""]`:       "Track name is empty",
		`["SWE"," Data "]`: "has spaces around it",
		`["a/b"]`:          "cannot contain /",
		`["x\u001b[2J"]`:   "control character",
	} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "workspace.json"), []byte(`{"tracks":`+list+`}`), 0o600); err != nil {
			t.Fatal(err)
		}
		s, err := store.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Tracks()
		if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "workspace.json: tracks:") {
			t.Fatalf("%s: %v, want %q", list, err, want)
		}
		if strings.Contains(err.Error(), "\x1b") {
			t.Fatalf("%q", err)
		}
	}
}
