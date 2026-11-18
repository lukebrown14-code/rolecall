package ui_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// The Stats funnel counts the Applications that ever reached each Status; a tab counts
// the ones at that Status now. On the fixture Workspace both read what the files say
// under those definitions, so the funnel is never below the tab, and Closed is Rejected
// plus Withdrawn on both screens, with Dismissed never in it.
func TestStatsFunnelAndTabCountsAgree(t *testing.T) {
	root := fixtureCopy(t, nil)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	everReached, now := map[string]int{}, map[string]int{}
	for _, a := range apps {
		_, table := ui.StatusLocation(a.Status)
		if a.Filtered() {
			table = "Filtered"
		}
		now[table]++
		if a.Malformed != nil {
			continue // Stats leaves an unreadable Application out of every figure
		}
		for _, st := range uniq(reporting.Reached(a)) {
			everReached[st]++
		}
	}

	stats := chipped(runKeysSized(t, root, 120, 40, "4"))
	if !strings.Contains(stats, "Ever reached each Status; tabs count where they are now") {
		t.Fatalf("the funnel does not say what it counts:\n%s", stats)
	}
	tabs := map[string]int{}
	for _, screen := range []string{"1", "2", "3"} {
		view := chipped(runKeysSized(t, root, 120, 40, screen))
		for name, n := range viewCounts(lineWith(view, "VIEW s")) {
			tabs[name] = n
		}
	}

	for _, status := range reporting.FunnelStatuses {
		table := status
		switch status {
		case store.StatusLead:
			table = "Leads"
		case store.StatusDraft:
			table = "Drafts"
		}
		funnel := count(t, stats, regexp.QuoteMeta(status)+`\s+[█░#.]+\s+(\d+)`)
		if funnel != everReached[status] {
			t.Errorf("funnel %s = %d, want %d that ever reached it", status, funnel, everReached[status])
		}
		if tabs[table] != now[table] {
			t.Errorf("tab %s = %d, want %d at it now", table, tabs[table], now[table])
		}
		if funnel < tabs[table]-malformedAt(apps, table) {
			t.Errorf("funnel %s = %d is below the %d at it now", status, funnel, tabs[table])
		}
	}

	closedNow := 0
	for _, a := range apps {
		if a.Status == store.StatusRejected || a.Status == store.StatusWithdrawn {
			closedNow++
		}
	}
	if tabs["Closed"] != closedNow {
		t.Errorf("tab Closed = %d, want %d Rejected or Withdrawn now (Dismissed is never Closed)", tabs["Closed"], closedNow)
	}
	closedEver := everReached[store.StatusRejected] + everReached[store.StatusWithdrawn]
	if got := count(t, stats, `Closed (\d+): `); got != closedEver || got < closedNow {
		t.Errorf("Stats Closed = %d, want %d (Rejected plus Withdrawn, ever reached)", got, closedEver)
	}
	if got := count(t, stats, `Dismissed (\d+), let go before applying`); got != everReached[store.StatusDismissed] {
		t.Errorf("Stats Dismissed = %d, want %d on its own line", got, everReached[store.StatusDismissed])
	}
}

// viewCounts reads a VIEW row, "VIEW s [Leads 7] Dismissed 2 …", into its counts.
func viewCounts(row string) map[string]int {
	out := map[string]int{}
	row, _, _ = strings.Cut(row, "│")
	for _, m := range regexp.MustCompile(`([A-Z][a-z]+) (\d+)`).FindAllStringSubmatch(row, -1) {
		n, _ := strconv.Atoi(m[2])
		out[m[1]] = n
	}
	return out
}

func count(t *testing.T, view, pattern string) int {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(view)
	if m == nil {
		t.Fatalf("no %q in:\n%s", pattern, view)
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

func uniq(statuses []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range statuses {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// malformedAt is how many unreadable Applications a tab lists, which Stats leaves out.
func malformedAt(apps []store.Application, table string) int {
	n := 0
	for _, a := range apps {
		if _, t := ui.StatusLocation(a.Status); a.Malformed != nil && t == table {
			n++
		}
	}
	return n
}
