package ui

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// Layout constants for the list view.
const (
	fallbackWidth  = 100
	fallbackHeight = 30
	minTermWidth   = 50 // below this, or minTermHeight, a "too small" notice replaces the layout
	minTermHeight  = 12
	splitWidth     = 100 // the preview pane appears from this width
	listShare      = 55  // percent of the width the list takes beside the preview
	minListWidth   = 56
	chromeLines    = 6 // top bar, gap, views, rule … feedback/rule, keys
	rightMargin    = 2
	rowPad         = 1 // keeps the last label off the edge of the selection bar
	soonDays       = 7 // "closes in Nd" turns amber at or below this
	labelWidth     = 13
	hoursPerDay    = 24
	percent        = 100
	minTitle       = 8 // a title never shrinks below this
	previewInset   = 3 // padding and divider around the preview pane
	errorLines     = 8 // lines of a parse error shown in the preview
	docLines       = 4 // lines of fit.md / notes.md shown in the preview
	titleLines     = 2 // a long title wraps to at most this in the preview
	minBody        = 4
	shedRolecall   = 1 // topBar detail levels: drop the word "rolecall"…
	shedCounts     = 2 // …then other screens' counts…
	shedNames      = 3 // …then their names
	previewLines   = 3
	daysPerWeek    = 7
	daysPerMonth   = 30
	weeksUntil     = 14 // relative dates switch from days to weeks here…
	monthsUntil    = 60 // …and from weeks to months here
)

// Right-hand columns, identical on every view: state · flag · Fit, Fit flush right.
// A column empty for every row of the current view is hidden.
const (
	slotState = 11 // "● ready", "not written", "✕ rejected", "↩ withdrawn"
	slotFlag  = 12 // "⚑ quiet 11mo"
	slotFit   = 8  // "Marginal"
)

func (m model) viewWidth() int {
	if m.width == 0 {
		return fallbackWidth
	}
	return m.width
}

func (m model) viewHeight() int {
	if m.height == 0 {
		return fallbackHeight
	}
	return m.height
}

func padRight(s string, w int) string { return s + strings.Repeat(" ", max(0, w-xansi.StringWidth(s))) }

// joinColumns sets blocks side by side at a fixed width, or stacks them when narrow.
func joinColumns(cols []string, w int, side bool) string {
	if !side {
		return strings.Join(cols, "\n\n")
	}
	styled := make([]string, len(cols))
	for i, c := range cols {
		styled[i] = lipgloss.NewStyle().Width(w).Render(c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, styled...)
}

// ago renders a date relative to today: "today", "in 4d", "3d ago", "2w ago", "2mo ago".
func ago(date string, now time.Time) (string, int) {
	t, err := time.Parse("2006-01-02", strings.Split(date, "T")[0])
	if err != nil {
		return date, 0
	}
	today, _ := time.Parse("2006-01-02", now.Format("2006-01-02"))
	days := int(t.Sub(today).Hours() / hoursPerDay)
	n := max(days, -days)
	var s string
	switch {
	case days == 0:
		return "today", 0
	case n < weeksUntil:
		s = fmt.Sprintf("%dd", n)
	case n < monthsUntil:
		s = fmt.Sprintf("%dw", n/daysPerWeek)
	default:
		s = fmt.Sprintf("%dmo", n/daysPerMonth)
	}
	if days > 0 {
		return "in " + s, days
	}
	return s + " ago", days
}

// futureFlag marks a Status date after today: almost always a data error, so it is
// flagged rather than shown as a normal "in 8mo" relative date.
const futureFlag = "⚠ future date"

// futureDate reports whether a Status entry's date is after today.
func (m model) futureDate(e *store.StatusEntry) bool {
	if e == nil {
		return false
	}
	t, err := time.Parse("2006-01-02", strings.Split(e.At, "T")[0])
	if err != nil {
		return false
	}
	today, _ := time.Parse("2006-01-02", m.now().Format("2006-01-02"))
	return t.After(today)
}

func (m model) View() tea.View {
	p := basePal
	var body string
	switch {
	case m.tooSmall():
		body = m.tooSmallView(p)
	case m.detail != nil:
		body = m.detailView(p)
	default:
		body = m.listView(p)
	}
	if box := m.overlayView(p); box != "" && !m.tooSmall() {
		base := m.listView(dimmedPal)
		if m.detail != nil {
			base = m.detailView(dimmedPal)
		}
		body = overlay(base, box, m.viewWidth())
	}
	v := tea.NewView(asciiFold(body))
	// Dark only: paint the terminal background so the palette reads the same on a light
	// terminal. NO_COLOR drops colour but keeps bold and underline.
	if noColor() {
		v.Content = stripColor(v.Content)
	} else {
		v.BackgroundColor, v.ForegroundColor = colorBg, colorFg
	}
	return v
}

// tooSmall reports a terminal below the smallest size the layout supports. Before the
// first size message the fallback size applies, which is never too small.
func (m model) tooSmall() bool {
	return m.width > 0 && (m.width < minTermWidth || m.height < minTermHeight)
}

// tooSmallView says so, in place of a layout that would spill off the screen.
func (m model) tooSmallView(p pal) string {
	msg := p.text(p.fg).Bold(true).Render("Terminal too small") + "\n" +
		p.text(p.dim).Render(fmt.Sprintf("%d×%d, needs %d×%d", m.width, m.height, minTermWidth, minTermHeight))
	lines := strings.Split(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(msg)), "\n")
	return strings.Join(lines[:min(len(lines), m.height)], "\n")
}

