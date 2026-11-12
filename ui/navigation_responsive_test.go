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
				view := ansi.Strip(m.View().Content)
				line := strings.Split(view, "\n")[0]
				if keys[0] == "/" && !strings.Contains(view, "/Orbit ×") {
					t.Errorf("missing search context: %s", view)
				}
				if keys[0] == "t" && !strings.Contains(view, "[SWE]") {
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
			line := strings.Split(ansi.Strip(runKeysSized(t, root, size[0], size[1], screen)), "\n")[0]
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
		ctx := strings.Split(ansi.Strip(runKeysSized(t, root, 120, 36, screen)), "\n")[2]
		if got := strings.Contains(ctx, "[All]"); got != want {
			t.Errorf("screen %s: track filter shown = %v, want %v: %q", screen, got, want, ctx)
		}
	}
}

// At wide sizes the tab bar's free right side carries the pipeline at a glance; below
// that it goes, and the wordmark stays down to the medium class.
func TestHeaderPulseAndWordmark(t *testing.T) {
	root := fixtureCopy(t, nil)
	top := func(w, h int) string { return strings.Split(ansi.Strip(runKeysSized(t, root, w, h, "1")), "\n")[0] }
	if wide := top(120, 36); !strings.Contains(wide, "10 live · 1 due") {
		t.Errorf("wide tab bar lacks the pulse: %q", wide)
	}
	if medium := top(80, 24); strings.Contains(medium, "live") || !strings.Contains(medium, "▞ rolecall") {
		t.Errorf("medium tab bar: %q, want the wordmark and no pulse", medium)
	}
	if narrow := top(60, 16); strings.Contains(narrow, "rolecall") || !strings.Contains(narrow, "1 Leads 4") {
		t.Errorf("narrow tab bar: %q, want the active tab whole and no wordmark", narrow)
	}
}
