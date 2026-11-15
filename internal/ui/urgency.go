package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Dates: relative while recent, a month and year once older than monthYearAfter days.
const (
	monthYearAfter = 90 // "~21mo ago" reads as "Jan 2025" past about three months
	urgentDays     = 2  // "closes in 2d ▲" turns danger at or below this
	urgentMark     = glyphUrgent
)

// sortNewest orders Leads by when they were added, newest first: the first Status
// entry's date, then folder.
func sortNewest(rows []store.Application) {
	added := func(a store.Application) string {
		if len(a.StatusLog) == 0 {
			return ""
		}
		return a.StatusLog[0].At
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if a, b := added(rows[i]), added(rows[j]); a != b {
			return a > b
		}
		return rows[i].Folder < rows[j].Folder
	})
}

// span is a distance in days as the list words it: "4d", "3w", "2mo", and from two
// years "2y", so a long-quiet Application never reads "82mo".
func span(n int) string {
	switch {
	case n < weeksUntil:
		return fmt.Sprintf("%dd", n)
	case n < monthsUntil:
		return fmt.Sprintf("%dw", n/daysPerWeek)
	case n < yearsFrom:
		return fmt.Sprintf("%dmo", n/daysPerMonth)
	}
	return fmt.Sprintf("%dy", n/daysPerYear)
}

// span switches from months to years at two years.
const (
	daysPerYear = 365
	yearsFrom   = 2 * daysPerYear
)

// parseDay reads the day of a status or close date: a YYYY-MM-DD value, or the date part
// of an RFC 3339 timestamp. It is the one place a date is turned into a day.
func parseDay(date string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.Split(date, "T")[0])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// day is now's date at midnight, so day differences ignore the time of day.
func day(now time.Time) time.Time {
	t, _ := time.Parse("2006-01-02", now.Format("2006-01-02"))
	return t
}

// daysFrom is the whole days from today to date (negative in the past), and whether
// date could be read.
func daysFrom(date string, now time.Time) (time.Time, int, bool) {
	t, ok := parseDay(date)
	if !ok {
		return time.Time{}, 0, false
	}
	return t, int(t.Sub(day(now)).Hours() / hoursPerDay), true
}

// followUpDue reports whether a sent Application's last change is older than the
// follow-up period: the whole-day rule the row's ⚑ flag and the Stats screen share, so
// the flag and the reasoning can never disagree. The time of day never matters.
func followUpDue(a store.Application, now time.Time, days int) bool {
	_, ok := followUpDaysPast(a, now, days)
	return ok
}

// heatHighAfter is how many days past its follow-up date a Follow-up due flag turns
// bold: the flag is always ⚑, and its weight, not another glyph, says it is long overdue.
const heatHighAfter = 7

// followUpDaysPast is how many whole days past its follow-up date a sent Application is
// (0 on the day it falls due), and whether it is Follow-up due at all.
func followUpDaysPast(a store.Application, now time.Time, days int) (int, bool) {
	if (a.Status != store.StatusApplied && a.Status != store.StatusInterviewing) || lastChange(a) == nil {
		return 0, false
	}
	t, ok := parseDay(lastChange(a).At)
	if !ok {
		return 0, false
	}
	past := int(day(now).Sub(t.AddDate(0, 0, days)).Hours() / hoursPerDay)
	return past, past >= 0
}

// followUpLate reports a Follow-up due flag drawn bold: more than heatHighAfter days past.
func followUpLate(past int) bool { return past > heatHighAfter }

// followUpWords is a Follow-up due as the detail and preview word it, "due · no change in 9mo":
// the same span from the last change as the row's "⚑ 9mo", so the two never disagree.
func (m model) followUpWords(a store.Application) string {
	return "due · no change in " + m.since(lastChange(a))
}

// shortDate is "3 Sep", with the year when it is not this year's.
func shortDate(t, now time.Time) string {
	if t.Year() == now.Year() {
		return t.Format("2 Jan")
	}
	return t.Format("2 Jan 2006")
}

// closeState is how pressing an Application's close date is.
type closeState int

const (
	closeNone   closeState = iota // no close date
	closeLater                    // more than soonDays away
	closeSoon                     // within soonDays: amber
	closeUrgent                   // within urgentDays: danger, with ▲
	closePast                     // passed: struck through and muted
)

// closeInfo is a close date in words, with its urgency: "closes in 6d", "closes in 2d ▲",
// "closes today ▲", "expired 3 Sep": the ad's close date, worded apart from a closed
// (Rejected or Withdrawn) Application.
func closeInfo(closes string, now time.Time) (string, closeState) {
	if closes == "" {
		return "", closeNone
	}
	t, days, ok := daysFrom(closes, now)
	switch {
	case !ok:
		return "closes " + closes, closeLater
	case days < 0:
		return "expired " + shortDate(t, now), closePast
	case days == 0:
		return "closes today " + urgentMark, closeUrgent
	}
	r, _ := ago(closes, now)
	switch {
	case days <= urgentDays:
		return "closes " + r + " " + urgentMark, closeUrgent
	case days <= soonDays:
		return "closes " + r, closeSoon
	}
	return "closes " + r, closeLater
}

// closeStyle draws a close date by its urgency, starting from st (the row's own style,
// so a selected row keeps its fill) and base, the colour of an unhurried date.
func (p pal) CloseStyle(s closeState, st func(color.Color) lipgloss.Style, base color.Color) lipgloss.Style {
	switch s {
	case closeUrgent:
		return st(p.Bad).Bold(true)
	case closeSoon:
		return st(p.Warn)
	case closePast:
		return st(p.Dim).Strikethrough(true)
	}
	return st(base)
}
