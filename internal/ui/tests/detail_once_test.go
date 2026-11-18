package ui_test

import (
	"strings"
	"testing"
)

// The malformed detail has no tabs and no card: its key line offers no tab key, and the
// error, its file and the keys that fix it take the whole width, with nothing beside them.
func TestMalformedDetailShowsTheErrorAndItsFixKeys(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {160, 44}, {80, 24}} {
		d := newDriver(t, fixtureCopy(t, nil), fixtureClock, size[0], size[1])
		d.Type("/malformed")
		d.Type("enter")
		d.Type("enter")
		d.Type("right") // no tab to switch to: the view stays
		view := d.plain()
		lines := strings.Split(view, "\n")
		keyLine := lines[len(lines)-1]
		if !strings.Contains(view, "Can't read SWE/malformed") {
			t.Fatalf("%v: not the malformed detail:\n%s", size, view)
		}
		if strings.Contains(keyLine, "tab") || strings.Contains(keyLine, "←→") {
			t.Errorf("%v: malformed detail key line offers tabs: %q", size, keyLine)
		}
		for _, want := range []string{"meta.yml line 4", "jobs/SWE/malformed/meta.yml", "e fix in editor  ·  r rescan"} {
			if !strings.Contains(view, want) {
				t.Errorf("%v: malformed detail lacks %q:\n%s", size, want, view)
			}
		}
		if strings.Contains(view, "┬") || strings.Contains(view, "│") {
			t.Errorf("%v: malformed detail splits off an empty pane:\n%s", size, view)
		}
	}
}

// At medium widths the strip under the title draws the timeline as a track, so the
// Overview does not repeat it as a TIMELINE block, and the FOLLOW-UP line names l once.
func TestMediumDetailShowsEachFactOnce(t *testing.T) {
	for _, size := range [][2]int{{90, 30}, {64, 24}} {
		d := newDriver(t, fixtureCopy(t, nil), fixtureClock, size[0], size[1])
		d.Type("3")
		d.Type("enter")
		view := d.plain()
		if !strings.Contains(view, "◌ Lead 24 Sep ─ ● Applied 25 Sep") {
			t.Fatalf("%v: strip lacks the whole track:\n%s", size, view)
		}
		if strings.Contains(view, "TIMELINE") || strings.Count(view, "24 Sep") != 1 || strings.Count(view, "25 Sep") != 1 {
			t.Errorf("%v: timeline shown twice:\n%s", size, view)
		}
		d.Type("J") // Engineer #1, Follow-up due
		view = d.plain()
		if !strings.Contains(view, "FOLLOW-UP") {
			t.Fatalf("%v: not the Follow-up due Application:\n%s", size, view)
		}
		if n := strings.Count(view, "followed up"); n != 1 {
			t.Errorf("%v: the follow-up action is named %d times:\n%s", size, n, view)
		}
	}
}

// Where the strip's track has to shed dates, the Overview keeps the TIMELINE block, so
// no date is lost.
func TestNarrowDetailKeepsTheTimelineWhenTheTrackSheds(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 56, 24)
	d.Type("3")
	d.Type("enter")
	if view := d.plain(); !strings.Contains(view, "TIMELINE") || !strings.Contains(view, "24 Sep") {
		t.Errorf("narrow detail lost the timeline's dates:\n%s", view)
	}
}
