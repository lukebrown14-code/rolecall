package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// dismissedOnDisk counts the Applications whose last logged Status is Dismissed.
func dismissedOnDisk(t *testing.T, root string) int {
	t.Helper()
	metas, _ := filepath.Glob(filepath.Join(root, "jobs", "*", "*", "meta.yml"))
	n := 0
	for _, f := range metas {
		b, _ := os.ReadFile(f)
		lines := strings.Split(strings.TrimSpace(string(b)), "\n")
		if last := lines[len(lines)-1]; strings.Contains(last, "status: Dismissed") {
			n++
		}
	}
	return n
}

// Marked rows are counted in the breadcrumb, esc clears them, and a mark reads without
// colour: ◉ on the row (ASCII: ]).
func TestMarkingRowsCountsThemAndEscClears(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "j", "*")
	view := chipped(m.View().Content)
	if !strings.Contains(view, "2 marked") || strings.Count(view, "◉") < 3 { // two rows and the breadcrumb
		t.Fatalf("marks are not shown:\n%s", view)
	}
	m = driveModel(t, m, "esc")
	if view := chipped(m.View().Content); strings.Contains(view, "marked") || strings.Contains(view, "◉") {
		t.Fatalf("esc did not clear the marks:\n%s", view)
	}
	// * again unmarks.
	m = driveModel(t, m, "*", "*")
	if view := chipped(m.View().Content); strings.Contains(view, "marked") {
		t.Fatalf("a second * did not unmark:\n%s", view)
	}
}

// x on marked Leads dismisses them all after a confirmation naming the count, and one u puts
// them all back.
func TestBulkDismissConfirmsAndUndoesAsOne(t *testing.T) {
	root := fixtureCopy(t, nil)
	was := dismissedOnDisk(t, root)
	m := driveStart(t, root, fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "j", "*", "x")
	view := chipped(m.View().Content)
	if !strings.Contains(view, "Dismiss 2 Applications?") || !strings.Contains(view, "y dismiss 2") {
		t.Fatalf("no confirmation naming the count:\n%s", view)
	}
	if got := dismissedOnDisk(t, root); got != was {
		t.Fatalf("dismissed before confirming: %d -> %d", was, got)
	}
	// n cancels and keeps the marks.
	if view := chipped(driveModel(t, m, "n").View().Content); !strings.Contains(view, "2 marked") || dismissedOnDisk(t, root) != was {
		t.Fatalf("n did not cancel cleanly:\n%s", view)
	}
	m = driveModel(t, m, "y")
	if got := dismissedOnDisk(t, root); got != was+2 {
		t.Fatalf("dismissed %d on disk, want %d", got, was+2)
	}
	view = chipped(m.View().Content)
	if !strings.Contains(view, "Dismissed · 2 Applications") || !strings.Contains(view, "u undo") || strings.Contains(view, "marked") {
		t.Fatalf("toast does not count them:\n%s", view)
	}
	m = driveModel(t, m, "u")
	if got := dismissedOnDisk(t, root); got != was {
		t.Fatalf("one u left %d dismissed, want %d", got, was)
	}
	if view := chipped(m.View().Content); !strings.Contains(view, "Undone · 2 Applications moved back") {
		t.Fatalf("undo toast:\n%s", view)
	}
}

// m on marked rows opens one Move pop-up for all of them, then asks once.
func TestBulkMoveAsksOnceThenMovesAll(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "j", "*", "m")
	view := chipped(m.View().Content)
	if !strings.Contains(view, "2 Applications marked") || !strings.Contains(view, "now Lead 2") {
		t.Fatalf("move pop-up does not name the marked rows:\n%s", view)
	}
	m = driveModel(t, m, "d") // Draft
	view = chipped(m.View().Content)
	if !strings.Contains(view, "Move 2 Applications to Draft?") {
		t.Fatalf("no bulk confirmation:\n%s", view)
	}
	m = driveModel(t, m, "y")
	if view := chipped(m.View().Content); !strings.Contains(view, "Moved to Draft · 2 Applications") {
		t.Fatalf("move did not say how many:\n%s", view)
	}
	if view := chipped(driveModel(t, m, "2").View().Content); !strings.Contains(view, "Drafts ›") {
		t.Fatalf("Drafts screen not reached:\n%s", view)
	}
}

// Marks belong to the view they were made in.
func TestMarksDoNotFollowToAnotherView(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "2")
	if view := chipped(m.View().Content); strings.Contains(view, "marked") || strings.Contains(view, "◉") {
		t.Fatalf("a mark followed to Drafts:\n%s", view)
	}
	m = driveModel(t, m, "x") // a single dismiss, not a bulk one
	if view := chipped(m.View().Content); strings.Contains(view, "Applications?") {
		t.Fatalf("x offered a bulk dismiss from a stale mark:\n%s", view)
	}
}

// Leaving the view drops the marks for good: coming back does not resurrect them.
func TestMarksAreGoneAfterLeavingTheView(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "2", "1")
	if view := chipped(m.View().Content); strings.Contains(view, "marked") {
		t.Fatalf("marks came back after a round trip:\n%s", view)
	}
}
