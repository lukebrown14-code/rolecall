package schedule

import (
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// FollowUpStatus reports a Status that can be Follow-up due: Applied or Interviewing.
func FollowUpStatus(status string) bool {
	return status == store.StatusApplied || status == store.StatusInterviewing
}

// FollowUpFrom is the day Follow-up due counts from: the later of the last Status change
// (its day in loc, the Owner's zone) and the last recorded follow-up, the change winning a
// tie. ok is false when neither can be read.
func FollowUpFrom(a store.Application, loc *time.Location) (from time.Time, ok bool) {
	if e := LastChange(a); e != nil {
		from, ok = store.LocalDay(e.At, loc)
	}
	if t, read := store.LocalDay(a.FollowedUp, loc); read && (!ok || t.After(from)) {
		from, ok = t, true
	}
	return from, ok
}

// FollowedUpLater reports a recorded follow-up later than the last Status change, so it
// is what the Follow-up due count runs from. A follow-up before a later move is history.
func FollowedUpLater(a store.Application, loc *time.Location) bool {
	t, ok := store.LocalDay(a.FollowedUp, loc)
	if !ok {
		return false
	}
	if e := LastChange(a); e != nil {
		if c, read := store.LocalDay(e.At, loc); read && !t.After(c) {
			return false
		}
	}
	return true
}

// FollowUpDue reports whether a sent Application's last change is at least the
// follow-up period: the whole-day rule the row's ⚑ flag and the Stats screen share, so
// the flag and the reasoning can never disagree. The time of day never matters.
func FollowUpDue(a store.Application, now time.Time, days int) bool {
	_, ok := FollowUpDaysPast(a, now, days)
	return ok
}

// FollowUpDaysPast is how many whole days past its follow-up date a sent Application is
// (0 on the day it falls due), and whether it is Follow-up due at all. The follow-up date
// is follow_up_days after FollowUpFrom, so l clears the flag until it falls due again. A
// next step booked for today or later holds the flag: something is already arranged, so
// nothing is due until it passes. Every Follow-up due reader (the row's ⚑, sorting, NEXT,
// Stats → Now, `rolecall now`) comes through here, so the hold is the same everywhere.
func FollowUpDaysPast(a store.Application, now time.Time, days int) (int, bool) {
	if !FollowUpStatus(a.Status) || NextStepBooked(a, now) {
		return 0, false
	}
	t, ok := FollowUpFrom(a, now.Location())
	if !ok {
		return 0, false
	}
	past := DaysBetween(t.AddDate(0, 0, days), store.Today(now))
	return past, past >= 0
}

// NextStepBooked reports a next step on or after today's date.
func NextStepBooked(a store.Application, now time.Time) bool {
	if a.NextStep == nil {
		return false
	}
	_, days, ok := DaysFrom(a.NextStep.Day(), now)
	return ok && days >= 0
}

// QuietDays is how many whole days a sent Application has been quiet: the days since
// FollowUpFrom, the same day FollowUpDaysPast counts from. ok is false when neither date
// can be read.
func QuietDays(a store.Application, now time.Time) (int, bool) {
	from, ok := FollowUpFrom(a, now.Location())
	if !ok {
		return 0, false
	}
	return max(DaysBetween(from, store.Today(now)), 0), true
}
