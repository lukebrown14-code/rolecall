package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var keyLineSizes = [][2]int{{120, 36}, {80, 24}, {60, 16}}

// lastLine is the key line of the View at a size after keys.
func lastLine(t *testing.T, w, h int, keys ...string) string {
	t.Helper()
	return keyLineOf(runKeysSized(t, fixtureCopy(t, nil), w, h, keys...))
}

// hasKey reports whether the key line lists key as a whole word.
func hasKey(line, key string) bool {
	for _, f := range strings.Fields(line) {
		if f == key {
			return true
		}
	}
	return false
}

// On Leads with a row selected the key line has the sort key o; on Applied it does not.
// o is a minor key, so it is the first to go when space is short: it is asserted where
// there is room (140), and its absence on Applied at every size.
func TestKeyLineShowsSortOnlyOnLeads(t *testing.T) {
	if line := lastLine(t, 140, 36, "1"); !hasKey(line, "o") {
		t.Errorf("140x36 Leads key line lacks o: %q", line)
	}
	for _, sz := range keyLineSizes {
		if line := lastLine(t, sz[0], sz[1], "3"); hasKey(line, "o") {
			t.Errorf("%dx%d Applied key line shows o: %q", sz[0], sz[1], line)
		}
	}
}

// With an empty list the row actions are absent.
func TestKeyLineHidesRowActionsOnAnEmptyList(t *testing.T) {
	for _, sz := range keyLineSizes {
		line := lastLine(t, sz[0], sz[1], "/", "z", "z", "z", "q", "x", "enter")
		for _, k := range []string{"m", "e", "x", "n"} {
			if hasKey(line, k) {
				t.Errorf("%dx%d empty list key line shows %s: %q", sz[0], sz[1], k, line)
			}
		}
		if !strings.Contains(line, "? help") {
			t.Errorf("%dx%d empty list key line lacks ? help: %q", sz[0], sz[1], line)
		}
	}
}

// u undo appears only once there is something to undo. It is a minor key, so it is
// asserted where there is room for it.
func TestKeyLineShowsUndoAfterAMove(t *testing.T) {
	for _, sz := range keyLineSizes[:1] {
		if line := lastLine(t, sz[0], sz[1], "3"); hasKey(line, "u") {
			t.Errorf("%dx%d key line shows u with nothing to undo: %q", sz[0], sz[1], line)
		}
		line := lastLine(t, sz[0], sz[1], "3", "m", "enter")
		if !strings.Contains(line, "u undo") {
			t.Errorf("%dx%d key line lacks u undo after a move: %q", sz[0], sz[1], line)
		}
	}
}

// At every size the key line fits one line and keeps ? help.
func TestKeyLineFitsAndEndsWithHelp(t *testing.T) {
	for _, sz := range keyLineSizes {
		for _, keys := range [][]string{{"1"}, {"3"}, {"3", "enter"}, {"6"}} {
			line := lastLine(t, sz[0], sz[1], keys...)
			if ansi.StringWidth(line) > sz[0] {
				t.Errorf("%dx%d %v: key line %d wide: %q", sz[0], sz[1], keys, ansi.StringWidth(line), line)
			}
			if !strings.Contains(line, "? help") {
				t.Errorf("%dx%d %v: lacks ? help: %q", sz[0], sz[1], keys, line)
			}
		}
	}
}

// In the move overlay the key line shows overlay keys only.
func TestKeyLineInMoveOverlayShowsOverlayKeysOnly(t *testing.T) {
	for _, sz := range keyLineSizes {
		line := lastLine(t, sz[0], sz[1], "3", "m")
		for _, k := range []string{"a", "n", "e", "o", "/", "?"} {
			if hasKey(line, k) {
				t.Errorf("%dx%d move overlay key line shows %s: %q", sz[0], sz[1], k, line)
			}
		}
		if !strings.Contains(line, "esc") {
			t.Errorf("%dx%d move overlay key line lacks esc: %q", sz[0], sz[1], line)
		}
	}
}

// With the detail pane hidden the key line says Enter opens it.
func TestKeyLineOffersEnterDetailWhenPaneHidden(t *testing.T) {
	for _, sz := range [][2]int{{80, 24}, {60, 16}} {
		if line := lastLine(t, sz[0], sz[1], "1"); !strings.Contains(line, "Enter detail") {
			t.Errorf("%dx%d lacks enter detail: %q", sz[0], sz[1], line)
		}
	}
}
