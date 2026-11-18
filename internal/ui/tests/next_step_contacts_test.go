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
	for _, want := range []string{"Stepco · next 4 Oct 10:30, Phone screen · SWE", "NEXT STEP", "Sam Lee, Recruiter · sam@example.test"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q:\n%s", want, view)
		}
	}
}

// appliedStepper is a Workspace with one Applied Application, stepper, whose meta.yml
// carries extra.
func appliedStepper(t *testing.T, extra string) string {
	t.Helper()
	return fixtureCopy(t, func(root string) {
		dir := filepath.Join(root, "jobs", "SWE", "stepper")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		meta := "schema: 1\ntitle: Stepper\ncompany: Stepco\ntrack: SWE\n" + extra +
			"status_log:\n  - {status: Applied, at: 2026-09-28}\n"
		if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
			t.Fatal(err)
		}
	})
}

// A booked next step shows on an Applied row too, right after the Company with its ▲
// first, at the common widths; NO_COLOR keeps the ▲.
func TestNextStepShowsOnAppliedRowAfterTheCompany(t *testing.T) {
	root := appliedStepper(t, "next_step: {at: 2026-10-02T14:30, what: \"Phone screen\"}\n")
	for _, size := range [][2]int{{80, 24}, {120, 36}, {160, 44}} {
		d := newDriver(t, root, fixtureClock, size[0], size[1])
		d.Type("3")
		d.Type("/stepper")
		d.Type("enter")
		want := "Stepco · ▲ next tomorrow 14:30"
		if size[0] >= 120 {
			want += ", Phone screen"
		}
		if v := d.plain(); !strings.Contains(v, want) {
			t.Errorf("%dx%d: row lacks %q:\n%s", size[0], size[1], want, v)
		}
	}
	t.Setenv("NO_COLOR", "1")
	d := newDriver(t, root, fixtureClock, 120, 36)
	d.Type("3")
	if v := d.plain(); !strings.Contains(v, "▲ next tomorrow 14:30") {
		t.Errorf("NO_COLOR row lost the ▲:\n%s", v)
	}
}

// On an Interviewing row at 120 columns the next step is never the fact the row cuts.
func TestInterviewingNextStepIsNotTruncatedAt120(t *testing.T) {
	root := stepWorkspace(t, "next_step: {at: 2026-10-02T09:00, what: \"Onsite panel\"}\n")
	d := newDriver(t, root, fixtureClock, 120, 36)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	if v := d.plain(); !strings.Contains(v, "Stepco · ▲ next tomorrow 09:00, Onsite panel · ") {
		t.Fatalf("next step cut on the Interviewing row:\n%s", v)
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
	editStepper(d) // an Interviewing Application's form opens on its next step
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
	d.Type("enter") // past the date, which stays empty
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
	// The pop-up says the move is done, lists every reason in words without ←→, and esc
	// skips rather than cancels.
	v := d.plain()
	for _, want := range []string{"Moved to Rejected · why did it end?", "No response", "After screen", "After interview",
		"Salary", "Role closed", "Other", "esc skip"} {
		if !strings.Contains(v, want) {
			t.Errorf("reason pop-up lacks %q:\n%s", want, v)
		}
	}
	for _, unwanted := range []string{"esc cancel", "no-response", "‹"} {
		if strings.Contains(v, unwanted) {
			t.Errorf("reason pop-up shows %q:\n%s", unwanted, v)
		}
	}
	d.Type("down") // the cursor starts on No response
	d.Type("enter")
	if got := metaText(t, root, "stepper"); !strings.Contains(got, "close_reason: \"after-screen\"\n") || !strings.Contains(got, "status: Rejected") {
		t.Fatalf("reason not recorded:\n%s", got)
	}
	v = d.plain()
	if !strings.Contains(v, "Moved to Rejected") || !strings.Contains(v, "u undo") {
		t.Errorf("the move's toast lost its undo:\n%s", v)
	}
	if !strings.Contains(v, "CLOSED") || !strings.Contains(v, "After screen") || strings.Contains(v, "after-screen") {
		t.Errorf("the card does not show the reason in words:\n%s", v)
	}
	// Undoing the move takes the reason with it.
	d.Type("u")
	if got := metaText(t, root, "stepper"); strings.Contains(got, "close_reason") {
		t.Fatalf("close_reason survived the undo:\n%s", got)
	}
}

// A number key picks a reason straight away.
func TestCloseReasonNumberKeyPicks(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("mr")
	d.Type("4")
	if got := metaText(t, root, "stepper"); !strings.Contains(got, "close_reason: \"salary\"\n") {
		t.Fatalf("4 did not record salary:\n%s", got)
	}
}

// rejectedStepper is a Workspace with one Rejected Application, stepper, whose meta.yml
// carries extra.
func rejectedStepper(t *testing.T, extra string) string {
	t.Helper()
	return fixtureCopy(t, func(root string) {
		dir := filepath.Join(root, "jobs", "SWE", "stepper")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		meta := "schema: 1\ntitle: Stepper\ncompany: Stepco\ntrack: SWE\n" + extra +
			"status_log:\n  - {status: Applied, at: 2026-09-28}\n  - {status: Rejected, at: 2026-09-29}\n"
		if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
			t.Fatal(err)
		}
	})
}

