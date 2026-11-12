package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// statsWeeks is the number of Monday-start weeks in the weekly chart.
const statsWeeks = 12

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

// Stats builds the Stats value from Applications and "now". followUpDays defaults
// to the configured default; pass it explicitly when the Owner has a setting.
func Stats(apps []store.Application, now time.Time, followUpDays ...int) StatsReport {
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
	if m.statsMaxTop() > 0 {
		keys = append(keys, help(groupNav, "↑↓", "scroll"), help(groupCursor, "j k", "scroll"), help(groupCursor, "g G", "top / bottom"))
	}
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
		m.statsTop = 0
	case "j", "down":
		m.statsTop = min(m.statsTop+1, m.statsMaxTop())
	case "k", "up":
		m.statsTop = max(0, min(m.statsTop, m.statsMaxTop())-1)
	case "g":
		m.statsTop = 0
	case "G":
		m.statsTop = m.statsMaxTop()
	case "r":
		m.rescanAll()
	case "?":
		m.overlay = &helpMode{}
	case "esc":
		m.text = "" // an error is cleared before the screen sees esc
	case "q":
		return m, m.quit()
	}
	return m, nil
}

// statsView is the Stats screen: no tables, no cursor, no preview pane. A report taller
// than the body scrolls with ↑↓, with the overflow cue where lines are hidden.
func (m model) statsView(p pal) string {
	l := m.layout()
	lines := p.clipLines(m.statsLines(p), min(m.statsTop, m.statsMaxTop()), l.bodyHeight(1), strings.Repeat(" ", inset))
	return m.frame(p, []string{m.viewRow(p)}, strings.Join(lines, "\n"), 0, focusNone, m.statusRow(p, l.w, m.keyLine(p, l.w)))
}

// statsLines is the report as the body draws it, one entry per line, set at the text
// column.
func (m model) statsLines(p pal) []string {
	l := m.layout()
	apps := m.statsApps()
	var lines []string
	if len(apps) == 0 {
		lines = []string{p.text(p.dim).Render("No Applications yet · ") + p.key().Render("a") + p.text(p.dim).Render(" to add a Lead")}
	} else {
		report := Stats(apps, m.now(), m.followUpDays)
		lines = m.statsBody(p, report, l.w-inset-rightMargin, l.bodyHeight(1))
	}
	pad := strings.Repeat(" ", inset)
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return lines
}

// statsMaxTop is the furthest Stats scrolls: the last line at the bottom of the body.
func (m model) statsMaxTop() int {
	return max(0, len(m.statsLines(m.pal()))-m.layout().bodyHeight(1))
}

// Stats layout: section labels sit in a column of their own, and what goes beside them
// starts at statsLabelWidth.
const (
	statsLabelWidth = 16
	statsMeterWidth = 12 // the Response rate meter's cells
	statsMinBar     = 4  // the funnel bar's least cells; it grows with the width
	statsMaxBar     = 24
	statsNameWidth  = 12 // the longest funnel Status, "Interviewing"
)

// statsBody renders the report top to bottom, w cells wide. The Response definition is
// kept only when the report still fits h lines. The per-track table is hidden when the
// track filter is on one track.
func (m model) statsBody(p pal, r StatsReport, w, h int) []string {
	response, definition := m.statsResponse(p, r, w)
	sections := [][]string{m.statsFunnel(p, r, w), response, statsWeekly(p, r, w)}
	if m.track == 0 {
		sections = append(sections, statsTracks(p, r))
	}
	total := len(sections) - 1 // the blank lines between sections
	for _, sec := range sections {
		total += len(sec)
	}
	if total+len(definition) <= h {
		sections[1] = append(sections[1], definition...)
	}
	var out []string
	for i, sec := range sections {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, sec...)
	}
	return out
}

// statsHeading is a section label padded to the label column, then rest beside it.
func statsHeading(p pal, label, rest string) string {
	return p.heading(label) + strings.Repeat(" ", max(1, statsLabelWidth-len(label))) + rest
}

// statsIndent leaves the label column blank, for a section's continuation lines.
var statsIndent = strings.Repeat(" ", statsLabelWidth)

