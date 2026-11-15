package ui

import (
	"regexp"
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
// start every column at the same cell, and the title keeps what is left.
func TestRowColumnsLineUp(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	flagged := store.Application{Title: "Flagged", Company: "Acme", Track: "SWE", Status: store.StatusInterviewing,
		Fit: "Marginal", AppliedDate: &store.StatusEntry{At: "2026-01-01"}, LastChange: &store.StatusEntry{Status: store.StatusInterviewing, At: "2026-01-01"}}
	plain := store.Application{Title: "A very long interview stage title", Company: "Acme", Track: "SWE", Status: store.StatusApplied,
		AppliedDate: &store.StatusEntry{At: "2026-09-27"}, LastChange: &store.StatusEntry{Status: store.StatusApplied, At: "2026-09-27"}}
	const w = 60
	show := m.slotsFor([]store.Application{flagged, plain})
	f := xansi.Strip(m.row(p, flagged, false, show, w)[0])
	l := xansi.Strip(m.row(p, plain, false, show, w)[0])
	if xansi.StringWidth(f) != w || xansi.StringWidth(l) != w {
		t.Fatalf("rows are not %d cells: %q %q", w, f, l)
	}
	if !strings.HasSuffix(strings.TrimRight(f, " "), "fit Marginal") || !strings.HasSuffix(strings.TrimRight(l, " "), "fit —") || !strings.Contains(l, " 4d ") || !strings.Contains(f, "⚑ 9mo") {
		t.Fatalf("columns wrong:\n%q\n%q", f, l)
	}
	at := func(row, s string) int { return xansi.StringWidth(row[:strings.Index(row, s)+len(s)]) }
	if at(f, "2026") != at(l, "4d") || at(f, "Marginal") != at(l, "—") {
		t.Fatalf("columns do not line up:\n%q\n%q", f, l)
	}
}

// Each column is as wide as its widest value in the view, and a column empty on every
// row is hidden, so the title takes the room: no Lead has a Follow-up flag, so a Leads
// view reserves no cells for one.
func TestRowColumnsFitTheView(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	lead := store.Application{Title: "Software Engineer at a long-named place", Company: "Acme", Track: "SWE", Status: store.StatusLead,
		LastChange: &store.StatusEntry{Status: store.StatusLead, At: "2026-09-25"}}
	show := m.slotsFor([]store.Application{lead})
	if show.flagW != 0 {
		t.Fatalf("a view without a flag reserves %d cells for one", show.flagW)
	}
	got := xansi.Strip(m.row(p, lead, false, show, 40)[0])
	if !strings.HasPrefix(got, "  ◌ Software Engineer at a") || !strings.Contains(got, "…  6d  fit — ") {
		t.Fatalf("the title does not take the unused room: %q", got)
	}
}

// The widest values side by side never touch: a long-quiet flag, an approximate
// month-and-year age and the longest Fit keep two cells between them; a years-old change
// reads in years, not "82mo"; and a same-day change reads "today" whatever its Status.
func TestRowColumnsKeepAGapAtTheirWidest(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := model{now: func() time.Time { return now }, followUpDays: 14}
	p := m.pal()
	widest := store.Application{Title: "Widest", Company: "Acme", Track: "SWE", Status: store.StatusInterviewing, Fit: store.FitMarginal,
		AppliedDate: &store.StatusEntry{At: "2024-10-20"}, LastChange: &store.StatusEntry{Status: store.StatusInterviewing, At: "2024-10-20", Approx: true}}
	one := func(a store.Application) string {
		return xansi.Strip(m.row(p, a, false, m.slotsFor([]store.Application{a}), 60)[0])
	}
	if got := one(widest); !strings.Contains(got, "⚑ 23mo  ~Oct 2024  fit Marginal") {
		t.Fatalf("widest columns run together or are cut: %q", got)
	}
	old := widest
	old.LastChange = &store.StatusEntry{Status: store.StatusApplied, At: "2019-01-01"}
	if got := one(old); !strings.Contains(got, "⚑ 7y") {
		t.Fatalf("a years-old change should read in years: %q", got)
	}
	for _, status := range []string{store.StatusLead, store.StatusInterviewing} {
		today := widest
		today.Status, today.Fit = status, store.FitStrong
		today.LastChange = &store.StatusEntry{Status: status, At: "2026-10-01"}
		if got := one(today); !regexp.MustCompile(`today {2,}fit Strong`).MatchString(got) {
			t.Fatalf("%s: a same-day change reads today, set apart from Fit: %q", status, got)
		}
	}
}