// A Rejected Application's close reason can be set or changed later from Edit, and is
// written to meta.yml; the card shows it in words.
func TestEditChangesTheCloseReason(t *testing.T) {
	root := rejectedStepper(t, "close_reason: no-response\n")
	d := newDriver(t, root, fixtureClock, 120, 40)
	editStepper(d)
	// The form opens on the reason, under its CLOSED heading.
	if v := d.plain(); !strings.Contains(v, "CLOSED") || !strings.Contains(lineWith(v, "Reason  ‹ No response ›"), "▌") {
		t.Fatalf("the Edit form lacks the close reason, or is not on it:\n%s", v)
	}
	d.Type("right") // No response → After screen
	d.Type("ctrl+s")
	if got := metaText(t, root, "stepper"); !strings.Contains(got, "close_reason: \"after-screen\"\n") {
		t.Fatalf("close reason not changed:\n%s", got)
	}
	if v := d.plain(); !strings.Contains(v, "After screen") || strings.Contains(v, "after-screen") {
		t.Fatalf("the card shows the slug:\n%s", v)
	}
	// none takes the key out.
	d.Type("e")
	d.Type("left")
	d.Type("left") // After screen → No response → none
	d.Type("ctrl+s")
	if got := metaText(t, root, "stepper"); strings.Contains(got, "close_reason") {
		t.Fatalf("none kept the close reason:\n%s", got)
	}
}

// The Owner's own words in close_reason read as words on the card and survive an Edit
// that leaves them alone.
func TestCloseReasonOwnWordsKeptAndReadable(t *testing.T) {
	root := rejectedStepper(t, "close_reason: went-elsewhere\nfit: Good\n")
	d := newDriver(t, root, fixtureClock, 120, 40)
	editStepper(d)
	d.Type("esc")
	if v := d.plain(); !strings.Contains(v, "Went elsewhere") {
		t.Fatalf("own words not shown in words:\n%s", v)
	}
	d.Type("e")
	d.Type("ctrl+s")
	if got := metaText(t, root, "stepper"); !strings.Contains(got, "close_reason: went-elsewhere\n") {
		t.Fatalf("an untouched Edit rewrote the close reason:\n%s", got)
	}
}

// Leads to Offers never see a close reason field.
func TestEditOfAnOpenApplicationHasNoCloseReason(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	editStepper(d)
	if v := d.plain(); strings.Contains(v, "Reason  ‹") || strings.Contains(v, "CLOSED") {
		t.Fatalf("an Interviewing Edit form asks a close reason:\n%s", v)
	}
}

