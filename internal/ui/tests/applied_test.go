package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

const draftMeta = "title: Role\ncompany: Company\ntrack: SWE\nstatus_log:\n  - {status: Draft, at: 2026-01-01T00:00:00Z}\n"

// appliedWorkspace builds a Workspace holding one Draft Application with the given files.
func appliedWorkspace(t *testing.T, files map[string]string) (root, folder string) {
	t.Helper()
	root = t.TempDir()
	folder = filepath.Join(root, "jobs", "SWE", "role")
	files["meta.yml"] = draftMeta
	for name, text := range files {
		path := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root, folder
}

// appliedSession starts the program on the Draft screen and returns a key sender and a waiter.
func appliedSession(t *testing.T, root string) (tm *driver, wait func(string)) {
	t.Helper()
	tm = newDriver(t, root, func() time.Time { return time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC) }, 120, 40)
	wait = func(text string) {
		t.Helper()
		if view := tm.plain(); !strings.Contains(view, text) {
			t.Fatalf("screen lacks %q:\n%s", text, view)
		}
	}
	tm.Type("2")
	wait("Drafts 1")
	return tm, wait
}

func quit(t *testing.T, tm *driver) {
	t.Helper()
	tm.Type("q")
}

func TestNotReadyMoveToAppliedPrompts(t *testing.T) {
	root, folder := appliedWorkspace(t, map[string]string{"resume.md": "written"})
	tm, wait := appliedSession(t, root)

	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("n")
	tm.Type("?")
	wait("Keys")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape}) // any key closes the help pop-up
	got, err := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if err != nil || string(got) != draftMeta {
		t.Fatalf("declining changed meta.yml: %q %v", got, err)
	}

	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("→ Applied")
	got, _ = os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !bytes.Contains(got, []byte("{status: Applied, at: 2026-02-01T00:00:00Z}")) {
		t.Fatalf("Applied not appended: %q", got)
	}
	quit(t, tm)
}

func TestReadyMoveToAppliedHasNoPrompt(t *testing.T) {
	root, folder := appliedWorkspace(t, map[string]string{
		"resume.md":                       "written",
		"out/Owner Resume Role.pdf":       "r",
		"out/Owner Role Cover Letter.pdf": "c",
	})
	tm, wait := appliedSession(t, root)

	tm.Type("ma")
	wait("→ Applied")
	got, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !bytes.Contains(got, []byte("{status: Applied, at: 2026-02-01T00:00:00Z}")) {
		t.Fatalf("Applied not appended: %q", got)
	}
	quit(t, tm)
}

