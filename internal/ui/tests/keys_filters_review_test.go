package ui_test

import (
	"regexp"
	"strings"
	"testing"
)

// Fixes from the 2026-11-18 design review: the compact filter row, the command palette,
// tab's one meaning and the search input (F14, F15, F17, F23).

// At 64 columns the compact filter row keeps both labels, so the two chips read apart
// without colour, and the tab bar never sets a count one space before the next screen's
// digit.
func TestCompactFilterRowNamesBothFilters(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 64, 24, "3"))
	if row := lineWith(view, "│"); !strings.Contains(row, "VIEW s [All 9]") || !strings.Contains(row, "TRACK t [All 9]") {
		t.Fatalf("compact filter row lacks its labels: %q", row)
	}
	// "Drafts 2 3 Pipeline": a name, its count, one space, the next tab's digit.
	countThenDigit := regexp.MustCompile(`[a-z] \d+ [1-6]\b`)
	for _, keys := range [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}, {"3", "/", "eng", "enter"}} {
		bar := strings.SplitN(chipped(runKeysSized(t, fixtureCopy(t, nil), 64, 24, keys...)), "\n", 2)[0]
		if countThenDigit.MatchString(bar) {
			t.Errorf("%v: a count runs into the next screen's digit: %q", keys, bar)
		}
	}
}

// Typing "harn" in the palette puts connect a Harness first, over the keys that need one,
// and enter opens Settings › This machine.
func TestPaletteHarnessFindsConnectFirst(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "1", ":", "harn"))
	if sel := lineWith(view, "▌ "); !strings.Contains(sel, "connect a Harness") {
		t.Fatalf("connect a Harness is not first: %q\n%s", sel, view)
	}
	view = chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "1", ":", "harn", "enter"))
	if !strings.Contains(view, "[6 Settings]") || strings.Contains(view, "No Harness set") {
		t.Fatalf("enter did not open Settings:\n%s", view)
	}
}

// The palette is titled Commands, and says how many more rows there are when they do not
// all fit, above and below.
func TestPaletteTitleAndOverflowCue(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 56, 14, "1", ":"))
	if !strings.Contains(view, "╭─ Commands: ") || !regexp.MustCompile(`↓ \d+ more`).MatchString(view) {
		t.Fatalf("palette lacks its title or ↓ N more:\n%s", view)
	}
	keys := []string{"1", ":"}
	for range 8 {
		keys = append(keys, "down")
	}
	view = chipped(runKeysSized(t, fixtureCopy(t, nil), 56, 14, keys...))
	if !regexp.MustCompile(`↑ \d+ more`).MatchString(view) || !regexp.MustCompile(`↓ \d+ more`).MatchString(view) {
		t.Fatalf("scrolled palette lacks ↑/↓ N more:\n%s", view)
	}
	// Every match fits: no cue.
	if view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "1", ":", "dism")); strings.Contains(view, " more") {
		t.Fatalf("palette shows a cue with nothing hidden:\n%s", view)
	}
}

// The palette opens the Edit form on one field: the next step, the Contact and, on a
// Rejected or Withdrawn Application, the close reason.
func TestPaletteOpensEditOnAField(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", ":", "next"))
	if !strings.Contains(view, "edit next step") {
		t.Fatalf("palette lacks edit next step:\n%s", view)
	}
	for _, tc := range []struct {
		keys  []string
		field string
	}{
		{[]string{"3", ":", "next step", "enter"}, "When"},
		{[]string{"3", ":", "contact", "enter"}, "Name"},
		{[]string{"3", "s", "s", "s", "s", ":", "close reason", "enter"}, "Reason"},
	} {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 40, tc.keys...))
		if !strings.Contains(view, "╭─ Edit ") {
			t.Fatalf("%v: no Edit form:\n%s", tc.keys, view)
		}
		if focused := lineWith(view, "▌ "); !strings.Contains(focused, tc.field) {
			t.Errorf("%v: Edit form is not on %s: %q", tc.keys, tc.field, focused)
		}
	}
	// An Application still open has no close reason to set.
	if view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", ":", "close reason")); strings.Contains(view, "set close reason") {
		t.Fatalf("palette offers a close reason on Applied:\n%s", view)
	}
}

// tab has one meaning, named where it acts: beside ←→ in a list's and the detail's help, and
// as focus on Profile's key line.
func TestHelpNamesTab(t *testing.T) {
	if help := helpAt(t, 120, 40, "3"); !strings.Contains(help, "←→ tab view") {
		t.Errorf("list help does not name tab: %q", help)
	}
	if help := helpAt(t, 120, 40, "3", "enter"); !strings.Contains(help, "tab next tab") {
		t.Errorf("detail help does not name tab: %q", help)
	}
	if line := lastLine(t, 120, 36, "5"); !strings.Contains(line, "tab focus") {
		t.Errorf("Profile key line does not name tab: %q", line)
	}
	// One row each: tab is not listed again elsewhere with other words.
	for keys, want := range map[string]string{"3": "←→ tab view", "3\r": "tab next tab", "6": "←→ tab section"} {
		help := helpAt(t, 120, 40, strings.Split(strings.ReplaceAll(keys, "\r", " enter"), " ")...)
		if n := strings.Count(help, want); n != 1 {
			t.Errorf("%q: help names %q %d times: %q", keys, want, n, help)
		}
		if n := strings.Count(help, "tab focus"); n != 0 {
			t.Errorf("%q: help names tab focus where there is one pane: %q", keys, help)
		}
	}
}

// / over a search selects the old query: typing replaces it, an arrow keeps it.
func TestSearchReopenSelectsTheOldQuery(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "/", "eng", "enter", "/"))
	if line := keyLineOf(view); !strings.Contains(line, "/ eng") || !strings.Contains(line, "type replace") {
		t.Fatalf("reopened search does not offer to replace: %q", line)
	}
	view = chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "/", "eng", "enter", "/", "data", "enter"))
	if !strings.Contains(view, "/data ×") || strings.Contains(view, "engdata") {
		t.Fatalf("typing did not replace the query:\n%s", view)
	}
	view = chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "/", "en", "enter", "/", "right", "g", "enter"))
	if !strings.Contains(view, "/eng ×") {
		t.Fatalf("an arrow did not keep the query to add to:\n%s", view)
	}
}

// backspace on a list clears the search, the Track and the view at once, and says what it
// cleared; help lists it while a filter is on.
func TestBackspaceClearsEveryFilter(t *testing.T) {
	keys := []string{"3", "s", "t", "/", "eng", "enter"}
	if help := helpAt(t, 120, 40, keys...); !strings.Contains(help, "backspace clear all filters") {
		t.Errorf("help lacks backspace: %q", help)
	}
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, append(keys, "backspace")...))
	if !strings.Contains(view, `Cleared search "/eng", Track SWE, view Applied`) {
		t.Fatalf("toast does not say what was cleared:\n%s", view)
	}
	if row := lineWith(view, "│ TRACK"); !strings.Contains(row, "VIEW s [All 9]") || !strings.Contains(row, "TRACK t [All 9]") || strings.Contains(row, "/eng") {
		t.Fatalf("filters still on: %q", row)
	}
	// esc clears only the search, and points at backspace for the rest.
	view = chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, append(keys, "esc")...))
	if !strings.Contains(view, "Search cleared") || !strings.Contains(view, "backspace clear all filters") {
		t.Fatalf("esc does not point at backspace:\n%s", view)
	}
}
