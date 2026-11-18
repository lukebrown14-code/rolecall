package ui

import (
	"fmt"
	"image/color"
	"math"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// statsApps is the scanned Applications after the track filter. Malformed
// Applications are dropped by Stats too; this keeps the screen's input honest.
func (m model) statsApps() []store.Application {
	var out []store.Application
	track, on := m.selectedTrack()
	for _, a := range m.apps {
		if a.Malformed != nil {
			continue
		}
		if on && a.Track != track {
			continue
		}
		out = append(out, a)
	}
	return out
}

// statsEmptyMessage and statsEmptyHints are Stats with nothing to report. Both keys are
// always live on Stats (a adds; t cycles the Track filter), so they are named directly:
// asking the registry here would ask Stats for its keys again.
func (m model) statsEmptyMessage() string {
	if m.track > 0 {
		return "No Applications in " + m.trackName()
	}
	return "No Applications yet"
}

func (m model) statsEmptyHints() []emptyHint {
	hints := []emptyHint{addLeadHint}
	if m.track > 0 {
		hints = append([]emptyHint{{"t", "change Track"}}, hints...)
	}
	return hints
}

// statsHelpKeys is the keys only Stats accepts: add. The track filter and the rest are
// global keys.
func (m model) statsHelpKeys() []helpKey {
	keys := []helpKey{help(groupAction, "a", "add").at(prioCore)}
	if m.statsMaxTop() > 0 {
		keys = append(keys, help(groupNav, "↑↓", "scroll").at(prioMain), help(groupCursor, "j k", "scroll"), help(groupCursor, "g G", "top / bottom"))
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
	case "u":
		// Undo is global: it moves the last moved Application back, or says there is none.
		return m, m.undoLast()
	case "?":
		m.overlay = &helpMode{}
	case "esc":
		m.clearSearch() // an error is cleared before the screen sees esc
	case "q":
		return m, m.quit()
	}
	return m, nil
}

// statsView is the Stats screen: no tables, no cursor, no preview pane. A report taller
// than the body scrolls with ↑↓, with the overflow cue where lines are hidden.
func (m model) statsView(p pal) string {
	l := m.layout()
	lines := p.ClipLines(m.statsLines(p), min(m.statsTop, m.statsMaxTop()), l.bodyHeight(1), strings.Repeat(" ", inset))
	return m.frame(p, []string{m.viewRow(p)}, strings.Join(lines, "\n"), 0, focusNone, m.statusRow(p, m.keyLine(p, l.w)))
}

// buildStatsReport builds the Stats value; a var so a test can count the builds.
var buildStatsReport = reporting.Build

// statsCache holds the report built for one interaction, so every Stats sub-view and
// scroll key reuses it rather than rescanning the Applications.
type statsCache struct {
	report reporting.StatsReport
	gaps   []store.GapCount
	built  bool
}

// statsReport is the report for the current interaction, built on first use and reused
// until Update resets it (see resetStats).
func (m model) statsReport() reporting.StatsReport {
	if m.stats == nil {
		return buildStatsReport(m.statsApps(), m.now(), m.followUpDays)
	}
	if !m.stats.built {
		m.stats.report = buildStatsReport(m.statsApps(), m.now(), m.followUpDays)
		m.stats.gaps = m.store.CommonGaps(m.statsApps())
		m.stats.built = true
	}
	return m.stats.report
}

// statsGaps is the gaps fit.md names across the Applications, cached with the report.
func (m model) statsGaps() []store.GapCount {
	if m.stats == nil {
		return m.store.CommonGaps(m.statsApps())
	}
	m.statsReport()
	return m.stats.gaps
}

// statsTopGaps is how many gaps the Insights panel lists.
const statsTopGaps = 8

// halves splits the gap panel: a gap name takes at most half its width.
const halves = 2

// statsInsights is the Insights panel: the gaps fit.md names most, each with how many
// Applications name it and how many not Dismissed it would unlock.
func (m model) statsInsights(p pal, w int) []string {
	gaps := m.statsGaps()
	textW := max(1, w-statsLabelWidth)
	if len(gaps) == 0 {
		return statsCaption(p, statsInsightsLabel, "No gaps recorded yet: assess-fit lists them under ## Gaps in fit.md", w)
	}
	gaps = gaps[:min(len(gaps), statsTopGaps)]
	nameW := 0
	for _, g := range gaps {
		nameW = max(nameW, xansi.StringWidth(g.Name))
	}
	nameW = min(nameW, max(1, textW/halves))
	lines := []string{statsHeading(p, statsInsightsLabel, p.Text(p.Dim).Render(xansi.Truncate("Applications naming each gap in fit.md", textW, "…")))}
	for _, g := range gaps {
		name := fmt.Sprintf("%-*s", nameW, xansi.Truncate(g.Name, nameW, "…"))
		line := p.Text(p.Fg).Render(name) + "  " + p.Text(p.Fg).Render(fmt.Sprintf("%2d", g.Count)) +
			p.Text(p.Dim).Render(" · would unlock "+plural(g.Unlocks, "Application", "Applications"))
		lines = append(lines, statsIndent+xansi.Truncate(line, textW, "…"))
	}
	return lines
}

// statsInsightsLabel is the gap panel's label, the GLOSSARY's word for it.
const statsInsightsLabel = "Insights"

// resetStats drops the cached report so the next interaction builds it once against the
// Workspace as it then stands.
func (m *model) resetStats() {
	if m.stats != nil {
		m.stats.built = false
	}
}

// statsLines is the report as the body draws it, one entry per line, set at the text
// column.
func (m model) statsLines(p pal) []string {
	l := m.layout()
	apps := m.statsApps()
	var lines []string
	if len(apps) == 0 {
		lines = m.emptyLines(p, l.w-inset-rightMargin, m.statsEmptyMessage(), m.statsEmptyHints())
	} else {
		lines = m.statsBody(p, m.statsReport(), l.w-inset-rightMargin, l.bodyHeight(1))
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
// starts at statsLabelWidth. Every bar Stats draws (the funnel, the Response rate, the
// weekly chart) is statsBarWidth cells, so the three read on one scale of width.
const (
	statsLabelWidth = 16
	statsWeekCells  = 2                                     // the weekly chart's cells per week
	statsBarWidth   = statsWeekCells * reporting.StatsWeeks // the one bar width
	statsNameWidth  = 12                                    // the longest funnel Status, "Interviewing"
	statsColumnMin  = 60                                    // a column's least width in the two-column layout
	statsColumnGap  = 4
)

// statsBody renders the report top to bottom, w cells wide: NOW, then the funnel and the
// Response rate, the weekly chart and the per-track table, then Insights. Where w holds
// two columns of statsColumnMin, the funnel and the Response rate sit on the left and
// the weekly chart and the per-track table on the right. The Response definition, then
// the per-track table's note, are kept only when the report still fits h lines. The
// per-track table is hidden when the track filter is on one track.
func (m model) statsBody(p pal, r reporting.StatsReport, w, h int) []string {
	two := w >= statsColumnMin*2+statsColumnGap
	colW := w
	if two {
		colW = (w - statsColumnGap) / 2 //nolint:mnd // two columns
	}
	now := m.statsNow(p, w)
	funnel := m.statsFunnel(p, r, colW)
	response, definition := m.statsResponse(p, r, colW)
	weekly := statsWeekly(p, r, colW)
	var tracks, trackNote []string
	if m.track == 0 {
		tracks, trackNote = m.statsTracks(p, r, colW)
	}
	insights := m.statsInsights(p, w)
	build := func(withDef, withNote bool) []string {
		resp, table := response, tracks
		if withDef {
			resp = append(slices.Clone(resp), definition...)
		}
		if withNote {
			table = append(slices.Clone(table), trackNote...)
		}
		if two {
			return statsStack(now, statsBeside(statsStack(funnel, resp), statsStack(weekly, table), colW+statsColumnGap), insights)
		}
		return statsStack(now, funnel, resp, weekly, table, insights)
	}
	withDef := len(build(true, false)) <= h
	return build(withDef, len(trackNote) > 0 && len(build(withDef, true)) <= h)
}

// statsStack stacks sections with a blank line between them; an empty one takes no room.
func statsStack(sections ...[]string) []string {
	var out []string
	for _, sec := range sections {
		if len(sec) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, sec...)
	}
	return out
}

// statsBeside sets right beside left, right starting at column at.
func statsBeside(left, right []string, at int) []string {
	out := make([]string, max(len(left), len(right)))
	for i := range out {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		if r != "" {
			l = padRight(l, at) + r
		}
		out[i] = l
	}
	return out
}

// statsHeading is a section label padded to the label column, then rest beside it.
func statsHeading(p pal, label, rest string) string {
	return p.Heading(label) + strings.Repeat(" ", max(1, statsLabelWidth-len(label))) + rest
}

// statsCaption is a section label with dim text beside it, wrapped under itself.
func statsCaption(p pal, label, text string, w int) []string {
	var lines []string
	for i, l := range wrapWords(text, max(1, w-statsLabelWidth)) {
		if i == 0 {
			lines = append(lines, statsHeading(p, label, p.Text(p.Dim).Render(l)))
		} else {
			lines = append(lines, statsIndent+p.Text(p.Dim).Render(l))
		}
	}
	return lines
}

// statsIndent leaves the label column blank, for a section's continuation lines.
var statsIndent = strings.Repeat(" ", statsLabelWidth)

// statsNow is what wants doing today, each count with the key that goes there: the
// Follow-up due Applications (3 Pipeline) and the Leads and Drafts closing within soonDays
// (2 Drafts, whose Deadlines view lists them). Nothing due draws nothing.
func (m model) statsNow(p pal, w int) []string {
	due := NowFor(m.statsApps(), m.now(), m.followUpDays, soonDays)
	goes := func(text, key, where string) string {
		return p.Text(p.Fg).Render(text) + p.Text(p.Dim).Render(" → ") + p.Key().Render(key) + p.Text(p.Dim).Render(" "+where)
	}
	var phrases []string
	if n := len(due.FollowUps); n > 0 {
		phrases = append(phrases, goes(plural(n, "follow-up due", "follow-ups due"), "3", screenPipeline))
	}
	if n := len(due.NextSteps); n > 0 {
		phrases = append(phrases, goes(fmt.Sprintf("%s within %d days", plural(n, "next step", "next steps"), soonDays), "3", screenPipeline))
	}
	if n := len(due.Closing); n > 0 {
		phrases = append(phrases, goes(fmt.Sprintf("%d closing within %d days", n, soonDays), "2", tableDeadlines))
	}
	if len(phrases) == 0 {
		return nil
	}
	rows := strings.Split(keepTogether(max(1, w-statsLabelWidth), p.Text(p.Faint).Render(" · "), phrases...), "\n")
	lines := []string{statsHeading(p, "Now", rows[0])}
	for _, row := range rows[1:] {
		lines = append(lines, statsIndent+row)
	}
	return lines
}

// statsFunnel is the funnel: one bar per Status in its colour and glyph, counts
// right-aligned, and beside each rung after the first how it relates to the previous rung
// (see funnelNote). Lead and Draft are optional starts, so where a rung holds
// Applications that skipped the one before, a caption says a bar can outgrow the one
// above. Then Closed (Rejected and Withdrawn) and, on its own line, Dismissed, muted.
// funnelCaption says what the funnel counts: every Application that ever reached a
// Status, where the tab counts are the ones at it now.
const funnelCaption = "Ever reached each Status; tabs count where they are now"

func (m model) statsFunnel(p pal, r reporting.StatsReport, w int) []string {
	caption := funnelCaption
	most, notes := 0, make([]string, len(r.Funnel))
	for i, step := range r.Funnel {
		most = max(most, step.Count)
		if i > 0 {
			notes[i] = funnelNote(step, r.Funnel[i-1].Status, i == 1)
			if step.Skipped > 0 {
				caption = funnelCaption + ". Any Status can be a start, so a bar can outgrow the one above"
			}
		}
	}
	lines := statsCaption(p, "Funnel", caption, w)
	countW := len(fmt.Sprint(most))
	for i, step := range r.Funnel {
		line := p.StatusMark(step.Status) + " " + p.Text(p.Fg).Render(fmt.Sprintf("%-*s", statsNameWidth, step.Status)) + "  " +
			statsBar(p, p.StatusColor(step.Status), step.Count, most) + " " +
			p.Text(p.Fg).Render(fmt.Sprintf("%*d", countW, step.Count))
		// A note too long for the row goes under it, beside the label column.
		var under []string
		switch {
		case notes[i] == "":
		case xansi.StringWidth(line)+gutter+len(notes[i]) <= w:
			line += "  " + p.Text(p.Dim).Render(notes[i])
		default:
			for _, l := range wrapWords(notes[i], max(1, w-statsLabelWidth)) {
				under = append(under, statsIndent+p.Text(p.Dim).Render(l))
			}
		}
		lines = append(append(lines, xansi.Truncate(line, w, "…")), under...)
	}
	// Closed is Rejected and Withdrawn, as the Pipeline screen's Closed view is; Dismissed
	// (an Application let go before it was sent) is never Closed, so it has its own line.
	mark := func(status string, n int) string { return fmt.Sprintf("%s %s %d", p.StatusGlyph(status), status, n) }
	closed := fmt.Sprintf("Closed %d: ", r.Rejected+r.Withdrawn)
	dismissed := mark(store.StatusDismissed, r.Dismissed) + ", let go before applying"
	for _, l := range strings.Split(keepTogether(w, " · ", closed+mark(store.StatusRejected, r.Rejected), mark(store.StatusWithdrawn, r.Withdrawn)), "\n") {
		lines = append(lines, p.Text(p.Dim).Render(l))
	}
	lines = append(lines, p.Text(p.Dim).Render(xansi.Truncate(dismissed, w, "…")))
	if len(r.DropOff) > 0 {
		// Where closed Applications ended, by the reason the Owner gave when they moved it.
		parts := make([]string, 0, len(r.DropOff)+1)
		for _, d := range r.DropOff {
			parts = append(parts, fmt.Sprintf("%s %d", d.Reason, d.Count))
		}
		if r.Unrecorded > 0 {
			parts = append(parts, fmt.Sprintf("no reason %d", r.Unrecorded))
		}
		for i, l := range strings.Split(keepTogether(max(1, w-statsLabelWidth), " · ", parts...), "\n") {
			if i == 0 {
				lines = append(lines, statsHeading(p, "Dropped off", p.Text(p.Dim).Render(l)))
			} else {
				lines = append(lines, statsIndent+p.Text(p.Dim).Render(l))
			}
		}
	}
	return lines
}

// funnelNote explains a rung against the previous one. Where every Application on the
// rung came through the previous one, it is the share of that rung: "29% of Applied".
// Lead and Draft are optional starts, so a rung can hold Applications that never were on
// the one before; a share of the previous rung would then read as a drop beside a full
// bar ("0% of Lead" beside 2), so it says how they arrived, as counts: on the rung after
// the first, "2 from Leads · 1 started at Draft"; later, "3 via Draft · 8 never a Draft", or
// "11 never a Draft" when none came through. Never a drop-off, which a rising
// ever-reached count makes a lie, and nothing when the previous rung is empty.
func funnelNote(step reporting.FunnelStep, prev string, second bool) string {
	switch {
	case step.Skipped == 0 && step.ConversionOK:
		return fmt.Sprintf("%d%% of %s", step.Conversion, prev)
	case step.Skipped == 0:
		return ""
	case second && step.Through == 0:
		return fmt.Sprintf("%d started at %s", step.Skipped, step.Status)
	case second:
		return fmt.Sprintf("%d from %s · %d started at %s", step.Through, statusPlural(prev), step.Skipped, step.Status)
	case step.Through == 0:
		return fmt.Sprintf("%d never a %s", step.Skipped, prev)
	default:
		return fmt.Sprintf("%d via %s · %d never a %s", step.Through, prev, step.Skipped, prev)
	}
}

// statusPlural is a Status as a group of Applications: Leads and Drafts read as nouns,
// the later Statuses as states ("came from Applied").
func statusPlural(status string) string {
	if status == store.StatusLead || status == store.StatusDraft {
		return status + "s"
	}
	return status
}

// statsResponse is the Response rate on the funnel's bar, then the Applications left out
// as too new to judge; definition is the one-line meaning of a Response, kept if space
// allows.
func (m model) statsResponse(p pal, r reporting.StatsReport, w int) (lines, definition []string) {
	value := p.Text(p.Dim).Render("not enough sent yet")
	if r.Response.OK {
		value = statsBar(p, p.Purple, r.Response.Reached, r.Response.Applied) + " " +
			p.Text(p.Fg).Bold(true).Render(fmt.Sprintf("%d%%", r.Response.Rate)) +
			p.Text(p.Dim).Render(fmt.Sprintf(" · %d of %d", r.Response.Reached, r.Response.Applied))
	}
	lines = []string{statsHeading(p, "Response rate", value)}
	textW := max(1, w-statsLabelWidth)
	if n := r.Response.TooNew; n > 0 {
		note := fmt.Sprintf("Not counted: %d applied in the last %d days (too new)", n, m.followUpDays)
		for _, l := range wrapWords(note, textW) {
			lines = append(lines, statsIndent+p.Text(p.Dim).Render(l))
		}
	}
	for _, l := range wrapWords("A Response is an Application that reached Interviewing or Offer.", textW) {
		definition = append(definition, statsIndent+p.Text(p.Dim).Render(l))
	}
	return lines, definition
}

// statsWeekly is Applied per week as a chart of the report's weeks, statsWeekCells cells
// each, empty weeks on the baseline; the first and last week's dates sit under its ends,
// and this week's count, the busiest week and the average beside it.
func statsWeekly(p pal, r reporting.StatsReport, w int) []string {
	if len(r.Weeks) == 0 {
		return nil
	}
	total, peak := 0, 0
	for _, week := range r.Weeks {
		total += week.Count
		peak = max(peak, week.Count)
	}
	first, last := r.Weeks[0], r.Weeks[len(r.Weeks)-1]
	chartW := len(r.Weeks) * statsWeekCells
	from, to := first.Start.Format("2 Jan"), last.Start.Format("2 Jan")
	dates := p.Text(p.Dim).Render(padRight(from, max(len(from)+1, chartW-len(to))) + to)
	phrases := []string{
		p.Text(p.Fg).Render(fmt.Sprintf("%d this week", last.Count)),
		p.Text(p.Dim).Render(fmt.Sprintf("max %d/wk", peak)),
		p.Text(p.Dim).Render(fmt.Sprintf("%d-week avg %.1f", len(r.Weeks), float64(total)/float64(len(r.Weeks)))),
	}
	// The figures sit beside the chart and its dates while they fit, else under them.
	room := max(1, w-statsLabelWidth-chartW-gutter)
	beside := strings.Split(keepTogether(room, p.Text(p.Faint).Render(" · "), phrases...), "\n")
	for _, b := range beside {
		if xansi.StringWidth(b) > room {
			beside = nil
			break
		}
	}
	rows := []string{sparkline(p, r.Weeks, statsWeekCells, p.Info), dates}
	switch {
	case len(beside) <= len(rows):
		for i, b := range beside {
			rows[i] += "  " + b
		}
	default:
		rows = append(rows, strings.Split(keepTogether(max(1, w-statsLabelWidth), p.Text(p.Faint).Render(" · "), phrases...), "\n")...)
	}
	lines := []string{statsHeading(p, "Applied / week", rows[0])}
	for _, row := range rows[1:] {
		lines = append(lines, statsIndent+row)
	}
	return lines
}

// statsTracks is the per-track table, its column names on the heading line, busiest
// Track first (see reporting.OrderTracks); the Tracks with nothing applied share one
// line under it. note is what Counted means: Applied is the funnel's definition; Counted
// leaves out those too new to judge, as the Response rate does, and Rate is Responses of
// those Counted.
func (m model) statsTracks(p pal, r reporting.StatsReport, w int) (lines, note []string) {
	const cols = "%7s  %7s  %9s  %4s"
	sent, idle := reporting.OrderTracks(r.Tracks, m.tracks)
	var names []string
	for _, t := range idle {
		names = append(names, trackLabel(t.Track))
	}
	idleText := "No Applications sent: " + strings.Join(names, ", ")
	if len(sent) == 0 {
		return statsCaption(p, "By track", idleText, w), nil
	}
	lines = []string{statsHeading(p, "By track", p.Text(p.Dim).Render(fmt.Sprintf(cols, "Applied", "Counted", "Responses", "Rate")))}
	for _, track := range sent {
		rate := ""
		if track.OK {
			rate = fmt.Sprintf("%d%%", track.Rate)
		}
		name := fmt.Sprintf("%-*s", statsLabelWidth, xansi.Truncate(trackLabel(track.Track), statsLabelWidth-1, "…"))
		lines = append(lines, p.Text(p.Fg).Render(name+fmt.Sprintf(cols, fmt.Sprint(track.Applied), fmt.Sprint(track.Judged), fmt.Sprint(track.Responses), rate)))
	}
	if len(idle) > 0 {
		for _, l := range wrapWords(idleText, max(1, w-statsLabelWidth)) {
			lines = append(lines, statsIndent+p.Text(p.Dim).Render(l))
		}
	}
	text := fmt.Sprintf("Counted: applied at least %d days ago, as in the Response rate", m.followUpDays)
	for _, l := range wrapWords(text, max(1, w-statsLabelWidth)) {
		note = append(note, statsIndent+p.Text(p.Dim).Render(l))
	}
	return lines, note
}

// statsBar is the one bar Stats draws: count of most as a solid run in colour c on a
// faint track, statsBarWidth cells in all ("████░░░░"). A non-zero count fills at least
// one cell, so "some" never reads as none; an empty one is the bare track.
func statsBar(p pal, c color.Color, count, most int) string {
	n := 0
	if count > 0 && most > 0 {
		n = min(statsBarWidth, max(1, int(math.Round(float64(count)*statsBarWidth/float64(most)))))
	}
	return p.Text(c).Render(strings.Repeat("█", n)) + p.Text(p.Faint).Render(strings.Repeat("░", statsBarWidth-n))
}

// sparkLevels are the sparkline's eight heights. The lowest is the baseline an empty week
// sits on; a week with any Applied is at least the second.
var sparkLevels = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// anyApplied reports whether any week has an Applied: an all-empty series draws no line.
func anyApplied(weeks []reporting.Week) bool {
	for _, w := range weeks {
		if w.Count > 0 {
			return true
		}
	}
	return false
}

// sparkline draws each week as cells cells in colour c, scaled to the busiest week. An
// empty week is the faint baseline, so the chart keeps its width and its empty weeks
// show; it is never the lowest bar, which would read as "some". Stats and the tab bar
// both draw it.
func sparkline(p pal, weeks []reporting.Week, cells int, c color.Color) string {
	most := 0
	for _, w := range weeks {
		most = max(most, w.Count)
	}
	top := len(sparkLevels) - 1
	var b strings.Builder
	for _, w := range weeks {
		if w.Count == 0 {
			b.WriteString(p.Text(p.Faint).Render(strings.Repeat(sparkLevels[0], cells)))
			continue
		}
		i := max(1, int(math.Ceil(float64(w.Count)*float64(top)/float64(most))))
		b.WriteString(p.Text(c).Render(strings.Repeat(sparkLevels[i], cells)))
	}
	return b.String()
}

// activityWeeks is how many weeks of the Applied series the tab bar's sparkline covers.
const activityWeeks = 8
