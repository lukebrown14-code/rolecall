package store_test

import (
	"github.com/lukebrown14-code/rolecall/store"
	"os"
	"path/filepath"
	"testing"
)

func TestFolderDocuments(t *testing.T) {
	for _, tc := range []struct {
		name, resume      string
		only, pdf, letter bool
		want              string
	}{
		{"none", " \n", false, false, false, ""}, {"written", "draft", false, false, false, "Written"}, {"partial", "draft", false, true, false, "Written"}, {"ready", "", false, true, true, "Ready"}, {"resume only", "", true, true, false, "Ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(root, "jobs/SWE/fern")
			if tc.only {
				f, _ := os.OpenFile(filepath.Join(dir, "meta.yml"), os.O_APPEND|os.O_WRONLY, 0600)
				f.WriteString("resume_only: true\n")
				f.Close()
			}
			os.WriteFile(filepath.Join(dir, "resume.md"), []byte(tc.resume), 0600)
			os.Mkdir(filepath.Join(dir, "out"), 0700)
			if tc.pdf {
				os.WriteFile(filepath.Join(dir, "out/Owner Resume Role.pdf"), nil, 0600)
			}
			if tc.letter {
				os.WriteFile(filepath.Join(dir, "out/Owner Role Cover Letter.pdf"), nil, 0600)
			}
			s, _ := store.Open(root)
			app, err := s.Rescan(dir)
			if err != nil {
				t.Fatal(err)
			}
			if app.Documents != tc.want {
				t.Fatalf("got %q want %q", app.Documents, tc.want)
			}
		})
	}
}

func TestWorkspaceReads(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(root, "workspace.json"))
	tracks, err := s.Tracks()
	if err != nil || len(tracks) != 0 {
		t.Fatalf("missing config: %v %v", tracks, err)
	}
	os.WriteFile(filepath.Join(root, "workspace.json"), []byte(`{"tracks":["SWE","Platform"]}`), 0600)
	tracks, err = s.Tracks()
	if err != nil || len(tracks) != 2 || tracks[0] != "SWE" || tracks[1] != "Platform" {
		t.Fatalf("tracks: %v %v", tracks, err)
	}
	os.WriteFile(filepath.Join(root, "workspace.json"), []byte(`{"gates":{}}`), 0600)
	tracks, err = s.Tracks()
	if err != nil || len(tracks) != 0 {
		t.Fatalf("no tracks: %v %v", tracks, err)
	}
	dir := filepath.Join(root, "jobs/SWE/fern")
	os.WriteFile(filepath.Join(dir, "resume.md"), []byte("draft"), 0600)
	a, err := s.Rescan(dir)
	if err != nil || a == nil || a.Documents != "Written" {
		t.Fatalf("rescan: %v %v", a, err)
	}
	os.Remove(filepath.Join(dir, "resume.md"))
	a, err = s.Rescan(dir)
	if err != nil || a.Documents != "" {
		t.Fatalf("removed document: %v %v", a, err)
	}
	for _, name := range []string{"fern", "orbit"} {
		f, _ := os.OpenFile(filepath.Join(root, "jobs/SWE", name, "meta.yml"), os.O_APPEND|os.O_WRONLY, 0600)
		f.WriteString("url: https://example.test/role\n")
		f.Close()
	}
	matches, err := s.FindByURL("https://example.test/role")
	if err != nil || len(matches) != 2 {
		t.Fatalf("matches: %v %v", matches, err)
	}
	matches, err = s.FindByURL("https://example.test/shared")
	if err != nil || len(matches) != 6 {
		t.Fatalf("all Statuses: %v %v", matches, err)
	}
	os.RemoveAll(dir)
	a, err = s.Rescan(dir)
	if err != nil || a != nil {
		t.Fatalf("gone: %v %v", a, err)
	}
}
