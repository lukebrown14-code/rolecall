package ui

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The Closes field takes the ad's close date as the Owner would say it: YYYY-MM-DD, today,
// tomorrow, a weekday (fri, friday: the next one, never today), +7d or +2w, "30 Nov" or
// "30/11" (day first, as in Australia). A date with no year is its next occurrence, today
// counting. The form echoes the parsed date back as the Owner types, and the Lead is saved
// with the YYYY-MM-DD form.

// dateHint is the one list of date forms every date field shows, as its placeholder and in
// its error, so Closes and the next step never word them differently.
const dateHint = "tomorrow · fri · +7d · 30 Nov · 2026-12-01"

const closesHelp = dateHint

var (
	datePattern     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	closesOffset    = regexp.MustCompile(`^\+(\d{1,3})\s*([dw])$`)
	closesDayMonth  = regexp.MustCompile(`^(\d{1,2})[ /-]([a-z]{3,9}|\d{1,2})(?:[ /-](\d{4}))?$`)
	closesMonthName = []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}
)

// parseCloses reads a typed close date against today (now's date), returning it as
// YYYY-MM-DD. ok is false for an empty or unreadable value; exists reports a value whose shape
// was right but whose date does not exist (30 Feb), so the error can say so.
func parseCloses(s string, now time.Time) (iso string, ok bool) {
	iso, ok, _ = readCloses(s, now)
	return iso, ok
}

func readCloses(s string, now time.Time) (iso string, ok, exists bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	today := day(now)
	switch s {
	case "":
		return "", false, true
	case "today":
		return today.Format(time.DateOnly), true, true
	case "tomorrow":
		return today.AddDate(0, 0, 1).Format(time.DateOnly), true, true
	}
	if datePattern.MatchString(s) {
		if _, err := time.Parse(time.DateOnly, s); err != nil {
			return "", false, false
		}
		return s, true, true
	}
	if d, ok := weekdayAhead(s, today); ok {
		return d.Format(time.DateOnly), true, true
	}
	if m := closesOffset.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		if m[2] == "w" {
			n *= 7 //nolint:mnd // days in a week
		}
		return today.AddDate(0, 0, n).Format(time.DateOnly), true, true
	}
	m := closesDayMonth.FindStringSubmatch(s)
	if m == nil {
		return "", false, true
	}
	d, _ := strconv.Atoi(m[1])
	mon := 0
	if n, err := strconv.Atoi(m[2]); err == nil {
		mon = n
	} else {
		for i, name := range closesMonthName {
			if strings.HasPrefix(m[2], name) {
				mon = i + 1
			}
		}
	}
	if mon < 1 || mon > 12 || d < 1 {
		return "", false, true
	}
	if m[3] != "" {
		year, _ := strconv.Atoi(m[3])
		if t, valid := calendarDay(year, mon, d); valid {
			return t.Format(time.DateOnly), true, true
		}
		return "", false, false
	}
	// No year: the next occurrence, today counting; a 29 Feb waits for a leap year.
	for year := today.Year(); year <= today.Year()+8; year++ {
		if t, valid := calendarDay(year, mon, d); valid && !t.Before(today) {
			return t.Format(time.DateOnly), true, true
		}
	}
	return "", false, false
}

// weekdayAhead reads a weekday name, "fri", "friday" or "next fri", as the next such day
// after today: on a Friday, "fri" is a week away (today is "today"). Any prefix of the name
// of three letters or more counts, so "tues" and "thurs" do too.
func weekdayAhead(s string, today time.Time) (time.Time, bool) {
	s = strings.TrimPrefix(s, "next ")
	if len(s) < 3 { //nolint:mnd // the shortest weekday abbreviation
		return time.Time{}, false
	}
	for wd := time.Sunday; wd <= time.Saturday; wd++ {
		if strings.HasPrefix(strings.ToLower(wd.String()), s) {
			ahead := (int(wd) - int(today.Weekday()) + daysPerWeek) % daysPerWeek
			if ahead == 0 {
				ahead = daysPerWeek
			}
			return today.AddDate(0, 0, ahead), true
		}
	}
	return time.Time{}, false
}

// calendarDay is the date year-month-day, and whether it exists (30 Feb does not).
func calendarDay(year, month, d int) (time.Time, bool) {
	t := time.Date(year, time.Month(month), d, 0, 0, 0, 0, time.UTC)
	return t, t.Day() == d && int(t.Month()) == month
}

