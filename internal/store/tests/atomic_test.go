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

type failingFS struct{ stage string }
type failingFile struct {
	*os.File
	stage string
}

func (f failingFS) CreateTemp(d, p string) (store.AtomicFile, error) {
	v, e := os.CreateTemp(d, p)
	return failingFile{v, f.stage}, e
}
func (f failingFS) Rename(a, b string) error {
	if f.stage == "rename" {
		return errors.New("injected rename")
	}
	return os.Rename(a, b)
}
func (f failingFS) Remove(p string) error { return os.Remove(p) }
func (f failingFile) Write(b []byte) (int, error) {
	if f.stage == "write" {
		if len(b) > 0 {
			f.File.Write(b[:1])
		}
		return 0, errors.New("injected write")
	}
	return f.File.Write(b)
}
func (f failingFile) Sync() error {
	if f.stage == "sync" {
		return errors.New("injected sync")
	}
	return f.File.Sync()
}
func TestAtomicFailuresPreserveOriginal(t *testing.T) {
	for _, stage := range []string{"write", "sync", "rename"} {
		for _, operation := range []string{"patch", "append", "create"} {
			t.Run(stage+"/"+operation, func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
					t.Fatal(err)
				}
				s, err := store.OpenWithFileSystem(root, failingFS{stage})
				if err != nil {
					t.Fatal(err)
				}
				folder := filepath.Join(root, "jobs/SWE/orbit")
				path := filepath.Join(folder, "meta.yml")
				before, _ := os.ReadFile(path)
				switch operation {
				case "patch":
					err = s.Patch(folder, map[string]string{"title": "Changed"})
				case "append":
					err = s.AppendStatus(folder, "Offer", time.Now())
				case "create":
					_, err = s.Create(store.CreateFields{Title: "New", Company: "New", Track: "SWE"}, time.Now())
				}
				if err == nil {
					t.Fatal("expected injected failure")
				}
				after, _ := os.ReadFile(path)
				if string(after) != string(before) {
					t.Fatal("original changed")
				}
				filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
					if e != nil {
						t.Fatal(e)
					}
					if strings.HasPrefix(d.Name(), ".meta-") {
						t.Fatalf("residue %s", p)
					}
					return nil
				})
			})
		}
	}
}
