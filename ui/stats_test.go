package ui_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

// statsApp builds a fixture-like Application with a status history.
func statsApp(folder, track string, entries ...store.StatusEntry) store.Application {
	return store.Application{Folder: folder, Track: track, Status: entries[len(entries)-1].Status, StatusLog: entries}
}

func at(status string, t time.Time) store.StatusEntry {
	return store.StatusEntry{Status: status, At: t.Format(time.RFC3339)}
}

var errTest = errors.New("malformed")

func TestStatsFunnelOverFixtures(t *testing.T) {
	root := fixtureCopy(t, nil)
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	got := ui.Stats(apps, fixtureNow)

	want := []struct {
		status string
		count  int
	}{
		{store.StatusLead, 2},
		{store.StatusDraft, 2},
		{store.StatusApplied, 7},
		{store.StatusInterviewing, 2},
		{store.StatusOffer, 2},
	}
	if len(got.Funnel) != len(want) {
		t.Fatalf("funnel has %d steps, want %d: %+v", len(got.Funnel), len(want), got.Funnel)
	}
	for i, w := range want {
		if got.Funnel[i].Status != w.status || got.Funnel[i].Count != w.count {
			t.Errorf("funnel step %d = %+v, want %s %d", i, got.Funnel[i], w.status, w.count)
		}
	}
	if got.Dismissed != 2 || got.Rejected != 2 || got.Withdrawn != 2 {
		t.Errorf("side counts = %d/%d/%d, want 2/2/2", got.Dismissed, got.Rejected, got.Withdrawn)
	}
	if !got.Response.OK || got.Response.Applied != 6 || got.Response.Reached != 2 || got.Response.Rate != 33 {
		t.Errorf("response = %+v, want 2/6 at 33%%", got.Response)
	}
	if len(got.Weeks) != 12 {
		t.Fatalf("weeks = %d, want 12", len(got.Weeks))
	}
	if got.Weeks[10].Count != 1 {
		t.Errorf("week starting 21 Sep = %+v, want 1", got.Weeks[10])
	}
	if len(got.Tracks) != 3 || got.Tracks[0].Track != "SWE" || got.Tracks[1].Track != "Platform" || got.Tracks[2].Track != "Unknown" {
		t.Fatalf("tracks = %+v, want SWE, Platform, Unknown", got.Tracks)
	}
	// SWE's one Applied (orbit, 25 Sep) is too new to judge: it counts as Applied, as
	// in the funnel, but not toward its Response rate.
	if got.Tracks[0].Applied != 1 || got.Tracks[0].Judged != 0 || got.Tracks[0].OK {
		t.Errorf("SWE = %+v, want 1 applied, none judged", got.Tracks[0])
	}
	if got.Tracks[1].Applied != 0 || got.Tracks[1].OK {
		t.Errorf("Platform = %+v, want no applied", got.Tracks[1])
	}
	if got.Tracks[2].Applied != 6 || got.Tracks[2].Judged != 6 || got.Tracks[2].Responses != 2 || got.Tracks[2].Rate != 33 {
		t.Errorf("Unknown = %+v, want 6/2 at 33%%", got.Tracks[2])
	}
	applied := 0
	for _, tr := range got.Tracks {
		applied += tr.Applied
	}
	if applied != got.Funnel[2].Count {
		t.Errorf("tracks sum to %d Applied, funnel says %d", applied, got.Funnel[2].Count)
	}
	if got.Response.TooNew != 1 {
		t.Errorf("too new = %d, want 1 (orbit)", got.Response.TooNew)
	}
}

func TestResponseExcludesFollowUpWindow(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	apps := []store.Application{
		statsApp("old", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -15)), at(store.StatusInterviewing, now)),
		statsApp("boundary", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -14)), at(store.StatusInterviewing, now)),
		statsApp("new", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -13)), at(store.StatusInterviewing, now)),
	}
	got := ui.Stats(apps, now, 14)
	if !got.Response.OK || got.Response.Applied != 2 || got.Response.Reached != 2 || got.Response.Rate != 100 {
		t.Fatalf("response = %+v, want 2/2 at 100%% (14 days old is outside the window)", got.Response)
	}
}

