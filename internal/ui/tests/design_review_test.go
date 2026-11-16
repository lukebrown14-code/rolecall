package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Fixes from the cold design review of the overhauled TUI.

// A row moved today carries no age column and no abbreviated Status name: the glyph and
// the section say the Status, and the preview says how long it has been there.
func TestRowMovedTodayKeepsItsStage(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 80, 24, "3", "m", "i"))
	if row := lineWith(view, "Software Engineer"); strings.Contains(row, "today") || strings.Contains(row, "Interv") {
		t.Errorf("moved row = %q, want no age and no abbreviated Status", row)
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

// The Menu clears only its own cells and a margin around them: the frame behind stays in
// view beside it, in whole words, and the pane divider is never cut.
func TestMenuClearsOnlyItsFootprint(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", " "))
	right := -1 // the Menu's right border column
	for _, l := range strings.Split(view, "\n") {
		if i := strings.Index(l, "╮"); i >= 0 && strings.Contains(l, "╭─ Menu") {
			right = ansi.StringWidth(l[:i])
		}
	}
	if right < 0 {
		t.Fatalf("no Menu:\n%s", view)
	}
	divider := colOf(lineWith(view, "┬"), "┬") // the pane divider's column; the centred Menu may cover it
	beside := 0
	for _, l := range strings.Split(view, "\n") {
		if trim := strings.TrimSpace(l); !strings.HasPrefix(trim, "│") && !strings.HasPrefix(trim, "╭") && !strings.HasPrefix(trim, "╰") {
			continue
		}
		rest := ansi.Cut(l, right+1, ansi.StringWidth(l))
		if !strings.HasPrefix(rest, " ") {
			t.Errorf("no margin beside the Menu border: %q", l)
		}
		if strings.Trim(rest, " │") != "" {
			beside++
		}
		if divider > right && !strings.Contains(rest, "│") {
			t.Errorf("the pane divider is cut beside the Menu: %q", l)
		}
	}
	if beside == 0 {
		t.Errorf("the Menu blanks the whole rows behind it:\n%s", view)
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
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), 60, 14, keys...))
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
