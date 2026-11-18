package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// appliedAt writes an Applied Application whose last change is the given number of days
// before the fixture clock, so it is that many days minus the follow-up period past due.
func appliedAt(t *testing.T, root, name, company string, daysAgo int) {
	t.Helper()
	dir := filepath.Join(root, "jobs", "SWE", name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	at := fixtureNow.AddDate(0, 0, -daysAgo).Format("2006-01-02")
	meta := "schema: 1\ntitle: " + name + "\ncompany: " + company + "\ntrack: SWE\nstatus_log:\n  - {status: Lead, at: " + at + "}\n  - {status: Applied, at: " + at + "}\n"
	if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
		t.Fatal(err)
	}
}

// heatWorkspace has Applications 1, 5 and 10 days past their follow-up date (14 days
// after the last change) and one not yet due.
func heatWorkspace(t *testing.T) string {
	t.Helper()
	return fixtureCopy(t, func(root string) {
		appliedAt(t, root, "heat-low", "Lowco", 15)
		appliedAt(t, root, "heat-mid", "Midco", 19)
		appliedAt(t, root, "heat-high", "Highco", 24)
		appliedAt(t, root, "heat-none", "Noneco", 3)
	})
}

// rowOf is the list half of the title line of the row named name.
func rowOf(view, name string) string {
	for _, l := range strings.Split(view, "\n") {
		left, _, _ := strings.Cut(l, "│")
		if strings.Contains(left, " "+name+" ") {
			return left
		}
	}
	return ""
}

var heatCell = regexp.MustCompile(`(?:^|\s)(\S) \d+(?:d|w|mo|y)(?:\s|$)`)

// heatOf is the glyph in a row's Follow-up due cell, or "" when it has none.
func heatOf(row string) string {
	if m := heatCell.FindStringSubmatch(row); m != nil {
		return m[1]
	}
	return ""
}

// Follow-up due is always the ⚑ flag (@ in ASCII), never a dot that could read as a
// Status or a separator; how long overdue shows in its weight and its "10d" count.
func TestFollowUpFlagGlyph(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ascii bool
		glyph string
	}{
		{"unicode", false, "⚑"},
		{"ascii", true, "@"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ascii {
				t.Setenv("ROLECALL_ASCII", "1")
			}
			// G: the preview shows the last row, so no heat row's title sits beside the list.
			v := plain(driveModel(t, driveStart(t, heatWorkspace(t), fixtureClock, 120, 40), "3", "G"))
			for _, name := range []string{"heat-low", "heat-mid", "heat-high"} {
				if got := heatOf(rowOf(v, name)); got != tc.glyph {
					t.Errorf("%s flag = %q, want %q:\n%s", name, got, tc.glyph, v)
				}
			}
			if got := heatOf(rowOf(v, "heat-none")); got != "" {
				t.Errorf("not-due row has a flag %q", got)
			}
		})
	}
}

// A flag more than a week overdue is bold; a fresh one is not.
func TestLongOverdueFlagIsBold(t *testing.T) {
	raw := driveModel(t, driveStart(t, heatWorkspace(t), fixtureClock, 120, 40), "3", "G").View().Content
	bold := func(name string) bool {
		for _, l := range strings.Split(raw, "\n") {
			if strings.Contains(chipped(l), " "+name+" ") {
				i := strings.Index(l, "⚑")
				return i >= 0 && strings.Contains(l[max(0, strings.LastIndex(l[:i], "\x1b[")):i], "[1;")
			}
		}
		return false
	}
	if !bold("heat-high") || bold("heat-low") {
		t.Errorf("heat-high bold %v, heat-low bold %v; want true, false", bold("heat-high"), bold("heat-low"))
	}
}

func TestDetailShowsFollowUpLine(t *testing.T) {
	m := driveStart(t, heatWorkspace(t), fixtureClock, 120, 40, ui.WithHarness("x {skill} {dir}"))
	v := plain(driveModel(t, m, "3", "/Highco", "enter", "enter"))
	if !strings.Contains(v, "FOLLOW-UP    due · no change in 3w") || !strings.Contains(v, "F draft follow-up") {
		t.Fatalf("no follow-up line:\n%s", v)
	}
}

func TestNudgeStartsHandoffAndNamesDraft(t *testing.T) {
	root := heatWorkspace(t)
	f := &fakeHarness{}
	m := driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(f.runner), ui.WithHarness(`claude "/{skill} {dir}"`))
	m = driveModel(t, m, "3", "/Highco", "enter", "F")
	folder := filepath.Join(root, "jobs", "SWE", "heat-high")
	want := []string{"/bin/sh", "-c", `claude "/${2} ${1}"`, "rolecall-harness", folder, "draft-follow-up"}
	if len(f.calls) != 1 || !slices.Equal(f.calls[0], want) {
		t.Fatalf("calls = %v, want %q", f.calls, want)
	}
	if v := plain(m); !strings.Contains(v, "follow-up.md") {
		t.Fatalf("toast does not name the draft:\n%s", v)
	}
}

func TestNudgeDisabledWhenNotDue(t *testing.T) {
	root := heatWorkspace(t)
	f := &fakeHarness{}
	m := driveStart(t, root, fixtureClock, 200, 40, ui.WithExternalRunner(f.runner), ui.WithHarness("x {skill} {dir}"))
	m = driveModel(t, m, "3", "/Noneco", "enter", "F")
	if len(f.calls) != 0 {
		t.Fatalf("F ran on a not-due Application: %v", f.calls)
	}
	if v := plain(m); strings.Contains(v, "draft follow-up") {
		t.Fatalf("draft follow-up offered on a not-due Application:\n%s", v)
	}
}