// listView is a screen: a screen without Application tables draws its own view;
// otherwise top bar, views, the list (with the preview pane when wide), the feedback
// line and the key line.
func (m model) listView(p pal) string {
	if !m.hasTables() {
		return screenDefs[m.screen].render(m, p)
	}
	w, bodyH := m.viewWidth(), max(minBody, m.viewHeight()-chromeLines)
	rows := m.rows()
	split := w >= splitWidth && len(rows) > 0
	listW := w
	if split {
		listW = max(minListWidth, w*listShare/percent)
	}
	previewW := w - listW - 1
	ruleTop, ruleBot := strings.Repeat("─", w), strings.Repeat("─", w)
	if split {
		ruleTop = strings.Repeat("─", listW) + "┬" + strings.Repeat("─", previewW)
		ruleBot = strings.Repeat("─", listW) + "┴" + strings.Repeat("─", previewW)
	}
	var body string
	if len(rows) == 0 {
		title, hint := emptyMessage(m.views()[m.table])
		if m.text != "" {
			title, hint = "No matches for “"+m.text+"”", "esc clears the search"
		}
		body = lipgloss.Place(w, bodyH, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, p.text(p.fg).Bold(true).Render(title), "", p.text(p.dim).Render(hint)))
	} else {
		list := lipgloss.NewStyle().PaddingLeft(1).Width(listW).Render(m.listBody(p, rows, listW-gutter, bodyH))
		body = list
		if split {
			sep := p.text(p.rule).Render(strings.TrimSuffix(strings.Repeat("│\n", bodyH), "\n"))
			body = lipgloss.JoinHorizontal(lipgloss.Top, list, sep, m.preview(p, rows[m.cursor], previewW, bodyH))
		}
	}
	keys := m.keyLine(p, w)
	if m.searching() {
		in := p.text(p.accent).Bold(true).Render(" / ") + p.text(p.fg).Render(m.text) + p.text(p.accent).Render("▏")
		keys = spread(in, p.keys(pairs(m.helpKeys(), groupNav)), w-1)
	}
	// The bottom rule is always drawn; the status row below it holds a toast, an error,
	// the search input or the key line, one at a time.
	return m.topBar(p) + "\n\n" + m.viewRow(p) + "\n" + p.text(p.rule).Render(ruleTop) + "\n" +
		body + "\n" + p.text(p.rule).Render(ruleBot) + "\n" + m.statusRow(p, w, keys)
}

// statusRow is the frame's last line: what just happened (a toast or error) when there is
// one, otherwise the line the caller prepared (the search input while typing, else the
// key line). The rule above it is never replaced.
func (m model) statusRow(p pal, w int, fallback string) string {
	if m.searching() {
		return fallback // the search input shows while typing, even over an error
	}
	if m.err != nil || m.toast != "" || m.building != nil {
		return m.feedbackLine(p, w)
	}
	return fallback
}

