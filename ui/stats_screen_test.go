package ui_test

import (
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
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4", "t"))
	if !strings.Contains(view, "Platform") {
		t.Fatalf("track filter did not show Platform:\n%s", view)
	}
	if strings.Contains(view, "By track") {
		t.Fatalf("per-track table still shown when filtered:\n%s", view)
	}
}

func TestStatsHelpBarOnlyListsValidKeys(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4", "?"))
	for _, want := range []string{"add", "rescan", "quit", "cycle track", "1-4", "screen"} {
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
