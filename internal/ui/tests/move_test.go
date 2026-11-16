package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

func TestMoveOptions(t *testing.T) {
	for _, tc := range []struct{ from, previous, prefix string }{
		{"Lead", "", "Draft Applied Dismissed"}, {"Draft", "", "Applied Lead Dismissed"},
		{"Applied", "", "Interviewing Rejected Withdrawn Offer"}, {"Interviewing", "", "Offer Rejected Withdrawn Applied"},
		{"Offer", "", "Withdrawn Rejected"}, {"Rejected", "", "Interviewing Applied"}, {"Withdrawn", "", "Interviewing Applied"},
		{"Dismissed", "Draft", "Draft"},
	} {
		got := ui.MoveOptions(tc.from, tc.previous)
		if !strings.HasPrefix(strings.Join(got, " "), tc.prefix) {
			t.Errorf("%s: %v", tc.from, got)
		}
		seen := map[string]bool{tc.from: true}
		for _, s := range got {
			if seen[s] {
				t.Errorf("duplicate/current %s", s)
			}
			seen[s] = true
		}
		if len(got) != 7 {
			t.Errorf("options: %v", got)
		}
	}
}

// Dismissing a Lead that was never moved and undoing it leaves it as it was: the card
// says "Not moved yet", no Dismissed stop, and the row gains no age.
func TestUndoingTheFirstMoveOfALeadLeavesItUnmoved(t *testing.T) {
	root := fixtureCopy(t, nil)
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("/Example") // the Empty Lead, wherever the deadline sort puts it
	d.Type("enter")
	before := rowWith(strings.Split(d.plain(), "\n"), "◌ Empty ")
	d.Type("x")
	d.Type("u")
	b, err := os.ReadFile(filepath.Join(root, "jobs", "SWE", "empty", "meta.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Dismissed") {
		t.Fatalf("the move and its undo were not logged:\n%s", b)
	}
	v := d.plain()
	if !strings.Contains(v, "Not moved yet") || strings.Contains(v, "⊘ Dismissed") {
		t.Fatalf("the undone move left a stop on the timeline:\n%s", v)
	}
	if after := rowWith(strings.Split(v, "\n"), "◌ Empty "); after != before {
		t.Fatalf("the row changed after the undo:\nbefore %q\nafter  %q", before, after)
	}
}

// Moving the only row out of a view leaves a centre line naming where it went and the
// one key that goes there; the sentence the toast says is not drawn twice.
func TestMovedOutViewNamesTheScreenKey(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	d.Type("t") // SWE
	d.Type("t") // Platform: one Lead
	d.Type("m")
	d.Type("d")
	v := d.plain()
	requireAll(t, v, "Now in Drafts · 2 to go there")
	if n := strings.Count(v, "Moved to Draft"); n != 1 {
		t.Fatalf("the move sentence is drawn %d times:\n%s", n, v)
	}
	if strings.Contains(v, "1-6") {
		t.Fatalf("the hint names a range of keys:\n%s", v)
	}
}