// emptyMessage says how Applications reach an empty view.
func emptyMessage(table string) (string, string) {
	switch table {
	case tableLeads:
		return "No Leads yet", "Press a to add one, or run capture-job from your harness."
	case tableDismissed:
		return "Nothing dismissed", "Press x on a Lead to dismiss it."
	case tableDraft:
		return "No Drafts", "Move a Lead here with m → d."
	case allView, tableApplied:
		return "Nothing applied for yet", "Mark a Draft as sent with m → a."
	case tableInterviewing:
		return "No interviews", "Move an Application here with m → i from Applied."
	case tableOffer:
		return "No offers yet", "Move an Application here with m → o from Interviewing."
	case tableClosed:
		return "Nothing closed", "Rejected and Withdrawn Applications land here (m → r / w)."
	}
	return "Nothing here", ""
}

func (m model) screenCount(si int) int {
	n := 0
	for _, t := range screenDefs[si].tables {
		if t != allView && t != tableDismissed {
			n += m.count(si, t)
		}
	}
	return n
}

// filtered reports whether a track or text filter is in force.
func (m model) filtered() bool { return m.track > 0 || m.text != "" }

// countLabel is a view's count: the total, or "shown/total" while a filter hides rows,
// so a filtered number is never read as the total.
func (m model) countLabel(screen int, table string) string {
	total := m.totalCount(screen, table)
	if !m.filtered() {
		return fmt.Sprintf("%d", total)
	}
	return fmt.Sprintf("%d/%d", m.count(screen, table), total)
}

// screenCountLabel is a screen's count over its views, filtered the same way.
func (m model) screenCountLabel(si int) string {
	total := 0
	for _, t := range screenDefs[si].tables {
		if t != allView && t != tableDismissed {
			total += m.totalCount(si, t)
		}
	}
	if !m.filtered() {
		return fmt.Sprintf("%d", total)
	}
	return fmt.Sprintf("%d/%d", m.screenCount(si), total)
}

// topBar: app mark and screens on the left, the track filter on the right. On narrow
// terminals it sheds detail in steps: the "rolecall" word, other screens' counts, their names.
func (m model) topBar(p pal) string {
	w := m.viewWidth()
	on := lipgloss.NewStyle().Background(p.accent).Foreground(p.onFill).Bold(true)
	build := func(level int) string {
		out := p.text(p.accent).Bold(true).Render(" ◆ rolecall ")
		if level >= shedRolecall {
			out = p.text(p.accent).Bold(true).Render(" ◆ ")
		}
		for si, d := range screenDefs {
			s := d.name
			n := m.screenCountLabel(si)
			if si == m.screen {
				// Only a screen with rows carries a count: Stats and Profile report.
				if !d.hasTables {
					out += "  " + on.Render(fmt.Sprintf(" %d %s ", si+1, s))
				} else {
					out += "  " + on.Render(fmt.Sprintf(" %d %s %s ", si+1, s, n))
				}
				continue
			}
			out += "  " + p.text(p.accent).Render(fmt.Sprintf(" %d", si+1))
			if level < shedNames {
				out += " " + p.text(p.fg).Render(s)
			}
			if level < shedCounts && d.hasTables {
				out += p.text(p.dim).Render(fmt.Sprintf(" %s", n))
			}
			out += " " // the same padding as the highlighted entry, so nothing shifts
		}
		return out
	}
	minTrack := xansi.StringWidth(m.trackSelector(p, 0))
	out := build(0)
	for level := shedRolecall; level <= shedNames && xansi.StringWidth(out)+minTrack+rightMargin+gutter > w; level++ {
		out = build(level)
	}
	if len(m.tracks) == 0 {
		return out
	}
	track := m.trackSelector(p, w-xansi.StringWidth(out)-rightMargin-gutter)
	return out + strings.Repeat(" ", max(gutter, w-xansi.StringWidth(out)-xansi.StringWidth(track)-rightMargin)) + track
}

