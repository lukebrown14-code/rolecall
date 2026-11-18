package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/doctor"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// sentFS fails the failAt-th CreateTemp (1-based), or every Rename onto failRename's base
// name, and passes everything else to the OS.
type sentFS struct {
	failAt     int
	failRename string
	n          *int
}

func (f sentFS) CreateTemp(d, p string) (store.AtomicFile, error) {
	*f.n++
	if *f.n == f.failAt {
		return nil, errors.New("injected disk full")
	}
	return os.CreateTemp(d, p)
}
func (f sentFS) Rename(a, b string) error {
	if f.failRename != "" && filepath.Base(b) == f.failRename {
		return errors.New("injected rename")
	}
	return os.Rename(a, b)
}
func (f sentFS) Remove(p string) error                     { return os.Remove(p) }
func (f sentFS) MkdirAll(p string, mode os.FileMode) error { return os.MkdirAll(p, mode) }
func (f sentFS) Mkdir(p string, mode os.FileMode) error    { return os.Mkdir(p, mode) }
func (f sentFS) RemoveAll(p string) error                  { return os.RemoveAll(p) }
func (f sentFS) SyncDir(string) error                      { return nil }

const sentDraftMeta = "title: Role\ncompany: Company\ntrack: SWE\nstatus_log:\n  - {status: Draft, at: 2026-01-01T00:00:00Z}\n"

// sentWorkspace is a private Workspace holding one Draft Application with files (0600).
func sentWorkspace(t *testing.T, files map[string]string) (root, folder string) {
	t.Helper()
	root = t.TempDir()
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	folder = filepath.Join(root, "jobs", "SWE", "role")
	files["meta.yml"] = sentDraftMeta
	for name, text := range files {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, dir := range []string{filepath.Join(root, "jobs"), filepath.Join(root, "jobs", "SWE")} {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return root, folder
}

// sentTree lists every entry under folder/sent with its contents ("" for a folder).
func sentTree(t *testing.T, folder string) map[string]string {
	t.Helper()
	got := map[string]string{}
	sent := filepath.Join(folder, "sent")
	err := filepath.WalkDir(sent, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(sent, path)
		if d.IsDir() {
			if rel != "." {
				got[rel+"/"] = ""
			}
			return nil
		}
		b, err := os.ReadFile(path)
		got[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatalf("sent/: %v", err)
	}
	return got
}

// noStaging fails if a sent.tmp-* folder is left in folder.
func noStaging(t *testing.T, folder string) {
	t.Helper()
	entries, _ := os.ReadDir(folder)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "sent.tmp-") {
			t.Fatalf("staging folder left behind: %s", e.Name())
		}
	}
}

var readyFiles = map[string]string{
	"resume.md": "resume", "cover-letter.md": "letter", "jd.md": "jd",
	"out/Owner Resume Role.pdf": "%PDF resume", "out/Owner Role Cover Letter.pdf": "%PDF letter",
}

func copyMap(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// A copy that fails part way leaves no sent/, so the next call builds a whole one.
func TestRecordSentFailureLeavesNoSentAndRetryCompletes(t *testing.T) {
	root, folder := sentWorkspace(t, copyMap(readyFiles))
	n := 0
	s, err := store.OpenWithFileSystem(root, sentFS{failAt: 2, n: &n})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RecordSent(folder); err == nil {
		t.Fatal("injected failure not returned")
	}
	if _, err := os.Lstat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ exists after a failed copy: %v", err)
	}
	noStaging(t, folder)

	s, _ = store.Open(root)
	if err := s.RecordSent(folder); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"resume.md": "resume", "cover-letter.md": "letter", "jd.md": "jd", "out/": "",
		"out/Owner Resume Role.pdf": "%PDF resume", "out/Owner Role Cover Letter.pdf": "%PDF letter"}
	if got := sentTree(t, folder); !equalTree(got, want) {
		t.Fatalf("sent/ after retry = %v, want %v", got, want)
	}
	noStaging(t, folder)
}

// The final rename failing leaves no sent/ and no staging folder either.
func TestRecordSentRenameFailureLeavesNothing(t *testing.T) {
	root, folder := sentWorkspace(t, copyMap(readyFiles))
	n := 0
	s, _ := store.OpenWithFileSystem(root, sentFS{failRename: "sent", n: &n})
	if err := s.RecordSent(folder); err == nil {
		t.Fatal("injected rename failure not returned")
	}
	if _, err := os.Lstat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ exists: %v", err)
	}
	noStaging(t, folder)
}

// A symlinked doc or PDF is not the harness's file: it is not copied, while regular
// files are, byte for byte with their mode.
func TestRecordSentSkipsSymlinks(t *testing.T) {
	root, folder := sentWorkspace(t, map[string]string{"resume.md": "resume", "out/Owner Resume Role.pdf": "%PDF"})
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("outside the Workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(folder, "resume.md"), 0o640); err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{"fit.md", "jd.md", "out/Linked.pdf"} {
		if err := os.Symlink(secret, filepath.Join(folder, link)); err != nil {
			t.Fatal(err)
		}
	}
	s, _ := store.Open(root)
	if err := s.RecordSent(folder); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"resume.md": "resume", "out/": "", "out/Owner Resume Role.pdf": "%PDF"}
	if got := sentTree(t, folder); !equalTree(got, want) {
		t.Fatalf("sent/ = %v, want %v", got, want)
	}
	info, err := os.Stat(filepath.Join(folder, "sent", "resume.md"))
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("sent/resume.md mode = %v, %v; want 0640", info, err)
	}
}

