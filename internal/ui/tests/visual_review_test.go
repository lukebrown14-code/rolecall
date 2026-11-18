package ui_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// Fixes from the visual review of the design overhaul (#166).

// space and ? have one place at every width: the key line's right end, ? help last. Where
// the line is short, space commands is the first key it drops, before the screen's own
// actions; ? help names it. The tab bar never carries them.
func TestKeyLineEndsWithHelpAndSpaceCommands(t *testing.T) {
	screens := map[string][]string{
		"leads":    {"1"},
		"applied":  {"3"},
		"detail":   {"3", "enter"},
		"stats":    {"4"},
		"profile":  {"5"},
		"settings": {"6"},
	}
	for _, sz := range append(keyLineSizes, [2]int{160, 44}, [2]int{64, 24}) {
		for name, keys := range screens {
			t.Run(fmt.Sprintf("%s/%dx%d", name, sz[0], sz[1]), func(t *testing.T) {
				view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], keys...))
				line := strings.TrimRight(keyLineOf(view), " ")
				if !strings.HasSuffix(line, "? help") {
					t.Errorf("key line does not end with ? help: %q", line)
				}
				if strings.Contains(line, "space commands") && !strings.HasSuffix(line, "space commands  ? help") {
					t.Errorf("space commands is not beside ? help: %q", line)
				}
				if bar := strings.SplitN(view, "\n", 2)[0]; strings.Contains(bar, "? help") || strings.Contains(bar, "space") {
					t.Errorf("tab bar carries space or ?: %q", bar)
				}
				if w := ansi.StringWidth(line); w > sz[0] {
					t.Errorf("key line is %d wide in %d: %q", w, sz[0], line)
				}
			})
		}
	}
	// With room, both show.
	for _, keys := range [][]string{{"1"}, {"3"}, {"6"}} {
		if line := lastLine(t, 120, 36, keys...); !strings.HasSuffix(strings.TrimRight(line, " "), "space commands  ? help") {
			t.Errorf("%v at 120: key line lacks space commands  ? help: %q", keys, line)
		}
	}
}

// A short key line keeps the screen's main action and drops the generic space commands
// first: x dismiss on Leads at 64x24, enter edit on Settings at 56x14.
func TestShortKeyLineKeepsTheMainAction(t *testing.T) {
	for _, tc := range []struct {
		w, h int
		keys []string
		want string
	}{
		{64, 24, []string{"1"}, "x dismiss"},
		{56, 14, []string{"6"}, "enter edit"},
	} {
		line := lastLine(t, tc.w, tc.h, tc.keys...)
		if !strings.Contains(line, tc.want) || !strings.Contains(line, "? help") {
			t.Errorf("%dx%d %v: key line lacks %q or ? help: %q", tc.w, tc.h, tc.keys, tc.want, line)
		}
	}
}

// Help lists every key: u undoes the last move even before there is one, and the keys
// of the command finder share one row.
func TestHelpListsUndoAndOneCommandsRow(t *testing.T) {
	for _, sz := range keyLineSizes[:2] {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "1", "?"))
		for _, want := range []string{"u undo", ": commands", "/ search"} {
			if !helpLists(view, want) {
				t.Errorf("%dx%d help lacks %q:\n%s", sz[0], sz[1], want, view)
			}
		}
		// The tab bar (wide) also names the key, so count below it.
		if n := strings.Count(strings.SplitN(view, "\n", 2)[1], "commands"); n != 1 {
			t.Errorf("%dx%d help lists commands %d times:\n%s", sz[0], sz[1], n, view)
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
		if tc.w < 120 && !strings.Contains(lineWith(view, "NEXT  "), "m move") {
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

// lineWith is the first line of view containing s.
func lineWith(view, s string) string {
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, s) {
			return l
		}
	}
	return ""
}