func TestMoveToWithdrawnCanSkipTheReason(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("mw")
	d.Type("down") // a choice made, then let go: esc still only skips
	d.Type("esc")
	got := metaText(t, root, "stepper")
	if strings.Contains(got, "close_reason") || !strings.Contains(got, "status: Withdrawn") {
		t.Fatalf("skipping wrote a reason or lost the move:\n%s", got)
	}
	v := d.plain()
	if strings.Contains(v, "Discard changes?") || strings.Contains(v, "why did it end?") {
		t.Fatalf("esc did not just close the pop-up:\n%s", v)
	}
	if !strings.Contains(v, "Moved to Withdrawn") || !strings.Contains(v, "u undo") {
		t.Fatalf("the move's toast lost its undo:\n%s", v)
	}
	d.Type("u")
	if got := metaText(t, root, "stepper"); !strings.HasSuffix(strings.TrimSpace(got), "{status: Interviewing, at: 2026-10-01T00:00:00Z}") {
		t.Fatalf("u did not undo the move after a skip:\n%s", got)
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
	if v := d.plain(); !strings.Contains(v, "DROPPED OFF") || !strings.Contains(v, "no response 2 · salary 1") {
		t.Fatalf("Stats lacks the drop-off line:\n%s", v)
	}
}

// twoInterviewing is a Workspace with two Interviewing Applications, stepper and stepper2.
func twoInterviewing(t *testing.T) string {
	t.Helper()
	return fixtureCopy(t, func(root string) {
		for _, name := range []string{"stepper", "stepper2"} {
			dir := filepath.Join(root, "jobs", "SWE", name)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			meta := "schema: 1\ntitle: Stepper\ncompany: " + name + "\ntrack: SWE\n" +
				"status_log:\n  - {status: Applied, at: 2026-09-28}\n  - {status: Interviewing, at: 2026-09-29}\n"
			if err := os.WriteFile(filepath.Join(dir, "meta.yml"), []byte(meta), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})
}

// A bulk move to Rejected moves at the pick (the Move pop-up is its one confirmation), then
// asks one "Why did it end?" and writes that reason to every Application that moved; one u
// takes the moves and the reasons back.
func TestBulkMoveToRejectedAppliesOneCloseReasonToAll(t *testing.T) {
	root := twoInterviewing(t)
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("*")
	d.Type("j")
	d.Type("*")
	d.Type("mr")
	if v := d.plain(); !strings.Contains(v, "Moved to Rejected · why did they end?") || !strings.Contains(v, "2 Applications marked") {
		t.Fatalf("no reason pop-up for the group:\n%s", v)
	}
	d.Type("2") // After screen
	d.Type("enter")
	for _, name := range []string{"stepper", "stepper2"} {
		if got := metaText(t, root, name); !strings.Contains(got, "close_reason: \"after-screen\"\n") || !strings.Contains(got, "status: Rejected") {
			t.Fatalf("%s: reason not recorded:\n%s", name, got)
		}
	}
	d.Type("u")
	for _, name := range []string{"stepper", "stepper2"} {
		if got := metaText(t, root, name); strings.Contains(got, "close_reason") || !strings.HasSuffix(strings.TrimSpace(got), "{status: Interviewing, at: 2026-10-01T00:00:00Z}") {
			t.Fatalf("%s: undo left the move or the reason:\n%s", name, got)
		}
	}
}

// Skipping the group's reason keeps the moves and writes no reason.
func TestBulkMoveToWithdrawnCanSkipTheReason(t *testing.T) {
	root := twoInterviewing(t)
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("*")
	d.Type("j")
	d.Type("*")
	d.Type("mw")
	d.Type("esc")
	for _, name := range []string{"stepper", "stepper2"} {
		if got := metaText(t, root, name); strings.Contains(got, "close_reason") || !strings.Contains(got, "status: Withdrawn") {
			t.Fatalf("%s: skip wrote a reason or lost the move:\n%s", name, got)
		}
	}
}

// The next step's date takes the Closes forms plus a time, echoes what it read, and is saved
// as YYYY-MM-DD[THH:MM].
func TestEditNextStepTakesFlexibleDates(t *testing.T) {
	root := stepWorkspace(t, "")
	d := newDriver(t, root, fixtureClock, 120, 40)
	editStepper(d)
	d.Type("tomorrow 14:30")
	if v := d.plain(); !strings.Contains(v, "→ Fri 2 Oct 2026 14:30") {
		t.Fatalf("no echo of the parsed next step:\n%s", v)
	}
	d.Type("enter")
	d.Type("Phone screen")
	d.Type("ctrl+s")
	if got := metaText(t, root, "stepper"); !strings.Contains(got, "next_step: {at: 2026-10-02T14:30, what: \"Phone screen\"}\n") {
		t.Fatalf("next step not saved normalised:\n%s", got)
	}
}

// The full-screen detail at 80x24 carries the next step and the Contact in its Overview.
func TestFullScreenDetailShowsNextStepAndContactAt80x24(t *testing.T) {
	root := stepWorkspace(t, "next_step: {at: 2026-10-04T10:30, what: \"Phone screen\"}\ncontacts:\n  - {name: \"Sam Lee\", role: \"Recruiter\", email: \"sam@example.test\"}\n")
	d := newDriver(t, root, fixtureClock, 80, 24)
	d.Type("3")
	d.Type("/stepper")
	d.Type("enter")
	d.Type("enter")
	v := d.plain()
	for _, want := range []string{"NEXT STEP", "Phone screen", "CONTACTS", "Sam Lee"} {
		if !strings.Contains(v, want) {
			t.Errorf("80x24 detail lacks %q:\n%s", want, v)
		}
	}
}
