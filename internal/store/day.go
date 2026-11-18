package store

import (
	"strings"
	"time"
)

// LocalDay is the day a date in a meta.yml counts on, as SCHEMA reads it: an RFC 3339
// timestamp (a status_log at) counts on its date in loc, the Owner's time zone; a
// YYYY-MM-DD value, or a zoneless YYYY-MM-DDTHH:MM (a next_step at), keeps the day
// written. The day is midnight UTC of that date, so whole-day differences never meet a
// daylight-saving hour. ok is false when the value holds no date. It is the one place a
// file's date is turned into a day; Today is the clock's side of the same comparison.
func LocalDay(at string, loc *time.Location) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, at); err == nil {
		y, m, d := t.In(loc).Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), true
	}
	date, _, _ := strings.Cut(at, "T")
	t, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// Today is now's date in now's own zone, as a day LocalDay can be compared with.
func Today(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