// trackSelector is a labelled field, "track  All  SWE  Platform  t track", collapsing
// in steps — "track  All +2  t track", then "All +2  t track", "All  t track", "All" —
// so the header never spills on a narrow terminal.
func (m model) trackSelector(p pal, w int) string {
	opts := append([]string{"All"}, m.tracks...)
	on := p.text(p.accent).Bold(true).Underline(true)
	// The active track is bracketed as well as underlined, so it is marked without colour.
	mark := func(name string) string {
		return p.text(p.accent).Render("[") + on.Render(name) + p.text(p.accent).Render("]")
	}
	parts := make([]string, len(opts))
	for i, t := range opts {
		parts[i] = p.text(p.dim).Render(t)
		if i == m.track {
			parts[i] = mark(t)
		}
	}
	key := p.text(p.accent).Bold(true).Render("  t track")
	label := p.text(p.dim).Render("track  ")
	name := "All"
	if m.track > 0 {
		name = m.tracks[m.track-1]
	}
	plus := p.text(p.dim).Render(fmt.Sprintf(" +%d", len(opts)-1))
	activeName := mark(name)
	// Longest to shortest; the last always fits any terminal the layout supports.
	for _, cand := range []string{
		label + strings.Join(parts, "  ") + key,
		label + activeName + plus + key,
		activeName + plus + key,
		activeName + key,
		activeName,
	} {
		if xansi.StringWidth(cand) <= w {
			return cand
		}
	}
	return activeName
}

// viewRow is the frame's context row: the screen's views (cycled with Tab), or the one
// view's name, plus the scope line on the right when a filter is on. The row is always
// present, so the layout never jumps between screens.
func (m model) viewRow(p pal) string {
	if !m.hasTables() {
		return ""
	}
	w := m.viewWidth()
	ts := m.views()
	row := "   " + p.text(p.dim).Render(ts[0])
	if len(ts) > 1 {
		for _, f := range []struct {
			gap    string
			counts bool
		}{{"    ", true}, {"  ", true}, {"  ", false}} {
			parts := make([]string, len(ts))
			for i, t := range ts {
				label := t
				if f.counts {
					label = fmt.Sprintf("%s %s", t, m.countLabel(m.screen, t))
				}
				parts[i] = p.text(p.dim).Render(label)
				if i == m.table {
					// Bracketed as well as bold and underlined: marked without colour.
					parts[i] = p.text(p.accent).Render("[") + p.text(p.fg).Bold(true).Underline(true).Render(label) + p.text(p.accent).Render("]")
				}
			}
			row = "   " + strings.Join(parts, f.gap)
			if xansi.StringWidth(row) <= w {
				break
			}
		}
	}
	if scope := m.scopeLine(p); scope != "" {
		// Keep the frame one line wide: the scope truncates to what the row leaves.
		avail := max(0, w-rightMargin-xansi.StringWidth(row)-gutter)
		row = spread(row, xansi.Truncate(scope, avail, "…"), w-rightMargin)
	}
	return row
}

// scopeLine names what is filtered, when anything is: the screen and view, the track, the
// search text and the shown-of-total count. Empty when no filter is on.
func (m model) scopeLine(p pal) string {
	if m.track == 0 && m.text == "" {
		return ""
	}
	label := screenDefs[m.screen].name
	if view := m.views()[m.table]; view != label {
		label += " › " + view
	}
	parts := []string{label}
	if m.track > 0 {
		parts = append(parts, "track "+m.tracks[m.track-1])
	}
	if m.text != "" {
		parts = append(parts, "search "+m.text)
	}
	parts = append(parts, fmt.Sprintf("%d of %d", len(m.rows()), m.totalCount(m.screen, m.views()[m.table])))
	return p.text(p.dim).Render(strings.Join(parts, " · "))
}

