package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// quietStepper is a Workspace with stepper Applied long ago (Follow-up due on its own),
// with extra in its meta.yml.
func quietStepper(t *testing.T, status, extra string) string {
	t.Helper()
	return fixtureCopy(t, func(root string) {
		dir := filepath.Join(root, "jobs", "SWE", "stepper")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		log := "status_log:\n  - {status: Applied, at: 2026-01-05}\n"
		if status != store.StatusApplied {
			log += "  - {status: " + status + ", at: 2026-01-06}\n"
		}
		meta := "schema: 1\ntitle: Stepper\ncompany: Stepco\ntrack: SWE\n" + extra + log
		if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
			t.Fatal(err)
		}
	})
}

// stepperRow is stepper's row on the Pipeline, found by search.
func stepperRow(t *testing.T, root string) string {
	t.Helper()
	d := newDriver(t, root, fixtureClock, 160, 44)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	return lineWith(d.plain(), "● Stepper")
}

// A next step booked for today or later holds the Follow-up due flag: the row, Stats → Now
// and sorting read the one rule, so none of them nags while something is arranged. Once
// the next step has passed, the flag is back.
func TestBookedNextStepHoldsTheFollowUpFlag(t *testing.T) {
	for _, tc := range []struct {
		name, extra string
		due         bool
	}{
		{"no next step", "", true},
		{"next step tomorrow", "next_step: {at: 2026-10-02, what: \"Phone screen\"}\n", false},
		{"next step today", "next_step: {at: 2026-10-01T16:00}\n", false},
		{"next step passed", "next_step: {at: 2026-09-20, what: \"Phone screen\"}\n", true},
	} {
		root := quietStepper(t, store.StatusApplied, tc.extra)
		if row := stepperRow(t, root); strings.Contains(row, "⚑") != tc.due {
			t.Errorf("%s: row %q, want ⚑ %v", tc.name, row, tc.due)
		}
		s, err := store.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		apps, err := s.Scan()
		if err != nil {
			t.Fatal(err)
		}
		listed := false
		for _, a := range schedule.NowFor(apps, fixtureClock(), 14, schedule.SoonDays).FollowUps {
			listed = listed || a.Title == "Stepper"
		}
		if listed != tc.due {
			t.Errorf("%s: Now lists it as Follow-up due %v, want %v", tc.name, listed, tc.due)
		}
	}
}

// Closing an Application with a pending next step keeps the next step on the card, struck
// through and marked closed, since it is still in meta.yml: nothing is hidden unsaid.
func TestClosedApplicationKeepsItsNextStepStruckThrough(t *testing.T) {
	root := quietStepper(t, store.StatusRejected, "next_step: {at: 2026-10-04, what: \"Phone screen\"}\n")
	d := newDriver(t, root, fixtureClock, 160, 44)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("enter")
	line := lineWith(d.plain(), "NEXT STEP")
	if !strings.Contains(line, "4 Oct · Phone screen · closed") {
		t.Fatalf("closed card's NEXT STEP row %q, want the next step marked closed:\n%s", line, d.plain())
	}
	if strings.Contains(line, "e edit") {
		t.Errorf("closed NEXT STEP row %q repeats the CLOSED row's e edit", line)
	}
	if !strings.Contains(d.View(), "\x1b[9") && !strings.Contains(d.View(), ";9m") {
		t.Errorf("closed next step is not struck through")
	}
}
