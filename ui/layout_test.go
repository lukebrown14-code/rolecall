package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

// The Applied screen opens on All: every Application past Draft, one section per table.
func TestAllViewGroupsApplicationsByTable(t *testing.T) {
	view := ansi.Strip(finalView(t, "3"))
	var at []int
	for _, section := range []string{"Applied · 1", "Interviewing · 2", "Offer · 2", "Closed · 4"} {
		i := strings.Index(view, section)
		if i < 0 {
			t.Fatalf("All view lacks section %q:\n%s", section, view)
		}
		at = append(at, i)
	}
	for i := 1; i < len(at); i++ {
		if at[i] < at[i-1] {
			t.Fatalf("sections out of order:\n%s", view)
		}
	}
}

// A Lead whose close date has passed moves to an Expired section at the end of Leads.
func TestExpiredLeadsSortLastUnderTheirOwnSection(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(root, "jobs", "SWE", "fern", "meta.yml")
	b, _ := os.ReadFile(meta)
	if err := os.WriteFile(meta, bytes.Replace(b, []byte("track: Platform\n"), []byte("track: Platform\ncloses: 2025-12-01\n"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	view := ansi.Strip(runKeys(t, root, 120, "1"))
	expired, fern, empty := strings.Index(view, "Expired · 1"), strings.Index(view, "Platform Engineer"), strings.Index(view, "Empty log")
	if expired < 0 || fern < expired || empty > expired || !strings.Contains(view, "expired 4w ago") {
		t.Fatalf("expired Lead not last under Expired:\n%s", view)
	}
}

// An error stays on the feedback line through other keys until esc dismisses it.
func TestErrorsStayUntilEsc(t *testing.T) {
	const want = "invalid status_log entry"
	if view := ansi.Strip(finalView(t, "G", "e", "j")); !strings.Contains(view, want) {
		t.Fatalf("error cleared by another key:\n%s", view)
	}
	view := ansi.Strip(runKeysThen(t, func(tm *teatest.TestModel) {
		tm.Type("G")
		tm.Type("e")
		tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	}))
	if strings.Contains(view, want) {
		t.Fatalf("esc did not dismiss the error:\n%s", view)
	}
}

// a in the overwrite prompt overwrites this file and every later clash without asking again.
func TestDownloadOverwriteAll(t *testing.T) {
	h, downloads := newDownloadHarness(t, clash)
	h.open("3", 4, "docs-ready")
	h.tm.Type("d")
	h.wait("File exists")
	h.tm.Type("a")
	h.lastRun()
	for _, name := range []string{"Owner Resume Role.pdf", "Owner Role Cover Letter.pdf"} {
		want := readFile(t, filepath.Join(h.folder("docs-ready"), "out", name))
		if got := readFile(t, filepath.Join(downloads, name)); got != want {
			t.Fatalf("%s not overwritten: %q", name, got)
		}
	}
	h.show("Saved 2 PDFs → ")
}

// On a narrow terminal nothing runs past the right edge: the screens row sheds
// detail, the preview pane is hidden and pop-ups shrink to fit.
func TestNarrowTerminalKeepsEveryLineInside(t *testing.T) {
	const width = 76
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{{"1"}, {"3"}, {"m"}, {"a"}, {"?"}, {"3", "enter"}} {
		view := runKeys(t, root, width, keys...)
		for _, line := range strings.Split(view, "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Fatalf("keys %q: line is %d cells wide, terminal is %d:\n%s", keys, w, width, ansi.Strip(view))
			}
		}
	}
}

// runKeys runs the program on a Workspace at the given width, types keys ("enter" sends
// Enter), and returns the final screen.
func runKeys(t *testing.T, root string, width int, keys ...string) string {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(width, 40))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("rolecall")) })
	for _, k := range keys {
		if k == "enter" {
			tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
		} else {
			tm.Type(k)
		}
	}
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
}

// runKeysThen runs the program on the fixture Workspace, lets drive send keys, and
// returns the final screen.
func runKeysThen(t *testing.T, drive func(*teatest.TestModel)) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(selectionWidth, 40))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("rolecall")) })
	drive(tm)
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
}
