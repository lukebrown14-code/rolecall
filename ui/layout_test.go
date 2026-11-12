package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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

// An error stays on the status row through other keys until esc dismisses it.
func TestErrorsStayUntilEsc(t *testing.T) {
	const want = "Can't read SWE/malformed ·"
	if view := ansi.Strip(finalView(t, "G", "m", "d", "j")); !strings.Contains(view, want) {
		t.Fatalf("error cleared by another key:\n%s", view)
	}
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), selectionWidth, "G", "m", "d", "esc"))
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
	m := driveStart(t, root, fixtureClock, width, height)
	return driveModel(t, m, keys...).View().Content
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

// At any size, from a sliver to the minimum and just above it, every screen and pop-up
// stays inside the terminal; below the minimum a notice says so instead.
func TestEverySizeFitsTheTerminal(t *testing.T) {
	root := fixtureCopy(t, nil)
	sizes := [][2]int{{1, 1}, {10, 3}, {49, 30}, {120, 11}, {50, 12}, {64, 13}}
	states := [][]string{{"1"}, {"3"}, {"5"}, {"6"}, {"m"}, {"a"}, {"?"}, {"3", "enter"}, {"G", "m", "d"}}
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