// statsFunnel is the funnel: one bar per Status in its colour and glyph, counts
// right-aligned, and beside each rung after the first what share of the previous rung
// reached it and how many skipped that rung. Then the closed Statuses, muted.
func (m model) statsFunnel(p pal, r StatsReport, w int) []string {
	var lines []string
	for i, l := range wrapWords("Applications that ever reached each Status", max(1, w-statsLabelWidth)) {
		if i == 0 {
			lines = append(lines, statsHeading(p, "Funnel", p.text(p.dim).Render(l)))
		} else {
			lines = append(lines, statsIndent+p.text(p.dim).Render(l))
		}
	}
	most, notes := 0, make([]string, len(r.Funnel))
	for i, step := range r.Funnel {
		most = max(most, step.Count)
		if i > 0 {
			notes[i] = funnelNote(step, r.Funnel[i-1].Status)
		}
	}
	countW := len(fmt.Sprint(most))
	noteW := 0
	for _, n := range notes {
		noteW = max(noteW, len(n))
	}
	fixed := statusMarkWidth + statsNameWidth + gutter + 1 + countW + gutter // glyph, name, gaps, count
	barW := min(max(w-fixed-noteW, statsMinBar), statsMaxBar)
	for i, step := range r.Funnel {
		line := p.statusMark(step.Status) + " " + p.text(p.fg).Render(fmt.Sprintf("%-*s", statsNameWidth, step.Status)) + "  " +
			p.text(p.statusColor(step.Status)).Render(statsBar(p, step.Count, most, barW)) + " " +
			p.text(p.fg).Render(fmt.Sprintf("%*d", countW, step.Count))
		// A note too long for the row goes under it, beside the label column.
		var under []string
		switch {
		case notes[i] == "":
		case xansi.StringWidth(line)+gutter+len(notes[i]) <= w:
			line += "  " + p.text(p.dim).Render(notes[i])
		default:
			for _, l := range wrapWords(notes[i], max(1, w-statsLabelWidth)) {
				under = append(under, statsIndent+p.text(p.dim).Render(l))
			}
		}
		lines = append(append(lines, xansi.Truncate(line, w, "…")), under...)
	}
	closed := []string{store.StatusRejected, store.StatusWithdrawn, store.StatusDismissed}
	counts := map[string]int{store.StatusRejected: r.Rejected, store.StatusWithdrawn: r.Withdrawn, store.StatusDismissed: r.Dismissed}
	parts := make([]string, len(closed))
	for i, status := range closed {
		parts[i] = fmt.Sprintf("%s %s %d", p.statusGlyph(status), status, counts[status])
	}
	parts[0] = "closed  " + parts[0]
	for _, l := range strings.Split(keepTogether(w, " · ", parts...), "\n") {
		lines = append(lines, p.text(p.dim).Render(l))
	}
	return lines
}

// funnelNote explains a rung against the previous one: "29% of Applied" where the
// previous rung has Applications, and "5 skipped Draft" for those that reached this
// rung without it. Never a drop-off, which a rising ever-reached count makes a lie.
func funnelNote(step FunnelStep, prev string) string {
	var parts []string
	if step.ConversionOK {
		parts = append(parts, fmt.Sprintf("%d%% of %s", step.Conversion, prev))
	}
	if step.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped %s", step.Skipped, prev))
	}
	return strings.Join(parts, " · ")
}

// statsResponse is the Response rate with a meter and the Applications left out as too
// new to judge; definition is the one-line meaning of a Response, kept if space allows.
func (m model) statsResponse(p pal, r StatsReport, w int) (lines, definition []string) {
	value := p.text(p.fg).Render("—")
	if r.Response.OK {
		value = p.text(p.fg).Render(fmt.Sprintf("%d of %d", r.Response.Reached, r.Response.Applied)) + "  " +
			statsMeter(p, r.Response.Reached, r.Response.Applied) + " " + p.text(p.fg).Bold(true).Render(fmt.Sprintf("%d%%", r.Response.Rate))
	}
	lines = []string{statsHeading(p, "Response rate", value)}
	textW := max(1, w-statsLabelWidth)
	if n := r.Response.TooNew; n > 0 {
		note := fmt.Sprintf("Not counted: %s applied less than %d days ago (too early to judge)", plural(n, "Application", "Applications"), m.followUpDays)
		for _, l := range wrapWords(note, textW) {
			lines = append(lines, statsIndent+p.text(p.dim).Render(l))
		}
	}
	for _, l := range wrapWords("A Response is an Application that reached Interviewing or Offer.", textW) {
		definition = append(definition, statsIndent+p.text(p.dim).Render(l))
	}
	return lines, definition
}

