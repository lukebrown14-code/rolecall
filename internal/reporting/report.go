// Package reporting calculates application statistics independently of terminal rendering.
package reporting

import (
	"math"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// statsWeeks is the number of Monday-start weeks in the weekly chart.
const (
	statsWeeks          = 12
	daysPerWeek         = 7
	defaultFollowUpDays = 14
)

// FunnelStatuses are the Statuses the funnel counts, in pipeline order.
var FunnelStatuses = []string{store.StatusLead, store.StatusDraft, store.StatusApplied, store.StatusInterviewing, store.StatusOffer}

// StatsReport is the pure Stats value. It is built from the Applications after the
// track filter and a fixed "now"; the screen only renders it.
type StatsReport struct {
	Funnel    []FunnelStep
	Dismissed int
	Rejected  int
	Withdrawn int
	Response  Response
	Weeks     []Week
	Tracks    []TrackStats
}

// FunnelStep is one rung of the ever-reached funnel. Because a rung counts every
// Application that ever reached its Status, a rung can be higher than the one before:
// Applications may start at any Status. Through is how many reached both the previous
// rung's Status and this one; Skipped is how many reached this one without the previous.
// Conversion is Through as a percentage of the previous rung, OK only when that rung is
// not empty. All are zero on the first rung.
type FunnelStep struct {
	Status       string
	Count        int
	Through      int
	Skipped      int
	Conversion   int
	ConversionOK bool
}

// Response is the Response rate. Reached and Applied are the numerator and
// denominator; OK is false when the denominator is zero, so the screen shows "—".
// TooNew is the Applications left out because they were applied inside the follow-up
// window.
type Response struct {
	Reached int
	Applied int
	Rate    int
	OK      bool
	TooNew  int
}

// Week is one Monday-start week in the weekly chart, local time.
type Week struct {
	Start time.Time
	Count int
}

// TrackStats is one row of the per-track table. Applied counts Applications that ever
// reached Applied, as the funnel does, so the tracks sum to the funnel's Applied rung.
// Judged is those applied outside the follow-up window (the Response denominator);
// Rate is Responses/Judged.
type TrackStats struct {
	Track     string
	Applied   int
	Judged    int
	Responses int
	Rate      int
	OK        bool
}

// Build builds the Stats value from Applications and "now". followUpDays defaults
// to the configured default; pass it explicitly when the Owner has a setting.
func Build(apps []store.Application, now time.Time, followUpDays ...int) StatsReport {
	days := defaultFollowUpDays
	if len(followUpDays) > 0 {
		days = followUpDays[0]
	}
	loc := now.Location()

	reached := map[string]int{}
	var report StatsReport
	report.Funnel = make([]FunnelStep, len(FunnelStatuses))
	for i, status := range FunnelStatuses {
		report.Funnel[i].Status = status
	}
	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		for _, status := range statusesIn(a) {
			reached[status]++
		}
		for i := 1; i < len(FunnelStatuses); i++ {
			if !everReached(a, FunnelStatuses[i]) {
				continue
			}
			if everReached(a, FunnelStatuses[i-1]) {
				report.Funnel[i].Through++
			} else {
				report.Funnel[i].Skipped++
			}
		}
	}
	for i := range report.Funnel {
		step := &report.Funnel[i]
		step.Count = reached[step.Status]
		if prev := i - 1; prev >= 0 && report.Funnel[prev].Count > 0 {
			step.Conversion = percentOf(step.Through, report.Funnel[prev].Count)
			step.ConversionOK = true
		}
	}
	report.Dismissed = reached[store.StatusDismissed]
	report.Rejected = reached[store.StatusRejected]
	report.Withdrawn = reached[store.StatusWithdrawn]

	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		applied, ok := appliedAt(a, loc)
		if !ok {
			continue
		}
		if tooNewToJudge(applied, now, days) {
			report.Response.TooNew++
			continue
		}
		report.Response.Applied++
		if everReached(a, store.StatusInterviewing) || everReached(a, store.StatusOffer) {
			report.Response.Reached++
		}
	}
	report.Response.OK = report.Response.Applied > 0
	if report.Response.OK {
		report.Response.Rate = percentOf(report.Response.Reached, report.Response.Applied)
	}

	report.Weeks = weeksOf(apps, now, statsWeeks)
	report.Tracks = tracksOf(apps, now, days)
	return report
}