// validCloses is the Closes field's rule against today.
func validCloses(now func() time.Time) func(string) error {
	return dateRule("Closes isn't a real date", closesHelp, now, readCloses)
}

// dateRule is a date field's rule: blank passes; unreadable fails with notReal when the
// shape was right and help when it was not.
func dateRule(notReal, help string, now func() time.Time, read func(string, time.Time) (string, bool, bool)) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		if _, ok, exists := read(s, now()); !ok {
			if !exists {
				return errors.New(notReal)
			}
			return errors.New(help)
		}
		return nil
	}
}

// echoCloses is the parsed date, shown beside the field as the Owner types: "→ Mon 30 Nov
// 2026". It is empty while the text is not a date yet.
func echoCloses(now func() time.Time) func(string) string {
	return func(s string) string {
		iso, ok := parseCloses(s, now())
		if !ok {
			return ""
		}
		t, _ := time.Parse(time.DateOnly, iso)
		return "→ " + t.Format("Mon 2 Jan 2006")
	}
}

// normaliseCloses is what a Lead is saved with: the YYYY-MM-DD form of a typed date, or the
// text unchanged when it is not one (the field's rule has refused it by then).
func normaliseCloses(s string, now time.Time) string {
	if iso, ok := parseCloses(s, now); ok {
		return iso
	}
	return strings.TrimSpace(s)
}

// The next step's date takes the Closes forms plus an optional time: "tomorrow 14:30",
// "fri 9:00", "+3d 9:00", "30 Nov 10:30", "2026-10-04T10:30". It is saved as YYYY-MM-DD or
// YYYY-MM-DDTHH:MM, and the Edit form reopens a saved one as it would be typed
// (nextStepText).

const nextStepHelp = closesHelp + " · with a time: fri 9:00"

var nextStepTime = regexp.MustCompile(`(?i)^(.*?)(?:\s+|t)(\d{1,2}):(\d{2})$`)

// readNextStepAt splits a trailing time off s and reads the rest as a Closes date. exists is
// false for a date or time that has the right shape but is not real.
func readNextStepAt(s string, now time.Time) (at string, ok, exists bool) {
	s = strings.TrimSpace(s)
	clock := ""
	if m := nextStepTime.FindStringSubmatch(s); m != nil {
		h, _ := strconv.Atoi(m[2])
		mi, _ := strconv.Atoi(m[3])
		if h > 23 || mi > 59 { //nolint:mnd // hours and minutes
			return "", false, false
		}
		s, clock = m[1], fmt.Sprintf("T%02d:%02d", h, mi)
	}
	iso, ok, exists := readCloses(s, now)
	if !ok {
		return "", false, exists
	}
	return iso + clock, true, true
}

// validNextStepAt is the next step date field's rule against today.
func validNextStepAt(now func() time.Time) func(string) error {
	return dateRule("Next step isn't a real date or time", nextStepHelp, now, readNextStepAt)
}

// echoNextStepAt is the parsed date and time beside the field: "→ Sat 4 Oct 2026 10:30".
func echoNextStepAt(now func() time.Time) func(string) string {
	return func(s string) string {
		at, ok, _ := readNextStepAt(s, now())
		if !ok {
			return ""
		}
		layout := "Mon 2 Jan 2006"
		if len(at) > len(time.DateOnly) {
			layout += " 15:04"
		}
		t, err := time.Parse("2006-01-02T15:04", at)
		if err != nil {
			t, _ = time.Parse(time.DateOnly, at)
		}
		return "→ " + t.Format(layout)
	}
}

// nextStepText is a saved next step's date as the Owner would type it, "21 Nov 09:00", for
// the Edit form to reopen with. It reads back to the same date, so an untouched field saves
// nothing new: a date that would read as another year says its year ("1 Oct 2026"), and
// one that is not a date at all is left as saved.
func nextStepText(at string, now time.Time) string {
	clock := ""
	t, err := time.Parse("2006-01-02T15:04", at)
	if err == nil {
		clock = t.Format(" 15:04")
	} else if t, err = time.Parse(time.DateOnly, at); err != nil {
		return at
	}
	for _, layout := range []string{"2 Jan", "2 Jan 2006"} {
		if text := t.Format(layout) + clock; normaliseNextStepAt(text, now) == at {
			return text
		}
	}
	return at
}

// normaliseNextStepAt is what the next step is saved with, or s unchanged when unreadable.
func normaliseNextStepAt(s string, now time.Time) string {
	if at, ok, _ := readNextStepAt(s, now); ok {
		return at
	}
	return strings.TrimSpace(s)
}
