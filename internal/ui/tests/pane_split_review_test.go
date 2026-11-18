package ui_test

import (
	"strings"
	"testing"
)

// splitColumn is the cell of the frame's ┬, where the body's divider meets the top rule;
// -1 with no divider.
func splitColumn(view string) int {
	for _, l := range strings.Split(view, "\n") {
		if i := strings.Index(l, "┬"); i >= 0 {
			return len([]rune(l[:i]))
		}
	}
	return -1
}

// The list screens and Settings put their divider in one column, the list's width with
// the card beside it, so moving between them the eye need not re-anchor. Profile's
// section list is a narrower navigation list beside its reader, by design.
func TestPaneSplitHoldsBetweenListsAndSettings(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {160, 40}, {100, 30}} {
		leads := splitColumn(chipped(runKeysSized(t, fixtureCopy(t, nil), size[0], size[1], "1")))
		if leads < 0 {
			t.Fatalf("%dx%d Leads has no divider", size[0], size[1])
		}
		for _, keys := range [][]string{{"6"}, {"6", "]"}, {"6", "]", "]", "]"}, {"6", "]", "]", "]", "]"}, {"6", "["}} {
			got := splitColumn(strings.Join(settingsAt(t, size[0], size[1], keys[1:]...), "\n"))
			if got != leads {
				t.Errorf("%dx%d Settings %v divider at %d, Leads at %d", size[0], size[1], keys, got, leads)
			}
		}
	}
}

// A Settings value is never cut while its pane has room: a long one takes the free width
// after it, and wraps under itself rather than end in "…".
func TestSettingsValuesUseTheFreeWidth(t *testing.T) {
	for _, keys := range [][]string{{}, {"]"}, {"]", "]"}, {"]", "]", "]"}, {"]", "]", "]", "]"}} {
		lines := settingsAt(t, 120, 36, keys...)
		for _, l := range lines[4 : len(lines)-3] {
			left := l
			if i := strings.Index(l, "│"); i >= 0 {
				left = l[:i]
			}
			if strings.Contains(left, "…") {
				t.Errorf("Settings %v cuts a value beside free width: %q", keys, l)
			}
		}
	}
	tracking := strings.Join(settingsAt(t, 120, 36, "]", "]", "]", "]"), "\n")
	requireAll(t, squash(tracking), "2 dismissed · 1 unreadable")
	rules := strings.Join(settingsAt(t, 120, 36, "]"), "\n")
	requireAll(t, rules, "1mo ago = discard, 30+ days ago =", "discard")
}
