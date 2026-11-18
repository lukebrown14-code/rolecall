// Package schedule holds the rules derived from an Application's dates: Follow-up due and
// the next-step hold, close dates and posting expiry, and Now, what wants doing today. The
// TUI and the CLI read them here, so a row's ⚑, Stats → Now and `rolecall now` can never
// count differently, and plugin/scripts/meta.py and rules.py mirror them against
// fixtures/conformance.json. Every function is pure: the clock comes in as now, and the
// Owner's time zone is now's own.
package schedule

import (
	"time"

	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// SoonDays is how far ahead "now" looks: a close date or a next step within it is wanted
// today. The rows' amber close date turns at the same distance.
const SoonDays = 7

const hoursPerDay = 24

// DaysFrom is the whole days from today to date (negative in the past), with date's day
// (store.LocalDay in now's zone), and whether date could be read.
func DaysFrom(date string, now time.Time) (time.Time, int, bool) {
	t, ok := store.LocalDay(date, now.Location())
	if !ok {
		return time.Time{}, 0, false
	}
	return t, DaysBetween(store.Today(now), t), true
}

// DaysBetween is the whole days from a to b, both days from store.LocalDay or store.Today.
func DaysBetween(a, b time.Time) int { return int(b.Sub(a).Hours() / hoursPerDay) }

// writtenDay reads a date-only field (closes, posted): the day written, as no zone applies.
func writtenDay(date string) (time.Time, bool) { return store.LocalDay(date, time.UTC) }

// EffectiveLog is the Status log the rows, timeline and ages read: undone moves taken out
// (reporting.EffectiveLog), as Stats reads it, so a fat-fingered move and its undo leave no
// phantom stop and do not reset the age in Status. An undone first move of a Lead empties
// it: the Lead reads as never moved, as it did before. Otherwise it falls back to the
// whole log if the two ever disagree on the current Status.
func EffectiveLog(a store.Application) []store.StatusEntry {
	log := reporting.EffectiveLog(a.StatusLog)
	if len(log) == 0 && a.Status == store.StatusLead {
		return nil
	}
	if len(log) == 0 || log[len(log)-1].Status != a.Status {
		return a.StatusLog
	}
	return log
}

// LastChange is when the Application reached its current Status, ignoring undone moves:
// the store's LastChange unless an undo took entries out.
func LastChange(a store.Application) *store.StatusEntry {
	log := EffectiveLog(a)
	if len(log) == len(a.StatusLog) {
		return a.LastChange
	}
	if len(log) == 0 {
		return nil
	}
	e := log[len(log)-1]
	return &e
}

// AppliedDate is the Application's first Applied entry that was not undone, or nil: the
// store's AppliedDate unless an undo took entries out.
func AppliedDate(a store.Application) *store.StatusEntry {
	log := EffectiveLog(a)
	if len(log) == len(a.StatusLog) {
		return a.AppliedDate
	}
	for _, e := range log {
		if e.Status == store.StatusApplied {
			return &e
		}
	}
	return nil
}
