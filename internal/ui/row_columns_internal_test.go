package ui

import (
	"strings"
	"testing"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// A row only reserves the right-hand columns it uses. A column another row opened (here
// the follow-up flag) must not steal room from a title that has nothing in it, or a long
// title is cut beside blank cells.
func TestRowColumnsReserveOnlyWhatTheRowUses(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	flagged := store.Application{Title: "Flagged", Company: "Acme", Track: "SWE", Status: store.StatusInterviewing,
		Fit: "Good", AppliedDate: &store.StatusEntry{At: "2026-01-01"}, LastChange: &store.StatusEntry{At: "2026-01-01"}}
	long := store.Application{Title: "A very long interview stage title", Company: "Acme", Track: "SWE", Status: store.StatusInterviewing,
		Fit: "Good", AppliedDate: &store.StatusEntry{At: "2026-09-30"}, LastChange: &store.StatusEntry{At: "2026-09-30"}}

	cols := m.columns([]store.Application{flagged, long})
	if cols.flagW == 0 {
		t.Fatal("the flagged row did not open the flag column")
	}
	const w = 60
	if line := xansi.Strip(m.row(p, long, false, cols, w)[0]); strings.Contains(line, "…") || xansi.StringWidth(line) != w {
		t.Fatalf("long title truncated by a column it does not use: %q", line)
	}
	if line := xansi.Strip(m.row(p, flagged, false, cols, w)[0]); !strings.Contains(line, "quiet") {
		t.Fatalf("flagged row lost its flag: %q", line)
	}
}