// feedbackLine is the status row when something just happened: a toast (successes clear
// on the next key), an error (stays until esc), or a build in flight (a spinner until it
// finishes). It never replaces the bottom rule.
func (m model) feedbackLine(p pal, w int) string {
	icon, c, msg, hint := "✓", p.ok, m.toast, m.toastHint
	raw := false
	switch {
	case m.err != nil:
		msg = m.rel(m.err.Error())
		icon, c, hint = "✕", p.bad, "esc hide"
	case m.building != nil && m.toast == "":
		msg, icon, c, raw = "Building "+m.building.title+"…", m.spinner.View(), p.accent, true
	}
	if msg == "" {
		return ""
	}
	bg := lipgloss.NewStyle().Background(colorPanel)
	var left string
	if raw {
		left = bg.Render(" ") + icon + bg.Foreground(p.fg).Render(msg)
	} else {
		left = bg.Foreground(c).Bold(true).Render(" "+icon+" ") + bg.Foreground(p.fg).Render(msg)
	}
	right := ""
	if hint != "" {
		k, d, _ := strings.Cut(hint, " ")
		right = bg.Foreground(p.accent).Bold(true).Render(k) + bg.Foreground(p.dim).Render(" "+d+" ")
	}
	n := max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))
	gap := bg.Render(strings.Repeat(" ", n))
	return xansi.Truncate(left+gap+right, w, "")
}

// listBody lays out the rows with section dividers. Rows get a blank line between them
// when everything fits, and run dense when the list is longer than the screen.
func (m model) listBody(p pal, rows []store.Application, w, h int) string {
	show := m.columns(rows)
	dividers := m.dividers(rows)
	build := func(gap bool) ([]block, int, int) {
		var blocks []block
		cur, total := 0, 0
		for i, a := range rows {
			if d, ok := dividers[i]; ok {
				label := fmt.Sprintf("  %s · %d ", d.label, d.n)
				lines := []string{p.text(p.dim).Bold(true).Render(label) + p.text(p.rule).Render(strings.Repeat("─", max(0, w-xansi.StringWidth(label))))}
				if i > 0 {
					lines = append([]string{""}, lines...) // air above each section
				}
				blocks = append(blocks, block{lines: lines})
				total += len(lines)
			}
			if i == m.cursor {
				cur = len(blocks)
			}
			lines := m.row(p, a, i == m.cursor, show, w)
			if _, next := dividers[i+1]; gap && i+1 < len(rows) && !next {
				lines = append(lines, "")
			}
			blocks = append(blocks, block{lines: lines, row: true})
			total += len(lines)
		}
		return blocks, cur, total
	}
	blocks, cur, total := build(true)
	if total > h {
		blocks, cur, _ = build(false)
	}
	return scroll(p, blocks, cur, h)
}

type block struct {
	lines []string
	row   bool // an Application, as opposed to a section divider
}

type divider struct {
	label string
	n     int
}

// dividers are the section headings drawn before a row: one per Status in the Applied
// screen's All view, and Expired at the end of Leads and Draft.
func (m model) dividers(rows []store.Application) map[int]divider {
	div := map[int]divider{}
	switch m.views()[m.table] {
	case allView:
		for i, a := range rows {
			if _, t := StatusLocation(a.Status); i == 0 || t != tableOf(rows[i-1]) {
				div[i] = divider{t, m.count(m.screen, t)}
			}
		}
	case tableLeads, tableDraft:
		for i, a := range rows {
			if m.expired(a) {
				div[i] = divider{"Expired", len(rows) - i}
				break
			}
		}
	}
	return div
}

func tableOf(a store.Application) string { _, t := StatusLocation(a.Status); return t }

// expired is a Lead or Draft whose close date has passed.
func (m model) expired(a store.Application) bool {
	t := tableOf(a)
	return beforeApplied(t) && a.Closes != "" && a.Closes < m.now().Format("2006-01-02")
}

// scroll fills h lines with blocks, keeping block cur in view. "↑/↓ n more" appear only
// when rows are really hidden, and count rows, not dividers.
func scroll(p pal, blocks []block, cur, h int) string {
	fit := func(start int) int {
		avail := h
		if start > 0 {
			avail--
		}
		used, end := 0, start
		for end < len(blocks) && used+len(blocks[end].lines) <= avail {
			used += len(blocks[end].lines)
			end++
		}
		for end < len(blocks) && used+1 > avail && end > start {
			end--
			used -= len(blocks[end].lines)
		}
		return end
	}
	start, end := 0, fit(0)
	for cur >= end && start < cur {
		start++
		end = fit(start)
	}
	count := func(bs []block) int {
		n := 0
		for _, b := range bs {
			if b.row {
				n++
			}
		}
		return n
	}
	var out []string
	if start > 0 {
		out = append(out, p.text(p.dim).Render(fmt.Sprintf(" ↑ %d more", count(blocks[:start]))))
	}
	for _, b := range blocks[start:end] {
		out = append(out, b.lines...)
	}
	if end < len(blocks) {
		out = append(out, p.text(p.dim).Render(fmt.Sprintf(" ↓ %d more", count(blocks[end:]))))
	}
	for len(out) < h {
		out = append(out, "")
	}
	return strings.Join(out[:h], "\n")
}

