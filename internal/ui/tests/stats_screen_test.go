package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestStatsScreenTabAndRowKeysAreInert(t *testing.T) {
	plain := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4"))
	withKeys := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4", "tab", "m", "v", "e", "n", "x", "d", "f", "p", "b", "j", "k", "g", "G", "enter"))
	if plain != withKeys {
		t.Fatalf("Tab or a row key changed Stats:\n--- plain ---\n%s\n--- with keys ---\n%s", plain, withKeys)
	}
}

func TestStatsTrackFilterHidesThePerTrackTable(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 140, "4", "t"))
	if !strings.Contains(view, "Platform") {
		t.Fatalf("track filter did not show Platform:\n%s", view)
	}
	if strings.Contains(view, "BY TRACK") {
		t.Fatalf("per-track table still shown when filtered:\n%s", view)
	}
}

func TestStatsHelpBarOnlyListsValidKeys(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4", "?"))
	for _, want := range []string{"add", "rescan", "quit", "next track", "1-6", "screen"} {
		if !strings.Contains(view, want) {
			t.Fatalf("help lacks %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"tab", "cursor", "move", "search", "enter", "open", "edit", "notes", "dismiss", "download"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("help shows %q on Stats:\n%s", unwanted, view)
		}
	}
}

func TestStatsAddWorks(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4", "a"))
	if !strings.Contains(view, "Add a Lead") {
		t.Fatalf("a did not open the Add form on Stats:\n%s", view)
	}
}

func TestStatsScreenSwitchWorks(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4", "1"))
	if !strings.Contains(view, "Leads") {
		t.Fatalf("1 did not leave Stats:\n%s", view)
	}
}

func TestStatsLiveRefresh(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.tm.Send(tea.KeyPressMsg{Code: '4'})
	h.wait("Stats")
	h.write(h.folder("fresh")+"/meta.yml", "schema: 1\ntitle: Fresh\ncompany: Example\ntrack: Fresh\nstatus_log:\n  - {status: Rejected, at: 2026-09-30T01:00:00Z}\n")
	h.wait("Fresh")
	if v := h.finalView(); !strings.Contains(v, "Rejected 3") || !strings.Contains(v, "Fresh") {
		t.Fatalf("Stats did not re-render after live refresh:\n%s", v)
	}
}

// A report taller than the body scrolls with j k, ↑↓ and the wheel, with the overflow
// cue saying what is hidden, and the key line offers it. The fixtures fit 80x24, so
// this runs smaller.
func TestStatsScrollsWhenTallerThanTheBody(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 60, 16)
	m = driveModel(t, m, "4")
	top := ansi.Strip(m.View().Content)
	up, down := regexp.MustCompile(`↑ \d+ more`), regexp.MustCompile(`↓ \d+ more`)
	if !down.MatchString(top) || up.MatchString(top) || !strings.Contains(top, "scroll") {
		t.Fatalf("tall Stats lacks the overflow cue or the scroll key:\n%s", top)
	}
	m = driveModel(t, m, "G")
	if view := ansi.Strip(m.View().Content); !up.MatchString(view) || down.MatchString(view) {
		t.Fatalf("G did not scroll to the bottom:\n%s", view)
	}
	m = driveModel(t, m, "g")
	m = runMsg(t, m, tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	if view := ansi.Strip(m.View().Content); !up.MatchString(view) {
		t.Fatalf("the wheel did not scroll Stats:\n%s", view)
	}
	m = driveModel(t, m, "k")
	if view := ansi.Strip(m.View().Content); view != top {
		t.Fatalf("k did not scroll back to the top:\n%s", view)
	}
}

// The funnel never calls a rise or a flat count a drop: each rung says what share of the
// one before reached it and how many skipped it. Everything fits 80x24 with no cue.
func TestStatsFunnelExplainsEachRung(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 80, 24, "4"))
	for _, want := range []string{
		"Applications that ever reached each Status",
		"2 without Lead logged",
		"50% of Draft · 6 skipped Draft",
		"14% of Applied · 1 skipped Applied",
		"closed  ✕ Rejected 2 · – Withdrawn 2 · ⊘ Dismissed 2",
		"2 of 6",
		"Not counted: 1 Application applied less than 14 days ago (too early to judge)",
		"13 Jul → 28 Sep",
		"0 this week · 12-week avg 0.1",
		"Unknown               6       6          2   33%",
	} {
		if !strings.Contains(strings.Join(strings.Fields(view), " "), strings.Join(strings.Fields(want), " ")) {
			t.Errorf("Stats lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "drop") || strings.Contains(view, "more") {
		t.Fatalf("Stats shows a drop-off or does not fit 80x24:\n%s", view)
	}
}

func TestStatsEmptyWorkspace(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		jobs := filepath.Join(root, "jobs")
		if err := os.RemoveAll(jobs); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(jobs, 0o755); err != nil {
			t.Fatal(err)
		}
	})
	view := ansi.Strip(runKeys(t, root, 120, "4"))
	if !strings.Contains(view, "No Applications · a add a Lead") || strings.Contains(view, "FUNNEL") {
		t.Fatalf("empty Stats lacks its empty state:\n%s", view)
	}
}