func TestHelpListsNudgeUnderAct(t *testing.T) {
	m := driveStart(t, heatWorkspace(t), fixtureClock, 200, 50, ui.WithHarness("x {skill} {dir}"))
	v := plain(driveModel(t, m, "3", "/Highco", "enter", "?"))
	if !helpLists(v, "F draft follow-up") {
		t.Fatalf("help lacks F draft follow-up:\n%s", v)
	}
}

// followedUpAt writes an Applied Application last changed changedAgo days before the
// fixture clock and followed up followedAgo days before it.
func followedUpAt(t *testing.T, root, name string, changedAgo, followedAgo int) {
	t.Helper()
	appliedAt(t, root, name, "Followco", changedAgo)
	path := filepath.Join(root, "jobs", "SWE", name, "meta.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	day := fixtureNow.AddDate(0, 0, -followedAgo).Format("2006-01-02")
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), "status_log:", "followed_up: "+day+"\nstatus_log:", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}

// l records today's follow-up without a Status change: the ⚑ clears, the card says when
// the next one is due, and u takes it back.
func TestFollowedUpClearsTheFlagUntilDueAgain(t *testing.T) {
	root := heatWorkspace(t)
	meta := filepath.Join(root, "jobs", "SWE", "heat-low", "meta.yml")
	before := readFile(t, meta)
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/heat-low")
	d.Type("enter")
	if !strings.Contains(rowOf(d.plain(), "heat-low"), "⚑") {
		t.Fatalf("heat-low is not Follow-up due:\n%s", d.plain())
	}
	d.Type("l")
	if got := readFile(t, meta); !strings.Contains(got, "followed_up: \""+fixtureNow.Format("2006-01-02")+"\"\n") || !strings.Contains(got, "{status: Applied") || strings.Count(got, "status:") != 2 {
		t.Fatalf("l did not record today's follow-up alone:\n%s", got)
	}
	v := d.plain()
	if strings.Contains(rowOf(v, "heat-low"), "⚑") {
		t.Fatalf("the flag stayed after z:\n%s", v)
	}
	requireAll(t, v, "Followed up · “heat-low” (Lowco)", "FOLLOW-UP    sent today · next due in 2w")
	d.Type("u")
	if got := readFile(t, meta); got != before {
		t.Fatalf("u did not take the follow-up back:\n%s", got)
	}
	if !strings.Contains(rowOf(d.plain(), "heat-low"), "⚑") {
		t.Fatalf("the flag did not come back after u:\n%s", d.plain())
	}
}

// A recorded follow-up re-arms the flag follow_up_days later; until then the card counts
// down to it. A follow-up older than the last Status change counts for nothing.
func TestFollowedUpReArmsAfterTheFollowUpPeriod(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		followedUpAt(t, root, "recent", 30, 3)
		followedUpAt(t, root, "lapsed", 40, 15)
		followedUpAt(t, root, "stale", 20, 25)
	})
	for _, tc := range []struct {
		name, want string
		flagged    bool
	}{
		{"recent", "FOLLOW-UP    sent 3d ago · next due in 11d", false},
		{"lapsed", "FOLLOW-UP    due · followed up 2w ago", true},
		{"stale", "FOLLOW-UP    due · no change in 2w", true},
	} {
		d := newDriver(t, root, fixtureClock, 120, 40)
		d.Type("3")
		d.Type("/" + tc.name)
		d.Type("enter")
		v := d.plain()
		if got := strings.Contains(rowOf(v, tc.name), "⚑"); got != tc.flagged {
			t.Errorf("%s: flagged %v, want %v:\n%s", tc.name, got, tc.flagged, v)
		}
		if !strings.Contains(v, tc.want) {
			t.Errorf("%s: card lacks %q:\n%s", tc.name, tc.want, v)
		}
	}
}

// The row's ⚑ age and the card's FOLLOW-UP line count from the same day as the flag
// itself: a lapsed follow-up's own day, not the old Status change behind it.
func TestFollowUpFlagAgeCountsFromTheFollowUp(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		followedUpAt(t, root, "lapsed", 56, 20)
		appliedAt(t, root, "quiet", "Quietco", 56)
	})
	for _, tc := range []struct{ name, flag, card string }{
		{"lapsed", "⚑ 2w", "FOLLOW-UP    due · followed up 2w ago"},
		{"quiet", "⚑ 8w", "FOLLOW-UP    due · no change in 8w"},
	} {
		d := newDriver(t, root, fixtureClock, 120, 40)
		d.Type("3")
		d.Type("/" + tc.name)
		d.Type("enter")
		v := d.plain()
		if row := rowOf(v, tc.name); !strings.Contains(row, tc.flag+" ") && !strings.HasSuffix(strings.TrimRight(row, " "), tc.flag) {
			t.Errorf("%s: row %q lacks %q:\n%s", tc.name, row, tc.flag, v)
		}
		if !strings.Contains(v, tc.card) {
			t.Errorf("%s: card lacks %q:\n%s", tc.name, tc.card, v)
		}
	}
}
