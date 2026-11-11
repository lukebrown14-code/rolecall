package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/store"
)

func TestPatchPreservesOtherBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	folder := filepath.Join(root, "jobs/SWE/orbit")
	before, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if err := s.Patch(folder, map[string]string{"fit": "Poor"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if string(after) == string(before) {
		t.Fatal("patch did not change Fit")
	}
}

func TestAppendStatus(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	folder := filepath.Join(root, "jobs/SWE/orbit")
	if err := s.AppendStatus(folder, "Offer", time.Date(2026, 11, 11, 1, 2, 3, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !strings.Contains(string(b), "  - {status: Offer, at: 2026-11-11T01:02:03Z}\n") {
		t.Fatal(string(b))
	}
	if err := s.AppendStatus(folder, "Offer", time.Now()); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if string(after) != string(b) {
		t.Fatal("duplicate changed bytes")
	}
}

func TestWritesRefuseMalformedAndInvalidValues(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	for _, name := range []string{"malformed", "orbit"} {
		folder := filepath.Join(root, "jobs/SWE", name)
		path := filepath.Join(folder, "meta.yml")
		before, _ := os.ReadFile(path)
		if err := s.AppendStatus(folder, "invalid", time.Now()); err == nil {
			t.Fatal("accepted invalid Status")
		}
		if name == "malformed" {
			if err := s.Patch(folder, map[string]string{"fit": "Good"}); err == nil {
				t.Fatal("patched malformed")
			}
			if err := s.AppendStatus(folder, "Offer", time.Now()); err == nil {
				t.Fatal("appended malformed")
			}
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(before) {
			t.Fatal("changed rejected write")
		}
	}
}

func TestPatchRereadsAndPreservesComments(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	if _, err := s.Scan(); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "jobs/SWE/orbit")
	path := filepath.Join(folder, "meta.yml")
	before := "schema: 1\ntitle: Engineer # keep title\ncompany: Changed externally\ntrack: SWE\nlegacy: untouched # keep unknown\nstatus_log: []\n"
	if err := os.WriteFile(path, []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.Patch(folder, map[string]string{"title": "Senior Engineer", "fit": "Good"}); err != nil {
		t.Fatal(err)
	}
	want := "schema: 1\ntitle: \"Senior Engineer\" # keep title\ncompany: Changed externally\ntrack: SWE\nlegacy: untouched # keep unknown\nfit: \"Good\"\nstatus_log: []\n"
	got, _ := os.ReadFile(path)
	if string(got) != want {
		t.Fatalf("got %s", got)
	}
	files, _ := filepath.Glob(filepath.Join(folder, ".meta-*"))
	if len(files) != 0 {
		t.Fatal(files)
	}
}