// statusesIn returns the distinct Statuses a StatusLog contains, in any order.
func statusesIn(a store.Application) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range a.StatusLog {
		if !seen[e.Status] {
			seen[e.Status] = true
			out = append(out, e.Status)
		}
	}
	return out
}

func everReached(a store.Application, status string) bool {
	for _, e := range a.StatusLog {
		if e.Status == status {
			return true
		}
	}
	return false
}

// appliedAt is the first Applied entry's date in loc, or false when the Application
// never reached Applied.
func appliedAt(a store.Application, loc *time.Location) (time.Time, bool) {
	for _, e := range a.StatusLog {
		if e.Status == store.StatusApplied {
			if t, ok := parseLocal(e.At, loc); ok {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// parseLocal reads a date-only or RFC3339 timestamp in loc. The zone comes from the
// caller's "now", not the process-global time.Local, so the Stats value is a pure
// function of its inputs.
func parseLocal(s string, loc *time.Location) (time.Time, bool) {
	if t, err := time.ParseInLocation("2006-01-02", s, loc); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(loc), true
	}
	return time.Time{}, false
}

// tooNewToJudge reports whether an Applied date is still inside the follow-up window,
// so there has not been time for a Response. Exactly followUpDays old is outside it.
func tooNewToJudge(applied, now time.Time, followUpDays int) bool {
	return applied.After(now.AddDate(0, 0, -followUpDays))
}

// percent rounds n/d to a whole percentage.
func percentOf(n, d int) int {
	if d <= 0 {
		return 0
	}
	return int(math.Round(float64(n) * 100 / float64(d)))
}

// weekStart is the Monday 00:00 in loc at the start of t's week.
func weekStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	day := (int(t.Weekday()) - int(time.Monday) + daysPerWeek) % daysPerWeek // Monday = 0
	t = t.AddDate(0, 0, -day)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// weeksOf returns the last n Monday-start weeks, current week last, with Applied
// counts. An Application outside the window is ignored.
func weeksOf(apps []store.Application, now time.Time, n int) []Week {
	loc := now.Location()
	current := weekStart(now, loc)
	out := make([]Week, n)
	index := make(map[time.Time]int, n)
	for i := range out {
		start := current.AddDate(0, 0, -(n-1-i)*daysPerWeek)
		out[i] = Week{Start: start}
		index[start] = i
	}
	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		applied, ok := appliedAt(a, loc)
		if !ok {
			continue
		}
		if i, ok := index[weekStart(applied, loc)]; ok {
			out[i].Count++
		}
	}
	return out
}

// tracksOf returns one row per track in first-appearance order, with eligible
// Applied and Response counts.
func tracksOf(apps []store.Application, now time.Time, followUpDays int) []TrackStats {
	loc := now.Location()
	var order []string
	seen := map[string]bool{}
	for _, a := range apps {
		if a.Malformed == nil && !seen[a.Track] {
			seen[a.Track] = true
			order = append(order, a.Track)
		}
	}
	out := make([]TrackStats, len(order))
	for i, track := range order {
		out[i].Track = track
		for _, a := range apps {
			if a.Malformed != nil || a.Track != track {
				continue
			}
			applied, ok := appliedAt(a, loc)
			if !ok {
				continue
			}
			out[i].Applied++
			if tooNewToJudge(applied, now, followUpDays) {
				continue
			}
			out[i].Judged++
			if everReached(a, store.StatusInterviewing) || everReached(a, store.StatusOffer) {
				out[i].Responses++
			}
		}
		out[i].OK = out[i].Judged > 0
		if out[i].OK {
			out[i].Rate = percentOf(out[i].Responses, out[i].Judged)
		}
	}
	return out
}
