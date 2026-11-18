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

// Marked rows are counted once, in the breadcrumb, esc clears them, and a mark reads
// without colour: ◉ on the row (ASCII: [). * marks and steps down, so * * * marks three rows
// in a row.
func TestMarkingRowsCountsThemAndEscClears(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "*", "*")
	view := chipped(m.View().Content)
	if !strings.Contains(view, "◉ 3 marked") || strings.Count(view, "◉") != 4 { // three rows and the breadcrumb
		t.Fatalf("* * * did not mark three rows:\n%s", view)
	}
	if n := strings.Count(view, "marked"); n != 1 {
		t.Fatalf("the count shows %d times, want once (the breadcrumb):\n%s", n, view)
	}
	if !strings.Contains(view, "esc clear marks") || strings.Count(view, "x dismiss") != 1 || strings.Count(view, "m move") != 1 {
		t.Fatalf("the bulk keys are not on the key line once:\n%s", view)
	}
	m = driveModel(t, m, "esc")
	if view := chipped(m.View().Content); strings.Contains(view, "marked") || strings.Contains(view, "◉") {
		t.Fatalf("esc did not clear the marks:\n%s", view)
	}
	// * again unmarks.
	m = driveModel(t, m, "*", "k", "*")
	if view := chipped(m.View().Content); strings.Contains(view, "marked") {
		t.Fatalf("a second * did not unmark:\n%s", view)
	}
}

// The mark has a column of its own: a marked row's Status glyph sits in the cell it takes
// on an unmarked row, under the cursor or not, and the ◉ touches neither the ▌ nor the
// glyph.
func TestMarkedRowsKeepTheStatusGlyphColumn(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "k") // first row marked, under the cursor
	m = driveModel(t, m, "j", "j", "*") // third row marked; cursor on the fourth
	m = driveModel(t, m, "k", "k")      // cursor on the (unmarked) second row
	view := chipped(m.View().Content)
	want := map[string]string{ // the gutter before each row's Status glyph
		"Platform Engineer": " ◉ ", // marked, off the cursor
		"Empty":             "▌  ", // the cursor, unmarked
		"Empty log":         " ◉ ", // marked
	}
	got := map[string]string{}
	for _, line := range strings.Split(view, "\n") {
		for title := range want {
			i := strings.Index(line, "◌ ")
			if i < 0 || strings.TrimSpace(strings.SplitN(line[i+len("◌ "):], "│", 2)[0]) != title {
				continue
			}
			{
				got[title] = string([]rune(line[:i])[len([]rune(line[:i]))-3:])
			}
		}
	}
	for title, w := range want {
		if got[title] != w {
			t.Errorf("%s: gutter %q, want %q:\n%s", title, got[title], w, view)
		}
	}
	m = driveModel(t, m, "k") // the cursor on a marked row: ▌ then ◉, apart
	for _, line := range strings.Split(chipped(m.View().Content), "\n") {
		if strings.Contains(line, "◌ Platform Engineer ") && !strings.Contains(line, "▌◉ ◌ Platform Engineer ") {
			t.Errorf("marked cursor row: %q", line)
		}
	}
}

// In ASCII mode the mark is "[", which no Status, key or other mark uses.
func TestMarkASCIIStandIn(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*")
	view := chipped(m.View().Content)
	if !strings.Contains(view, " [ ( Platform Engineer") || !strings.Contains(view, "[ 1 marked") {
		t.Fatalf("ASCII mark:\n%s", view)
	}
}

// Mark-all marks every row in the view and the breadcrumb counts them; V again unmarks.
func TestMarkAllMarksEveryRowInTheView(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "V")
	view := chipped(m.View().Content)
	if !strings.Contains(view, "◉ 7 marked") {
		t.Fatalf("V did not mark all seven Leads:\n%s", view)
	}
	m = driveModel(t, m, "V")
	if view := chipped(m.View().Content); strings.Contains(view, "marked") || !strings.Contains(view, "Marks cleared") {
		t.Fatalf("V with all marked did not clear them:\n%s", view)
	}
}

