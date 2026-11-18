package ui_test

import (
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// In the detail view e has one meaning, edit the fields, as on a row: the card's NEXT STEP
// row, the help and the key itself agree, and opening the tab's file is o, which the empty
// tabs name. The help says q goes back, as it does there.
func TestDetailEditKeyHasOneMeaning(t *testing.T) {
	root := stepWorkspace(t, "next_step: {at: 2026-10-04T10:30, what: \"Phone screen\"}\n")
	var runs []string
	m := driveStart(t, root, fixtureClock, 120, 36, ui.WithEditor("myeditor"), ui.WithExternalRunner(recordEditor(&runs)))
	m = driveModel(t, m, "3", "/stepper", "enter", "enter")
	view := plain(m)
	if row := lineWith(view, "NEXT STEP"); !strings.Contains(row, "e edit") {
		t.Errorf("the card's NEXT STEP row does not offer e edit:\n%s", view)
	}
	if !strings.Contains(view, "No JD saved · o to open jd.md") {
		t.Errorf("the empty JD tab does not name o:\n%s", view)
	}
	help := helpText(driveModel(t, m, "?").View().Content)
	for _, want := range []string{"e edit", "o open in editor", "q esc back"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "q quit") {
		t.Errorf("the detail's help says q quits:\n%s", help)
	}
	edited := plain(driveModel(t, m, "e"))
	if len(runs) != 0 || !strings.Contains(edited, "Phone screen") || !strings.Contains(edited, "Title") {
		t.Errorf("e did not open the Edit form (editor runs %q):\n%s", runs, edited)
	}
	driveModel(t, m, "o")
	if len(runs) != 1 || !strings.HasSuffix(runs[0], "jd.md") {
		t.Errorf("o ran %q, want the Editor on jd.md", runs)
	}
	if v := plain(driveModel(t, m, "q")); strings.Contains(v, "‹ esc") {
		t.Errorf("q did not go back to the list:\n%s", v)
	}
}

// At 120x36 the detail's key line shows m, e, n and J/K beside x on a Lead, and drops x
// first, then J/K, as it narrows.
func TestDetailKeyLineShowsTheMainActions(t *testing.T) {
	line := lastLine(t, 120, 36, "1", "enter")
	for _, want := range []string{"m move", "e edit", "n notes", "J/K next/prev", "x dismiss", "esc back"} {
		if !strings.Contains(line, want) {
			t.Errorf("120x36 detail key line lacks %q: %q", want, line)
		}
	}
	if m, e, n := strings.Index(line, "m move"), strings.Index(line, "e edit"), strings.Index(line, "n notes"); m > e || e > n {
		t.Errorf("detail key line out of priority order: %q", line)
	}
	if line := lastLine(t, 120, 36, "3", "enter"); strings.Contains(line, "x dismiss") || !strings.Contains(line, "e edit") {
		t.Errorf("Applied detail key line: %q", line)
	}
	narrow := lastLine(t, 60, 16, "1", "enter")
	if !strings.Contains(narrow, "m move") || !strings.Contains(narrow, "esc back") {
		t.Errorf("60x16 detail key line lost its core: %q", narrow)
	}
	if strings.Contains(narrow, "x dismiss") && !strings.Contains(narrow, "J/K") {
		t.Errorf("60x16 dropped J/K before x: %q", narrow)
	}
	if strings.Contains(narrow, "J/K") && !strings.Contains(narrow, "n notes") {
		t.Errorf("60x16 dropped n before J/K: %q", narrow)
	}
}

// x in the detail dismisses the open Lead and opens the next row on the same tab; u puts
// it back. On the last row it goes back to the list.
func TestDetailDismissOpensTheNextRow(t *testing.T) {
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 36)
	m = driveModel(t, m, "1", "enter")
	first := detailTitle(plain(m))
	if first == "" {
		t.Fatalf("no detail open:\n%s", plain(m))
	}
	m = driveModel(t, m, "right", "x")
	after := plain(m)
	if !strings.Contains(after, "Dismissed") {
		t.Fatalf("x did not dismiss from the detail:\n%s", after)
	}
	next := detailTitle(after)
	if next == "" || next == first {
		t.Fatalf("x did not open the next row (was %q, now %q):\n%s", first, next, after)
	}
	if !strings.Contains(after, "[Fit]") {
		t.Errorf("the next row's detail is not on the same tab:\n%s", after)
	}
	m = driveModel(t, m, "u")
	if v := plain(m); !strings.Contains(v, "Undone · back to Lead") {
		t.Errorf("u did not undo the dismiss:\n%s", v)
	}
	if v := plain(driveModel(t, m, "esc", "esc")); !strings.Contains(v, first) {
		t.Errorf("the dismissed Lead is not back on the list:\n%s", v)
	}

	last := driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36), "2", "G", "enter", "x")
	if v := plain(last); strings.Contains(v, "‹ esc") || !strings.Contains(v, "Dismissed") {
		t.Errorf("x on the last row did not go back to the list:\n%s", v)
	}
}

// detailTitle is the open detail's Application title: what follows "‹ esc" on its title row.
func detailTitle(view string) string {
	row := lineWith(view, "‹ esc  ")
	_, after, ok := strings.Cut(row, "‹ esc  ")
	if !ok {
		return ""
	}
	title, _, _ := strings.Cut(after, " · ")
	return strings.TrimSpace(title)
}
