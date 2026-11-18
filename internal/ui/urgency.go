package ui

import (
	"fmt"
	"image/color"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Dates: relative while recent, a month and year once older than monthYearAfter days.
const (
	monthYearAfter = 90       // "~21mo ago" reads as "Jan 2025" past about three months
	soonMark       = "(soon)" // the word that marks a close date within soonDays, beside the amber
	urgentDays     = 2        // "closes in 2d ▲" turns danger at or below this
	urgentMark     = glyphUrgent
)

// sortNewest orders Leads by when they were added, newest first: the first Status
// entry's date, then folder.
func sortNewest(rows []store.Application) {
	added := func(a store.Application) string {
		log := schedule.EffectiveLog(a) // an undone first move does not make a Lead new
		if len(log) == 0 {
			return ""
		}
		return log[0].At
	}
	sortStableBy(rows, func(a store.Application) datedRow { return datedRow{added(a), a.Folder} }, func(a, b datedRow) bool {
		if a.date != b.date {
			return a.date > b.date
		}
		return a.folder < b.folder
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

// heatHighAfter is how many days past its follow-up date a Follow-up due flag turns
// bold: the flag is always ⚑, and its weight, not another glyph, says it is long overdue.
const heatHighAfter = 7

// followUpLate reports a Follow-up due flag drawn bold: more than heatHighAfter days past.
func followUpLate(past int) bool { return past > heatHighAfter }

// followUpWords is a Follow-up due as the detail and preview word it: "due · no change in
// 9mo", or "due · followed up 3w ago" once a recorded follow-up is what it counts from. The
// span is followUpQuiet, the row's "⚑ 9mo", so the two never disagree.
func (m model) followUpWords(a store.Application) string {
	if schedule.FollowedUpLater(a, m.now().Location()) {
		return "due · followed up " + m.followUpQuiet(a) + " ago"
	}
	return "due · no change in " + m.followUpQuiet(a)
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
	closeSoon                     // within soonDays: amber, with the word (soon)
	closeUrgent                   // within urgentDays: danger, with ▲
	closePast                     // passed: struck through and muted
)

// closeInfo is a close date in words, with its urgency: "closes in 6d (soon)", "closes in 2d ▲",
// "closes today ▲", "expired 3 Sep": the ad's close date, worded apart from a closed
// (Rejected or Withdrawn) Application.
func closeInfo(closes string, now time.Time) (string, closeState) {
	if closes == "" {
		return "", closeNone
	}
	t, days, ok := schedule.DaysFrom(closes, now)
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
		return "closes " + r + " " + soonMark, closeSoon
	}
	return "closes " + r, closeLater
}

// closeStyle draws a close date by its urgency, starting from st (the row's own style,
// so a selected row keeps its fill) and base, the colour of an unhurried date. Weight
// carries the urgency as well as colour, so it reads under NO_COLOR and on the selection
// bar: a date within a week is bold and says (soon), one within two days bold with ▲. The
// words and the glyph carry the urgency alone, so it survives NO_COLOR and colour blindness.
func (p pal) CloseStyle(s closeState, st func(color.Color) lipgloss.Style, base color.Color) lipgloss.Style {
	switch s {
	case closeUrgent:
		return st(p.Bad).Bold(true)
	case closeSoon:
		return st(p.Warn).Bold(true)
	case closePast:
		return st(p.Dim).Strikethrough(true)
	}
	return st(base)
}
