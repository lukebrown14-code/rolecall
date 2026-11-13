package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// threeEntryLog gives status-interviewing a Status log of three entries.
func threeEntryLog(root string) {
	p := filepath.Join(root, "jobs", "SWE", "status-interviewing", "meta.yml")
	b, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	s := strings.Replace(string(b), "title: \"Engineer #1\"", "title: zedcorp", 1)
	s = strings.Replace(s, "  - {status: Applied, at: 2025-01-01, approx: true}\n",
		"  - {status: Lead, at: 2024-12-01}\n  - {status: Draft, at: 2024-12-10}\n  - {status: Applied, at: 2025-01-01, approx: true}\n", 1)
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		panic(err)
	}
}

func detailPane(t *testing.T, screen, name string, w, h int, edit func(string)) string {
	t.Helper()
	m := driveStart(t, fixtureCopy(t, edit), fixtureClock, w, h, ui.WithHarness("h {skill} {dir}"), ui.WithPlugin("/plugin"))
	return ansi.Strip(driveModel(t, m, screen, "/"+name).View().Content)
}

// The pane draws labelled rows, each ending in its key and action.
func TestDetailPaneRowsEndInKeys(t *testing.T) {
	view := detailPane(t, "3", "docs-written", 120, 36, nil)
	for _, want := range []string{"STATUS", "[m] move", "FIT", "[f] assess", "TRACK", "[e] edit", "NOTES", "[n] notes"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail lacks %q:\n%s", want, view)
		}
	}
}

// The Status log is a vertical timeline, newest first, joined by connectors.
func TestDetailTimelineIsVerticalNewestFirst(t *testing.T) {
	view := detailPane(t, "3", "zedcorp", 120, 36, threeEntryLog)
	var order []string
	connectors := 0
	for _, l := range strings.Split(view, "\n") {
		i := strings.Index(l, "│")
		if i < 0 {
			continue
		}
		right := strings.TrimSpace(l[i+len("│"):])
		if right == "│" {
			connectors++
			continue
		}
		for _, s := range []string{"Interviewing", "Applied", "Lead", "Draft"} {
			if strings.HasPrefix(strings.TrimLeft(right, "◆●◌✎ "), s) {
				order = append(order, s)
			}
		}
	}
	// the STATUS row names the current Status first; the timeline follows, newest first
	if got := strings.Join(order, ","); !strings.HasSuffix(got, "Interviewing,Applied,Draft,Lead") {
		t.Errorf("timeline order = %v:\n%s", order, view)
	}
	if connectors < 3 {
		t.Errorf("%d connectors, want 3:\n%s", connectors, view)
	}
}

func TestDetailNotAssessedFit(t *testing.T) {
	view := detailPane(t, "2", "docs-none", 120, 36, nil)
	if !strings.Contains(view, "[f] assess fit") || !strings.Contains(view, "FIT     ·") {
		t.Errorf("no not-assessed Fit row:\n%s", view)
	}
}

func TestDetailMissingDocumentsShowFixingKeys(t *testing.T) {
	view := detailPane(t, "2", "docs-none", 120, 36, nil)
	if !strings.Contains(view, "[b] build") {
		t.Errorf("missing PDF lacks [b] build:\n%s", view)
	}
	view = detailPane(t, "3", "docs-written", 120, 36, nil)
	if strings.Contains(view, "[p] prepare docs") {
		t.Errorf("an Interviewing Application is offered prepare docs:\n%s", view)
	}
	view = detailPane(t, "2", "docs-none", 120, 36, nil)
	if !strings.Contains(view, "[p] prepare docs") {
		t.Errorf("Lead lacks [p] prepare docs:\n%s", view)
	}
}

// Rows never wrap in the sheet and full views: the Overview tab holds the same rows.
func TestDetailOverviewFitsSmallSizes(t *testing.T) {
	for _, s := range [][2]int{{80, 24}, {60, 16}} {
		view := ansi.Strip(driveModel(t, driveStart(t, fixtureCopy(t, threeEntryLog), fixtureClock, s[0], s[1], ui.WithHarness("h")), "3", "/zedcorp", "enter", "enter").View().Content)
		if !strings.Contains(view, "STATUS") || !strings.Contains(view, "[m]") {
			t.Errorf("%dx%d: overview lacks rows:\n%s", s[0], s[1], view)
		}
		for _, l := range strings.Split(view, "\n") {
			if ansi.StringWidth(l) > s[0] {
				t.Errorf("%dx%d: line too wide: %q", s[0], s[1], l)
			}
		}
	}
}
