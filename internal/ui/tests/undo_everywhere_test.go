package ui_test

import (
	"strings"
	"testing"
)

// u is never silent: on Stats, Profile and Settings with nothing to undo it says so, and
// on Stats and Profile it undoes the last move, as on the lists.
func TestUndoAnswersOnEveryScreen(t *testing.T) {
	for _, c := range []struct {
		keys []string
		want string
	}{
		{[]string{"4", "u"}, "Nothing to undo"},
		{[]string{"5", "u"}, "Nothing to undo"},
		{[]string{"6", "u"}, "Nothing to undo in Settings"},
		{[]string{"2", "m", "a", "y", "4", "u"}, "Undone · back to Draft"},
		{[]string{"2", "m", "a", "y", "5", "u"}, "Undone · back to Draft"},
		{[]string{"2", "m", "a", "y", "t", "4", "t", "u"}, "Undone · back to Draft ·"},
	} {
		got := chipped(runKeys(t, fixtureCopy(t, nil), 120, c.keys...))
		if !strings.Contains(got, c.want) || strings.Contains(got, "hidden by filter") {
			t.Errorf("%q: want %q in the footer:\n%s", c.keys, c.want, got)
		}
	}
	// The undone move reaches Stats.
	before := chipped(runKeys(t, fixtureCopy(t, nil), 120, "4"))
	after := chipped(runKeys(t, fixtureCopy(t, nil), 120, "2", "m", "a", "y", "4", "u"))
	funnel := func(v string) string { return v[strings.Index(v, "FUNNEL"):strings.Index(v, "Closed ")] }
	if funnel(before) != funnel(after) {
		t.Fatalf("undo on Stats did not reach the funnel:\n%s\n%s", funnel(before), funnel(after))
	}
}
