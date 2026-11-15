package ui

import (
	"strings"
	"testing"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// follow-up is measured in whole days from the last change, so the flag never depends on
// the time of day the check runs, and only sent Applications are followed up.
func TestFollowUpDueIsDayBased(t *testing.T) {
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	sent := store.Application{Status: store.StatusInterviewing, LastChange: &store.StatusEntry{At: "2026-10-06T23:00:00Z"}}
	if !followUpDue(sent, now, 14) {
		t.Fatal("a change exactly follow-up-days ago is due")
	}
	sent.LastChange = &store.StatusEntry{At: "2026-10-07T00:00:00Z"}
	if followUpDue(sent, now, 14) {
		t.Fatal("a change inside the window is not due")
	}
	sent.Status, sent.LastChange = store.StatusLead, &store.StatusEntry{At: "2026-01-01"}
	if followUpDue(sent, now, 14) {
		t.Fatal("an unsent Application is never due")
	}
	sent.Status, sent.LastChange = store.StatusInterviewing, &store.StatusEntry{At: "2027-01-01"}
	if followUpDue(sent, now, 14) {
		t.Fatal("a future date is not due")
	}
	sent.LastChange = nil
	if followUpDue(sent, now, 14) {
		t.Fatal("no last change is not due")
	}
}

// The right-hand columns are fixed: a flagged row and an unflagged one start every
// column at the same cell, and the title keeps what is left.
func TestRowColumnsAreFixed(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	flagged := store.Application{Title: "Flagged", Company: "Acme", Track: "SWE", Status: store.StatusInterviewing,
		Fit: "Marginal", AppliedDate: &store.StatusEntry{At: "2026-01-01"}, LastChange: &store.StatusEntry{Status: store.StatusInterviewing, At: "2026-01-01"}}
	plain := store.Application{Title: "A very long interview stage title", Company: "Acme", Track: "SWE", Status: store.StatusApplied,
		AppliedDate: &store.StatusEntry{At: "2026-09-27"}, LastChange: &store.StatusEntry{Status: store.StatusApplied, At: "2026-09-27"}}
	const w = 60
	f := xansi.Strip(m.row(p, flagged, false, fixedSlots, w)[0])
	l := xansi.Strip(m.row(p, plain, false, fixedSlots, w)[0])
	if xansi.StringWidth(f) != w || xansi.StringWidth(l) != w {
		t.Fatalf("rows are not %d cells: %q %q", w, f, l)
	}
	if !strings.HasSuffix(strings.TrimRight(f, " "), "fit Marg.") || !strings.HasSuffix(strings.TrimRight(l, " "), "fit —") || !strings.Contains(l, "Applied 4d") || !strings.Contains(f, "⚑ 9mo") {
		t.Fatalf("columns wrong:\n%q\n%q", f, l)
	}
}
