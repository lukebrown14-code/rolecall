package schedule_test

import (
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// sydney is the Owner's zone in these tests: fixed, so they never read the machine's TZ.
var sydney = time.FixedZone("AEDT", 11*60*60)

func applied(at string) store.Application {
	e := store.StatusEntry{Status: store.StatusApplied, At: at}
	return store.Application{Status: store.StatusApplied, StatusLog: []store.StatusEntry{e}, LastChange: &e, AppliedDate: &e}
}

// Applied at 09:30 on 4 Nov in Sydney (22:30Z on 3 Nov) falls due on 18 Nov, not 17 Nov.
func TestFollowUpCountsFromTheLocalDayOfTheChange(t *testing.T) {
	a := applied("2026-11-03T22:30:00Z")
	if schedule.FollowUpDue(a, time.Date(2026, 11, 17, 20, 0, 0, 0, sydney), 14) {
		t.Error("due on 17 Nov, a day early")
	}
	past, due := schedule.FollowUpDaysPast(a, time.Date(2026, 11, 18, 8, 0, 0, 0, sydney), 14)
	if !due || past != 0 {
		t.Errorf("18 Nov: past %d due %v, want 0 true", past, due)
	}
	if n := schedule.NowFor([]store.Application{a}, time.Date(2026, 11, 17, 20, 0, 0, 0, sydney), 14, schedule.SoonDays); len(n.FollowUps) != 0 {
		t.Error("Now lists it on 17 Nov")
	}
}

// A move at 07:00 local on 18 Nov and an l the same day: the move wins, as SCHEMA says.
func TestSameLocalDayMoveWinsOverFollowUp(t *testing.T) {
	a := applied("2026-11-17T20:00:00Z")
	a.FollowedUp = "2026-11-18"
	if schedule.FollowedUpLater(a, sydney) {
		t.Error("followed_up on the move's local day reads as later than the move")
	}
	if from, ok := schedule.FollowUpFrom(a, sydney); !ok || from.Format(time.DateOnly) != "2026-11-18" {
		t.Errorf("from %v %v, want 2026-11-18", from, ok)
	}
}

// DaysFrom reads a timestamp on its day in now's zone and a date as written.
func TestDaysFromUsesNowsZone(t *testing.T) {
	now := time.Date(2026, 11, 18, 8, 0, 0, 0, sydney)
	for at, want := range map[string]int{
		"2026-11-17T22:00:00Z": 0, // 09:00 on 18 Nov in Sydney
		"2026-11-17T12:00:00Z": -1,
		"2026-11-17":           -1,
		"2026-11-19T14:30":     1,
	} {
		if _, got, ok := schedule.DaysFrom(at, now); !ok || got != want {
			t.Errorf("DaysFrom(%q) = %d %v, want %d", at, got, ok, want)
		}
	}
}
