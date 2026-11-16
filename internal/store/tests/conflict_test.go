package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// racingFS lets another writer change meta.yml after the temporary file is durable and
// before the rename, the window update must guard; times is how often it does so.
type racingFS struct {
	meta   string
	times  *int
	synced *[]string
}
type racingFile struct {
	*os.File
	fs racingFS
}

func (f racingFS) CreateTemp(d, p string) (store.AtomicFile, error) {
	v, e := os.CreateTemp(d, p)
	return racingFile{v, f}, e
}
func (f racingFS) Rename(a, b string) error                  { return os.Rename(a, b) }
func (f racingFS) Remove(p string) error                     { return os.Remove(p) }
func (f racingFS) MkdirAll(p string, mode os.FileMode) error { return os.MkdirAll(p, mode) }
func (f racingFS) Mkdir(p string, mode os.FileMode) error    { return os.Mkdir(p, mode) }
func (f racingFS) RemoveAll(p string) error                  { return os.RemoveAll(p) }
func (f racingFS) SyncDir(p string) error                    { *f.synced = append(*f.synced, p); return nil }
func (f racingFile) Sync() error {
	if *f.fs.times > 0 {
		*f.fs.times--
		b, _ := os.ReadFile(f.fs.meta)
		if !strings.Contains(string(b), "fit: Good\n") {
			b = []byte(strings.Replace(string(b), "track: SWE\n", "track: SWE\nfit: Good\n", 1))
		} else {
			b = append(b, "# touched\n"...)
		}
		os.WriteFile(f.fs.meta, b, 0o644)
	}
	return f.File.Sync()
}

func openRacing(t *testing.T, times int) (*store.Store, string, *[]string) {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "jobs/SWE/orbit")
	synced := &[]string{}
	s, err := store.OpenWithFileSystem(root, racingFS{filepath.Join(folder, "meta.yml"), &times, synced})
	if err != nil {
		t.Fatal(err)
	}
	return s, folder, synced
}

func TestUpdateKeepsAConcurrentWrite(t *testing.T) {
	s, folder, synced := openRacing(t, 1)
	wrote, err := s.AppendStatus(folder, "Offer", time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC))
	if err != nil || !wrote {
		t.Fatal(wrote, err)
	}
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !strings.Contains(string(b), "fit: Good\n") || !strings.Contains(string(b), "{status: Offer, at: 2026-11-11T00:00:00Z}") {
		t.Fatalf("a write was lost:\n%s", b)
	}
	if len(*synced) == 0 || (*synced)[len(*synced)-1] != folder {
		t.Fatalf("parent directory not synced: %v", *synced)
	}
}

func TestUpdateGivesUpWithErrConflict(t *testing.T) {
	s, folder, _ := openRacing(t, 100)
	if _, err := s.AppendStatus(folder, "Offer", time.Now()); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if strings.Contains(string(b), "Offer") {
		t.Fatalf("conflicting write published:\n%s", b)
	}
}

func TestAppendStatusReportsNoOp(t *testing.T) {
	s, folder, _ := openRacing(t, 0)
	if wrote, err := s.AppendStatus(folder, "Applied", time.Now()); err != nil || wrote {
		t.Fatalf("no-op reported wrote=%v err=%v", wrote, err)
	}
}
