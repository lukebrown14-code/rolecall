package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stepWorkspace has an Interviewing Application with a next step in 3 days and an
// Applied one with none, on the fixture clock (1 Oct 2026).
func stepWorkspace(t *testing.T, extra string) string {
	t.Helper()
	return fixtureCopy(t, func(root string) {
		dir := filepath.Join(root, "jobs", "SWE", "stepper")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		meta := "schema: 1\ntitle: Stepper\ncompany: Stepco\ntrack: SWE\n" + extra +
			"status_log:\n  - {status: Applied, at: 2026-09-28}\n  - {status: Interviewing, at: 2026-09-29}\n"
		if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
			t.Fatal(err)
		}
	})
}

func metaText(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "jobs", "SWE", name, "meta.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNextStepShowsOnInterviewingRowAndOverview(t *testing.T) {
	root := stepWorkspace(t, "next_step: {at: 2026-10-04T10:30, what: \"Phone screen\"}\ncontacts:\n  - {name: \"Sam Lee\", role: Recruiter, email: sam@example.test}\n")
	d := newDriver(t, root, fixtureClock, 220, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	view := d.plain()
	for _, want := range []string{"next 4 Oct 10:30 · Phone screen", "NEXT STEP", "Sam Lee, Recruiter · sam@example.test"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

func TestStatsNowCountsNextStepsWithinAWeek(t *testing.T) {
	root := stepWorkspace(t, "next_step: {at: 2026-10-04, what: Call}\n")
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("4")
	if v := d.plain(); !strings.Contains(v, "1 next step within 7 days") {
		t.Fatalf("Now lacks the next step:\n%s", v)
	}
	far := stepWorkspace(t, "next_step: {at: 2026-11-30, what: Call}\n")
	d = newDriver(t, far, fixtureClock, 120, 40)
	d.Type("4")
	if v := d.plain(); strings.Contains(v, "next step") {
		t.Fatalf("a next step beyond a week is not Now:\n%s", v)
	}
}

func editStepper(d *driver) {
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("e")
}

func TestEditSavesNextStepAndFirstContact(t *testing.T) {
	root := stepWorkspace(t, "contact: Old Name\n")
	d := newDriver(t, root, fixtureClock, 120, 40)
	editStepper(d)
	for range 7 { // URL … Resume only
		d.Type("enter")
	}
	d.Type("2026-10-05T09:00")
	d.Type("enter")
	d.Type("Technical interview")
	d.Type("enter")
	d.Type("enter") // keep the legacy contact's name
	d.Type("Hiring manager")
	d.Type("enter")
	d.Type("old@example.test")
	d.Type("enter")
	got := metaText(t, root, "stepper")
	for _, want := range []string{
		"next_step: {at: 2026-10-05T09:00, what: \"Technical interview\"}\n",
		"contacts:\n  - {name: \"Old Name\", role: \"Hiring manager\", email: \"old@example.test\"}\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("meta.yml lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\ncontact:") {
		t.Errorf("the legacy contact key was kept:\n%s", got)
	}
}

func TestEditRefusesABadNextStepDate(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	editStepper(d)
	for range 7 {
		d.Type("enter")
	}
	d.Type("2026-02-30")
	d.Type("enter")
	if v := d.plain(); !strings.Contains(v, "isn't a real date") {
		t.Fatalf("no date error:\n%s", v)
	}
	if strings.Contains(metaText(t, root, "stepper"), "next_step") {
		t.Fatal("a bad date was saved")
	}
}

func TestEditRefusesWhatWithoutADate(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	editStepper(d)
	for range 8 { // past the date, which stays empty
		d.Type("enter")
	}
	d.Type("Call")
	d.Type("ctrl+s")
	if v := d.plain(); !strings.Contains(v, "Add a date, or clear What") {
		t.Fatalf("no cross-field error:\n%s", v)
	}
	if strings.Contains(metaText(t, root, "stepper"), "next_step") {
		t.Fatal("a next step without a date was saved")
	}
}

func TestMoveToRejectedAsksWhyAndRecordsIt(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("mr")
	if v := d.plain(); !strings.Contains(v, "Why did it end?") {
		t.Fatalf("no reason pop-up:\n%s", v)
	}
	d.Type("right") // skip → no-response
	d.Type("right") // after-screen
	d.Type("enter")
	if got := metaText(t, root, "stepper"); !strings.Contains(got, "close_reason: \"after-screen\"\n") || !strings.Contains(got, "status: Rejected") {
		t.Fatalf("reason not recorded:\n%s", got)
	}
	// Undoing the move takes the reason with it.
	d.Type("u")
	if got := metaText(t, root, "stepper"); strings.Contains(got, "close_reason") {
		t.Fatalf("close_reason survived the undo:\n%s", got)
	}
}

func TestMoveToWithdrawnCanSkipTheReason(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("mw")
	d.Type("enter") // "skip" is the first choice
	got := metaText(t, root, "stepper")
	if strings.Contains(got, "close_reason") || !strings.Contains(got, "status: Withdrawn") {
		t.Fatalf("skipping wrote a reason or lost the move:\n%s", got)
	}
}

func TestStatsShowsWhereApplicationsDroppedOff(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		for i, reason := range []string{"no-response", "no-response", "salary"} {
			dir := filepath.Join(root, "jobs", "SWE", "closed-"+string(rune('a'+i)))
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			meta := "schema: 1\ntitle: Closed\ncompany: Co\ntrack: SWE\nclose_reason: " + reason +
				"\nstatus_log:\n  - {status: Applied, at: 2026-09-01}\n  - {status: Rejected, at: 2026-09-10}\n"
			if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
	d := newDriver(t, root, fixtureClock, 140, 50)
	d.Type("4")
	if v := d.plain(); !strings.Contains(v, "DROPPED OFF") || !strings.Contains(v, "no-response 2 · salary 1") {
		t.Fatalf("Stats lacks the drop-off line:\n%s", v)
	}
}
