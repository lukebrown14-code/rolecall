package ui

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The Closes field takes the ad's close date as the Owner would say it: YYYY-MM-DD, today,
// tomorrow, +7d or +2w, "30 Nov" or "30/11" (day first, as in Australia). A date with no
// year is its next occurrence, today counting. The form echoes the parsed date back as the
// Owner types, and the Lead is saved with the YYYY-MM-DD form.

const closesHelp = "Closes: use YYYY-MM-DD, tomorrow, +7d or 30 Nov"

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

// calendarDay is the date year-month-day, and whether it exists (30 Feb does not).
func calendarDay(year, month, d int) (time.Time, bool) {
	t := time.Date(year, time.Month(month), d, 0, 0, 0, 0, time.UTC)
	return t, t.Day() == d && int(t.Month()) == month
}

// validCloses is the Closes field's rule against today.
func validCloses(now func() time.Time) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		if _, ok, exists := readCloses(s, now()); !ok {
			if !exists {
				return errors.New("Closes isn't a real date")
			}
			return errors.New(closesHelp)
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
