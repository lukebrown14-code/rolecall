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
	return chipped(driveModel(t, m, screen, "/"+name).View().Content)
}

// The pane leads with the Status and the Fit on one line, then labelled sections that each
// end in at most one bare key: "DOCS … p prepare docs", "NEXT … m move", "NOTES … n add".
func TestDetailPaneSectionsEndInOneKey(t *testing.T) {
	view := detailPane(t, "3", "docs-written", 120, 36, nil)
	for _, want := range []string{"◆ Interviewing", "fit —", "DOCS", "NEXT", "TIMELINE", "NOTES", "n notes"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail lacks %q:\n%s", want, view)
		}
	}
	for _, gone := range []string{"STATUS  ", "FIT  ", "TRACK  ", "[m]", "[n]"} {
		if strings.Contains(view, gone) {
			t.Errorf("detail keeps %q:\n%s", gone, view)
		}
	}
	// Sections run in the design's order.
	at := -1
	for _, s := range []string{"DOCS", "NEXT", "TIMELINE", "NOTES"} {
		i := strings.Index(view, s)
		if i < at {
			t.Errorf("%s comes before the section above it:\n%s", s, view)
		}
		at = i
	}
}

// The Status log is a vertical timeline, oldest first, with the next Status faint below.
func TestDetailTimelineRunsOldestFirst(t *testing.T) {
	view := detailPane(t, "3", "zedcorp", 120, 36, threeEntryLog)
	var order []string
	timeline := false
	for _, l := range strings.Split(view, "\n") {
		i := strings.Index(l, "│")
		if i < 0 {
			continue
		}
		right := strings.TrimSpace(l[i+len("│"):])
		switch {
		case right == "TIMELINE":
			timeline = true
			continue
		case right == "" || strings.HasPrefix(right, "NOTES"):
			timeline = false
		}
		if !timeline {
			continue
		}
		for _, s := range []string{"Interviewing", "Applied", "Lead", "Draft", "Offer"} {
			if strings.HasPrefix(strings.TrimLeft(right, "◆●◌✎★ "), s) {
				order = append(order, s)
			}
		}
	}
	if got := strings.Join(order, ","); got != "Lead,Draft,Applied,Interviewing,Offer" {
		t.Errorf("timeline order = %v:\n%s", order, view)
	}
	if !strings.Contains(lineWith(view, "★ Offer"), "next") {
		t.Errorf("the next Status is not marked next:\n%s", view)
	}
}

// An assessed Fit shows in its colour at the Status line's right edge.
func TestDetailAssessedFit(t *testing.T) {
	view := detailPane(t, "3", "zedcorp", 120, 36, threeEntryLog)
	if !hasLineWith(view, "fit Good", "◆ Interviewing") {
		t.Errorf("the Status line lacks the Fit:\n%s", view)
	}
}

func TestDetailNotAssessedFit(t *testing.T) {
	view := detailPane(t, "2", "docs-none", 120, 36, nil)
	if !hasLineWith(view, "fit —", "✎ Draft") {
		t.Errorf("the Status line does not say the Fit is not assessed:\n%s", view)
	}
}

// DOCS offers the one key that fixes the next missing Document: prepare docs while a
// source Document is missing, build once only the PDF is.
func TestDetailMissingDocumentsShowFixingKeys(t *testing.T) {
	view := detailPane(t, "2", "docs-none", 120, 36, nil)
	if docs := lineWith(view, "DOCS"); !strings.Contains(docs, "p prepare docs") || strings.Contains(view, "b build") {
		t.Errorf("Draft missing its letter: DOCS %q:\n%s", docs, view)
	}
	written := func(root string) {
		dir := filepath.Join(root, "jobs", "SWE", "docs-none")
		for _, f := range []string{"jd.md", "resume.md", "cover-letter.md"} {
			if err := os.WriteFile(filepath.Join(dir, f), []byte("# Text\n\nWritten.\n"), 0o600); err != nil {
				panic(err)
			}
		}
	}
	view = detailPane(t, "2", "docs-none", 120, 36, written)
	if docs := lineWith(view, "DOCS"); !strings.Contains(docs, "b build") {
		t.Errorf("Draft missing only the PDF: DOCS %q:\n%s", docs, view)
	}
	view = detailPane(t, "3", "docs-written", 120, 36, nil)
	if strings.Contains(view, "p prepare docs") {
		t.Errorf("an Interviewing Application is offered prepare docs:\n%s", view)
	}
	if docs := lineWith(view, "○ PDF"); !strings.Contains(docs, "JD   ✓ Resume   ✓ Cover letter") {
		t.Errorf("the Documents are not side by side on one line: %q:\n%s", docs, view)
	}
}

// Sections never overflow in the sheet and full views: the Overview tab holds the same
// sections.
func TestDetailOverviewFitsSmallSizes(t *testing.T) {
	for _, s := range [][2]int{{80, 24}, {60, 16}} {
		view := chipped(driveModel(t, driveStart(t, fixtureCopy(t, threeEntryLog), fixtureClock, s[0], s[1], ui.WithHarness("h")), "3", "/zedcorp", "enter", "enter").View().Content)
		if !strings.Contains(view, "DOCS") || !strings.Contains(view, "◆ Interviewing  ") {
			t.Errorf("%dx%d: overview lacks its sections:\n%s", s[0], s[1], view)
		}
		for _, l := range strings.Split(view, "\n") {
			if ansi.StringWidth(l) > s[0] {
				t.Errorf("%dx%d: line too wide: %q", s[0], s[1], l)
			}
		}
	}
}
