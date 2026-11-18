package ui_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// phraseSplit ends a phrase of the frame: two spaces or more, or a rule.
var phraseSplit = regexp.MustCompile(`\s{2,}|[│┬┴─╭╮╰╯]`)

// phrases is every phrase of a plain view: its runs of words joined by single spaces.
func phrases(view string) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(view, "\n") {
		for _, ph := range phraseSplit.Split(l, -1) {
			if ph = strings.TrimSpace(ph); ph != "" {
				out[ph] = true
			}
		}
	}
	return out
}

// cutBeside returns the phrases shown beside the pop-up whose title row holds title that
// the frame without it does not hold whole: the fragments a pop-up's edge cuts.
func cutBeside(view, title string, whole map[string]bool) []string {
	var cut []string
	in := false
	for _, row := range strings.Split(view, "\n") {
		in = in || strings.Contains(row, title)
		if !in {
			continue
		}
		open, end := strings.IndexAny(row, "╭│╰"), strings.LastIndexAny(row, "╮│╯")
		for _, side := range []string{row[:open], row[end+len("│"):]} {
			for ph := range phrases(side) {
				if !whole[ph] {
					cut = append(cut, ph)
				}
			}
		}
		if strings.Contains(row, "╰─") {
			break
		}
	}
	return cut
}

// The first-run welcome behind the Contact and Add pop-ups shows whole phrases or
// nothing beside them: no "add your" or "connect a" cut off at a border. Without colour
// (no dimmed frame) the same holds.
func TestPopUpCutsNoPhraseOfTheWelcome(t *testing.T) {
	for _, noColor := range []bool{false, true} {
		if noColor {
			t.Setenv("NO_COLOR", "1")
		}
		for _, sz := range [][2]int{{120, 36}, {80, 24}} {
			d := newDriver(t, freshWorkspace(t), setupNow, sz[0], sz[1], ui.WithFirstRunSetup())
			contact := d.plain()
			d.Type("esc")
			whole := phrases(d.plain()) // the welcome with nothing over it
			if cut := cutBeside(contact, "╭─ Welcome to rolecall", whole); len(cut) > 0 {
				t.Errorf("%dx%d no colour %v: the Contact pop-up cuts %q:\n%s", sz[0], sz[1], noColor, cut, contact)
			}
			d.Type("a")
			add := d.plain()
			if cut := cutBeside(add, "╭─ Add a Lead", whole); len(cut) > 0 {
				t.Errorf("%dx%d no colour %v: the Add pop-up cuts %q:\n%s", sz[0], sz[1], noColor, cut, add)
			}
		}
	}
}
