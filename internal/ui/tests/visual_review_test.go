package ui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// Fixes from the visual review of the design overhaul (#166).

// The key line ends with "? help" and "space menu" on every screen where space opens the
// Workspace menu, and keeps both at every size, dropping lower-priority keys first.
func TestKeyLineEndsWithHelpAndSpaceMenu(t *testing.T) {
	screens := map[string][]string{
		"leads":    {"1"},
		"applied":  {"3"},
		"detail":   {"3", "enter"},
		"stats":    {"4"},
		"profile":  {"5"},
		"settings": {"6"},
	}
	for _, sz := range keyLineSizes {
		for name, keys := range screens {
			t.Run(fmt.Sprintf("%s/%dx%d", name, sz[0], sz[1]), func(t *testing.T) {
				line := lastLine(t, sz[0], sz[1], keys...)
				if !strings.HasSuffix(strings.TrimRight(line, " "), "? help · space menu") {
					t.Errorf("key line does not end with ? help · space menu: %q", line)
				}
				if w := ansi.StringWidth(line); w > sz[0] {
					t.Errorf("key line is %d wide in %d: %q", w, sz[0], line)
				}
			})
		}
	}
}

// Help lists every key: space opens the menu, u undoes the last move even before there is
// one, and the two keys of the command palette share one row.
func TestHelpListsMenuUndoAndOneCommandsRow(t *testing.T) {
	for _, sz := range keyLineSizes[:2] {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "1", "?"))
		for _, want := range []string{"[space]  menu", "[u]      undo", "[: ctrl+k] commands", "[/]        search"} { // the column's words line up after the merged key
			if !strings.Contains(view, want) {
				t.Errorf("%dx%d help lacks %q:\n%s", sz[0], sz[1], want, view)
			}
		}
		if n := strings.Count(view, "commands"); n != 1 {
			t.Errorf("%dx%d help lists commands %d times:\n%s", sz[0], sz[1], n, view)
		}
	}
}

// An unsaved edit's saved value sits right after the edited value, not out in the column
// the group's longest value sets.
func TestSettingsWasSitsBesideTheEditedValue(t *testing.T) {
	for _, sz := range keyLineSizes {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1],
			"6", "]", "]", "j", "j", "j", "enter", "backspace", "backspace", "7", "enter"))
		if row := lineWith(view, "Follow-up due after"); !strings.Contains(row, "7 days  was 14") {
			t.Errorf("%dx%d: was 14 is not beside 7 days: %q\n%s", sz[0], sz[1], row, view)
		}
	}
}

// At medium width the strip still names its Statuses, short where it must, and keeps the
// docs share; only narrow drops the labels, and it keeps the share when there is room.
func TestMediumStripKeepsLabelsAndDocs(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		want []string
	}{
		{80, 24, []string{"◌ Leads 4", "◆ Interv. 2", "★ Offer 2", "docs 8%"}},
		{60, 16, []string{"◌4", "◆2", "docs 8%"}},
	} {
		line := stripLine(t, runKeysSized(t, fixtureCopy(t, nil), tc.w, tc.h, "3"))
		for _, want := range tc.want {
			if !strings.Contains(line, want) {
				t.Errorf("%dx%d strip lacks %q: %q", tc.w, tc.h, want, line)
			}
		}
		if w := ansi.StringWidth(line); w >= tc.w {
			t.Errorf("%dx%d strip is %d wide: %q", tc.w, tc.h, w, line)
		}
	}
}

// The Workspace menu's cursor stands apart from its label, "▌ ▸ Leads", and every row's
// label starts in the same column.
func TestWorkspaceMenuCursorIsSpacedAndLabelsAlign(t *testing.T) {
	for _, sz := range keyLineSizes {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "3", " "))
		sel := lineWith(view, "▌ ▸ Leads")
		if sel == "" {
			t.Fatalf("%dx%d: no \"▌ ▸ Leads\" row:\n%s", sz[0], sz[1], view)
		}
		want := colOf(sel, "Leads")
		box := sel[:strings.Index(sel, "▌")] // the box's left edge, as the selected row starts
		for _, label := range []string{"Drafts", "Applied", "Settings"} {
			row := ""
			for _, l := range strings.Split(view, "\n") {
				if strings.HasPrefix(l, box) && strings.Contains(l, label) && colOf(l, label) < colOf(sel, "1│") {
					row = l
					break
				}
			}
			if got := colOf(row, label); got != want {
				t.Errorf("%dx%d: %s at column %d, Leads at %d:\n%s\n%s", sz[0], sz[1], label, got, want, sel, row)
			}
		}
	}
}

