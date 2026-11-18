package ui

import (
	"strings"
	"testing"
	"time"
)

func TestParseCloses(t *testing.T) {
	now := time.Date(2026, time.November, 17, 15, 30, 0, 0, time.UTC) // a Tuesday
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"2026-12-01", "2026-12-01", true},
		{"2026-02-30", "", false},
		{"today", "2026-11-17", true},
		{"Tomorrow", "2026-11-18", true},
		{"+7d", "2026-11-24", true},
		{"+2w", "2026-12-01", true},
		{"+0d", "2026-11-17", true},
		{"30 Nov", "2026-11-30", true},
		{"30 november", "2026-11-30", true},
		{"17 Nov", "2026-11-17", true}, // today counts
		{"16 Nov", "2027-11-16", true}, // already passed: next year
		{"3 jan", "2027-01-03", true},
		{"30/11", "2026-11-30", true},
		{"30/11/2027", "2027-11-30", true},
		{"1 Mar 2025", "2025-03-01", true},
		{"29 Feb", "2028-02-29", true}, // waits for a leap year
		{"31 Apr", "", false},
		{"32 Nov", "", false},
		{"soon", "", false},
		{"", "", false},
	} {
		got, ok := parseCloses(tc.in, now)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseCloses(%q) = %q, %v; want %q, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestValidClosesNamesTheProblem(t *testing.T) {
	now := func() time.Time { return time.Date(2026, time.November, 17, 0, 0, 0, 0, time.UTC) }
	v := validCloses(now)
	if err := v(""); err != nil {
		t.Errorf("empty is allowed: %v", err)
	}
	if err := v("+7d"); err != nil {
		t.Errorf("+7d refused: %v", err)
	}
	if err := v("2026-02-30"); err == nil || err.Error() != "Closes isn't a real date" {
		t.Errorf("2026-02-30: %v", err)
	}
	if err := v("someday"); err == nil || err.Error() != closesHelp {
		t.Errorf("someday: %v", err)
	}
	if got := echoCloses(now)("tomorrow"); got != "→ Wed 18 Nov 2026" {
		t.Errorf("echo = %q", got)
	}
}

func TestReadNextStepAt(t *testing.T) {
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	for in, want := range map[string]string{
		"2026-10-05":       "2026-10-05",
		"2026-10-05T09:00": "2026-10-05T09:00",
		"tomorrow 14:30":   "2026-10-02T14:30",
		"+1w 9:05":         "2026-10-08T09:05",
		"30 Nov 10:00":     "2026-11-30T10:00",
	} {
		if got, ok, _ := readNextStepAt(in, now); !ok || got != want {
			t.Errorf("readNextStepAt(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"soon", "tomorrow 25:00", "2026-02-30T10:00", "10:30"} {
		if got, ok, _ := readNextStepAt(in, now); ok {
			t.Errorf("readNextStepAt(%q) = %q, want unreadable", in, got)
		}
	}
}

// A weekday name is the next such day, never today, with or without a time, in both the
// Closes and the next step fields.
func TestDatesTakeWeekdayNames(t *testing.T) {
	wed := time.Date(2026, time.November, 18, 15, 30, 0, 0, time.UTC) // a Wednesday
	for in, want := range map[string]string{
		"fri":      "2026-11-20",
		"Friday":   "2026-11-20",
		"next fri": "2026-11-20",
		"thurs":    "2026-11-19",
		"tue":      "2026-11-24",
		"wed":      "2026-11-25", // today is "today"
		"sun":      "2026-11-22",
	} {
		if got, ok := parseCloses(in, wed); !ok || got != want {
			t.Errorf("parseCloses(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"fr", "fridays", "frx"} {
		if got, ok := parseCloses(in, wed); ok {
			t.Errorf("parseCloses(%q) = %q, want unreadable", in, got)
		}
	}
	if got, ok, _ := readNextStepAt("fri 9:00", wed); !ok || got != "2026-11-20T09:00" {
		t.Errorf("readNextStepAt(fri 9:00) = %q, %v", got, ok)
	}
	if got := echoNextStepAt(func() time.Time { return wed })("fri 9:00"); got != "→ Fri 20 Nov 2026 09:00" {
		t.Errorf("echo = %q", got)
	}
}

// Closes and the next step show one list of date forms, and every form in it is one the
// parsers read.
func TestDateHintIsSharedAndTrue(t *testing.T) {
	if !strings.HasPrefix(nextStepHelp, closesHelp) {
		t.Errorf("the next step's help %q does not start with Closes' %q", nextStepHelp, closesHelp)
	}
	now := time.Date(2026, time.November, 18, 0, 0, 0, 0, time.UTC)
	for _, form := range strings.Split(dateHint, " · ") {
		if _, ok := parseCloses(form, now); !ok {
			t.Errorf("the hint offers %q, which Closes refuses", form)
		}
	}
}

// A saved next step reopens as it would be typed, and reads back to the same date.
func TestNextStepTextReadsBackTheSame(t *testing.T) {
	now := time.Date(2026, time.November, 18, 10, 0, 0, 0, time.UTC)
	for at, want := range map[string]string{
		"2026-11-21T09:00": "21 Nov 09:00",
		"2026-11-21":       "21 Nov",
		"2027-01-04":       "4 Jan",
		"2026-10-01T10:30": "1 Oct 2026 10:30", // past: "1 Oct" would be next year
		"2028-03-01":       "1 Mar 2028",       // more than a year off
		"someday":          "someday",
	} {
		got := nextStepText(at, now)
		if got != want {
			t.Errorf("nextStepText(%q) = %q, want %q", at, got, want)
		}
		if at != "someday" && normaliseNextStepAt(got, now) != at {
			t.Errorf("%q reads back as %q, not %q", got, normaliseNextStepAt(got, now), at)
		}
	}
}
