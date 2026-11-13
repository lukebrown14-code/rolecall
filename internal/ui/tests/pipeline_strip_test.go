package ui_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// stripLine is the pipeline strip's line in a drawn view: the one that holds the Leads
// cell and the Offer cell.
func stripLine(t *testing.T, view string) string {
	t.Helper()
	for _, l := range strings.Split(ansi.Strip(view), "\n") {
		if strings.Contains(l, "◌") && strings.Contains(l, "★") && !strings.Contains(l, "rolecall") {
			return l
		}
	}
	t.Fatalf("no pipeline strip in:\n%s", ansi.Strip(view))
	return ""
}

// stripCounts reads the six Status counts (Leads, Drafts, Applied, Interviewing, Offer,
// closed) off a strip drawn at wide or medium size.
func stripCounts(t *testing.T, line string) []int {
	t.Helper()
	re := regexp.MustCompile(`(Leads|Drafts|Applied|Interviewing|Offer|closed) (\d+)`)
	var out []int
	for _, m := range re.FindAllStringSubmatch(line, -1) {
		n, _ := strconv.Atoi(m[2])
		out = append(out, n)
	}
	if len(out) != 6 {
		t.Fatalf("want six counts in %q, got %v", line, out)
	}
	return out
}

func TestPipelineStripCountsMatchTheListGroups(t *testing.T) {
	view := runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3")
	line := stripLine(t, view)
	if !strings.Contains(line, "──▶") || !strings.Contains(line, "docs ") || !strings.Contains(line, "%") {
		t.Fatalf("wide strip lacks its arrows or docs share: %q", line)
	}
	counts := stripCounts(t, line)
	plain := ansi.Strip(view)
	// The Applied screen's group rules carry the same counts as the strip's cells.
	for i, group := range []string{"", "", "Applied", "Interviewing", "Offer", "Closed"} {
		if group == "" {
			continue
		}
		want := regexp.MustCompile(group + ` · (\d+)`).FindStringSubmatch(plain)
		if want == nil {
			t.Fatalf("no %s group rule in:\n%s", group, plain)
		}
		n, _ := strconv.Atoi(want[1])
		// The closed cell also holds the Dismissed Leads, the theme's Closed group.
		if group == "Closed" {
			if counts[i] < n {
				t.Errorf("closed: strip says %d, fewer than the Closed group's %d", counts[i], n)
			}
			continue
		}
		if n != counts[i] {
			t.Errorf("%s: strip says %d, group rule says %d", group, counts[i], n)
		}
	}
	// Summed, the cells are every listed Application: the explain line's active and closed.
	sum := 0
	for _, n := range counts {
		sum += n
	}
	why := regexp.MustCompile(`(\d+) applications · (\d+) active · (\d+) closed`).FindStringSubmatch(plain)
	if why == nil {
		t.Fatalf("no explain line to sum against:\n%s", plain)
	}
	if total, _ := strconv.Atoi(why[1]); sum != total {
		t.Errorf("strip sums to %d, the Workspace lists %d", sum, total)
	}
}

func TestSTwiceMovesTheStripHighlightAndTheBreadcrumb(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "s", "s"))
	if line := stripLine(t, view); !strings.Contains(line, "[◆ Interviewing 2]") || strings.Count(line, "[") != 1 {
		t.Errorf("highlight is not on Interviewing alone: %q", line)
	}
	if !strings.Contains(view, "Applied › Interviewing ·") {
		t.Errorf("breadcrumb disagrees:\n%s", view)
	}
	if strings.Contains(view, "● Applied ·") || strings.Contains(view, "★ Offer ·") {
		t.Errorf("list is not filtered to Interviewing:\n%s", view)
	}
}

func TestPipelineStripIsOneLineAtEverySize(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {100, 30}, {90, 24}, {80, 24}, {72, 20}, {60, 16}} {
		view := runKeysSized(t, fixtureCopy(t, nil), size[0], size[1], "3")
		line := stripLine(t, view)
		if w := ansi.StringWidth(line); w > size[0] {
			t.Errorf("%dx%d: strip is %d wide: %q", size[0], size[1], w, line)
		}
		if size[0] < 72 && !strings.Contains(line, "◌4 ✎2 ●1 ◆2 ★2 ✕6 ⚑1") {
			t.Errorf("%dx%d: narrow strip is not glyphs and counts: %q", size[0], size[1], line)
		}
		if size[0] < 100 && strings.Contains(line, "──▶") {
			t.Errorf("%dx%d: arrows should be dropped: %q", size[0], size[1], line)
		}
	}
}

