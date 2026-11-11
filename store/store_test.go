package store_test

import (
	"encoding/json"
	"github.com/lukebrown14-code/rolecall/store"
	"os"
	"reflect"
	"testing"
)

func TestScanFixture(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../fixtures/expected.json")
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
