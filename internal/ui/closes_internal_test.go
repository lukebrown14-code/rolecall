package ui

import (
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