// slotSet says which right-hand columns the current view shows.
type slotSet struct{ state, flag, fit bool }

// slotText is an Application's right-hand column text: state (docs on Draft, outcome
// on Closed), follow-up flag and Fit.
func (m model) slotText(a store.Application) (state, flag, fit string) {
	if a.Malformed != nil {
		return "", "", ""
	}
	t := tableOf(a)
	if cols := ApplicationColumns(a, t, m.now(), m.followUpDays); (t == tableApplied || t == tableInterviewing) && cols[len(cols)-1] == followUpFlag && a.LastChange != nil {
		rel, _ := ago(a.LastChange.At, m.now())
		flag = followUpFlag + " quiet " + strings.TrimSuffix(rel, " ago")
	}
	switch {
	case t == tableDraft:
		state, _ = basePal.docsLabel(a)
	case a.Status == store.StatusRejected:
		state = "✕ rejected"
	case a.Status == store.StatusWithdrawn:
		state = "↩ withdrawn"
	}
	if t != tableDismissed {
		fit = a.Fit
		if fit == "" {
			fit = "—"
		}
	}
	return state, flag, fit
}

// docsLabel is the one wording for documents, glossary terms Ready and Written, and
// its colour.
func (p pal) docsLabel(a store.Application) (string, color.Color) {
	switch strings.ToLower(a.Documents) {
	case "ready":
		return "● ready", p.ok
	case "written":
		return "○ written", p.warn
	}
	return "not written", p.dim
}

// stateColor is the colour of a row's state slot.
func (p pal) stateColor(a store.Application, dimC color.Color) color.Color {
	switch {
	case tableOf(a) == tableDraft:
		if _, c := p.docsLabel(a); c != p.dim {
			return c
		}
	case a.Status == store.StatusRejected:
		return p.bad
	}
	return dimC
}

func (m model) columns(rows []store.Application) slotSet {
	var s slotSet
	for _, a := range rows {
		st, f, ft := m.slotText(a)
		s.state, s.flag, s.fit = s.state || st != "", s.flag || f != "", s.fit || ft != ""
	}
	return s
}

// row is one Application as two lines: title and right-hand columns, then company ·
// track · dates. The selected row is a full-width bar; text on it is never dimmed.
func (m model) row(p pal, a store.Application, sel bool, show slotSet, w int) []string {
	line := lipgloss.NewStyle().Width(w).MaxWidth(w)
	bar := "  "
	titleC, dimC := p.fg, p.dim
	if m.expired(a) {
		titleC = p.dim
	}
	st := func(c color.Color) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(c)
		if sel {
			s = s.Background(p.sel)
		}
		return s
	}
	if sel {
		line = line.Background(p.sel)
		bar = st(p.accent).Render("▌ ")
		titleC, dimC = p.accent, p.sub
	}
	if a.Malformed != nil {
		return []string{line.Render(bar + st(p.bad).Bold(true).Render("⚠ "+filepath.Base(a.Folder))),
			line.Render(bar + st(dimC).Render(xansi.Truncate(m.rel(a.Malformed.Error()), w-ellipsis, "…")))}
	}
	state, flag, fit := m.slotText(a)
	stateC := p.stateColor(a, dimC)
	fitC := p.fitColor(a.Fit)
	if fitC == p.dim {
		fitC = dimC
	}
	slot := func(width int, c color.Color, s string) string {
		return st(c).Width(width).Align(lipgloss.Right).Render(s)
	}
	var cells []string
	if show.state {
		cells = append(cells, slot(slotState, stateC, state))
	}
	if show.flag {
		cells = append(cells, slot(slotFlag, p.warn, flag))
	}
	if show.fit {
		cells = append(cells, slot(slotFit, fitC, fit))
	}
	right := strings.Join(cells, st(p.fg).Render("  ")) + st(p.fg).Render(strings.Repeat(" ", rowPad))
	left := bar + st(titleC).Bold(sel).Render(xansi.Truncate(a.Title, max(minTitle, w-xansi.StringWidth(right)-ellipsis), "…"))
	gap := st(p.fg).Render(strings.Repeat(" ", max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))))
	meta := strings.Join(m.meta(p, a, st, dimC), st(dimC).Render(" · "))
	return []string{line.Render(left + gap + right), line.Render(xansi.Truncate(bar+meta, w, "…"))}
}