// x on one marked row is a plain x on it: no confirmation, a toast and an undo.
func TestOneMarkedRowDismissesWithoutConfirming(t *testing.T) {
	root := fixtureCopy(t, nil)
	was := dismissedOnDisk(t, root)
	m := driveStart(t, root, fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "j", "*", "g", "x") // the mark is on row 2; the cursor on row 1
	view := chipped(m.View().Content)
	if strings.Contains(view, "Applications?") || dismissedOnDisk(t, root) != was+1 {
		t.Fatalf("one mark asked to confirm or did not dismiss:\n%s", view)
	}
	if !strings.Contains(view, "Dismissed · Empty") || !strings.Contains(view, "u undo") || strings.Contains(view, "marked") {
		t.Fatalf("toast does not name the marked row with an undo:\n%s", view)
	}
	if !strings.Contains(view, "Platform Engineer") {
		t.Fatalf("the row under the cursor was dismissed instead:\n%s", view)
	}
	if driveModel(t, m, "u"); dismissedOnDisk(t, root) != was {
		t.Fatalf("u did not put it back")
	}
}

// x on marked Leads dismisses them all after a confirmation naming the count, and one u puts
// them all back.
func TestBulkDismissConfirmsAndUndoesAsOne(t *testing.T) {
	root := fixtureCopy(t, nil)
	was := dismissedOnDisk(t, root)
	m := driveStart(t, root, fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "*", "x")
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

// m on marked rows opens one Move pop-up for all of them; the pick moves them, with no
// second confirmation, and one u puts them all back.
func TestBulkMoveAsksOnceThenMovesAll(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "*", "m")
	view := chipped(m.View().Content)
	if !strings.Contains(view, "2 Applications marked") || !strings.Contains(view, "now Lead 2") {
		t.Fatalf("move pop-up does not name the marked rows:\n%s", view)
	}
	m = driveModel(t, m, "d") // Draft
	if view := chipped(m.View().Content); !strings.Contains(view, "Moved to Draft · 2 Applications") || strings.Contains(view, "Applications to Draft?") {
		t.Fatalf("the pick did not move them at once:\n%s", view)
	}
	if view := chipped(driveModel(t, m, "2").View().Content); !strings.Contains(view, "Drafts ›") {
		t.Fatalf("Drafts screen not reached:\n%s", view)
	}
	if view := chipped(driveModel(t, m, "u").View().Content); !strings.Contains(view, "Undone · 2 Applications moved back") {
		t.Fatalf("one u did not undo the bulk move:\n%s", view)
	}
}

// m on one marked row is the single Move pop-up for that row.
func TestOneMarkedRowMovesAsASingleMove(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "*", "m")
	view := chipped(m.View().Content)
	if strings.Contains(view, "Applications marked") || !strings.Contains(view, "Platform Engineer") {
		t.Fatalf("one mark opened the bulk pop-up:\n%s", view)
	}
	m = driveModel(t, m, "d")
	if view := chipped(m.View().Content); !strings.Contains(view, "Moved to Draft · Platform Engineer") || strings.Contains(view, "marked") {
		t.Fatalf("not a plain move:\n%s", view)
	}
}

// Changing the view, the Track or the search drops the marks, and says so.
func TestChangingViewSaysMarksCleared(t *testing.T) {
	for _, keys := range [][]string{{"right"}, {"2"}, {"t"}, {"/", "e", "enter"}} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
		m = driveModel(t, m, "1", "*")
		m = driveModel(t, m, keys...)
		view := chipped(m.View().Content)
		if strings.Contains(view, "marked") || !strings.Contains(view, "Marks cleared") {
			t.Fatalf("keys %q: marks were not cleared with a note:\n%s", keys, view)
		}
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
