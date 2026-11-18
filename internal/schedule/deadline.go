package schedule

import (
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Deadline is the last day a Lead or Draft is worth sending: its close date, or for a
// Lead the last day its ad is under max_posting_age_days (the day the row reads
// "expires in 1 day"), whichever is sooner. ok is false when it has neither.
func Deadline(a store.Application, rules store.SearchRules) (time.Time, bool) {
	closes, hasCloses := writtenDay(a.Closes)
	expires, hasExpiry := PostingExpiry(a, rules)
	switch {
	case hasCloses && hasExpiry:
		if expires.Before(closes) {
			return expires, true
		}
		return closes, true
	case hasExpiry:
		return expires, true
	}
	return closes, hasCloses
}

// HasCloses reports a close date that reads as a day.
func HasCloses(a store.Application) bool {
	_, ok := writtenDay(a.Closes)
	return ok
}

// ClosesPassed reports a close date before today.
func ClosesPassed(a store.Application, now time.Time) bool {
	t, ok := writtenDay(a.Closes)
	return ok && t.Before(store.Today(now))
}

// Passed reports a Deadline before today.
func Passed(a store.Application, rules store.SearchRules, now time.Time) bool {
	t, ok := Deadline(a, rules)
	return ok && t.Before(store.Today(now))
}

// PostingExpiry is the last day a Lead's ad is under its max_posting_age_days (its
// Source's own, else the global one): the day before it reaches the limit, when it is
// expired. ok is false when no limit applies or the ad does not say when it was posted.
// Only a Lead has one, as only a Lead is warned of it.
func PostingExpiry(a store.Application, rules store.SearchRules) (time.Time, bool) {
	limit := rules.AgeLimit(a.Source)
	if limit <= 0 || a.Status != store.StatusLead {
		return time.Time{}, false
	}
	posted, ok := writtenDay(a.Posted)
	if !ok {
		return time.Time{}, false
	}
	return posted.AddDate(0, 0, limit-1), true
}

// PostedDays is how many whole days ago a Lead's ad was posted, and whether it says.
func PostedDays(a store.Application, now time.Time) (int, bool) {
	_, d, ok := DaysFrom(a.Posted, now)
	return -d, ok
}

// PostingDaysLeft is a Lead's max_posting_age_days (its Source's own, else the global
// one) minus its ad's age in days: SCHEMA's days_left, so 0 or less is expired (an ad
// exactly at the limit is too old, as rules.py check reads it). ok is false when the
// Application is not a Lead, no limit applies or the ad has no date.
func PostingDaysLeft(a store.Application, rules store.SearchRules, now time.Time) (int, bool) {
	limit := rules.AgeLimit(a.Source)
	if limit <= 0 || a.Status != store.StatusLead {
		return 0, false
	}
	d, ok := PostedDays(a, now)
	if !ok {
		return 0, false
	}
	return limit - d, true
}

// PostingExpiring reports a Lead whose ad is within expiry_warning_days of
// max_posting_age_days and not yet at it: the rows that read "expires in N days".
func PostingExpiring(a store.Application, rules store.SearchRules, now time.Time) bool {
	left, ok := PostingDaysLeft(a, rules, now)
	return ok && left >= 1 && left <= rules.ExpiryWarningDays
}