// meta is a row's second line: company, track, and the dates that matter for its table.
func (m model) meta(p pal, a store.Application, st func(color.Color) lipgloss.Style, dimC color.Color) []string {
	dim := st(dimC)
	// when renders a Status date, flagging one in the future as a data error.
	when := func(e *store.StatusEntry, prefix string) string {
		if m.futureDate(e) {
			return st(p.warn).Render(prefix + futureFlag)
		}
		return dim.Render(prefix + m.agoEntry(e))
	}
	out := []string{dim.Render(a.Company), dim.Render(a.Track)}
	switch t := tableOf(a); t {
	case tableLeads, tableDraft:
		switch r, days := ago(a.Closes, m.now()); {
		case a.Closes == "":
			out = append(out, dim.Render("no close date"))
		case days < 0:
			out = append(out, dim.Render("expired "+r))
		case days == 0:
			out = append(out, st(p.bad).Bold(true).Render("closes today"))
		case days <= soonDays:
			out = append(out, st(p.warn).Render("closes "+r))
		default:
			out = append(out, dim.Render("closes "+r))
		}
	case tableDismissed:
		if a.LastChange != nil {
			out = append(out, when(a.LastChange, "dismissed "))
		}
	case tableClosed:
		if a.LastChange != nil {
			out = append(out, when(a.LastChange, "closed "))
		}
	default:
		if a.AppliedDate != nil {
			out = append(out, when(a.AppliedDate, "applied "))
		}
		if a.LastChange != nil && a.LastChange.At != entryAt(a.AppliedDate) {
			out = append(out, when(a.LastChange, strings.ToLower(a.LastChange.Status)+" "))
		}
	}
	return out
}

// rel shows paths inside the Workspace relative to it, so messages fit their line and
// read the same on every machine.
func (m model) rel(s string) string {
	return strings.ReplaceAll(s, m.store.Root()+string(filepath.Separator), "")
}

// agoEntry is a Status entry's date relative to today, with SCHEMA's ~ for an
// approximate date.
func (m model) agoEntry(e *store.StatusEntry) string {
	r, _ := ago(e.At, m.now())
	if e.Approx {
		r = "~" + r
	}
	return r
}

func entryAt(e *store.StatusEntry) string {
	if e == nil {
		return ""
	}
	return e.At
}

// preview is the selected Application beside the list: status, documents, timeline,
// and the start of fit.md, jd.md and notes.md.
func (m model) preview(p pal, a store.Application, w, h int) string {
	inner := w - previewInset
	wrap := func(s string, c color.Color, maxLines int) string {
		lines := strings.Split(lipgloss.NewStyle().Width(inner).Render(s), "\n")
		if len(lines) > maxLines {
			lines = append(lines[:maxLines-1], "…")
		}
		return p.text(c).Render(strings.Join(lines, "\n"))
	}
	var b strings.Builder
	if a.Malformed != nil {
		b.WriteString(p.text(p.bad).Bold(true).Render("⚠ malformed meta.yml") + "\n\n" + wrap(m.rel(a.Malformed.Error()), p.dim, errorLines))
	} else {
		b.WriteString(wrap(a.Title, p.fg, titleLines) + "\n" + p.text(p.dim).Render(a.Company+" · "+a.Track) + "\n\n")
		label, c := p.docsLabel(a)
		b.WriteString(p.statusChip(a.Status) + p.fitText(a) + "\n")
		b.WriteString(m.fileMarks(p, a) + "   " + p.text(c).Render(label) + "\n\n")
		b.WriteString(p.heading("Timeline") + "\n" + m.timeline(p, a, true) + "\n")
		b.WriteString(p.heading("Fit") + "\n")
		if t := m.docText(a, "fit.md"); t != "" {
			b.WriteString(wrap(t, p.fg, docLines) + "\n")
		} else {
			b.WriteString(p.text(p.dim).Render("No fit.md yet") + "\n")
		}
		if beforeApplied(tableOf(a)) {
			if jd := m.docText(a, "jd.md"); jd != "" {
				b.WriteString("\n" + p.heading("JD") + "\n" + wrap(jd, p.sub, previewLines) + "\n" +
					p.text(p.accent).Render("enter") + p.text(p.dim).Render(" full JD") + "\n")
			}
		}
		b.WriteString("\n" + p.heading("Notes") + "\n")
		if t := m.docText(a, "notes.md"); t != "" {
			b.WriteString(wrap(t, p.fg, docLines))
		} else {
			b.WriteString(p.text(p.dim).Render("No notes · ") + p.text(p.accent).Render("n") + p.text(p.dim).Render(" to add"))
		}
		b.WriteString("\n\n" + p.text(p.dim).Render("? all keys"))
	}
	return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).Padding(0, 1).Render(b.String())
}

