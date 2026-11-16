package reporting_test

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/reporting"
)

// The per-track table lists the busiest Track first, ties in workspace.json order and
// off-list Tracks after; the Tracks with nothing applied are kept apart, in that order.
func TestOrderTracksBusiestFirst(t *testing.T) {
	rows := []reporting.TrackStats{
		{Track: "Unknown", Applied: 2}, {Track: "Platform"}, {Track: "SWE", Applied: 2}, {Track: "Data", Applied: 5}, {Track: "Legacy"}, {Track: "Analyst"},
	}
	sent, idle := reporting.OrderTracks(rows, []string{"Analyst", "SWE", "Platform", "Data"})
	names := func(rs []reporting.TrackStats) []string {
		var out []string
		for _, r := range rs {
			out = append(out, r.Track)
		}
		return out
	}
	if got, want := names(sent), []string{"Data", "SWE", "Unknown"}; !equal(got, want) {
		t.Errorf("sent = %v, want %v", got, want)
	}
	if got, want := names(idle), []string{"Analyst", "Platform", "Legacy"}; !equal(got, want) {
		t.Errorf("idle = %v, want %v", got, want)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
