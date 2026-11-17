package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

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

func TestCVsListsMarkdownGeneralFirstWithItsPDF(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string, at time.Time) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	write("cv/zeta.md", "z", now)
	write("cv/general.md", "g", now)
	write("cv/master.md", "m", now)
	write("cv/imported.pdf", "%PDF", now)
	write("cv/out/general/Alex Quinn Resume.pdf", "%PDF", now.Add(time.Minute))
	write("cv/out/master/Old.pdf", "%PDF", now.Add(-time.Hour))
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}

	cvs, err := s.CVs()

	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, cv := range cvs {
		names = append(names, cv.Name)
	}
	if !slices.Equal(names, []string{"general", "master", "zeta"}) {
		t.Fatalf("names = %v", names)
	}
	if g := cvs[0]; g.Path != filepath.Join("cv", "general.md") || g.Body != "g" || g.PDF != "Alex Quinn Resume.pdf" || g.Stale {
		t.Errorf("general = %+v", g)
	}
	if m := cvs[1]; m.PDF != "Old.pdf" || !m.Stale {
		t.Errorf("master = %+v, want its older PDF marked stale", m)
	}
	if z := cvs[2]; z.PDF != "" || z.Stale {
		t.Errorf("zeta = %+v, want no PDF", z)
	}

	downloads := filepath.Join(t.TempDir(), "Downloads")
	if err := s.CopyCVPDF(cvs[0], downloads, false); err != nil {
		t.Fatal(err)
	}
	if err := s.CopyCVPDF(cvs[0], downloads, false); !errors.Is(err, os.ErrExist) {
		t.Errorf("second copy err = %v, want ErrExist", err)
	}
	if err := s.CopyCVPDF(cvs[2], downloads, false); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("copy of an unbuilt CV err = %v, want ErrNotExist", err)
	}
}

func TestCVsWithoutACVFolderIsEmpty(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cvs, err := s.CVs(); err != nil || len(cvs) != 0 {
		t.Fatalf("CVs = %v, %v", cvs, err)
	}
}
