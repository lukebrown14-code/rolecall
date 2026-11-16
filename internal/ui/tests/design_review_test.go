package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Fixes from the cold design review of the overhauled TUI.

// A row moved today reads "today" like any other Status's, with no abbreviated Status
// name beside it: the glyph and the section say the Status.
func TestRowMovedTodayKeepsItsStage(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 80, 24, "3", "m", "i"))
	if row := lineWith(view, "Software Engineer"); !strings.Contains(row, " today ") || strings.Contains(row, "Interv") {
		t.Errorf("moved row = %q, want today and no abbreviated Status", row)
	}
}

// A status filter named like its screen still shows in the breadcrumb: Applied › Applied
// is filtered, Applied › All is not.
func TestBreadcrumbShowsAStatusFilterNamedLikeItsScreen(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "3", "s"))
	if !strings.Contains(view, "Applied › Applied · All tracks") {
		t.Errorf("breadcrumb drops the Applied filter:\n%s", view)
	}
}

// The Menu blanks the rows it covers, so no word is cut off against its border.
func TestMenuBlanksTheRowsBehindIt(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 80, 24, "3", " "))
	for _, l := range strings.Split(view, "\n") {
		trim := strings.TrimSpace(l)
		if !strings.HasPrefix(trim, "│") && !strings.HasPrefix(trim, "╭") && !strings.HasPrefix(trim, "╰") {
			continue
		}
		end := max(strings.LastIndex(l, "│"), strings.LastIndex(l, "╮"), strings.LastIndex(l, "╯"))
		if rest := l[end:]; strings.TrimSpace(strings.TrimLeft(rest, "│╮╯")) != "" {
			t.Errorf("text beside the Menu border: %q", l)
		}
	}
}

// After a Settings save, u undo is on the key line at every size.
func TestUndoKeyShowsAfterASettingsSave(t *testing.T) {
	for _, sz := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		line := lastLine(t, sz[0], sz[1], "6", "]", "]", "]", "enter", "backspace", "backspace", "backspace", "300", "enter")
		if !strings.Contains(line, "u undo") || strings.Contains(line, "ctrl+s") {
			t.Errorf("%dx%d: saved change but no undo key: %q", sz[0], sz[1], line)
		}
	}
}

// The detail tabs keep to one line: hidden ones are marked ‹ ›, and the Overview's cue
// still shows under it.
func TestDetailTabsNeverWrap(t *testing.T) {
	for _, keys := range [][]string{{"3", "/docs-written", "enter", "enter"}, {"3", "/docs-written", "enter", "enter", "left", "left", "left", "left", "left"}} {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), 60, 16, keys...))
		if strings.Contains(view, "\n  Notes   Docs") {
			t.Errorf("keys %q: tabs wrap:\n%s", keys, view)
		}
		if !strings.Contains(view, "›") && !strings.Contains(view, "‹") {
			t.Errorf("keys %q: no mark for hidden tabs:\n%s", keys, view)
		}
		if !strings.Contains(view, "↓ ") {
			t.Errorf("keys %q: cut content has no cue:\n%s", keys, view)
		}
	}
}

// While a field is open, the key line offers only the field's keys: digits, : and ? type
// into it.
func TestTypingHidesTheGlobalKeys(t *testing.T) {
	for _, sz := range [][2]int{{120, 36}, {60, 16}} {
		line := lastLine(t, sz[0], sz[1], "6", "]", "]", "]", "]", "]", "enter")
		for _, k := range []string{"1-6 screen", ": commands", "? help"} {
			if strings.Contains(line, k) {
				t.Errorf("%dx%d: %q offered while typing: %q", sz[0], sz[1], k, line)
			}
		}
	}
}

// An empty field starts in the same column focused or not: the cursor takes its first cell.
func TestCursorDoesNotShiftAnEmptyField(t *testing.T) {
	unfocused := lineWith(chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "a")), "Title*")
	focused := lineWith(chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "a", "enter")), "Title*")
	start := func(l string) int { return strings.IndexAny(l, "_▏") }
	if start(unfocused) < 0 || start(focused) < 0 {
		t.Fatalf("no field:\n%q\n%q", unfocused, focused)
	}
	if a, b := ansi.StringWidth(unfocused[:start(unfocused)]), ansi.StringWidth(focused[:start(focused)]); a != b {
		t.Errorf("empty field at column %d unfocused, %d focused:\n%q\n%q", a, b, unfocused, focused)
	}
}

// Filter options sit two cells apart, the chosen chip included.
func TestFilterOptionsAreEvenlySpaced(t *testing.T) {
	row := strings.Split(chipped(runKeys(t, fixtureCopy(t, nil), 120, "3")), "\n")[2]
	for _, want := range []string{"Applied 1  Interviewing 2  Offer 2  Closed 4"} {
		if !strings.Contains(row, want) {
			t.Errorf("filter row %q lacks %q", row, want)
		}
	}
}

// On the selection fill a row's words are never a status colour, which falls below 4.5:1
// there; the glyphs keep it.
func TestSelectedRowWordsClearContrast(t *testing.T) {
	raw := runKeys(t, fixtureCopy(t, nil), 120, "3", "s", "s")
	for _, l := range strings.Split(raw, "\n") {
		if !strings.Contains(l, selectionBg) || !strings.Contains(ansi.Strip(l), "Good") {
			continue
		}
		for _, c := range []string{"38;2;88;166;255;" + selectionBg, "38;2;240;136;62;" + selectionBg} { // Info, Warn
			if strings.Contains(l, c) {
				t.Errorf("selected row has coloured words: %s", fmt.Sprintf("%q", l))
			}
		}
	}
}
