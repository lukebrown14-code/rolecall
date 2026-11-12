package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// statsWeeks is the number of Monday-start weeks in the weekly chart.
const statsWeeks = 12

// statsBarWidth is the number of cells in a Unicode block bar.
const statsBarWidth = 12

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

// FunnelStep is one rung of the ever-reached funnel. DropOff is the percentage of
// the previous rung that did not reach this one; 0 for the first rung and for a
// rung that did not shrink.
type FunnelStep struct {
	Status  string
	Count   int
	DropOff int
}

// Response is the Response rate. Reached and Applied are the numerator and
// denominator; OK is false when the denominator is zero, so the screen shows "—".
type Response struct {
	Reached int
	Applied int
	Rate    int
	OK      bool
}

// Week is one Monday-start week in the weekly chart, local time.
type Week struct {
	Start time.Time
	Count int
}

// TrackStats is one row of the per-track table. Applied is the eligible Applied
// count (the same denominator as Response); Rate is Responses/Applied.
type TrackStats struct {
	Track     string
	Applied   int
	Responses int
	Rate      int
	OK        bool
}

// Stats builds the Stats value from Applications and "now". followUpDays defaults
// to the configured default; pass it explicitly when the Owner has a setting.
func Stats(apps []store.Application, now time.Time, followUpDays ...int) StatsReport {
	days := defaultFollowUpDays
	if len(followUpDays) > 0 {
		days = followUpDays[0]
	}
	loc := now.Location()

	reached := map[string]int{}
	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		for _, status := range statusesIn(a) {
			reached[status]++
		}
	}

	var report StatsReport
	prev := 0
	for i, status := range FunnelStatuses {
		n := reached[status]
		step := FunnelStep{Status: status, Count: n}
		if i > 0 {
			step.DropOff = dropOff(prev, n)
		}
		report.Funnel = append(report.Funnel, step)
		prev = n
	}
	report.Dismissed = reached[store.StatusDismissed]
	report.Rejected = reached[store.StatusRejected]
	report.Withdrawn = reached[store.StatusWithdrawn]

	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		applied, ok := appliedAt(a, loc)
		if !ok || tooNewToJudge(applied, now, days) {
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

// dropOff is the percentage of the previous rung that did not reach this one.
func dropOff(prev, next int) int {
	if prev <= 0 || next >= prev {
		return 0
	}
	return percentOf(prev-next, prev)
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
			if !ok || tooNewToJudge(applied, now, followUpDays) {
				continue
			}
			out[i].Applied++
			if everReached(a, store.StatusInterviewing) || everReached(a, store.StatusOffer) {
				out[i].Responses++
			}
		}
		out[i].OK = out[i].Applied > 0
		if out[i].OK {
			out[i].Rate = percentOf(out[i].Responses, out[i].Applied)
		}
	}
	return out
}

// statsApps is the scanned Applications after the track filter. Malformed
// Applications are dropped by Stats too; this keeps the screen's input honest.
func (m model) statsApps() []store.Application {
	var out []store.Application
	for _, a := range m.apps {
		if a.Malformed != nil {
			continue
		}
		if m.track > 0 && a.Track != m.tracks[m.track-1] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// statsHelpKeys is the keys only Stats accepts: add. The track filter and the rest are
// global keys.
func (m model) statsHelpKeys() []helpKey {
	keys := []helpKey{help(groupAction, "a", "add").at(prioCore)}
	if m.err != nil {
		keys = append(keys, help(groupNav, "esc", "clear error").at(prioPinned))
	}
	return keys
}

// statsKey handles the keys Stats accepts. Tab and row keys are inert there.
func (m model) statsKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "a":
		return m, m.openAdd()
	case "t", "T":
		m.cycleTrack(k.String() == "T")
	case "r":
		m.rescanAll()
	case "?":
		m.overlay = &helpMode{}
	case "esc":
		if m.err != nil {
			m.err = nil
		}
	case "q":
		return m, m.quit()
	}
	return m, nil
}

