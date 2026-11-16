package reporting_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
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
	root := "../../../fixtures/workspace"
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	got := reporting.Build(apps, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))

	// Hand-counted from fixtures/workspace/jobs/*/meta.yml (16 well-formed, malformed left
	// out). A later Status implies Applied (and an Offer, Interviewing), so these match
	// the screens:
	//   Lead: fern, orbit, empty, empty-log (an empty log is Lead)              = 4
	//   Draft: docs-none, status-draft                                          = 2
	//   Applied: orbit, the six status-* legacy rows, and docs-partial (Offer),
	//     docs-ready (Rejected), docs-resume-only (Withdrawn), docs-written
	//     (Interviewing), whose Applied is implied                              = 11
	//   Interviewing: docs-written, status-interviewing, plus docs-partial and
	//     status-offer, whose Offer implies it                                  = 4
	//   Offer: docs-partial, status-offer                                       = 2
	want := []struct {
		status           string
		count, through   int
		skipped, percent int
	}{
		{store.StatusLead, 4, 0, 0, 0},
		// Neither Draft was a Lead first.
		{store.StatusDraft, 2, 0, 2, 0},
		// status-draft went Applied then Draft, so its Draft came after: no Applied came
		// from a Draft.
		{store.StatusApplied, 11, 0, 11, 0},
		{store.StatusInterviewing, 4, 4, 0, 36},
		{store.StatusOffer, 2, 2, 0, 50},
	}
	if len(got.Funnel) != len(want) {
		t.Fatalf("funnel has %d steps, want %d: %+v", len(got.Funnel), len(want), got.Funnel)
	}
	for i, w := range want {
		s := got.Funnel[i]
		if s.Status != w.status || s.Count != w.count || s.Through != w.through || s.Skipped != w.skipped || s.Conversion != w.percent {
			t.Errorf("funnel step %d = %+v, want %+v", i, s, w)
		}
	}
	if got.Dismissed != 3 || got.Rejected != 2 || got.Withdrawn != 2 {
		t.Errorf("side counts = %d/%d/%d, want 3/2/2", got.Dismissed, got.Rejected, got.Withdrawn)
	}
	// The six legacy rows were applied in 2025, so they are judged; status-interviewing
	// and status-offer are Responses. orbit (25 Sep) and the four docs-* rows (24 Sep)
	// are inside the 14-day window.
	if !got.Response.OK || got.Response.Applied != 6 || got.Response.Reached != 2 || got.Response.Rate != 33 {
		t.Errorf("response = %+v, want 2/6 at 33%%", got.Response)
	}
	if got.Response.TooNew != 5 {
		t.Errorf("too new = %d, want 5 (orbit and four docs-* rows)", got.Response.TooNew)
	}
	if len(got.Weeks) != 12 {
		t.Fatalf("weeks = %d, want 12", len(got.Weeks))
	}
	// Only orbit has a logged Applied in range; an implied Applied has no date of its own.
	if got.Weeks[10].Count != 1 {
		t.Errorf("week starting 21 Sep = %+v, want 1", got.Weeks[10])
	}
	if len(got.Tracks) != 3 || got.Tracks[0].Track != "SWE" || got.Tracks[1].Track != "Platform" || got.Tracks[2].Track != "Unknown" {
		t.Fatalf("tracks = %+v, want SWE, Platform, Unknown", got.Tracks)
	}
	// SWE's five sent (orbit and four docs-*) are all too new to judge, as on the Applied
	// screen's SWE count.
	if tr := got.Tracks[0]; tr.Applied != 5 || tr.Judged != 0 || tr.OK {
		t.Errorf("SWE = %+v, want 5 applied, none judged", tr)
	}
	if got.Tracks[1].Applied != 0 || got.Tracks[1].OK {
		t.Errorf("Platform = %+v, want no applied", got.Tracks[1])
	}
	if tr := got.Tracks[2]; tr.Applied != 6 || tr.Judged != 6 || tr.Responses != 2 || tr.Rate != 33 {
		t.Errorf("Unknown = %+v, want 6/2 at 33%%", tr)
	}
	applied := 0
	for _, tr := range got.Tracks {
		applied += tr.Applied
	}
	if applied != got.Funnel[2].Count {
		t.Errorf("tracks sum to %d Applied, funnel says %d", applied, got.Funnel[2].Count)
	}
}

