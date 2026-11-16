package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func TestPatchPreservesOtherBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
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
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	folder := filepath.Join(root, "jobs/SWE/orbit")
	if _, err := s.AppendStatus(folder, "Offer", time.Date(2026, 11, 11, 1, 2, 3, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !strings.Contains(string(b), "  - {status: Offer, at: 2026-11-11T01:02:03Z}\n") {
		t.Fatal(string(b))
	}
	if _, err := s.AppendStatus(folder, "Offer", time.Now()); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if string(after) != string(b) {
		t.Fatal("duplicate changed bytes")
	}
}

func TestWritesRefuseMalformedAndInvalidValues(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	for _, name := range []string{"malformed", "orbit"} {
		folder := filepath.Join(root, "jobs/SWE", name)
		path := filepath.Join(folder, "meta.yml")
		before, _ := os.ReadFile(path)
		if _, err := s.AppendStatus(folder, "invalid", time.Now()); err == nil {
			t.Fatal("accepted invalid Status")
		}
		if name == "malformed" {
			if err := s.Patch(folder, map[string]string{"fit": "Good"}); err == nil {
				t.Fatal("patched malformed")
			}
			if _, err := s.AppendStatus(folder, "Offer", time.Now()); err == nil {
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
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
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

// followed_up is set as one patched line before status_log, read back, and removed again
// leaving the file as it was. A value that is not a date reads as unset, not malformed.
func TestSetFollowedUp(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	folder := filepath.Join(root, "jobs/SWE/orbit")
	path := filepath.Join(folder, "meta.yml")
	before, _ := os.ReadFile(path)
	if err := s.SetFollowedUp(folder, "2026-11-16"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "followed_up: \"2026-11-16\"\nstatus_log:") {
		t.Fatalf("followed_up not written before status_log:\n%s", b)
	}
	a, err := s.Rescan(folder)
	if err != nil || a == nil || a.FollowedUp != "2026-11-16" {
		t.Fatalf("read back %+v (%v)", a, err)
	}
	if err := s.SetFollowedUp(folder, "16 Nov"); err == nil {
		t.Fatal("a non-date was written")
	}
	if err := s.SetFollowedUp(folder, ""); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("removing followed_up did not restore the file:\n%s", after)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(before), "status_log:", "followed_up: soon\nstatus_log:", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if a, err := s.Rescan(folder); err != nil || a.Malformed != nil || a.FollowedUp != "" {
		t.Fatalf("an unreadable followed_up: %+v (%v)", a, err)
	}
}