// statsView is the Stats screen: no tables, no cursor, no preview pane.
func (m model) statsView(p pal) string {
	w := m.viewWidth()
	bodyH := max(minBody, m.viewHeight()-chromeLines)
	report := Stats(m.statsApps(), m.now(), m.followUpDays)
	body := m.statsBody(p, report, w)
	lines := strings.Split(body, "\n")
	if len(lines) > bodyH {
		lines = lines[:bodyH]
	}
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	ruleTop, ruleBot := strings.Repeat("─", w), strings.Repeat("─", w)
	keys := m.keyLine(p, w)
	return m.topBar(p) + "\n\n" + m.viewRow(p) + "\n" + p.text(p.rule).Render(ruleTop) + "\n" +
		strings.Join(lines, "\n") + "\n" + p.text(p.rule).Render(ruleBot) + "\n" + m.statusRow(p, w, keys)
}

// statsBody renders the report top to bottom. The per-track table is hidden when
// the track filter is on one track.
func (m model) statsBody(p pal, r StatsReport, w int) string {
	var b strings.Builder
	b.WriteString(p.text(p.dim).Bold(true).Render("Funnel") + "\n")
	for i, step := range r.Funnel {
		line := fmt.Sprintf("%-13s %d", step.Status, step.Count)
		if i > 0 {
			line += fmt.Sprintf("  %d%% drop", step.DropOff)
		}
		b.WriteString(p.text(p.fg).Render(line) + "\n")
	}
	b.WriteString(p.text(p.dim).Render(fmt.Sprintf("Dismissed %d · Rejected %d · Withdrawn %d", r.Dismissed, r.Rejected, r.Withdrawn)) + "\n\n")

	b.WriteString(p.text(p.dim).Bold(true).Render("Response rate") + "\n")
	if r.Response.OK {
		b.WriteString(p.text(p.fg).Render(fmt.Sprintf("%d of %d  ·  %d%%", r.Response.Reached, r.Response.Applied, r.Response.Rate)) + "\n")
	} else {
		b.WriteString(p.text(p.fg).Render("—") + "\n")
	}
	b.WriteString(p.text(p.dim).Render(fmt.Sprintf("Applied within %d days left out", m.followUpDays)) + "\n\n")

	b.WriteString(p.text(p.dim).Bold(true).Render("Applied per week") + "\n")
	maxWeek := 0
	for _, week := range r.Weeks {
		maxWeek = max(maxWeek, week.Count)
	}
	for i, week := range r.Weeks {
		label := week.Start.Format("Mon 2 Jan")
		line := fmt.Sprintf("%-12s %s %d", label, bar(week.Count, maxWeek, statsBarWidth), week.Count)
		if i == len(r.Weeks)-1 {
			line += "  this week"
		}
		b.WriteString(p.text(p.fg).Render(line) + "\n")
	}

	if m.track == 0 {
		b.WriteString("\n" + p.text(p.dim).Bold(true).Render("By track") + "\n")
		b.WriteString(p.text(p.dim).Render(fmt.Sprintf("%-12s %8s %10s %6s", "Track", "Applied", "Responses", "Rate")) + "\n")
		for _, track := range r.Tracks {
			rate := "—"
			if track.OK {
				rate = fmt.Sprintf("%d%%", track.Rate)
			}
			b.WriteString(p.text(p.fg).Render(fmt.Sprintf("%-12s %8d %10d %6s", track.Track, track.Applied, track.Responses, rate)) + "\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// bar is a Unicode block bar of width cells, scaled to max. A non-zero count gets
// at least one block; a zero count is a dot.
func bar(count, max, width int) string {
	if count <= 0 {
		return "·" + strings.Repeat(" ", width-1)
	}
	n := count * width / max
	if n < 1 {
		n = 1
	}
	return strings.Repeat("█", n) + strings.Repeat(" ", width-n)
}
