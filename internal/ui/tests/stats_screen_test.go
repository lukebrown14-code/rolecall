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
	if !down.MatchString(top) || up.MatchString(top) || !strings.Contains(helpText(driveModel(t, m, "?").View().Content), "scroll") {
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

// A tall Stats offers j/k on its key line, and pgdn and pgup scroll it a page at a time.
func TestStatsPagesWithPgdnAndOffersJK(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 20)
	m = driveModel(t, m, "4")
	top := chipped(m.View().Content)
	if keys := lastLineOf(top); !strings.Contains(keys, "j/k scroll") {
		t.Errorf("the Stats key line lacks j/k scroll: %q", keys)
	}
	m = runMsg(t, m, tea.KeyPressMsg{Code: tea.KeyPgDown})
	paged := chipped(m.View().Content)
	if paged == top || !regexp.MustCompile(`↑ \d+ more`).MatchString(paged) {
		t.Fatalf("pgdn did not scroll Stats:\n%s", paged)
	}
	// A page moves further than a line.
	if line := chipped(driveModel(t, driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 20), "4"), "j").View().Content); line == paged {
		t.Errorf("pgdn moved only a line:\n%s", paged)
	}
	m = runMsg(t, m, tea.KeyPressMsg{Code: tea.KeyPgUp})
	if view := chipped(m.View().Content); view != top {
		t.Errorf("pgup did not scroll back to the top:\n%s", view)
	}
}

// The Track filter offers the off-list Tracks rows are on, after the workspace.json ones,
// so its options add up to All, and t reaches them. Stats leaves the malformed row out of
// its counts, as it does of every figure.
func TestStatsTrackFilterOffersOffListTracks(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 140, "4"))
	// 20 well-formed: SWE 13 (docs-*, empty, empty-log, filtered-expired, lead-*, orbit), Platform 1 (fern), and the
	// six legacy status-* rows on Unknown, which workspace.json does not list.
	if !strings.Contains(view, "STATS │ TRACK t [All 20] SWE 13  Platform 1  Unknown 6") {
		t.Fatalf("Stats track filter does not add up:\n%s", view)
	}
	view = chipped(runKeys(t, fixtureCopy(t, nil), 140, "4", "t", "t", "t"))
	if !strings.Contains(view, "[Unknown 6]") || !strings.Contains(view, "Closed 2: ✕ Rejected 1 · ↩ Withdrawn 1") || !strings.Contains(view, "⊘ Dismissed 1, let go before applying") {
		t.Fatalf("t did not reach the Unknown Track, or Stats is not filtered to it:\n%s", view)
	}
	// On Applied, Platform has no rows, so its chip is left out.
	view = chipped(runKeys(t, fixtureCopy(t, nil), 140, "3"))
	if !strings.Contains(view, "TRACK t [All 9] SWE 5  Unknown 4") {
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
	// j: docs-none, the second Draft (Engineer #1 has a Fit and leads).
	after := body(chipped(runKeys(t, fixtureCopy(t, nil), 120, "2", "j", "m", "a", "y", "u", "4")))
	if before != after {
		t.Fatalf("an undone move changed Stats:\n--- before ---\n%s\n--- after ---\n%s", before, after)
	}
	moved := body(chipped(runKeys(t, fixtureCopy(t, nil), 120, "2", "j", "m", "a", "y", "4")))
	if moved == before {
		t.Fatalf("the move itself did not reach Stats, so the undo test proves nothing:\n%s", moved)
	}
}

// The funnel never calls a rise or a flat count a drop: a rung that all came through the
// one before says what share of it reached it; one that did not says how they arrived,
// as counts, never "0% of" or "none", and a caption says a bar can outgrow the one
// above. A later Status implies Applied, so the numbers match the screens. At 80 wide
// everything down to the By track table reads; Insights follows.
func TestStatsFunnelExplainsEachRung(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 80, 40, "4"))
	for _, want := range []string{
		"Ever reached each Status; tabs count where they are now",
		"so a bar can outgrow the one above",
		"◌ Lead ███████████████████████░░░░░░░░░░░░░ 7",
		"2 2 started at Draft",
		"11 11 never a Draft",
		"4 36% of Applied",
		"2 50% of Interviewing",
		"Closed 4: ✕ Rejected 2 · ↩ Withdrawn 2",
		"⊘ Dismissed 3, let go before applying",
		"33% · 2 of 6",
		"Not counted: 5 applied in the last 14 days (too new)",
		"13 Jul 28 Sep",
		"0 this week · max 1/wk",
		"Applied  Counted  Responses  Rate",
		"Unknown               6        6          2   33%",
		"SWE                   5        0          0",
		"No Applications sent: Platform",
	} {
		if !strings.Contains(strings.Join(strings.Fields(view), " "), strings.Join(strings.Fields(want), " ")) {
			t.Errorf("Stats lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "none") {
		t.Errorf("a funnel note says none:\n%s", view)
	}
	for _, jargon := range []string{"logged", "skipped", "Judged"} {
		if strings.Contains(view, jargon) {
			t.Errorf("Stats uses the data-model word %q:\n%s", jargon, view)
		}
	}
	// Insights, below the By track table, may scroll into view at 80x24.
	if strings.Contains(view, "drop") {
		t.Fatalf("Stats shows a drop-off:\n%s", view)
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