// hasLineWith reports a line of view that contains every one of parts.
func hasLineWith(view string, parts ...string) bool {
	for _, l := range strings.Split(view, "\n") {
		all := true
		for _, p := range parts {
			all = all && strings.Contains(l, p)
		}
		if all {
			return true
		}
	}
	return false
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
		{"next", []string{"3"}, "NEXT         wait to hear back", "m move", false},
		{"lead next", []string{"1"}, "NEXT         add the JD", "enter open", false},
		{"follow-up", []string{"3", "/Engineer", "enter", "j"}, "FOLLOW-UP    due · no change in 9mo", "l followed up", false},
		{"follow-up with a Harness", []string{"3", "/Engineer", "enter", "j"}, "FOLLOW-UP    due · no change in 9mo", "F draft follow-up", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opts []ui.Option
			if tc.harness {
				opts = append(opts, ui.WithHarness("h {skill} {dir}"))
			}
			m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36, opts...)
			view := chipped(driveModel(t, m, tc.keys...).View().Content)
			// The narrow card puts the key on the line under the words.
			row := lineWith(view, tc.label)
			if !strings.Contains(row, tc.key) {
				row = lineAfter(view, tc.label)
			}
			if row == "" || !strings.Contains(row, tc.key) {
				t.Fatalf("no row %q … %q:\n%s", tc.label, tc.key, view)
			}
			notes := lineWith(view, "NOTES  ")
			end := func(l, s string) int { return colOf(l, s) + ansi.StringWidth(s) }
			nk := strings.Fields(notes)
			last := nk[len(nk)-2] + " " + nk[len(nk)-1]
			if got, want := end(row, tc.key), end(notes, last); got != want {
				t.Errorf("%q ends at %d, NOTES row's key at %d:\n%s\n%s", tc.key, got, want, notes, row)
			}
		})
	}
}

// A Resume only Application's Cover letter is one Docs row, marked and lined up with the
// others.
func TestResumeOnlyCoverLetterRowIsAligned(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "/docs-resume-only", "enter"))
	row := lineWith(view, "Cover letter")
	if !strings.Contains(row, "✓ Resume") || !strings.Contains(row, "— Cover letter") {
		t.Fatalf("docs line = %q, want the Resume and a dashed Cover letter side by side:\n%s", row, view)
	}
}

// A malformed row is two lines under one selection bar, like any row: no line outside it
// is marked (#169).
func TestMalformedRowCarriesOneMark(t *testing.T) {
	for _, sz := range keyLineSizes {
		t.Run(fmt.Sprintf("%dx%d", sz[0], sz[1]), func(t *testing.T) {
			view := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], "G"))
			if n := strings.Count(view, "▌"); n != 2 || len(markedRows(view, "▌")) != 1 {
				t.Errorf("malformed row shows %d marks, want its two lines:\n%s", n, view)
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

// The tab bar, the filter row and the detail tabs draw the active option filled and
// bracketed, "[Drafts 2]", so it reads without colour (the fill alone is under 3:1 on the
// page); it is underlined only where there is no fill (NO_COLOR, covered in a11y_test).
func TestBarSelectionIsAFilledBlock(t *testing.T) {
	view := runKeysSized(t, fixtureCopy(t, nil), 120, 36, "2")
	plain := ansi.Strip(view)
	for _, want := range []string{"[Drafts 2]", "[All 2]", "[2 Drafts 2]"} {
		if !strings.Contains(plain, want) {
			t.Errorf("bar lacks %q:\n%s", want, plain)
		}
	}
	detail := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "enter"))
	if !strings.Contains(detail, "[JD]") {
		t.Errorf("the active detail tab is not bracketed:\n%s", detail)
	}
	if strings.Contains(view, "\x1b[4m") || strings.Contains(view, "\x1b[4;") || strings.Contains(view, "\x1b[1;4;") {
		t.Errorf("bar selection is still underlined:\n%q", view)
	}
}

// Opened with a relative --dir such as ./fixtures/workspace, a malformed row still names
// its folder under jobs/, "SWE/malformed", not the whole path.
func TestMalformedRowNamesTheFolderWithARelativeDir(t *testing.T) {
	root := fixtureCopy(t, nil)
	t.Chdir(filepath.Dir(root))
	view := chipped(runKeysSized(t, "./"+filepath.Base(root), 120, 36, "G"))
	if !strings.Contains(view, "⚠ Can't read SWE/malformed") {
		t.Errorf("malformed row is not named under jobs/:\n%s", view)
	}
}

// A key is offered only where it works: without a Harness or Plugin the detail's DOCS row
// offers neither p nor b, as the key line and NEXT do not.
func TestDetailRowsHideHarnessKeysWithoutAHarness(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "1"))
	for _, bad := range []string{"p prepare docs", "b build"} {
		if strings.Contains(view, bad) {
			t.Errorf("detail offers %q with nothing set:\n%s", bad, view)
		}
	}
	if strings.Contains(view, "read-only") {
		t.Errorf("FIT still reads read-only:\n%s", view)
	}
}
