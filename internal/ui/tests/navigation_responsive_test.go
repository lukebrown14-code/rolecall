package ui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A filter never costs the tab bar its screens: every digit stays, and the active
// screen keeps its name and count at every size class.
func TestFilteredNavigationKeepsScreenNames(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		for _, keys := range [][]string{{"/", "Orbit", "enter"}, {"t"}} {
			t.Run(fmt.Sprint(size, keys), func(t *testing.T) {
				m := driveStart(t, root, nil, size[0], size[1])
				m = driveModel(t, m, append([]string{"3"}, keys...)...)
				view := chipped(m.View().Content)
				line := strings.Split(view, "\n")[0]
				if keys[0] == "/" && !strings.Contains(view, "/Orbit ×") {
					t.Errorf("missing search context: %s", view)
				}
				if keys[0] == "t" && !strings.Contains(view, "[SWE") && !strings.Contains(view, "t:SWE") {
					t.Errorf("missing track context: %s", view)
				}
				if i := strings.Index(line, "3 Applied "); i < 0 || !strings.Contains(line[i:], "/9") {
					t.Errorf("active tab lost its name or count: %s", line)
				}
				for _, digit := range []string{"1", "2", "4", "5", "6"} {
					if !strings.Contains(line, digit) {
						t.Errorf("navigation missing %q: %s", digit, line)
					}
				}
				if ansi.StringWidth(line) > size[0]-1 {
					t.Errorf("navigation reaches the right edge: %s", line)
				}
			})
		}
	}
}

// Tabs keep the same padding whether active or not, so moving between screens never
// shifts the tab bar.
func TestTabBarDoesNotMoveBetweenScreens(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, size := range [][2]int{{120, 36}, {80, 24}} {
		var want []int
		for _, screen := range []string{"1", "2", "3", "4", "5", "6"} {
			line := strings.Split(chipped(runKeysSized(t, root, size[0], size[1], screen)), "\n")[0]
			var at []int
			for _, digit := range []string{"1 ", "2 ", "3 ", "4 ", "5 ", "6 "} {
				at = append(at, strings.Index(line, digit+map[string]string{"1 ": "Leads", "2 ": "Drafts", "3 ": "Applied", "4 ": "Stats", "5 ": "Profile", "6 ": "Settings"}[digit]))
			}
			if want == nil {
				want = at
			} else if fmt.Sprint(at) != fmt.Sprint(want) {
				t.Errorf("%dx%d screen %s: tabs at %v, screen 1 had them at %v:\n%s", size[0], size[1], screen, at, want, line)
			}
		}
	}
}

// The track filter shows only where it changes what is shown: the Application screens
// and Stats, not Profile or Settings.
func TestTrackFilterOnlyWhereItApplies(t *testing.T) {
	root := fixtureCopy(t, nil)
	for screen, want := range map[string]bool{"1": true, "2": true, "3": true, "4": true, "5": false, "6": false} {
		lines := strings.Split(chipped(runKeysSized(t, root, 120, 36, screen)), "\n")
		ctx := lines[2] + "\n" + lines[3]
		if got := strings.Contains(ctx, "[All"); got != want {
			t.Errorf("screen %s: track filter shown = %v, want %v: %q", screen, got, want, ctx)
		}
	}
}

// The tab bar carries no pipeline pulse (the pipeline strip owns the counts), and the
// wordmark stays down to the medium class.
func TestHeaderPulseAndWordmark(t *testing.T) {
	root := fixtureCopy(t, nil)
	top := func(w, h int) string { return strings.Split(chipped(runKeysSized(t, root, w, h, "1")), "\n")[0] }
	if wide := top(120, 36); strings.Contains(wide, "live") {
		t.Errorf("wide tab bar still carries the pulse: %q", wide)
	}
	if medium := top(80, 24); strings.Contains(medium, "live") || strings.Contains(medium, "▞") {
		t.Errorf("medium tab bar: %q, want no brand mark and no pulse", medium)
	}
	if narrow := top(60, 16); strings.Contains(narrow, "rolecall") || !strings.Contains(narrow, "1 Leads 4") {
		t.Errorf("narrow tab bar: %q, want the active tab whole and no wordmark", narrow)
	}
}

// At wide sizes the brand mark leads the tab bar and the Stats tab carries an 8-cell
// sparkline of weekly Status activity: the fixtures have eight entries on 2026-09-24
// (the seventh week) and one the day after (the last) inside eight weeks of the held
// clock. Below the wide class neither shows, and the tab bar keeps the active screen.
func TestNavBrandMarkAndActivitySparkline(t *testing.T) {
	root := fixtureCopy(t, nil)
	top := func(w, h int) string { return strings.Split(chipped(runKeysSized(t, root, w, h, "1")), "\n")[0] }
	wide := top(120, 36)
	if !strings.HasPrefix(wide, "▞ rolecall") {
		t.Errorf("wide tab bar does not lead with the brand mark: %q", wide)
	}
	if !strings.Contains(wide, "Stats ▁▁▁▁▁▁█▂") {
		t.Errorf("wide tab bar lacks the sparkline beside Stats: %q", wide)
	}
	for _, size := range [][2]int{{80, 24}, {60, 16}} {
		bar := top(size[0], size[1])
		if strings.ContainsAny(bar, "▞▁▂▃▄▅▆▇█") || strings.Contains(bar, "rolecall") {
			t.Errorf("%dx%d tab bar shows the mark or sparkline: %q", size[0], size[1], bar)
		}
		if !strings.Contains(bar, "1 Leads 4") {
			t.Errorf("%dx%d tab bar lost the active tab: %q", size[0], size[1], bar)
		}
	}
	t.Setenv("ROLECALL_ASCII", "1")
	bar := top(120, 36)
	for _, r := range bar {
		if r > 126 {
			t.Fatalf("ASCII tab bar has non-ASCII %q: %q", r, bar)
		}
	}
	if !strings.Contains(bar, "Stats ______#.") {
		t.Errorf("ASCII tab bar lacks the sparkline: %q", bar)
	}
}
