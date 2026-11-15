package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestStatsScreenTabAndRowKeysAreInert(t *testing.T) {
	plain := chipped(runKeys(t, fixtureCopy(t, nil), 120, "4"))
	withKeys := chipped(runKeys(t, fixtureCopy(t, nil), 120, "4", "tab", "m", "v", "e", "n", "x", "d", "f", "p", "b", "j", "k", "g", "G", "enter"))
	if plain != withKeys {
		t.Fatalf("Tab or a row key changed Stats:\n--- plain ---\n%s\n--- with keys ---\n%s", plain, withKeys)
	}
}

func TestStatsTrackFilterHidesThePerTrackTable(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 140, "4", "t"))
	if !strings.Contains(view, "Platform") {
		t.Fatalf("track filter did not show Platform:\n%s", view)
	}
	if strings.Contains(view, "BY TRACK") {
		t.Fatalf("per-track table still shown when filtered:\n%s", view)
	}
}

func TestStatsHelpBarOnlyListsValidKeys(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "4", "?"))
	for _, want := range []string{"add", "rescan", "quit", "track", "1-6", "screen"} {
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
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "4", "a"))
	if !strings.Contains(view, "Add a Lead") {
		t.Fatalf("a did not open the Add form on Stats:\n%s", view)
	}
}

func TestStatsScreenSwitchWorks(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "4", "1"))
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
	top := chipped(m.View().Content)
	up, down := regexp.MustCompile(`↑ \d+ more`), regexp.MustCompile(`↓ \d+ more`)
	if !down.MatchString(top) || up.MatchString(top) || !strings.Contains(top, "scroll") {
		t.Fatalf("tall Stats lacks the overflow cue or the scroll key:\n%s", top)
	}
	m = driveModel(t, m, "G")
	if view := chipped(m.View().Content); !up.MatchString(view) || down.MatchString(view) {
		t.Fatalf("G did not scroll to the bottom:\n%s", view)
	}
	m = driveModel(t, m, "g")
	m = runMsg(t, m, tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	if view := chipped(m.View().Content); !up.MatchString(view) {
		t.Fatalf("the wheel did not scroll Stats:\n%s", view)
	}
	m = driveModel(t, m, "k")
	if view := chipped(m.View().Content); view != top {
		t.Fatalf("k did not scroll back to the top:\n%s", view)
	}
}

// The Track filter offers the off-list Tracks rows are on, after the workspace.json ones,
// so its options add up to All, and t reaches them. Stats leaves the malformed row out of
// its counts, as it does of every figure.
func TestStatsTrackFilterOffersOffListTracks(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 140, "4"))
	// 16 well-formed: SWE 9 (docs-*, empty, empty-log, orbit), Platform 1 (fern), and the
	// six legacy status-* rows on Unknown, which workspace.json does not list.
	if !strings.Contains(view, "TRACK t [All 16] SWE 9 Platform 1 Unknown 6") {
		t.Fatalf("Stats track filter does not add up:\n%s", view)
	}
	view = chipped(runKeys(t, fixtureCopy(t, nil), 140, "4", "t", "t", "t"))
	if !strings.Contains(view, "[Unknown 6]") || !strings.Contains(view, "closed  ✕ Rejected 1 · ↩ Withdrawn 1 · ⊘ Dismissed 1") {
		t.Fatalf("t did not reach the Unknown Track, or Stats is not filtered to it:\n%s", view)
	}
	view = chipped(runKeys(t, fixtureCopy(t, nil), 140, "3"))
	if !strings.Contains(view, "TRACK t [All 9] SWE 5  Platform 0  Unknown 4") {
		t.Fatalf("Applied track filter does not add up:\n%s", view)
	}
	// T from All goes back to the last option, the off-list one.
	view = chipped(runKeys(t, fixtureCopy(t, nil), 140, "3", "T"))
	if !strings.Contains(view, "[Unknown 4]") {
		t.Fatalf("T did not cycle back to Unknown:\n%s", view)
	}
}

// A move that is undone leaves Stats as it was: undo appends the earlier Status, and
// Stats reads the pair as never having happened.
func TestStatsIgnoresAnUndoneMove(t *testing.T) {
	body := func(view string) string {
		lines := strings.Split(view, "\n")
		return strings.Join(lines[4:len(lines)-3], "\n") // below the context row, above the footer
	}
	before := body(chipped(runKeys(t, fixtureCopy(t, nil), 120, "4")))
	after := body(chipped(runKeys(t, fixtureCopy(t, nil), 120, "2", "m", "a", "y", "u", "4")))
	if before != after {
		t.Fatalf("an undone move changed Stats:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	moved := body(chipped(runKeys(t, fixtureCopy(t, nil), 120, "2", "m", "a", "y", "4")))
	if moved == before {
		t.Fatalf("the move itself did not reach Stats, so the undo test proves nothing:\n%s", moved)
	}
}

// The funnel never calls a rise or a flat count a drop: each rung says what share of the
// one before reached it and how many were not on it first, in one wording. A later
// Status implies Applied, so the numbers match the screens. Everything fits 80x24 with
// no cue.
func TestStatsFunnelExplainsEachRung(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 80, 24, "4"))
	for _, want := range []string{
		"Applications that ever reached each Status",
		"◌ Lead ██████ 4",
		"2 0% of Lead · 2 weren't Leads first",
		"11 0% of Draft · 11 weren't Drafts first",
		"4 36% of Applied",
		"2 50% of Interviewing",
		"closed  ✕ Rejected 2 · ↩ Withdrawn 2 · ⊘ Dismissed 2",
		"2 of 6",
		"Not counted: 5 applied in the last 14 days (too new)",
		"13 Jul → 28 Sep",
		"0 this week · 12-week avg 0.1",
		"Applied  Counted  Responses  Rate",
		"SWE                   5        0          0     —",
		"Unknown               6        6          2   33%",
	} {
		if !strings.Contains(strings.Join(strings.Fields(view), " "), strings.Join(strings.Fields(want), " ")) {
			t.Errorf("Stats lacks %q:\n%s", want, view)
		}
	}
	for _, jargon := range []string{"logged", "skipped", "Judged"} {
		if strings.Contains(view, jargon) {
			t.Errorf("Stats uses the data-model word %q:\n%s", jargon, view)
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
	view := chipped(runKeys(t, root, 120, "4"))
	if !strings.Contains(view, "No Applications yet · a add a Lead") || strings.Contains(view, "FUNNEL") {
		t.Fatalf("empty Stats lacks its empty state:\n%s", view)
	}
}