// sentFiles lists every regular file under sent/ with its contents, keyed by relative path.
func sentFiles(t *testing.T, folder string) map[string]string {
	t.Helper()
	out := map[string]string{}
	sent := filepath.Join(folder, "sent")
	err := filepath.WalkDir(sent, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		rel, _ := filepath.Rel(sent, path)
		out[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatalf("sent/ unreadable: %v", err)
	}
	return out
}

func equalFiles(a, b map[string]string) bool {
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

func TestAppliedRecordsExactlyTheListedFilesThatExist(t *testing.T) {
	// Only some listed files exist; notes.md and a stray
	// file are not on the list and must stay out of sent/.
	root, folder := appliedWorkspace(t, map[string]string{
		"resume.md":                 "resume text",
		"jd.md":                     "jd text",
		"gaps.md":                   "gaps text",
		"notes.md":                  "private notes",
		"scratch.txt":               "stray",
		"out/Owner Resume Role.pdf": "pdf bytes",
		"out/readme.txt":            "not a pdf",
	})
	tm, wait := appliedSession(t, root)

	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("→ Applied")
	quit(t, tm)

	want := map[string]string{
		"resume.md":                 "resume text",
		"jd.md":                     "jd text",
		"gaps.md":                   "gaps text",
		"out/Owner Resume Role.pdf": "pdf bytes",
	}
	if got := sentFiles(t, folder); !equalFiles(got, want) {
		t.Fatalf("sent/ = %v, want %v", got, want)
	}
}

func TestSecondMoveToAppliedLeavesExistingSentUntouched(t *testing.T) {
	root, folder := appliedWorkspace(t, map[string]string{"resume.md": "original"})
	tm, wait := appliedSession(t, root)

	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("→ Applied")
	tm.Type("3")
	wait("Applied 1")
	tm.Type("mw") // away from Applied
	wait("→ Withdrawn")
	before := sentFiles(t, folder)

	if err := os.WriteFile(filepath.Join(folder, "resume.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "jd.md"), []byte("new jd"), 0o644); err != nil {
		t.Fatal(err)
	}
	tm.Type("r")             // rescan so the new files are seen
	for i := 0; i < 4; i++ { // All → … → Closed
		tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("→ Applied")
	quit(t, tm)

	if got := sentFiles(t, folder); !equalFiles(got, before) {
		t.Fatalf("sent/ changed: %v, was %v", got, before)
	}
	if before["resume.md"] != "original" || len(before) != 1 {
		t.Fatalf("unexpected first record: %v", before)
	}
}

// Undoing the move to Applied that made sent/ sets it aside, deleting nothing, so the
// next real Applied freezes the documents as they are then.
func TestUndoAfterAppliedSetsSentAside(t *testing.T) {
	root, folder := appliedWorkspace(t, map[string]string{"resume.md": "original"})
	tm, wait := appliedSession(t, root)

	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("→ Applied")
	tm.Type("u")
	wait("Undid move ")
	wait("sent/ kept as sent.undone-2026-02-01/")

	aside, err := os.ReadFile(filepath.Join(folder, "sent.undone-2026-02-01", "resume.md"))
	if err != nil || string(aside) != "original" {
		t.Fatalf("set-aside copy = %q, %v", aside, err)
	}
	if _, err := os.Stat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ still there after undo: %v", err)
	}
	meta, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !bytes.HasSuffix(meta, []byte("{status: Draft, at: 2026-02-01T00:00:00Z}\n")) {
		t.Fatalf("undo did not append Draft: %q", meta)
	}

	if err := os.WriteFile(filepath.Join(folder, "resume.md"), []byte("finished"), 0o644); err != nil {
		t.Fatal(err)
	}
	tm.Type("r")
	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("→ Applied")
	quit(t, tm)
	if got := sentFiles(t, folder); !equalFiles(got, map[string]string{"resume.md": "finished"}) {
		t.Fatalf("sent/ after the real Applied = %v", got)
	}
}

// An undo of a move whose sent/ was already there leaves it alone.
func TestUndoKeepsASentItDidNotMake(t *testing.T) {
	root, folder := appliedWorkspace(t, map[string]string{"resume.md": "original", "sent/resume.md": "earlier"})
	tm, wait := appliedSession(t, root)
	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("→ Applied")
	tm.Type("u")
	wait("Undid move ")
	quit(t, tm)
	if got := sentFiles(t, folder); !equalFiles(got, map[string]string{"resume.md": "earlier"}) {
		t.Fatalf("sent/ after undo = %v", got)
	}
}

func TestMalformedMoveToAppliedShowsErrorWithoutPrompt(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "jobs", "SWE", "role")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	broken := "title: Role\ncompany: Company\nstatus_log:\n  - {status: Draft, at: 2026-01-01T00:00:00Z}\n" // no track
	if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, nil, ui.WithExternalRunner(fakeExternal(t)))
	if err != nil {
		t.Fatal(err)
	}
	m = runMsg(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m = driveModel(t, m, "2", "ma", "q") // the broken row still carries its logged Status, Draft
	view := m.View().Content
	if !strings.Contains(view, "add a track: line") || strings.Contains(view, "Docs not Ready") {
		t.Fatalf("want the store error and no prompt, got:\n%s", view)
	}

	if got, _ := os.ReadFile(filepath.Join(folder, "meta.yml")); string(got) != broken {
		t.Fatalf("malformed meta.yml changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ created for malformed Application: %v", err)
	}
}