// Where the card is folded away, detail opens on Overview, the first tab, which holds the
// card's rows; beside a card it opens on the JD.
func TestDetailOpensOnOverviewWhereTheCardIsFolded(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		tabs string
	}{
		{120, 36, "[JD]"},
		{80, 24, "[Overview]  JD"},
		{60, 16, "[Overview]  JD"},
	} {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), tc.w, tc.h, "3", "enter"))
		if !strings.Contains(view, tc.tabs) {
			t.Errorf("%dx%d: tabs do not start %q:\n%s", tc.w, tc.h, tc.tabs, view)
		}
		if tc.w < 120 && !strings.Contains(lineWith(view, "STATUS"), "[m] move") {
			t.Errorf("%dx%d: Overview does not show the card's rows:\n%s", tc.w, tc.h, view)
		}
		if n := strings.Count(view, "NEXT "); n != 1 {
			t.Errorf("%dx%d: the next step shows %d times:\n%s", tc.w, tc.h, n, view)
		}
	}
}

// Widening the window brings the card back and folds the Overview tab away: the detail
// moves to the JD quietly, with no error.
func TestWideningFromOverviewMovesToTheJD(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 80, 24)
	m = driveModel(t, m, "3", "enter")
	m = runMsg(t, m, tea.WindowSizeMsg{Width: 120, Height: 36})
	view := chipped(m.View().Content)
	if !strings.Contains(view, "[JD]") || strings.Contains(view, "gone") {
		t.Errorf("widened detail is not on the JD:\n%s", view)
	}
}

// While the strip has the keys, its gutter mark is the only one: the list row under the
// cursor gives up its mark until the focus comes back.
func TestFocusedStripIsTheOnlyMarkedLine(t *testing.T) {
	for _, sz := range keyLineSizes {
		view := runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "3", "S", "right")
		marked := markedLines(view, "▌")
		if len(marked) != 1 || !strings.Contains(chipped(marked[0]), "[◌") {
			t.Errorf("%dx%d: want the strip alone marked, got %d lines:\n%s", sz[0], sz[1], len(marked), chipped(strings.Join(marked, "\n")))
		}
		back := runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "3", "S", "esc")
		if n := len(markedLines(back, "▌")); n != 1 {
			t.Errorf("%dx%d: after esc %d lines are marked", sz[0], sz[1], n)
		}
	}
}

// lineWith is the first line of view containing s.
func lineWith(view, s string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, s) {
			return l
		}
	}
	return ""
}

// lineAfter is the line after the first line of view containing s.
func lineAfter(view, s string) string {
	lines := strings.Split(view, "\n")
	for i, l := range lines[:len(lines)-1] {
		if strings.Contains(l, s) {
			return lines[i+1]
		}
	}
	return ""
}

// colOf is the display column where s starts in line, or -1.
func colOf(line, s string) int {
	i := strings.Index(line, s)
	if i < 0 {
		return -1
	}
	return ansi.StringWidth(line[:i])
}

// The next-step row is a field row like the rest: label, value, then [key] action ending
// at the same right edge as the STATUS row's.
func TestNextAndFollowUpRowsAreFieldRows(t *testing.T) {
	for _, tc := range []struct {
		name       string
		keys       []string
		label, key string
		harness    bool
	}{
		{"next", []string{"3"}, "NEXT    wait to hear back", "[m] move", false},
		{"lead next", []string{"1"}, "NEXT    add the JD", "[enter] open", false},
		{"follow-up", []string{"3", "/Engineer", "enter", "j"}, "FOLLOW-UP  due 259 days ago", "[n] note", false},
		{"follow-up with a Harness", []string{"3", "/Engineer", "enter", "j"}, "FOLLOW-UP  due 259 days ago", "[r] draft nudge", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opts []ui.Option
			if tc.harness {
				opts = append(opts, ui.WithHarness("h {skill} {dir}"))
			}
			m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36, opts...)
			view := chipped(driveModel(t, m, tc.keys...).View().Content)
			row := lineWith(view, tc.label)
			if row == "" || !strings.Contains(row, tc.key) {
				t.Fatalf("no row %q … %q:\n%s", tc.label, tc.key, view)
			}
			status := lineWith(view, "STATUS")
			end := func(l, s string) int { return colOf(l, s) + ansi.StringWidth(s) }
			if got, want := end(row, tc.key), end(status, "[m] move"); got != want {
				t.Errorf("%q ends at %d, STATUS row's key at %d:\n%s\n%s", tc.key, got, want, status, row)
			}
		})
	}
}

