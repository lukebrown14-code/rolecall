package store_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// A folder that is not an Application is not a write target: every folder-scoped
// method refuses it before touching anything.
func TestApplicationGateRefusesFolderWithoutMetadata(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "jobs/SWE/not-an-application")
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadDocument(folder, "notes.md"); err == nil {
		t.Fatal("ReadDocument accepted a folder without meta.yml")
	}
	if err := s.WriteDocument(folder, "jd.md", []byte("x")); err == nil {
		t.Fatal("WriteDocument accepted a folder without meta.yml")
	}
	if err := s.EnsureNotes(folder); err == nil {
		t.Fatal("EnsureNotes accepted a folder without meta.yml")
	}
	if err := s.RecordSent(folder); err == nil {
		t.Fatal("RecordSent accepted a folder without meta.yml")
	}
	if err := s.CopyPDF(folder, "x.pdf", t.TempDir(), false); err == nil {
		t.Fatal("CopyPDF accepted a folder without meta.yml")
	}
	if err := s.Patch(folder, map[string]string{"fit": "Good"}); err == nil {
		t.Fatal("Patch accepted a folder without meta.yml")
	}
	if _, err := s.AppendStatus(folder, "Offer", time.Now()); err == nil {
		t.Fatal("AppendStatus accepted a folder without meta.yml")
	}
}

// A malformed Application is still an Application, but its write methods refuse it.
func TestApplicationGateRefusesMalformedFolder(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "jobs/SWE/malformed")
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteDocument(folder, "jd.md", []byte("x")); err == nil {
		t.Fatal("WriteDocument accepted a malformed Application")
	}
	if err := s.EnsureNotes(folder); err == nil {
		t.Fatal("EnsureNotes accepted a malformed Application")
	}
	if err := s.RecordSent(folder); err == nil {
		t.Fatal("RecordSent accepted a malformed Application")
	}
	if err := s.CopyPDF(folder, "x.pdf", t.TempDir(), false); err == nil {
		t.Fatal("CopyPDF accepted a malformed Application")
	}
}

// Every write routed through the injected filesystem fails when the filesystem does.
func TestFailureInjectionCoversEveryWrite(t *testing.T) {
	for _, stage := range []string{"write", "rename"} {
		for _, tc := range []struct {
			name string
			run  func(t *testing.T, s *store.Store, root string) error
		}{
			{"write document", func(t *testing.T, s *store.Store, root string) error {
				return s.WriteDocument(filepath.Join(root, "jobs/SWE/orbit"), "resume.md", []byte("draft"))
			}},
			{"ensure notes", func(t *testing.T, s *store.Store, root string) error {
				return s.EnsureNotes(filepath.Join(root, "jobs/SWE/orbit"))
			}},
			{"copy pdf", func(t *testing.T, s *store.Store, root string) error {
				return s.CopyPDF(filepath.Join(root, "jobs/SWE/orbit"), "Resume.pdf", t.TempDir(), false)
			}},
			{"record sent", func(t *testing.T, s *store.Store, root string) error {
				return s.RecordSent(filepath.Join(root, "jobs/SWE/orbit"))
			}},
		} {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
					t.Fatal(err)
				}
				out := filepath.Join(root, "jobs/SWE/orbit", "out")
				if err := os.MkdirAll(out, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(out, "Resume.pdf"), []byte("%PDF-1.4\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				s, err := store.OpenWithFileSystem(root, failingFS{stage})
				if err != nil {
					t.Fatal(err)
				}
				if err := tc.run(t, s, root); err == nil {
					t.Fatal("expected injected failure")
				}
			})
		}
	}
}

// Initialize writes through the same seam.
func TestFailureInjectionCoversInitialize(t *testing.T) {
	for _, stage := range []string{"write", "rename"} {
		t.Run(stage, func(t *testing.T) {
			root := t.TempDir()
			if err := store.InitializeWorkspace(root, nil, nil, store.SearchArea{}, failingFS{stage}); err == nil {
				t.Fatal("expected injected failure")
			}
		})
	}
}

// Folder creation is injectable too: a failing Mkdir fails both Create and Initialize,
// so no test path reaches the real OS for a folder a write needs.
func TestFailureInjectionCoversFolderCreation(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.OpenWithFileSystem(root, failingFS{"mkdir"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(store.CreateFields{Title: "New", Company: "New", Track: "SWE"}, time.Now()); err == nil {
		t.Fatal("Create ignored a failed folder creation")
	}
	if err := store.InitializeWorkspace(t.TempDir(), nil, nil, store.SearchArea{}, failingFS{"mkdir"}); err == nil {
		t.Fatal("InitializeWorkspace ignored a failed folder creation")
	}
}