func TestResponseExcludesFollowUpWindow(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	apps := []store.Application{
		statsApp("old", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -15)), at(store.StatusInterviewing, now)),
		statsApp("boundary", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -14)), at(store.StatusInterviewing, now)),
		statsApp("new", "SWE", at(store.StatusApplied, now.AddDate(0, 0, -13)), at(store.StatusInterviewing, now)),
	}
	got := reporting.Build(apps, now, 14)
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
	got := reporting.Build(apps, now, 14)
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
	got := reporting.Build(apps, now, 14)
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
	got := reporting.Build(apps, now, 14)
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
	got := reporting.Build([]store.Application{good, bad}, now, 14)
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
	got := reporting.Build(apps, now, 14)
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
	got := reporting.Build(nil, now, 14)
	if got.Response.OK || got.Response.Applied != 0 {
		t.Fatalf("response = %+v, want no denominator", got.Response)
	}
}

// A rung counts every Application that ever reached it, so it can rise: Applications
// that started later in the pipeline are counted as skipping the rung before, and the
// conversion counts only those that reached both.
func TestStatsFunnelThroughAndSkipped(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	old := now.AddDate(0, 0, -40)
	apps := []store.Application{
		statsApp("drafted", "SWE", at(store.StatusDraft, old), at(store.StatusApplied, old)),
		statsApp("straight", "SWE", at(store.StatusApplied, old)),
		statsApp("again", "SWE", at(store.StatusApplied, old), at(store.StatusInterviewing, old)),
		statsApp("stuck", "SWE", at(store.StatusDraft, old)),
	}
	got := reporting.Build(apps, now, 14)
	applied, interviewing := got.Funnel[2], got.Funnel[3]
	if applied.Count != 3 || applied.Through != 1 || applied.Skipped != 2 || !applied.ConversionOK || applied.Conversion != 50 {
		t.Errorf("Applied = %+v, want 3 with 1 from Draft (50%%) and 2 skipped", applied)
	}
	if interviewing.Count != 1 || interviewing.Through != 1 || interviewing.Skipped != 0 || interviewing.Conversion != 33 {
		t.Errorf("Interviewing = %+v, want 1 of 3 Applied (33%%)", interviewing)
	}
	if lead, draft := got.Funnel[0], got.Funnel[1]; lead.ConversionOK || draft.ConversionOK || draft.Skipped != 2 {
		t.Errorf("Lead = %+v, Draft = %+v: want no conversion from an empty rung and 2 skipped Lead", lead, draft)
	}
}

// A move back is counted in log order: an Application that went Applied then Draft did
// not reach Applied from a Draft.
func TestStatsFunnelRespectsLogOrder(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	old := now.AddDate(0, 0, -40)
	apps := []store.Application{
		statsApp("back", "SWE", at(store.StatusApplied, old), at(store.StatusDraft, old.AddDate(0, 0, 1))),
		statsApp("forward", "SWE", at(store.StatusDraft, old), at(store.StatusApplied, old.AddDate(0, 0, 1))),
	}
	applied := reporting.Build(apps, now, 14).Funnel[2]
	if applied.Count != 2 || applied.Through != 1 || applied.Skipped != 1 || applied.Conversion != 50 {
		t.Fatalf("Applied = %+v, want 2 with 1 from Draft (50%%) and 1 not a Draft first", applied)
	}
}

