package ui_test

import (
	"regexp"
	"strings"
	"testing"
)

// A malformed Application is a row on its list but has no Status to count, so wherever a
// count includes or leaves it out, the difference is labelled: the group header that
// holds it says how many of its rows are unreadable; Settings › Tracking counts it apart
// from the active ones; and Stats, which leaves it out of every figure, says so.
func TestUnreadableRowsAreCountedApart(t *testing.T) {
	leads := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1"))
	if head := lineWith(leads, "◌ Leads ·"); !strings.Contains(head, "· 1 unreadable") {
		t.Errorf("the Leads header does not label its unreadable row: %q\n%s", head, leads)
	}
	// The tab, the view chip and the breadcrumb count the same rows the groups hold.
	n := regexp.MustCompile(`\[1 Leads (\d+)\]`).FindStringSubmatch(leads)
	if n == nil || !strings.Contains(leads, "[Leads "+n[1]+"]") || !strings.Contains(leads, "row 1 of "+n[1]) {
		t.Errorf("the Leads tab, view chip and breadcrumb disagree:\n%s", leads)
	}

	tracking := chipped(runKeys(t, fixtureCopy(t, nil), 160, "6", "]", "]", "]", "]"))
	// The breakdown wraps under itself beside the about pane, so the row is its two lines.
	swe, rows := "", strings.Split(tracking, "\n")
	for i, l := range rows {
		if strings.Contains(l, "14 applications") && i+1 < len(rows) {
			swe = l + " " + rows[i+1]
		}
	}
	if !strings.Contains(swe, "14 applications") || !strings.Contains(swe, "1 unreadable") ||
		!strings.Contains(swe, "9 active") {
		t.Errorf("Settings › Tracking SWE = %q, want 14 applications · 9 active … · 1 unreadable", swe)
	}

	stats := chipped(runKeysSized(t, fixtureCopy(t, nil), 160, 44, "4"))
	if !strings.Contains(stats, "SWE 13") || !strings.Contains(lineWith(stats, "NOT COUNTED"), "1 unreadable Application") {
		t.Errorf("Stats does not say it leaves the unreadable Application out:\n%s", stats)
	}
}

// Each Stats figure says what it counts: the funnel is what ever reached a Status, the
// By track table's Applied is the funnel's and its Counted the Response rate's.
func TestStatsNumbersSayWhatTheyCount(t *testing.T) {
	stats := strings.Join(strings.Fields(chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 60, "4"))), " ")
	for _, want := range []string{
		"Ever reached each Status; tabs count where they are now",
		"Applied: ever reached Applied, as in the funnel.",
		"Counted: applied at least 14 days ago",
		"A Response is an Application that reached Interviewing or Offer.",
	} {
		if !strings.Contains(stats, want) {
			t.Errorf("Stats lacks %q:\n%s", want, stats)
		}
	}
}
