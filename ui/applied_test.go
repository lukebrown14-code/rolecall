package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
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
func appliedSession(t *testing.T, root string) (tm *teatest.TestModel, wait func(string)) {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, func() time.Time { return time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	tm = teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	wait = func(text string) {
		t.Helper()
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(text)) }, teatest.WithDuration(3*time.Second))
	}
	wait("Leads")
	tm.Type("2")
	wait("Draft (1)")
	return tm, wait
}

func quit(t *testing.T, tm *teatest.TestModel) {
	t.Helper()
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestNotReadyMoveToAppliedPrompts(t *testing.T) {
	root, folder := appliedWorkspace(t, map[string]string{"resume.md": "written"})
	tm, wait := appliedSession(t, root)

	tm.Type("ma")
	wait("Docs not Ready. Mark Applied anyway? y/n")
	tm.Type("n")
	tm.Type("?")
	wait("Full help")
	got, err := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if err != nil || string(got) != draftMeta {
		t.Fatalf("declining changed meta.yml: %q %v", got, err)
	}

	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("Moved to Applied")
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
	wait("Moved to Applied")
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
	wait("Moved to Applied")
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
	wait("Moved to Applied")
	tm.Type("3")
	wait("Applied (1)")
	tm.Type("mw") // away from Applied
	wait("Moved to Withdrawn")
	before := sentFiles(t, folder)

	if err := os.WriteFile(filepath.Join(folder, "resume.md"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "jd.md"), []byte("new jd"), 0o644); err != nil {
		t.Fatal(err)
	}
	tm.Type("r") // rescan so the new files are seen
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("Moved to Applied")
	quit(t, tm)

	if got := sentFiles(t, folder); !equalFiles(got, before) {
		t.Fatalf("sent/ changed: %v, was %v", got, before)
	}
	if before["resume.md"] != "original" || len(before) != 1 {
		t.Fatalf("unexpected first record: %v", before)
	}
}

func TestUndoAfterAppliedLeavesSentInPlace(t *testing.T) {
	root, folder := appliedWorkspace(t, map[string]string{"resume.md": "original"})
	tm, wait := appliedSession(t, root)

	tm.Type("ma")
	wait("Docs not Ready")
	tm.Type("y")
	wait("Moved to Applied")
	tm.Type("u")
	wait("Moved to Draft")
	quit(t, tm)

	if got := sentFiles(t, folder); !equalFiles(got, map[string]string{"resume.md": "original"}) {
		t.Fatalf("sent/ after undo = %v", got)
	}
	meta, _ := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if !bytes.HasSuffix(meta, []byte("{status: Draft, at: 2026-02-01T00:00:00Z}\n")) {
		t.Fatalf("undo did not append Draft: %q", meta)
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
	m, err := ui.New(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("Leads")) }, teatest.WithDuration(3*time.Second))
	tm.Type("2") // the broken row still carries its logged Status, Draft
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("⚠")) }, teatest.WithDuration(3*time.Second))
	tm.Type("ma")
	tm.Type("q")
	view := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
	if !strings.Contains(view, "missing track") || strings.Contains(view, "Docs not Ready") {
		t.Fatalf("want the store error and no prompt, got:\n%s", view)
	}

	if got, _ := os.ReadFile(filepath.Join(folder, "meta.yml")); string(got) != broken {
		t.Fatalf("malformed meta.yml changed: %q", got)
	}
	if _, err := os.Stat(filepath.Join(folder, "sent")); !os.IsNotExist(err) {
		t.Fatalf("sent/ created for malformed Application: %v", err)
	}
}
