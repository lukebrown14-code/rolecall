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

// The right-hand columns line up: within one view a flagged row and an unflagged one
// start every column at the same cell, the Fit word starts its column's first cell
// whatever its length, and the title keeps what is left. A row has no age column.
func TestRowColumnsLineUp(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	flagged := store.Application{Title: "Flagged", Company: "Acme", Track: "SWE", Status: store.StatusInterviewing,
		Fit: "Marginal", AppliedDate: &store.StatusEntry{At: "2026-01-01"}, LastChange: &store.StatusEntry{Status: store.StatusInterviewing, At: "2026-01-01"}}
	good := store.Application{Title: "A very long interview stage title", Company: "Acme", Track: "SWE", Status: store.StatusApplied,
		Fit: "Good", AppliedDate: &store.StatusEntry{At: "2026-09-27"}, LastChange: &store.StatusEntry{Status: store.StatusApplied, At: "2026-09-27"}}
	plain := good
	plain.Fit = ""
	const w = 60
	show := m.slotsFor([]store.Application{flagged, good, plain})
	f := xansi.Strip(m.row(p, flagged, false, show, w)[0])
	g := xansi.Strip(m.row(p, good, false, show, w)[0])
	l := xansi.Strip(m.row(p, plain, false, show, w)[0])
	for _, r := range []string{f, g, l} {
		if xansi.StringWidth(r) != w {
			t.Fatalf("row is not %d cells: %q", w, r)
		}
		if strings.Contains(r, "fit") || strings.Contains(r, " 4d") || strings.Contains(r, glyphNone) {
			t.Fatalf("a row shows the word fit, an age or a dash: %q", r)
		}
	}
	if !strings.HasSuffix(strings.TrimRight(f, " "), "⚑ 9mo  Marginal") || !strings.HasSuffix(strings.TrimRight(g, " "), "Good") {
		t.Fatalf("columns wrong:\n%q\n%q", f, g)
	}
	start := func(row, s string) int { return xansi.StringWidth(row[:strings.Index(row, s)]) }
	if start(f, "Marginal") != start(g, "Good") || !strings.HasSuffix(strings.TrimRight(l, " "), "title") {
		t.Fatalf("Fit words do not start at one cell, or a blank Fit drew something:\n%q\n%q\n%q", f, g, l)
	}
}

// Each column is as wide as its widest value in the view, and a column empty on every
// row is hidden, so the title takes the room: no Lead has a Follow-up flag and none here
// is assessed, so a Leads view reserves no cells for either.
func TestRowColumnsFitTheView(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	lead := store.Application{Title: "Software Engineer at a long-named place", Company: "Acme", Track: "SWE", Status: store.StatusLead,
		LastChange: &store.StatusEntry{Status: store.StatusLead, At: "2026-09-25"}}
	show := m.slotsFor([]store.Application{lead})
	if show.flagW != 0 || show.fitW != 0 {
		t.Fatalf("a view without a flag or a Fit reserves cells for them: %+v", show)
	}
	got := xansi.Strip(m.row(p, lead, false, show, 40)[0])
	if !strings.HasPrefix(got, "  ◌ Software Engineer at a long-named…") {
		t.Fatalf("the title does not take the unused room: %q", got)
	}
	lead.Fit = store.FitStrong
	show = m.slotsFor([]store.Application{lead})
	if show.fitW != columnGap-1+fitSlotWidth {
		t.Fatalf("the Fit column is %d cells, want the width of Marginal and its gap", show.fitW)
	}
}

// The widest values side by side never touch: a long-quiet flag and the longest Fit keep
// two cells between them, and a years-old change reads in years, not "82mo".
func TestRowColumnsKeepAGapAtTheirWidest(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	widest := store.Application{Title: "Widest", Company: "Acme", Track: "SWE", Status: store.StatusInterviewing, Fit: store.FitMarginal,
		AppliedDate: &store.StatusEntry{At: "2024-10-20"}, LastChange: &store.StatusEntry{Status: store.StatusInterviewing, At: "2024-10-20", Approx: true}}
	one := func(a store.Application) string {
		return xansi.Strip(m.row(p, a, false, m.slotsFor([]store.Application{a}), 60)[0])
	}
	if got := one(widest); !strings.Contains(got, "⚑ 23mo  Marginal") {
		t.Fatalf("widest columns run together or are cut: %q", got)
	}
	old := widest
	old.LastChange = &store.StatusEntry{Status: store.StatusApplied, At: "2019-01-01"}
	if got := one(old); !strings.Contains(got, "⚑ 7y") {
		t.Fatalf("a years-old change should read in years: %q", got)
	}
}