// A later Status implies the earlier ones it cannot happen without, so an Application
// whose log starts at Interviewing, Offer, Rejected or Withdrawn counts as Applied, and
// an Offer as Interviewing too.
func TestStatsLaterStatusImpliesApplied(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	old := now.AddDate(0, 0, -40)
	apps := []store.Application{
		statsApp("interviewing", "SWE", at(store.StatusInterviewing, old)),
		statsApp("offer", "SWE", at(store.StatusDraft, old), at(store.StatusOffer, old)),
		statsApp("rejected", "SWE", at(store.StatusRejected, old)),
		statsApp("withdrawn", "SWE", at(store.StatusWithdrawn, old)),
	}
	got := reporting.Build(apps, now, 14)
	if applied := got.Funnel[2]; applied.Count != 4 || applied.Through != 1 || applied.Skipped != 3 {
		t.Errorf("Applied = %+v, want 4, one from Draft", applied)
	}
	if iv := got.Funnel[3]; iv.Count != 2 || iv.Through != 2 || iv.Conversion != 50 {
		t.Errorf("Interviewing = %+v, want 2 of 4 Applied", iv)
	}
	if offer := got.Funnel[4]; offer.Count != 1 || offer.Through != 1 || offer.Skipped != 0 {
		t.Errorf("Offer = %+v, want 1 through Interviewing", offer)
	}
	if got.Response.Applied != 4 || got.Response.Reached != 2 || got.Tracks[0].Applied != 4 {
		t.Errorf("response = %+v, track = %+v, want 2 of 4", got.Response, got.Tracks[0])
	}
	if got := reporting.Reached(apps[1]); strings.Join(got, ",") != "Draft,Applied,Interviewing,Offer" {
		t.Errorf("Reached(offer) = %v", got)
	}
	// An implied Applied takes the implying Status's date for the Response window, but
	// has no week of its own.
	recent := reporting.Build([]store.Application{statsApp("new", "SWE", at(store.StatusInterviewing, now.AddDate(0, 0, -2)))}, now, 14)
	if recent.Response.TooNew != 1 || recent.Weeks[len(recent.Weeks)-1].Count != 0 {
		t.Errorf("recent implied Applied: response %+v, weeks %+v", recent.Response, recent.Weeks)
	}
}

// An undone move does not count: undo appends the earlier Status again within moments,
// and Stats reads that pair as never having happened. A move back days later is a real
// move.
func TestStatsLeavesUndoneMovesOut(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	old := now.AddDate(0, 0, -40)
	apps := []store.Application{
		// m a y u on a Draft: Applied, then Draft again a few seconds later.
		statsApp("undone", "SWE", at(store.StatusDraft, old), at(store.StatusApplied, now), at(store.StatusDraft, now.Add(5*time.Second))),
		// Two moves undone in turn.
		statsApp("twice", "SWE", at(store.StatusApplied, old), at(store.StatusInterviewing, now), at(store.StatusOffer, now.Add(time.Second)),
			at(store.StatusInterviewing, now.Add(2*time.Second)), at(store.StatusApplied, now.Add(3*time.Second))),
		// An empty log's first move undone back to Lead.
		statsApp("first", "SWE", at(store.StatusApplied, now), at(store.StatusLead, now.Add(time.Second))),
		// A deliberate move back, days later, still counts.
		statsApp("back", "SWE", at(store.StatusDraft, old), at(store.StatusApplied, old), at(store.StatusDraft, old.AddDate(0, 0, 3))),
	}
	got := reporting.Build(apps, now, 14)
	if applied := got.Funnel[2]; applied.Count != 2 {
		t.Errorf("Applied = %+v, want 2 (twice, back)", applied)
	}
	if got.Funnel[3].Count != 0 || got.Funnel[4].Count != 0 {
		t.Errorf("undone Interviewing/Offer counted: %+v", got.Funnel)
	}
	if got.Funnel[0].Count != 1 {
		t.Errorf("Lead = %+v, want 1 (first, undone back to Lead)", got.Funnel[0])
	}
	if last := got.Weeks[len(got.Weeks)-1]; last.Count != 0 {
		t.Errorf("this week = %+v, want 0: every Applied this week was undone", last)
	}
	if log := reporting.EffectiveLog(apps[0].StatusLog); len(log) != 1 || log[0].Status != store.StatusDraft {
		t.Errorf("EffectiveLog(undone) = %+v, want the first Draft only", log)
	}
}

// The tab bar draws the same Applied series as Stats, so the two never disagree.
func TestAppliedWeeksMatchesStats(t *testing.T) {
	s, err := store.Open("../../../fixtures/workspace")
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	stats := reporting.Build(apps, now).Weeks
	got := reporting.AppliedWeeks(apps, now, 8)
	if len(got) != 8 {
		t.Fatalf("got %d weeks, want 8", len(got))
	}
	for i, w := range got {
		if want := stats[len(stats)-8+i]; w != want {
			t.Fatalf("week %d = %+v, Stats has %+v", i, w, want)
		}
	}
}