// fileMarks is the one-line document set: ✓ present, ○ missing. A source mark reflects
// content, not just presence, so it never reads "✓ resume" beside "not written".
func (m model) fileMarks(p pal, a store.Application) string {
	mark := func(ok bool, name string) string {
		if ok {
			return p.text(p.ok).Render("✓") + p.text(p.fg).Render(" "+name)
		}
		return p.text(p.dim).Render("○ " + name)
	}
	pdfs := "pdf"
	if n := len(a.PDFs); n > 0 {
		pdfs = fmt.Sprintf("%d pdf", n)
	}
	return strings.Join([]string{mark(a.HasJD, "jd"), mark(a.HasResume && m.hasContent(a, "resume.md"), "resume"),
		mark(a.HasCoverLetter && m.hasContent(a, "cover-letter.md"), "cover"), mark(len(a.PDFs) > 0, pdfs)}, "  ")
}

// hasContent reports whether a document holds non-whitespace text.
func (m model) hasContent(a store.Application, name string) bool {
	b, err := m.store.ReadDocument(a.Folder, name)
	return err == nil && strings.TrimSpace(string(b)) != ""
}

// timeline is the Status log, newest last, plus the close date for Leads and Drafts.
// Exact dates are shown where there is room for them.
func (m model) timeline(p pal, a store.Application, dates bool) string {
	var b strings.Builder
	for i, e := range a.StatusLog {
		dot, nameC := p.text(p.faint).Render("○"), p.dim
		if i == len(a.StatusLog)-1 {
			dot, nameC = p.text(p.accent).Render("●"), p.fg
		}
		when, whenC := m.agoEntry(&e), p.dim
		if m.futureDate(&e) {
			when, whenC = futureFlag, p.warn
		}
		if dates {
			r := when
			if !m.futureDate(&e) {
				r, _ = ago(e.At, m.now())
			}
			when = entryDate(&e) + "  " + r
		}
		b.WriteString(dot + " " + p.text(nameC).Render(padRight(e.Status, labelWidth+1)) + p.text(whenC).Render(when) + "\n")
	}
	if a.Closes != "" && beforeApplied(tableOf(a)) {
		r, days := ago(a.Closes, m.now())
		c := p.dim
		if days >= 0 && days <= soonDays {
			c = p.warn
		}
		date := ""
		if dates {
			date = a.Closes + "  "
		}
		b.WriteString(p.text(p.faint).Render("◌ ") + p.text(p.dim).Render(padRight("closes", labelWidth+1)+date) + p.text(c).Render(r) + "\n")
	}
	return b.String()
}

// docText is a document's text with Markdown headings and list markers dropped, for
// short previews. Missing files read as "".
func (m model) docText(a store.Application, name string) string {
	b, err := m.store.ReadDocument(a.Folder, name)
	if err != nil {
		return ""
	}
	var keep []string
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") && !strings.HasPrefix(l, "---") {
			keep = append(keep, strings.TrimLeft(l, "-* "))
		}
	}
	return strings.Join(keep, " ")
}