// statsWeekly is Applied per week as one sparkline over the report's weeks, with the
// week range, this week's count and the average.
func statsWeekly(p pal, r StatsReport, w int) []string {
	if len(r.Weeks) == 0 {
		return nil
	}
	total := 0
	for _, week := range r.Weeks {
		total += week.Count
	}
	first, last := r.Weeks[0], r.Weeks[len(r.Weeks)-1]
	phrases := []string{
		p.text(p.info).Render(sparkline(p, r.Weeks)),
		p.text(p.dim).Render(first.Start.Format("2 Jan") + " → " + last.Start.Format("2 Jan")),
		p.text(p.fg).Render(fmt.Sprintf("%d this week", last.Count)) +
			p.text(p.dim).Render(fmt.Sprintf(" · %d-week avg %.1f", len(r.Weeks), float64(total)/float64(len(r.Weeks)))),
	}
	rows := strings.Split(keepTogether(max(1, w-statsLabelWidth), "  ", phrases...), "\n")
	lines := []string{statsHeading(p, "Applied / week", rows[0])}
	for _, row := range rows[1:] {
		lines = append(lines, statsIndent+row)
	}
	return lines
}

// statsTracks is the per-track table, its column names on the heading line. Applied is
// the funnel's definition; Judged leaves out those too new to judge, as Response does.
func statsTracks(p pal, r StatsReport) []string {
	const cols = "%7s  %6s  %9s  %4s"
	lines := []string{statsHeading(p, "By track", p.text(p.dim).Render(fmt.Sprintf(cols, "Applied", "Judged", "Responses", "Rate")))}
	for _, track := range r.Tracks {
		rate := "—"
		if track.OK {
			rate = fmt.Sprintf("%d%%", track.Rate)
		}
		name := fmt.Sprintf("%-*s", statsLabelWidth, xansi.Truncate(track.Track, statsLabelWidth-1, "…"))
		lines = append(lines, p.text(p.fg).Render(name+fmt.Sprintf(cols, fmt.Sprint(track.Applied), fmt.Sprint(track.Judged), fmt.Sprint(track.Responses), rate)))
	}
	return lines
}

// statsBar is a block bar of width cells, scaled to most. A non-zero count gets at
// least one block; a zero count is a dot.
func statsBar(p pal, count, most, width int) string {
	block, dot := "█", "·"
	if p.ascii {
		block, dot = "#", "."
	}
	if count <= 0 || most <= 0 {
		return dot + strings.Repeat(" ", width-1)
	}
	n := max(1, count*width/most)
	return strings.Repeat(block, n) + strings.Repeat(" ", width-n)
}

// statsMeter is a share as a bar of statsMeterWidth cells: "▕████░░░░▏", or "[##--]".
func statsMeter(p pal, n, d int) string {
	full, empty, open, shut := "█", "░", "▕", "▏"
	if p.ascii {
		full, empty, open, shut = "#", "-", "[", "]"
	}
	filled := 0
	if d > 0 {
		filled = min(statsMeterWidth, int(math.Round(float64(n)*statsMeterWidth/float64(d))))
	}
	return p.text(p.faint).Render(open) + p.text(p.purple).Render(strings.Repeat(full, filled)) +
		p.text(p.faint).Render(strings.Repeat(empty, statsMeterWidth-filled)+shut)
}

// sparkLevels are the sparkline's eight heights, Unicode then ASCII.
var sparkLevels = [2][]string{
	{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"},
	{"_", ".", "-", "~", "=", "+", "*", "#"},
}

// sparkline draws one cell per week, scaled to the busiest week. An empty week is the
// lowest level; a week with any Applied is at least one above it.
func sparkline(p pal, weeks []Week) string {
	levels := sparkLevels[0]
	if p.ascii {
		levels = sparkLevels[1]
	}
	most := 0
	for _, w := range weeks {
		most = max(most, w.Count)
	}
	top := len(levels) - 1
	var b strings.Builder
	for _, w := range weeks {
		i := 0
		if w.Count > 0 {
			i = max(1, int(math.Ceil(float64(w.Count)*float64(top)/float64(most))))
		}
		b.WriteString(levels[i])
	}
	return b.String()
}
