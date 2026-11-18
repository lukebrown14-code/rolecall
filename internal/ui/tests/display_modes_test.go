package ui_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Stats, the tab bar, pop-up layering, the Track filter and the display modes
// (NO_COLOR, ROLECALL_PLAIN, the light theme) from the design review.

// One rule sheds the tab bar by width: other screens' counts, then the gap between tabs,
// then other screens' names. From 61 columns every screen stays named; the active chip never
// touches the tab before it.
func TestNarrowTabBarKeepsNamesFrom61Columns(t *testing.T) {
	for screen, want := range map[string]string{
		"4": "1 Leads 2 Drafts 3 Pipeline [4 Stats] 5 Profile 6 Settings",
		"5": "1 Leads 2 Drafts 3 Pipeline 4 Stats [5 Profile] 6 Settings",
		"6": "1 Leads 2 Drafts 3 Pipeline 4 Stats 5 Profile [6 Settings]",
	} {
		bar := strings.TrimSpace(strings.Split(chipped(runKeysSized(t, fixtureCopy(t, nil), 61, 24, screen)), "\n")[0])
		if bar != want {
			t.Errorf("screen %s: tab bar %q, want %q", screen, bar, want)
		}
	}
	// Below 60 the names go for every screen alike, and the chip is still spaced.
	for screen, want := range map[string]string{"4": "1 2 3 [4 Stats] 5 6", "5": "1 2 3 4 [5 Profile] 6", "6": "1 2 3 4 5 [6 Settings]"} {
		bar := strings.TrimSpace(strings.Split(chipped(runKeysSized(t, fixtureCopy(t, nil), 56, 24, screen)), "\n")[0])
		if bar != want {
			t.Errorf("screen %s at 56: tab bar %q, want %q", screen, bar, want)
		}
	}
}

// The tab bar's detail never depends on which screen is open: every screen's bar at a width
// names the same set of other screens.
func TestTabBarDetailDoesNotDependOnTheScreen(t *testing.T) {
	for _, w := range []int{56, 60, 64, 66, 72, 80} {
		var named []int
		for _, screen := range []string{"1", "2", "3", "4", "5", "6"} {
			bar := chipped(runKeysSized(t, fixtureCopy(t, nil), w, 24, screen))
			bar = strings.Split(bar, "\n")[0]
			n := 0
			for _, name := range []string{"Leads", "Drafts", "Pipeline", "Stats", "Profile", "Settings"} {
				if strings.Contains(bar, name) {
					n++
				}
			}
			named = append(named, n)
		}
		for _, n := range named[1:] {
			if n != named[0] {
				t.Errorf("%d columns: screens name %v screens in their bar", w, named)
				break
			}
		}
	}
}

// Below the medium width Settings has no group list beside its rows, so the key line
// keeps ←→ section, even with esc back on it.
func TestNarrowSettingsKeepsTheSectionKey(t *testing.T) {
	for _, keys := range [][]string{{"6"}, {"5", "6"}, {"6", "right"}} {
		if line := lastLine(t, 64, 24, keys...); !strings.Contains(line, "←→ section") {
			t.Errorf("%v: key line %q lacks ←→ section", keys, line)
		}
	}
}

// With room, the tab bar's sparkline says what it counts.
func TestTabBarSparklineIsLabelled(t *testing.T) {
	bar := strings.Split(chipped(runKeysSized(t, fixtureCopy(t, nil), 160, 40, "1")), "\n")[0]
	if !strings.Contains(bar, "4 Stats  ▁▁▁▁▁▁█▁ applied/wk ") {
		t.Errorf("wide tab bar sparkline lacks its label: %q", bar)
	}
}

// The weekly chart draws every one of its 12 weeks, the empty ones on the baseline, with
// the busiest week's count.
func TestStatsWeeklyChartShowsEveryWeek(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 40, "4"))
	line := lineWith(view, "APPLIED / WEEK")
	if !strings.Contains(line, "▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁██▁▁") || !strings.Contains(line, "max 1/wk") {
		t.Errorf("weekly chart %q, want 12 two-cell weeks on the baseline and the peak", line)
	}
}

// One bar grammar: the funnel's bars and the Response rate's are the same width, a
// solid run on a faint track.
func TestStatsBarsShareOneTrack(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 40, "4"))
	bar := regexp.MustCompile(`[█░]+`)
	widths := map[int]bool{}
	for _, name := range []string{"◌ Lead", "★ Offer", "RESPONSE RATE"} {
		run := bar.FindString(lineWith(view, name))
		if !strings.Contains(run, "░") {
			t.Errorf("%s bar %q has no track", name, run)
		}
		widths[len([]rune(run))] = true
	}
	if len(widths) != 1 {
		t.Errorf("the bars differ in width: %v\n%s", widths, view)
	}
}

// Wide Stats sets the weekly chart and the per-track table beside the funnel; NOW says
// what is due today and which key goes there.
func TestStatsUsesTheWidth(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 160, 44, "4"))
	if !strings.Contains(lineWith(view, "FUNNEL"), "APPLIED / WEEK") {
		t.Errorf("wide Stats does not set the weekly chart beside the funnel:\n%s", view)
	}
	if now := lineWith(view, "NOW"); !strings.Contains(now, "follow-up") || !strings.Contains(now, "→ 3 Pipeline") {
		t.Errorf("NOW = %q, want the follow-ups due and the key to them", now)
	}
	if !strings.Contains(view, "INSIGHTS") || strings.Contains(view, "COMMON GAPS") {
		t.Errorf("the gap panel is not called Insights:\n%s", view)
	}
}

