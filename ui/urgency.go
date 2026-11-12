package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// Dates: relative while recent, a month and year once older than monthYearAfter days.
const (
	monthYearAfter = 90 // "~21mo ago" reads as "Jan 2025" past about three months
	urgentDays     = 2  // "closes in 2d ▲" turns danger at or below this
	urgentMark     = "▲"
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

// span is a distance in days as the list words it: "4d", "3w", "2mo".
func span(n int) string {
	switch {
	case n < weeksUntil:
		return fmt.Sprintf("%dd", n)
	case n < monthsUntil:
		return fmt.Sprintf("%dw", n/daysPerWeek)
	}
	return fmt.Sprintf("%dmo", n/daysPerMonth)
}

// daysFrom is the whole days from today to date (negative in the past), and whether
// date could be read.
func daysFrom(date string, now time.Time) (time.Time, int, bool) {
	t, err := time.Parse("2006-01-02", strings.Split(date, "T")[0])
	if err != nil {
		return time.Time{}, 0, false
	}
	today, _ := time.Parse("2006-01-02", now.Format("2006-01-02"))
	return t, int(t.Sub(today).Hours() / hoursPerDay), true
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
// "closes today ▲", "closed 3 Sep".
func closeInfo(closes string, now time.Time) (string, closeState) {
	if closes == "" {
		return "", closeNone
	}
	t, days, ok := daysFrom(closes, now)
	switch {
	case !ok:
		return "closes " + closes, closeLater
	case days < 0:
		return "closed " + shortDate(t, now), closePast
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
func (p pal) closeStyle(s closeState, st func(color.Color) lipgloss.Style, base color.Color) lipgloss.Style {
	switch s {
	case closeUrgent:
		return st(p.bad).Bold(true)
	case closeSoon:
		return st(p.warn)
	case closePast:
		return st(p.dim).Strikethrough(true)
	}
	return st(base)
}
