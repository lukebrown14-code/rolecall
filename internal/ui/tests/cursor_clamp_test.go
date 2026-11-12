package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A move that takes the last row out of the current view used to leave the cursor past
// the end of the rows and panic in View(). The Drafts screen holds two Applications, so
// moving the one under the cursor on the last row reproduces it.
func TestMoveLastRowKeepsCursorInRange(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	m = driveModel(t, m, "2", "G", "m", "a", "y")

	view := ansi.Strip(m.View().Content) // must not panic
	if n := strings.Count(view, "\n") + 1; n != termHeight {
		t.Fatalf("frame broke after moving the last row: %d lines\n%s", n, view)
	}
}

// Dismissing the only row of a view leaves it empty; the view falls back to its empty
// state instead of reading a row that is gone.
func TestDismissOnlyRowShowsEmptyState(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	// Drafts holds two; dismiss both to empty it.
	m = driveModel(t, m, "2", "G", "x", "G", "x")

	view := ansi.Strip(m.View().Content) // must not panic
	if n := strings.Count(view, "\n") + 1; n != termHeight {
		t.Fatalf("frame broke after emptying the view: %d lines\n%s", n, view)
	}
}
