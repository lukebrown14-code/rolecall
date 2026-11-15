package ui

import (
	"fmt"
	"math"
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
	return "No Applications"
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
	lines := p.ClipLines(m.statsLines(p), min(m.statsTop, m.statsMaxTop()), l.bodyHeight(1), strings.Repeat(" ", inset))
	return m.frame(p, []string{m.viewRow(p)}, strings.Join(lines, "\n"), 0, focusNone, m.statusRow(p, l.w, m.keyLine(p, l.w)))
}

// buildStatsReport builds the Stats value; a var so a test can count the builds.
var buildStatsReport = reporting.Build

// statsCache holds the report built for one interaction, so every Stats sub-view and
// scroll key reuses it rather than rescanning the Applications.
type statsCache struct {
	report reporting.StatsReport
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
		m.stats.built = true
	}
	return m.stats.report
}

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
// starts at statsLabelWidth.
const (
	statsLabelWidth = 16
	statsMeterWidth = 12 // the Response rate meter's cells
	statsMinBar     = 4  // the funnel bar's least cells; it grows with the width
	statsMaxBar     = 24
	statsNameWidth  = 12 // the longest funnel Status, "Interviewing"
)

// statsBody renders the report top to bottom, w cells wide. The Response definition,
// then the per-track table's note, are kept only when the report still fits h lines. The
// per-track table is hidden when the track filter is on one track.
func (m model) statsBody(p pal, r reporting.StatsReport, w, h int) []string {
	response, definition := m.statsResponse(p, r, w)
	sections := [][]string{m.statsFunnel(p, r, w), response, statsWeekly(p, r, w)}
	var trackNote []string
	if m.track == 0 {
		var table []string
		table, trackNote = m.statsTracks(p, r, w)
		sections = append(sections, table)
	}
	total := len(sections) - 1 // the blank lines between sections
	for _, sec := range sections {
		total += len(sec)
	}
	if total+len(definition) <= h {
		sections[1] = append(sections[1], definition...)
		total += len(definition)
	}
	if len(trackNote) > 0 && total+len(trackNote) <= h {
		sections[3] = append(sections[3], trackNote...)
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
	return p.Heading(label) + strings.Repeat(" ", max(1, statsLabelWidth-len(label))) + rest
}

// statsIndent leaves the label column blank, for a section's continuation lines.
var statsIndent = strings.Repeat(" ", statsLabelWidth)

// statsFunnel is the funnel: one bar per Status in its colour and glyph, counts
// right-aligned, and beside each rung after the first what share of the previous rung
// reached it and how many skipped that rung. Then the closed Statuses, muted.
func (m model) statsFunnel(p pal, r reporting.StatsReport, w int) []string {
	var lines []string
	for i, l := range wrapWords("Applications that ever reached each Status", max(1, w-statsLabelWidth)) {
		if i == 0 {
			lines = append(lines, statsHeading(p, "Funnel", p.Text(p.Dim).Render(l)))
		} else {
			lines = append(lines, statsIndent+p.Text(p.Dim).Render(l))
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
		line := p.StatusMark(step.Status) + " " + p.Text(p.Fg).Render(fmt.Sprintf("%-*s", statsNameWidth, step.Status)) + "  " +
			p.Text(p.StatusColor(step.Status)).Render(statsBar(p, step.Count, most, barW)) + " " +
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
	closed := []string{store.StatusRejected, store.StatusWithdrawn, store.StatusDismissed}
	counts := map[string]int{store.StatusRejected: r.Rejected, store.StatusWithdrawn: r.Withdrawn, store.StatusDismissed: r.Dismissed}
	parts := make([]string, len(closed))
	for i, status := range closed {
		parts[i] = fmt.Sprintf("%s %s %d", p.StatusGlyph(status), status, counts[status])
	}
	parts[0] = "closed  " + parts[0]
	for _, l := range strings.Split(keepTogether(w, " · ", parts...), "\n") {
		lines = append(lines, p.Text(p.Dim).Render(l))
	}
	return lines
}

// funnelNote explains a rung against the previous one: "29% of Applied" where the
// previous rung has Applications, and "5 weren't Drafts first" for those that reached
// this rung without the previous one before it. Never a drop-off, which a rising
// ever-reached count makes a lie. The share is left out only when the previous rung is
// empty, where "0% of" would read as a drop; the wording is otherwise the same.
func funnelNote(step reporting.FunnelStep, prev string) string {
	var parts []string
	if step.ConversionOK {
		parts = append(parts, fmt.Sprintf("%d%% of %s", step.Conversion, prev))
	}
	if step.Skipped > 0 {
		parts = append(parts, notFirst(step.Skipped, prev))
	}
	return strings.Join(parts, " · ")
}

// notFirst is "1 wasn't a Lead first" or "5 weren't Drafts first": Lead and Draft read as
// nouns, the later Statuses as states ("2 weren't Applied first").
func notFirst(n int, status string) string {
	noun := status
	if status == store.StatusLead || status == store.StatusDraft {
		noun = "a " + status
		if n != 1 {
			noun = status + "s"
		}
	}
	verb := "wasn't"
	if n != 1 {
		verb = "weren't"
	}
	return fmt.Sprintf("%d %s %s first", n, verb, noun)
}

// statsResponse is the Response rate with a meter and the Applications left out as too
// new to judge; definition is the one-line meaning of a Response, kept if space allows.
func (m model) statsResponse(p pal, r reporting.StatsReport, w int) (lines, definition []string) {
	value := p.Text(p.Fg).Render("—")
	if r.Response.OK {
		value = p.Text(p.Fg).Render(fmt.Sprintf("%d of %d", r.Response.Reached, r.Response.Applied)) + "  " +
			statsMeter(p, r.Response.Reached, r.Response.Applied) + " " + p.Text(p.Fg).Bold(true).Render(fmt.Sprintf("%d%%", r.Response.Rate))
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

// statsWeekly is Applied per week as one sparkline over the report's weeks, with the
// week range, this week's count and the average.
func statsWeekly(p pal, r reporting.StatsReport, w int) []string {
	if len(r.Weeks) == 0 {
		return nil
	}
	total := 0
	for _, week := range r.Weeks {
		total += week.Count
	}
	first, last := r.Weeks[0], r.Weeks[len(r.Weeks)-1]
	phrases := []string{
		p.Text(p.Info).Render(sparkline(p, r.Weeks)),
		p.Text(p.Dim).Render(first.Start.Format("2 Jan") + " → " + last.Start.Format("2 Jan")),
		p.Text(p.Fg).Render(fmt.Sprintf("%d this week", last.Count)) +
			p.Text(p.Dim).Render(fmt.Sprintf(" · %d-week avg %.1f", len(r.Weeks), float64(total)/float64(len(r.Weeks)))),
	}
	rows := strings.Split(keepTogether(max(1, w-statsLabelWidth), "  ", phrases...), "\n")
	lines := []string{statsHeading(p, "Applied / week", rows[0])}
	for _, row := range rows[1:] {
		lines = append(lines, statsIndent+row)
	}
	return lines
}

// statsTracks is the per-track table, its column names on the heading line, then what
// Counted means. Applied is the funnel's definition; Counted leaves out those too new to
// judge, as the Response rate does, and Rate is Responses of those Counted.
func (m model) statsTracks(p pal, r reporting.StatsReport, w int) (lines, note []string) {
	const cols = "%7s  %7s  %9s  %4s"
	lines = []string{statsHeading(p, "By track", p.Text(p.Dim).Render(fmt.Sprintf(cols, "Applied", "Counted", "Responses", "Rate")))}
	for _, track := range r.Tracks {
		rate := "—"
		if track.OK {
			rate = fmt.Sprintf("%d%%", track.Rate)
		}
		name := fmt.Sprintf("%-*s", statsLabelWidth, xansi.Truncate(track.Track, statsLabelWidth-1, "…"))
		lines = append(lines, p.Text(p.Fg).Render(name+fmt.Sprintf(cols, fmt.Sprint(track.Applied), fmt.Sprint(track.Judged), fmt.Sprint(track.Responses), rate)))
	}
	text := fmt.Sprintf("Counted: applied at least %d days ago, as in the Response rate", m.followUpDays)
	for _, l := range wrapWords(text, max(1, w-statsLabelWidth)) {
		note = append(note, statsIndent+p.Text(p.Dim).Render(l))
	}
	return lines, note
}

// statsBar is a block bar of width cells, scaled to most. A non-zero count gets at
// least one block; a zero count is a dot.
func statsBar(p pal, count, most, width int) string {
	block, dot := "█", "·"
	if p.Ascii {
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
	if p.Ascii {
		full, empty, open, shut = "#", "-", "[", "]"
	}
	filled := 0
	if d > 0 {
		filled = min(statsMeterWidth, int(math.Round(float64(n)*statsMeterWidth/float64(d))))
	}
	return p.Text(p.Faint).Render(open) + p.Text(p.Purple).Render(strings.Repeat(full, filled)) +
		p.Text(p.Faint).Render(strings.Repeat(empty, statsMeterWidth-filled)+shut)
}

// sparkLevels are the sparkline's eight heights, Unicode then ASCII.
var sparkLevels = [2][]string{
	{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"},
	{"_", ".", "-", "~", "=", "+", "*", "#"},
}

// sparkline draws one cell per week, scaled to the busiest week. An empty week is blank;
// a week with any Applied is at least one bar. Stats and the tab bar both draw it.
func sparkline(p pal, weeks []reporting.Week) string {
	levels := sparkLevels[0]
	if p.Ascii {
		levels = sparkLevels[1]
	}
	most := 0
	for _, w := range weeks {
		most = max(most, w.Count)
	}
	top := len(levels) - 1
	var b strings.Builder
	for _, w := range weeks {
		// An empty week is blank, not the lowest bar: a bar there would read as "some".
		if w.Count == 0 {
			b.WriteString(" ")
			continue
		}
		i := max(1, int(math.Ceil(float64(w.Count)*float64(top)/float64(most))))
		b.WriteString(levels[i])
	}
	return b.String()
}

// activityWeeks is how many weeks of the Applied series the tab bar's sparkline covers.
const activityWeeks = 8