func TestRejectedWithoutInterviewIsNotAResponse(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	old := now.AddDate(0, 0, -40)
	apps := []store.Application{
		statsApp("rejected", "SWE", at(store.StatusApplied, old), at(store.StatusRejected, now)),
		statsApp("interviewed", "SWE", at(store.StatusApplied, old), at(store.StatusInterviewing, now), at(store.StatusRejected, now)),
	}
	got := ui.Stats(apps, now, 14)
	if got.Response.Applied != 2 || got.Response.Reached != 1 || got.Response.Rate != 50 {
		t.Fatalf("response = %+v, want 1/2 at 50%%", got.Response)
	}
}

func TestStatsWeeksStartOnMonday(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	apps := []store.Application{
		statsApp("sunday", "SWE", at(store.StatusApplied, time.Date(2026, 9, 27, 23, 0, 0, 0, time.Local))),
		statsApp("monday", "SWE", at(store.StatusApplied, time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local))),
	}
	got := ui.Stats(apps, now, 14)
	byStart := map[string]int{}
	for _, w := range got.Weeks {
		byStart[w.Start.Format("2006-01-02")] = w.Count
	}
	if byStart["2026-09-21"] != 1 || byStart["2026-09-28"] != 1 {
		t.Fatalf("week buckets = %v, want Sunday 27 Sep in 21 Sep and Monday 28 Sep in 28 Sep", byStart)
	}
}

func TestStatsUsesLocalTimeForTheWeek(t *testing.T) {
	// The zone comes from "now", so the test never mutates the process-global
	// time.Local (which would race with the runtime timer goroutine under -race).
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.FixedZone("Test", -7*60*60))
	apps := []store.Application{statsApp("edge", "SWE", store.StatusEntry{Status: store.StatusApplied, At: "2026-09-25T01:00:00Z"})}
	got := ui.Stats(apps, now, 14)
	if len(got.Weeks) != 12 {
		t.Fatalf("weeks = %d", len(got.Weeks))
	}
	// 01:00Z is 18:00 on 24 Sep in the test zone, so it belongs to the 21 Sep week.
	if got.Weeks[11].Count != 0 {
		t.Errorf("current week = %+v, want 0", got.Weeks[11])
	}
	if got.Weeks[10].Count != 1 {
		t.Errorf("week starting 21 Sep = %+v, want 1", got.Weeks[10])
	}
}

func TestStatsLeavesMalformedOut(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	good := statsApp("good", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -40)))
	bad := statsApp("bad", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -40)))
	bad.Malformed = errTest
	got := ui.Stats([]store.Application{good, bad}, now, 14)
	if got.Funnel[2].Count != 1 || got.Response.Applied != 1 {
		t.Fatalf("malformed counted: %+v", got)
	}
	if got.Tracks[0].Applied != 1 {
		t.Fatalf("malformed counted in track: %+v", got.Tracks[0])
	}
}

func TestStatsPerTrackSplit(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	old := now.AddDate(0, 0, -40)
	apps := []store.Application{
		statsApp("swe", "SWE", at(store.StatusApplied, old), at(store.StatusInterviewing, old)),
		statsApp("platform", "Platform", at(store.StatusApplied, old), at(store.StatusRejected, old)),
	}
	got := ui.Stats(apps, now, 14)
	if len(got.Tracks) != 2 {
		t.Fatalf("tracks = %+v", got.Tracks)
	}
	if got.Tracks[0].Track != "SWE" || got.Tracks[0].Applied != 1 || got.Tracks[0].Responses != 1 || got.Tracks[0].Rate != 100 {
		t.Errorf("SWE = %+v", got.Tracks[0])
	}
	if got.Tracks[1].Track != "Platform" || got.Tracks[1].Applied != 1 || got.Tracks[1].Responses != 0 || got.Tracks[1].Rate != 0 {
		t.Errorf("Platform = %+v", got.Tracks[1])
	}
}

func TestStatsEmptyResponseIsDash(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	got := ui.Stats(nil, now, 14)
	if got.Response.OK || got.Response.Applied != 0 {
		t.Fatalf("response = %+v, want no denominator", got.Response)
	}
}