func TestMediumStripSeparatesWithDots(t *testing.T) {
	line := stripLine(t, runKeysSized(t, fixtureCopy(t, nil), 90, 24, "3"))
	if !strings.Contains(line, "Leads 4 · ") {
		t.Errorf("medium strip lacks the dot separator: %q", line)
	}
}

func TestTrackOptionsShowCountsAndExplainThem(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3"))
	if !regexp.MustCompile(`SWE \d+`).MatchString(view) || !regexp.MustCompile(`Platform \d+`).MatchString(view) {
		t.Errorf("Track options lack counts:\n%s", view)
	}
	if !regexp.MustCompile(`\d+ applications · \d+ active · \d+ closed`).MatchString(view) {
		t.Errorf("no explain-counts line:\n%s", view)
	}
}

func TestSettingsTracksExplainTheirCounts(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "6", "tab", "tab"))
	if !regexp.MustCompile(`SWE\s+\d+ applications · \d+ active · \d+ closed`).MatchString(view) {
		t.Errorf("Settings Tracks lack the breakdown:\n%s", view)
	}
}

func TestGroupRulesCarryRollups(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3"))
	var rule string
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "Interviewing · 2") {
			rule = l
		}
	}
	if !strings.Contains(rule, "· 1 follow-up due") {
		t.Errorf("Interviewing rule lacks its rollup: %q", rule)
	}
}

// withoutStrip is a drawn view less the pipeline strip's line, for a check on a glyph the
// strip itself draws.
func withoutStrip(view string) string {
	var keep []string
	for _, l := range strings.Split(view, "\n") {
		if !strings.Contains(l, "✎ Drafts") && !strings.Contains(l, "✎2") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

func TestOldStatusBarIsGone(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3"))
	for _, old := range []string{"[All 9]", "10 live", "Closed 4  "} {
		if strings.Contains(view, old) {
			t.Errorf("old status bar text %q still renders:\n%s", old, view)
		}
	}
}

// S focuses the strip: ← → then move the status filter cell by cell, enter keeps it and
// hands the keys back to the list, esc too. The focus shows in the footer and in the
// strip's gutter mark, and the highlighted cell is the one selection style.
func TestFocusedStripMovesTheStatusFilterWithArrowsAndEnter(t *testing.T) {
	for _, tc := range []struct {
		w, h          int
		interviewing  string
		offerHighlit  string
		footerWantKey string
	}{
		{120, 36, "[◆ Interviewing 2]", "[★ Offer 2]", "←→"},
		{80, 24, "[◆2]", "[★2]", "←→"},
		{60, 16, "[◆2]", "[★2]", "←→"},
	} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, tc.w, tc.h)
		m = driveModel(t, m, "3", "S", "right", "right", "right", "right")
		view := ansi.Strip(m.View().Content)
		line := stripLine(t, view)
		if !strings.Contains(line, tc.interviewing) || strings.Count(line, "[") != 1 {
			t.Errorf("%dx%d: → did not highlight Interviewing alone: %q", tc.w, tc.h, line)
		}
		if !strings.Contains(line, "▌") {
			t.Errorf("%dx%d: focused strip has no gutter mark: %q", tc.w, tc.h, line)
		}
		lines := strings.Split(view, "\n")
		foot := lines[len(lines)-1]
		if !strings.Contains(foot, tc.footerWantKey) {
			t.Errorf("%dx%d: footer does not show the strip keys: %q", tc.w, tc.h, foot)
		}
		m = driveModel(t, m, "right", "enter")
		view = ansi.Strip(m.View().Content)
		if line := stripLine(t, view); !strings.Contains(line, tc.offerHighlit) || strings.Contains(line, "▌") {
			t.Errorf("%dx%d: enter should keep Offer and drop focus: %q", tc.w, tc.h, line)
		}
		// Focus is gone: → now cycles the list's view as before, not the strip's cursor.
		m = driveModel(t, m, "S", "esc", "right")
		if line := stripLine(t, ansi.Strip(m.View().Content)); strings.Contains(line, "▌") {
			t.Errorf("%dx%d: esc did not drop strip focus: %q", tc.w, tc.h, line)
		}
	}
}

func TestStripFocusKeyIsInHelp(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "?"))
	if !strings.Contains(view, "[S]") || !strings.Contains(view, "focus strip") {
		t.Errorf("Help does not list the strip focus key:\n%s", view)
	}
}
