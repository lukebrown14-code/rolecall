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

// helpText is the view with the ? pop-up's "[k]   desc" cells read as "k desc".
func helpText(view string) string {
	return strings.Join(strings.Fields(strings.NewReplacer("[", "", "]", "").Replace(chipped(view))), " ")
}

// helpAt is the ? pop-up's text at a size after keys.
func helpAt(t *testing.T, w, h int, keys ...string) string {
	t.Helper()
	return helpText(runKeysSized(t, fixtureCopy(t, nil), w, h, append(keys, "?")...))
}

// The sort key o is offered on Leads, not Applied. It is a minor key: ? lists it, the key
// line never does.
func TestKeyLineShowsSortOnlyOnLeads(t *testing.T) {
	if help := helpAt(t, 140, 40, "1"); !strings.Contains(help, "o sort newest") {
		t.Errorf("Leads help lacks o: %q", help)
	}
	if help := helpAt(t, 140, 40, "3"); strings.Contains(help, "o sort") {
		t.Errorf("Applied help offers o: %q", help)
	}
	for _, sz := range keyLineSizes {
		if line := lastLine(t, sz[0], sz[1], "1"); hasKey(line, "o") {
			t.Errorf("%dx%d Leads key line shows o: %q", sz[0], sz[1], line)
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

// While a move can be undone the key line keeps u undo, past the next key that clears the
// toast; once every move is undone it goes. ? lists it either way.
func TestKeyLineShowsUndoWhileAMoveCanBeUndone(t *testing.T) {
	for _, sz := range keyLineSizes {
		if line := lastLine(t, sz[0], sz[1], "1"); hasKey(line, "u") {
			t.Errorf("%dx%d key line shows u before any move: %q", sz[0], sz[1], line)
		}
		for _, keys := range [][]string{{"3", "m", "enter"}, {"1", "x", "j"}} {
			if line := lastLine(t, sz[0], sz[1], keys...); !strings.Contains(line, "u undo") {
				t.Errorf("%dx%d %v: key line lacks u undo: %q", sz[0], sz[1], keys, line)
			}
		}
		if line := lastLine(t, sz[0], sz[1], "1", "x", "j", "u", "j"); hasKey(line, "u") {
			t.Errorf("%dx%d key line keeps u after the last undo: %q", sz[0], sz[1], line)
		}
	}
	if help := helpAt(t, 120, 40, "3", "m", "enter"); !strings.Contains(help, "u undo") {
		t.Errorf("help lacks u undo after a move: %q", help)
	}
}

// The lean key line lists x dismiss on a Lead or Draft and nowhere else; at 64 columns it
// still keeps the core keys.
func TestKeyLineListsDismissOnLeadsAndDrafts(t *testing.T) {
	for _, sz := range [][2]int{{160, 44}, {120, 36}} {
		if line := lastLine(t, sz[0], sz[1], "1"); !strings.Contains(line, "x dismiss") {
			t.Errorf("%dx%d Lead row key line lacks x dismiss: %q", sz[0], sz[1], line)
		}
		if line := lastLine(t, sz[0], sz[1], "2"); !strings.Contains(line, "x dismiss") {
			t.Errorf("%dx%d Draft row key line lacks x dismiss: %q", sz[0], sz[1], line)
		}
		if line := lastLine(t, sz[0], sz[1], "3"); hasKey(line, "x") {
			t.Errorf("%dx%d Applied row key line shows x: %q", sz[0], sz[1], line)
		}
	}
	line := lastLine(t, 64, 24, "1")
	for _, want := range []string{"enter open", "m move", "a add", "? help"} {
		if !strings.Contains(line, want) {
			t.Errorf("64x24 key line lacks %q: %q", want, line)
		}
	}
}

// A known row key that does not apply on this row says why; an unknown key stays silent.
func TestKeysThatDoNotApplySayWhy(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, tc := range []struct {
		keys []string
		want string
	}{
		{[]string{"3", "x"}, "x: only Leads and Drafts can be dismissed · m to move"},
		{[]string{"3", "o"}, "o: sorts the Leads view only"},
		{[]string{"3", "F"}, "F: only on a Follow-up due Application"},
	} {
		if v := chipped(runKeys(t, root, 120, tc.keys...)); !strings.Contains(v, tc.want) {
			t.Errorf("%v: no %q:\n%s", tc.keys, tc.want, v)
		}
	}
	if base, got := chipped(runKeys(t, root, 120, "3")), chipped(runKeys(t, root, 120, "3", "Z")); got != base {
		t.Errorf("an unknown key changed the screen:\n%s", got)
	}
}

// The detail view gives the same notes as a list row for the row and view keys it does
// not run, and an unknown key there stays silent too.
func TestDetailKeysThatDoNotApplySayWhy(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, tc := range []struct {
		keys []string
		want string
	}{
		{[]string{"3", "enter", "x"}, "x: only Leads and Drafts can be dismissed · m to move"},
		{[]string{"1", "enter", "x"}, "x: dismisses from the list only · m to move"},
		{[]string{"1", "enter", "o"}, "o: sorts the Leads view only"},
		{[]string{"1", "enter", "s"}, "s: changes the view from the list only"},
		{[]string{"1", "enter", "l"}, "l: only on Applied and Interviewing Applications"},
		{[]string{"3", "enter", "F"}, "F: only on a Follow-up due Application"},
	} {
		if v := chipped(runKeys(t, root, 120, tc.keys...)); !strings.Contains(v, tc.want) {
			t.Errorf("%v: no %q:\n%s", tc.keys, tc.want, v)
		}
	}
	if v := chipped(runKeys(t, root, 120, "1", "enter", "x")); strings.Contains(v, "Dismissed") {
		t.Errorf("x dismissed from the detail view:\n%s", v)
	}
	if base, got := chipped(runKeys(t, root, 120, "3", "enter")), chipped(runKeys(t, root, 120, "3", "enter", "Z")); got != base {
		t.Errorf("an unknown key changed the detail view:\n%s", got)
	}
}

// Without a Harness f and p say so alike on a list row and in the detail view.
func TestHandoffKeysWithoutAHarnessSayTheSameInTheDetail(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, key := range []string{"f", "p"} {
		for _, keys := range [][]string{{"3", key}, {"3", "enter", key}} {
			if v := chipped(runKeys(t, root, 120, keys...)); !strings.Contains(v, "No Harness set") {
				t.Errorf("%v: no Harness error:\n%s", keys, v)
			}
		}
	}
}

// l says why on a list row it does not apply to, as in the detail view.
func TestFollowedUpKeySaysWhyOnALead(t *testing.T) {
	if v := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "l")); !strings.Contains(v, "l: only on Applied and Interviewing Applications") {
		t.Errorf("no note for l on a Lead:\n%s", v)
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
		if line := lastLine(t, sz[0], sz[1], "1"); !strings.Contains(line, "enter open") {
			t.Errorf("%dx%d lacks enter open: %q", sz[0], sz[1], line)
		}
	}
}
