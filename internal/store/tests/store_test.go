package store_test

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func TestScanFixture(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../../fixtures/expected.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected []struct {
		store.Application
		MalformedExpected bool
	}
	var want []store.Application
	if err = json.Unmarshal(b, &expected); err != nil {
		t.Fatal(err)
	}
	for _, e := range expected {
		want = append(want, e.Application)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		if (got[i].Malformed != nil) != expected[i].MalformedExpected {
			t.Fatalf("unexpected malformed error: %v", got[i].Malformed)
		}
		got[i].Malformed = nil
		got[i].Folder = ""
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestRepeatedStatusEntriesNamesEachRepeatOnce(t *testing.T) {
	a := store.Application{StatusLog: []store.StatusEntry{
		{Status: "Applied", At: "2026-03-01"},
		{Status: "Lead", At: "2026-03-05"},
		{Status: "Applied", At: "2026-03-01"},
		{Status: "Applied", At: "2026-03-01"},
		{Status: "Interviewing", At: "2026-04-01T00:00:00Z"},
	}}

	got := a.RepeatedStatusEntries()

	want := []store.StatusEntry{{Status: "Applied", At: "2026-03-01"}}
	if !slices.Equal(got, want) {
		t.Errorf("RepeatedStatusEntries = %v, want %v", got, want)
	}
	if (store.Application{}).RepeatedStatusEntries() != nil {
		t.Error("an empty log has no repeats")
	}
}
