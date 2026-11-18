package ui_test

import (
	"strings"
	"testing"
)

// After m d the toast offers i go to it, and i lands on the Application on Drafts, the
// cursor on its row: from the list, and from the detail.
func TestMoveOffersToGoToTheApplication(t *testing.T) {
	for _, keys := range [][]string{{"1", "m", "d"}, {"1", "enter", "m", "d"}} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
		m = driveModel(t, m, keys...)
		moved := chipped(m.View().Content)
		if !strings.Contains(moved, "Moved to Draft · Platform Engineer") || !strings.Contains(moved, "i go to it") {
			t.Fatalf("%q: the move's toast does not offer i go to it:\n%s", keys, moved)
		}
		m = driveModel(t, m, "i")
		view := chipped(m.View().Content)
		if !strings.Contains(view, "Drafts › All tracks") || !strings.Contains(lineWith(view, "▌"), "Platform Engineer") {
			t.Fatalf("%q then i: not on Drafts with the cursor on the moved row:\n%s", keys, view)
		}
	}
}

// With the moved row still in view (Pipeline › All) there is nowhere to go, so no offer.
func TestMoveInViewOffersNoGoTo(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "3", "m", "i")
	if view := chipped(m.View().Content); !strings.Contains(view, "Moved to Interviewing") || strings.Contains(view, "go to it") {
		t.Fatalf("a move the view still lists offers go to it:\n%s", view)
	}
}

// An undo says where the Application is back in the Owner's words, with no folder or file
// names, and i goes there: undoing a Draft's move to Applied from Pipeline lands on Drafts.
func TestUndoToastSaysWhereAndGoesThere(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "2", "m", "a", "y", "3", "u")
	view := chipped(m.View().Content)
	if !strings.Contains(view, "Undone · back to Draft · on Drafts · ") || !strings.Contains(view, "i go to it") {
		t.Fatalf("undo toast lacks where it went or i go to it:\n%s", view)
	}
	for _, jargon := range []string{"meta.yml", "sent/", "sent.undone", ".yml"} {
		if strings.Contains(view, jargon) {
			t.Errorf("undo toast names %q:\n%s", jargon, view)
		}
	}
	m = driveModel(t, m, "i")
	if view := chipped(m.View().Content); !strings.Contains(view, "Drafts › All tracks") {
		t.Fatalf("i after the undo did not go to Drafts:\n%s", view)
	}
}

// The help says undo lasts for this session.
func TestHelpSaysUndoLastsThisSession(t *testing.T) {
	for _, keys := range [][]string{{"1", "?"}, {"1", "x", "?"}} {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, keys...))
		if !strings.Contains(view, "undo (this session)") {
			t.Errorf("%q: help does not say undo lasts this session:\n%s", keys, view)
		}
	}
}