// A Resume only Application's Cover letter is one Docs row, marked and lined up with the
// others.
func TestResumeOnlyCoverLetterRowIsAligned(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "/docs-resume-only", "enter"))
	row := lineWith(view, "Cover letter")
	if !strings.Contains(row, "— Cover letter  not needed") {
		t.Fatalf("Cover letter row = %q:\n%s", row, view)
	}
	mark := func(l string) int {
		for _, g := range []string{"● ", "○ ", "— "} {
			if c := colOf(l, g); c >= 0 {
				return c
			}
		}
		return -1
	}
	resume := lineWith(view, " Resume")
	if mark(row) < 0 || mark(row) != mark(resume) {
		t.Errorf("Cover letter mark at %d, Resume mark at %d:\n%s\n%s", mark(row), mark(resume), resume, row)
	}
}

// A malformed row is two lines, but only its first carries the selection mark (#169).
func TestMalformedRowCarriesOneMark(t *testing.T) {
	for _, sz := range keyLineSizes {
		t.Run(fmt.Sprintf("%dx%d", sz[0], sz[1]), func(t *testing.T) {
			view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "G"))
			if n := strings.Count(view, "▌"); n != 1 {
				t.Errorf("malformed row shows %d marks, want 1:\n%s", n, view)
			}
		})
	}
}

// The detail of a malformed Application wraps the fix to the pane: no line is wider than
// the terminal.
func TestMalformedDetailFitsTheWidth(t *testing.T) {
	for _, sz := range keyLineSizes {
		t.Run(fmt.Sprintf("%dx%d", sz[0], sz[1]), func(t *testing.T) {
			view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "G", "enter"))
			for _, l := range strings.Split(view, "\n") {
				if w := ansi.StringWidth(l); w > sz[0] {
					t.Errorf("line is %d wide in %d: %q", w, sz[0], l)
				}
			}
			if !strings.Contains(view, "status_log") {
				t.Errorf("detail lost the fix:\n%s", view)
			}
		})
	}
}

// The strip, the tab bar and the Track filter draw the active option as the design's
// filled block, " ✎ Drafts 2 ", with no brackets or underline; brackets come back only
// where there is no fill to see (NO_COLOR, covered in a11y_test).
func TestBarSelectionIsAFilledBlock(t *testing.T) {
	view := runKeysSized(t, fixtureCopy(t, nil), 120, 36, "2")
	plain := ansi.Strip(view)
	for _, gone := range []string{"[✎ Drafts 2]", "[All 17]", "[2 Drafts 2]"} {
		if strings.Contains(plain, gone) {
			t.Errorf("bar still brackets %q:\n%s", gone, plain)
		}
	}
	for _, want := range []string{" ✎ Drafts 2 ", " All 17 ", " 2 Drafts 2 "} {
		if !strings.Contains(plain, want) {
			t.Errorf("bar lacks %q:\n%s", want, plain)
		}
	}
	if strings.Contains(view, "\x1b[4m") || strings.Contains(view, "\x1b[4;") || strings.Contains(view, "\x1b[1;4;") {
		t.Errorf("bar selection is still underlined:\n%q", view)
	}
}

// In the strip a cell's label takes its Status colour and its count the bright text
// colour, as in the design mock: "◆ Interviewing" pink, "2" white.
func TestStripCountsAreBright(t *testing.T) {
	view := runKeysSized(t, fixtureCopy(t, nil), 120, 36, "1")
	strip := ""
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(chipped(l), "Interviewing 2") {
			strip = l
			break
		}
	}
	fg := "38;2;230;237;243m2"
	i := strings.Index(strip, "Interviewing")
	if i < 0 || !strings.Contains(strip[i:min(len(strip), i+60)], fg) {
		t.Errorf("Interviewing's count is not in the text colour: %q", strip)
	}
}
