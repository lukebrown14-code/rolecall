package store_test

import (
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// A status_log timestamp counts on its date in the Owner's zone; a date-only value, and a
// zoneless next_step time, keep the day written (SCHEMA status_log).
func TestLocalDayReadsATimestampInTheOwnersZone(t *testing.T) {
	sydney := time.FixedZone("AEDT", 11*60*60)
	newYork := time.FixedZone("EST", -5*60*60)
	for _, tc := range []struct {
		at   string
		loc  *time.Location
		want string
	}{
		{"2026-11-03T22:30:00Z", sydney, "2026-11-04"},
		{"2026-11-03T22:30:00Z", time.UTC, "2026-11-03"},
		{"2026-11-05T02:00:00Z", newYork, "2026-11-04"},
		{"2026-11-04T09:30:00+11:00", time.UTC, "2026-11-03"},
		{"2026-11-04", sydney, "2026-11-04"},
		{"2026-11-04", newYork, "2026-11-04"},
		{"2026-11-04T14:30", newYork, "2026-11-04"},
	} {
		got, ok := store.LocalDay(tc.at, tc.loc)
		if !ok || got.Format(time.DateOnly) != tc.want {
			t.Errorf("LocalDay(%q, %s) = %s, %v; want %s", tc.at, tc.loc, got.Format(time.DateOnly), ok, tc.want)
		}
		if got.Location() != time.UTC || got.Hour() != 0 {
			t.Errorf("LocalDay(%q) = %v, want midnight UTC of the day", tc.at, got)
		}
	}
	for _, bad := range []string{"", "soon", "2026-13-01"} {
		if _, ok := store.LocalDay(bad, sydney); ok {
			t.Errorf("LocalDay(%q) read as a day", bad)
		}
	}
}

// Today is the clock's own date, whatever the process zone, comparable with LocalDay.
func TestTodayIsTheClocksOwnDate(t *testing.T) {
	now := time.Date(2026, 11, 18, 7, 0, 0, 0, time.FixedZone("AEDT", 11*60*60))
	if got := store.Today(now); got.Format(time.DateOnly) != "2026-11-18" || got.Location() != time.UTC {
		t.Errorf("Today = %v", got)
	}
}
