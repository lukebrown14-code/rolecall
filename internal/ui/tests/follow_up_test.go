package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
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

var heatCell = regexp.MustCompile(`(?:^|\s)(\S) \d+(?:d|w|mo)\s+Applied`)

// heatOf is the glyph in a row's Follow-up due cell, or "" when it has none.
func heatOf(row string) string {
	if m := heatCell.FindStringSubmatch(row); m != nil {
		return m[1]
	}
	return ""
}

// Follow-up due is always the ⚑ flag (? in ASCII), never a dot that could read as a
// Status or a separator; how long overdue shows in its weight and its "10d" count.
func TestFollowUpFlagGlyph(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ascii bool
		glyph string
	}{
		{"unicode", false, "⚑"},
		{"ascii", true, "?"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ascii {
				t.Setenv("ROLECALL_ASCII", "1")
			}
			v := plain(driveModel(t, driveStart(t, heatWorkspace(t), fixtureClock, 120, 40), "3"))
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
	raw := driveModel(t, driveStart(t, heatWorkspace(t), fixtureClock, 120, 40), "3").View().Content
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
	if !strings.Contains(v, "FOLLOW-UP  due 10 days ago") || !strings.Contains(v, "r draft nudge") {
		t.Fatalf("no follow-up line:\n%s", v)
	}
}

func TestNudgeStartsHandoffAndNamesDraft(t *testing.T) {
	root := heatWorkspace(t)
	f := &fakeHarness{}
	m := driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(f.runner), ui.WithHarness(`claude "/{skill} {dir}"`))
	m = driveModel(t, m, "3", "/Highco", "enter", "r")
	folder := filepath.Join(root, "jobs", "SWE", "heat-high")
	want := `claude "/draft-follow-up '` + folder + `'"`
	if len(f.calls) != 1 || f.calls[0][2] != want {
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
	m = driveModel(t, m, "3", "/Noneco", "enter", "r")
	if len(f.calls) != 0 {
		t.Fatalf("r ran on a not-due Application: %v", f.calls)
	}
	if v := plain(m); strings.Contains(v, "draft nudge") {
		t.Fatalf("draft nudge offered on a not-due Application:\n%s", v)
	}
}

func TestHelpListsNudgeUnderAct(t *testing.T) {
	m := driveStart(t, heatWorkspace(t), fixtureClock, 200, 50, ui.WithHarness("x {skill} {dir}"))
	v := plain(driveModel(t, m, "3", "/Highco", "enter", "?"))
	if !helpLists(v, "r draft nudge") {
		t.Fatalf("help lacks r draft nudge:\n%s", v)
	}
}
