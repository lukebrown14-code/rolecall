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
	if err := os.WriteFile(meta, bytes.Replace(b, []byte("track: Platform\n"), []byte("track: Platform\ncloses: 2026-09-01\n"), 1), 0o600); err != nil {
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

// runKeys runs the program on a Workspace at the given width and returns the final
// screen. Keys are typed as text, except the names drive understands.
func runKeys(t *testing.T, root string, width int, keys ...string) string {
	t.Helper()
	return runKeysSized(t, root, width, termHeight, keys...)
}

const termHeight = 40

func runKeysSized(t *testing.T, root string, width, height int, keys ...string) string {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(width, height))
	// Any first frame will do: narrow screens drop "rolecall", tiny ones every word.
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return len(b) > 0 })
	drive(tm, keys...)
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
}

// drive sends keys: "enter", "esc", "tab", "backspace" and the arrows by name, "paste:…" as a
// bracketed paste, anything else typed as text.
func drive(tm *teatest.TestModel, keys ...string) {
	named := map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "backspace": tea.KeyBackspace,
		"right": tea.KeyRight, "left": tea.KeyLeft, "up": tea.KeyUp, "down": tea.KeyDown}
	for _, k := range keys {
		if code, ok := named[k]; ok {
			tm.Send(tea.KeyPressMsg{Code: code})
		} else if text, ok := strings.CutPrefix(k, "paste:"); ok {
			tm.Send(tea.PasteMsg{Content: text})
		} else {
			tm.Type(k)
		}
	}
}

// fixtureCopy is a fresh copy of the fixture Workspace, with edit applied to it.
func fixtureCopy(t *testing.T, edit func(root string)) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(root)
	}
	return root
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
	m, err := ui.New(s, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(selectionWidth, termHeight))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("rolecall")) })
	drive(tm)
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
}

// At any size, from a sliver to the minimum and just above it, every screen and pop-up
// stays inside the terminal; below the minimum a notice says so instead.
func TestEverySizeFitsTheTerminal(t *testing.T) {
	root := fixtureCopy(t, nil)
	sizes := [][2]int{{1, 1}, {10, 3}, {49, 30}, {120, 11}, {50, 12}, {64, 13}}
	states := [][]string{{"1"}, {"3"}, {"m"}, {"a"}, {"?"}, {"3", "enter"}, {"G", "e"}}
	for _, size := range sizes {
		for _, keys := range states {
			w, h := size[0], size[1]
			view := runKeysSized(t, root, w, h, keys...)
			lines := strings.Split(view, "\n")
			if len(lines) > h {
				t.Fatalf("%dx%d keys %q: %d lines:\n%s", w, h, keys, len(lines), ansi.Strip(view))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > w {
					t.Fatalf("%dx%d keys %q: a line is %d cells wide:\n%s", w, h, keys, ansi.StringWidth(line), ansi.Strip(view))
				}
			}
			// The notice needs a few cells to be readable at all.
			if small := w < 50 || h < 12; w >= 20 && h >= 2 && small != strings.Contains(ansi.Strip(view), "too small") {
				t.Fatalf("%dx%d: too-small notice shown = %v:\n%s", w, h, !small, ansi.Strip(view))
			}
		}
	}
}