// The per-track table lists the busiest Track first and folds the Tracks with nothing
// applied into one line, so no all-zero row shows.
func TestStatsTracksBusiestFirstWithoutZeroRows(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 60, "4"))
	unknown, swe := strings.Index(view, "Unknown               6"), strings.Index(view, "SWE                   5")
	if unknown < 0 || swe < 0 || unknown > swe {
		t.Errorf("By track is not busiest first:\n%s", view)
	}
	if strings.Contains(view, "Platform              0") || !strings.Contains(view, "No Applications sent: Platform") {
		t.Errorf("By track shows a zero row:\n%s", view)
	}
}

// A pop-up leaves the frame beside it in view: the rows it covers keep their text left
// and right of it.
func TestPopUpKeepsTheFrameBesideIt(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 160, 44, "1", "m"))
	row := lineWith(view, "╭─ Move")
	left := strings.TrimSpace(row[:strings.Index(row, "╭")])
	if left == "" {
		t.Errorf("the rows behind the pop-up are blanked edge to edge: %q", row)
	}
	if right := strings.TrimSpace(row[strings.LastIndex(row, "╮")+len("╮"):]); right == "" {
		t.Errorf("the preview beside the pop-up is blanked: %q", row)
	}
}

// With NO_COLOR, nothing behind a pop-up is bold: the pop-up's title is the one bold on
// screen, ringed by a cleared margin.
func TestNoColorBackdropDropsBold(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	bold := regexp.MustCompile("\x1b\\[([0-9;]*;)?1(;[0-9;]*)?m")
	before := strings.Split(runKeys(t, fixtureCopy(t, nil), 120, "1"), "\n")[0]
	if !bold.MatchString(before) {
		t.Fatalf("the active tab is not bold without a pop-up, so the test proves nothing: %q", before)
	}
	view := runKeys(t, fixtureCopy(t, nil), 120, "1", "m")
	if bar := strings.Split(view, "\n")[0]; bold.MatchString(bar) {
		t.Errorf("the tab bar behind the pop-up is bold: %q", bar)
	}
	if !bold.MatchString(view) {
		t.Errorf("the pop-up lost its bold title")
	}
}

// t and T pass over Tracks with no rows in this view, and their chips are not drawn.
func TestTrackCycleSkipsEmptyTracks(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "3")
	for i := 0; i < 6; i++ {
		m = driveModel(t, m, "t")
		ctx := strings.Split(plain(m), "\n")[2]
		if strings.Contains(ctx, "Platform") {
			t.Fatalf("press %d: the Track filter offers Platform, empty on Applied: %q", i+1, ctx)
		}
		if strings.Contains(ctx, " 0]") {
			t.Fatalf("press %d: t landed on an empty Track: %q", i+1, ctx)
		}
	}
}

// ROLECALL_PLAIN draws one region: no preview beside the list at any width, no line of
// rules alone, Status glyphs said as words, and the terminal cursor on the selection.
func TestPlainModeIsOneRegionWithTheCursorOnTheSelection(t *testing.T) {
	t.Setenv("ROLECALL_PLAIN", "1")
	m := driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, 160, 44), "3", "j")
	v := m.View()
	lines := strings.Split(ansi.Strip(v.Content), "\n")
	boxOnly := regexp.MustCompile(`^[\s─│┬┴┼├┤╭╮╰╯]+$`)
	for _, l := range lines {
		if strings.Contains(l, "┬") || strings.Contains(l, "NEXT ") {
			t.Errorf("plain mode draws a second pane: %q", l)
		}
		if strings.TrimSpace(l) != "" && boxOnly.MatchString(l) {
			t.Errorf("plain mode draws a line of rules: %q", l)
		}
	}
	if !strings.Contains(ansi.Strip(v.Content), "Interviewing Engineer #1") {
		t.Errorf("plain mode keeps a bare Status glyph before a title:\n%s", ansi.Strip(v.Content))
	}
	if v.Cursor == nil {
		t.Fatal("plain mode does not place the terminal cursor")
	}
	if row := lines[v.Cursor.Y]; !strings.Contains(row, "▌") || !strings.Contains(row, "Engineer #1") {
		t.Errorf("the cursor is on %q, not the selected row", row)
	}
}

// The light theme paints its white background on every cell with SGR, so a terminal
// that ignores the OSC 11 background still shows dark text on white.
func TestLightThemePaintsEveryCell(t *testing.T) {
	t.Setenv("ROLECALL_THEME", "light")
	t.Setenv("COLORFGBG", "")
	const white = "\x1b[48;2;255;255;255m"
	view := runKeys(t, fixtureCopy(t, nil), 120, "1", "m")
	for i, l := range strings.Split(view, "\n") {
		if !strings.HasPrefix(l, white) || ansi.StringWidth(l) != 120 {
			t.Fatalf("line %d is not painted white edge to edge: %q", i, l)
		}
		// Every reset but the line's last puts the white back.
		body := strings.TrimSuffix(l, "\x1b[m")
		if strings.Count(body, "\x1b[m") != strings.Count(body, "\x1b[m"+white) {
			t.Fatalf("line %d: a reset leaves the terminal's own background: %q", i, l)
		}
	}
}