// sent/ is as private as the rest of the Workspace, so doctor says nothing about it.
func TestRecordSentIsPrivate(t *testing.T) {
	root, folder := sentWorkspace(t, copyMap(readyFiles))
	s, _ := store.Open(root)
	if err := s.RecordSent(folder); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"sent", "sent/out"} {
		info, err := os.Stat(filepath.Join(folder, dir))
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode = %v, %v; want 0700", dir, info, err)
		}
	}
	warnings, err := doctor.Check(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) > 0 {
		t.Fatalf("doctor warns after Applied: %v", warnings)
	}
}

// An existing sent/ is never touched, and Move says it made none.
func TestMoveKeepsAnExistingSent(t *testing.T) {
	root, folder := sentWorkspace(t, map[string]string{"resume.md": "new", "sent/resume.md": "earlier"})
	s, _ := store.Open(root)
	r, err := s.Move(folder, store.StatusApplied, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || !r.Wrote || r.MadeSent {
		t.Fatalf("Move = %+v, %v; want wrote, no sent made", r, err)
	}
	if got := sentTree(t, folder); !equalTree(got, map[string]string{"resume.md": "earlier"}) {
		t.Fatalf("sent/ = %v", got)
	}
}

// Move to Applied freezes sent/ and appends; a failed copy appends nothing.
func TestMoveToAppliedFreezesThenAppends(t *testing.T) {
	at := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	root, folder := sentWorkspace(t, copyMap(readyFiles))
	meta := filepath.Join(folder, "meta.yml")
	n := 0
	failing, _ := store.OpenWithFileSystem(root, sentFS{failAt: 2, n: &n})
	if _, err := failing.Move(folder, store.StatusApplied, at); err == nil {
		t.Fatal("injected failure not returned")
	}
	if b, _ := os.ReadFile(meta); string(b) != sentDraftMeta {
		t.Fatalf("meta.yml changed by a failed move: %q", b)
	}

	s, _ := store.Open(root)
	r, err := s.Move(folder, store.StatusApplied, at)
	if err != nil || !r.Wrote || !r.MadeSent {
		t.Fatalf("Move = %+v, %v", r, err)
	}
	if b, _ := os.ReadFile(meta); !strings.Contains(string(b), "{status: Applied, at: 2026-02-01T00:00:00Z}") {
		t.Fatalf("Applied not appended: %q", b)
	}
	if len(sentTree(t, folder)) == 0 {
		t.Fatal("no sent/")
	}
}

// A failed append removes the sent/ the move just made, so nothing is half moved.
func TestMoveAppendFailureRemovesTheSentItMade(t *testing.T) {
	root, folder := sentWorkspace(t, copyMap(readyFiles))
	n := 0
	s, _ := store.OpenWithFileSystem(root, sentFS{failRename: "meta.yml", n: &n})
	if _, err := s.Move(folder, store.StatusApplied, time.Now()); err == nil {
		t.Fatal("injected append failure not returned")
	}
	if _, err := os.Lstat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ left after a failed append: %v", err)
	}
}

// A move to the Status it already has writes nothing: an Applied Application without
// sent/ (imported, say) is not frozen from today's files.
func TestMoveToCurrentStatusWritesNothing(t *testing.T) {
	root, folder := sentWorkspace(t, copyMap(readyFiles))
	meta := filepath.Join(folder, "meta.yml")
	applied := sentDraftMeta + "  - {status: Applied, at: 2026-01-02T00:00:00Z}\n"
	if err := os.WriteFile(meta, []byte(applied), 0o600); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Open(root)
	r, err := s.Move(folder, store.StatusApplied, time.Now())
	if err != nil || r.Wrote || r.MadeSent {
		t.Fatalf("Move = %+v, %v; want nothing done", r, err)
	}
	if b, _ := os.ReadFile(meta); string(b) != applied {
		t.Fatalf("meta.yml changed: %q", b)
	}
	if _, err := os.Lstat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ made for an Application already Applied: %v", err)
	}
}

// Moves to other Statuses make no sent/.
func TestMoveElsewhereMakesNoSent(t *testing.T) {
	root, folder := sentWorkspace(t, copyMap(readyFiles))
	s, _ := store.Open(root)
	r, err := s.Move(folder, store.StatusInterviewing, time.Now())
	if err != nil || !r.Wrote || r.MadeSent {
		t.Fatalf("Move = %+v, %v", r, err)
	}
	if _, err := os.Lstat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ made by a move to Interviewing: %v", err)
	}
	if _, err := s.Move(folder, "Hired", time.Now()); err == nil {
		t.Fatal("unknown Status accepted")
	}
}

func equalTree(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}
