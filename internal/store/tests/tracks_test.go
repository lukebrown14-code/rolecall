package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
