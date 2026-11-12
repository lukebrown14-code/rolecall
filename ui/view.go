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
	shedEdge       = 1 // topBar detail levels: drop the space before the wordmark…
	shedCounts     = 2 // …then other screens' counts…
	shedWordmark   = 3 // …then the word "rolecall"…
	shedNames      = 4 // …then other screens' names, keeping their digits…
	shedMark       = 5 // …then the wordmark's mark
	previewLines   = 3
	daysPerWeek    = 7
	daysPerMonth   = 30
	weeksUntil     = 14 // relative dates switch from days to weeks here…
	monthsUntil    = 60 // …and from weeks to months here
)

// Right-hand columns, identical on every view: state · flag · Fit, Fit flush right.
// A column empty for every row of the current view is hidden.
const (
	slotState = 11 // "✓ ready", "not written", "rejected", "withdrawn"
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
	p := m.pal()
	var body string
	switch box := m.overlayView(p); {
	case m.tooSmall():
		body = m.tooSmallView(p)
	case box != "":
		body = m.overlayBody(p, m.screenView(m.dimmedPal()), box)
	default:
		body = m.screenView(p)
	}
	v := tea.NewView(asciiFold(body, m.ascii))
	v.MouseMode = tea.MouseModeCellMotion // clicks and the wheel; see mouse.go
	// Paint the theme's background so its roles keep their contrast whatever the
	// terminal's own colours. NO_COLOR drops colour but keeps bold and underline.
	if noColor() {
		v.Content = stripColor(v.Content)
	} else {
		v.BackgroundColor, v.ForegroundColor = p.bg, p.fg
	}
	return v
}

// screenView is the open detail view, or the current screen.
func (m model) screenView(p pal) string {
	if m.detail != nil {
		return m.detailView(p)
	}
	return m.listView(p)
}

// listView is a screen: a screen without Application tables draws its own view;
// otherwise top bar, views, the list (with the preview pane when wide), the feedback
// line and the key line.
func (m model) listView(p pal) string {
	if !m.hasTables() {
		return screenDefs[m.screen].render(m, p)
	}
	l := m.layout()
	w, bodyH := l.w, l.bodyHeight(1)
	rows := m.rows()
	listW, split := m.listPane(len(rows))
	divider := 0
	if split {
		divider = listW
	}
	previewW := w - listW - 1
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
			body = lipgloss.JoinHorizontal(lipgloss.Top, list, p.divider(bodyH), m.preview(p, rows[m.cursor], previewW, bodyH))
		}
	}
	keys := m.keyLine(p, w)
	if m.searching() {
		in := p.text(p.accent).Bold(true).Render(" / ") + p.text(p.fg).Render(m.text) + p.text(p.accent).Render("▏")
		keys = spread(in, p.keys(pairs(m.helpKeys(), groupAction, groupNav)), w-1)
	}
	// The bottom rule is always drawn; the status row below it holds a toast, an error,
	// the search input or the key line, one at a time.
	// The list holds the cursor, so with a preview beside it the list's rule is the accent.
	focus := focusNone
	if divider > 0 {
		focus = focusLeft
	}
	return m.frame(p, []string{m.viewRow(p)}, body, divider, focus, m.statusRow(p, w, keys))
}

// listPane is the list's width and whether the preview pane sits beside it, for a view
// of n rows. A mouse click reads the same geometry the frame is drawn with.
func (m model) listPane(n int) (int, bool) {
	l := m.layout()
	if l.wide() && n > 0 {
		return l.w - l.panel(previewShare, previewMin, previewMax) - 1, true
	}
	return l.w, false
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

// tabs is the tab bar's left side, fitted to the width, and the column each screen's
// tab starts at, so a click reads the same geometry the bar is drawn with. The active
// tab is bracketed where the others have spaces: marked without colour, and the same
// width, so nothing shifts. The detail level is the one every screen's bar fits at, so
// switching screens never sheds or restores a count.
func (m model) tabs(p pal) (string, []int) {
	room := m.layout().w - rightMargin
	on := p.activeTab()
	build := func(level, active int) (string, []int) {
		mark := " " + wordmark + " rolecall "
		switch {
		case level >= shedMark:
			mark = ""
		case level >= shedWordmark:
			mark = " " + wordmark + " "
		case level >= shedEdge:
			mark = wordmark + " rolecall "
		}
		out := p.text(p.accent).Bold(true).Render(mark)
		starts := make([]int, len(screenDefs))
		for si, d := range screenDefs {
			starts[si] = xansi.StringWidth(out) + 1
			digit, name, count := fmt.Sprintf("%d", si+1), " "+d.name, ""
			if d.hasTables {
				count = " " + m.screenCountLabel(si)
			}
			if si == active {
				out += on.Render("[" + digit + name + count + "]")
				continue
			}
			if level >= shedCounts {
				count = ""
			}
			if level >= shedNames {
				name = ""
			}
			out += " " + p.key().Render(digit) + p.text(p.fg).Render(name) + p.text(p.dim).Render(count) + " "
		}
		return out, starts
	}
	fits := func(level int) bool {
		for si := range screenDefs {
			if out, _ := build(level, si); xansi.StringWidth(out) > room {
				return false
			}
		}
		return true
	}
	level := 0
	for level < shedMark && !fits(level) {
		level++
	}
	return build(level, m.screen)
}

// topBar is the tab bar: the wordmark and the screens on the left and, at wide sizes,
// the pipeline summary on the right. Every tab keeps the same padding whether active
// or not, so nothing moves between screens. As it narrows it sheds, in order, other
// screens' counts, the word "rolecall", other screens' names (their digits stay) and
// the wordmark's mark; the active screen keeps its name and count at every width.
func (m model) topBar(p pal) string {
	l := m.layout()
	room := l.w - rightMargin
	out, _ := m.tabs(p)
	out = xansi.Truncate(out, room, "")
	if !l.wide() {
		return out
	}
	// The pulse fills the free right-hand side, dropping its later parts to fit.
	const pulseGap = 4
	parts := m.pulse(p)
	for len(parts) > 0 {
		pulse := strings.Join(parts, p.text(p.dim).Render(" · "))
		if xansi.StringWidth(out)+pulseGap+xansi.StringWidth(pulse) <= room {
			return spread(out, pulse, room)
		}
		parts = parts[:len(parts)-1]
	}
	return out
}

// pulseSoonDays is how close a close date counts in the pulse.
const pulseSoonDays = 2

// pulse is the one-glance pipeline summary: Applications still live, follow-ups due and
// Leads or Drafts closing within pulseSoonDays. It counts every Track, whatever the
// filter, and leaves out a part that is zero.
func (m model) pulse(p pal) []string {
	live, due, closing := 0, 0, 0
	for _, a := range m.apps {
		t := tableOf(a)
		if a.Malformed != nil || t == tableDismissed || t == tableClosed {
			continue
		}
		live++
		if _, flag, _ := m.slotText(a); flag != "" {
			due++
		}
		if _, days := ago(a.Closes, m.now()); beforeApplied(t) && a.Closes != "" && days >= 0 && days <= pulseSoonDays {
			closing++
		}
	}
	parts := []string{p.text(p.dim).Render(fmt.Sprintf("%d live", live))}
	if due > 0 {
		parts = append(parts, p.text(p.warn).Render(fmt.Sprintf("%d due", due)))
	}
	if closing > 0 {
		parts = append(parts, p.text(p.warn).Render(fmt.Sprintf("%d closes ≤%dd", closing, pulseSoonDays)))
	}
	return parts
}

// trackApplies reports whether the track filter changes the current screen: the
// Application screens and Stats. Profile and Settings ignore it, so they do not show it.
func (m model) trackApplies() bool {
	return len(m.tracks) > 0 && (m.hasTables() || screenDefs[m.screen].name == screenStats)
}

// trackSelector is the track filter on the context row, "t  [All]  SWE  Platform", or
// compact, "t [All]▸", where the full one does not fit. The key reads once, as "t".
func (m model) trackSelector(p pal, compact bool) string {
	if !m.trackApplies() {
		return ""
	}
	on := p.text(p.accent).Bold(true).Underline(true)
	// The active track is bracketed as well as underlined, so it is marked without colour.
	mark := func(name string) string {
		return p.text(p.accent).Render("[") + on.Render(name) + p.text(p.accent).Render("]")
	}
	key := p.key().Render("t")
	if compact {
		name := "All"
		if m.track > 0 {
			name = m.tracks[m.track-1]
		}
		return key + " " + mark(name) + p.text(p.dim).Render("▸")
	}
	opts := append([]string{"All"}, m.tracks...)
	parts := make([]string, len(opts))
	for i, t := range opts {
		parts[i] = p.text(p.dim).Render(t)
		if i == m.track {
			parts[i] = mark(t)
		}
	}
	return key + "  " + strings.Join(parts, "  ")
}

// viewRow is the frame's context row: the screen's views (cycled with Tab) on the left;
// on the right what a filter hides, then the track filter where it applies. As the row
// narrows the views drop their counts, the track filter goes compact, and then only
// the current view is named. The row is always present, so the layout never jumps.
func (m model) viewRow(p pal) string {
	w := m.viewWidth()
	pad := strings.Repeat(" ", inset)
	var lefts []string
	if !m.hasTables() {
		lefts = []string{pad + p.heading(screenDefs[m.screen].name)}
	} else {
		ts := m.views()
		current := func(label string) string {
			// Bracketed as well as bold and underlined: marked without colour.
			return p.text(p.accent).Render("[") + p.text(p.fg).Bold(true).Underline(true).Render(label) + p.text(p.accent).Render("]")
		}
		for _, f := range []struct {
			gap    string
			counts bool
			glyphs bool
		}{{"    ", true, true}, {"  ", true, true}, {"  ", false, true}, {"  ", false, false}} {
			parts := make([]string, len(ts))
			for i, t := range ts {
				label := t
				if f.counts {
					label = fmt.Sprintf("%s %s", t, m.countLabel(m.screen, t))
				}
				// A view of one Status leads with its glyph, in its colour.
				glyph := ""
				if s := tableStatus(t); s != "" && f.glyphs {
					glyph = p.statusMark(s) + " "
				}
				parts[i] = glyph + p.text(p.dim).Render(label)
				if i == m.table {
					parts[i] = glyph + current(label)
				}
			}
			lefts = append(lefts, pad+strings.Join(parts, f.gap))
		}
		if len(ts) > 1 {
			lefts = append(lefts, pad+current(ts[m.table]))
		}
	}
	scope := m.withChip(p, m.scopeLine(p))
	right := func(compact bool) string {
		var parts []string
		for _, s := range []string{scope, m.trackSelector(p, compact)} {
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "   ")
	}
	fits := func(left, right string) bool {
		return xansi.StringWidth(left)+gutter+xansi.StringWidth(right) <= w-rightMargin
	}
	for _, left := range lefts {
		for _, compact := range []bool{false, true} {
			if fits(left, right(compact)) {
				return contextRow(left, right(compact), w)
			}
		}
	}
	// Still too wide: the scope gives way, so the track filter always shows whole.
	left, sel := lefts[len(lefts)-1], m.trackSelector(p, true)
	if sel == "" {
		return contextRow(left, scope, w)
	}
	avail := w - rightMargin - xansi.StringWidth(left) - xansi.StringWidth(sel) - gutter - len("   ")
	if scope = xansi.Truncate(scope, max(0, avail), "…"); xansi.StringWidth(scope) > 1 {
		sel = scope + "   " + sel
	}
	return contextRow(left, sel, w)
}

// scopeLine is the shown-of-total count when a filter is on. The search chip beside it
// names the text and the track filter names the track.
func (m model) scopeLine(p pal) string {
	if !m.hasTables() || (m.track == 0 && m.text == "") {
		return ""
	}
	return p.text(p.dim).Render(fmt.Sprintf("%d of %d", len(m.rows()), m.totalCount(m.screen, m.views()[m.table])))
}

// listBody lays out the rows with section dividers. Rows get a blank line between them
// when everything fits, and run dense when the list is longer than the screen.
func (m model) listBody(p pal, rows []store.Application, w, h int) string {
	blocks, cur := m.listBlocks(p, rows, w, h)
	return scroll(p, blocks, cur, h)
}

// listBlocks is the list as blocks — dividers and two-line rows — and the cursor's block,
// spaced when they all fit in h and dense when they do not.
func (m model) listBlocks(p pal, rows []store.Application, w, h int) ([]block, int) {
	show := m.columns(rows)
	dividers := m.dividers(rows)
	build := func(gap bool) ([]block, int, int) {
		var blocks []block
		cur, total := 0, 0
		for i, a := range rows {
			if d, ok := dividers[i]; ok {
				lines := []string{p.groupHeader(d.label, tableStatus(d.label), d.n, w)}
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
			blocks = append(blocks, block{lines: lines, row: true, idx: i})
			total += len(lines)
		}
		return blocks, cur, total
	}
	blocks, cur, total := build(true)
	if total > h {
		blocks, cur, _ = build(false)
	}
	return blocks, cur
}

type block struct {
	lines []string
	row   bool // an Application, as opposed to a section divider
	idx   int  // the row's index in the view, when row is set
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
	start, end := scrollWindow(blocks, cur, h)
	count := func(bs []block) int {
		n := 0
		for _, b := range bs {
			if b.row {
				n++
			}
		}
		return n
	}
	// The cues sit at the rows' text column, past the selection bar.
	const cueIndent = "  "
	var out []string
	if start > 0 {
		out = append(out, cueIndent+p.moreCue(true, count(blocks[:start])))
	}
	for _, b := range blocks[start:end] {
		out = append(out, b.lines...)
	}
	// The ↓ cue sits on the body's last line, where it is on every screen.
	for len(out) < h-1 {
		out = append(out, "")
	}
	if end < len(blocks) {
		out = append(out, cueIndent+p.moreCue(false, count(blocks[end:])))
	}
	for len(out) < h {
		out = append(out, "")
	}
	return strings.Join(out[:h], "\n")
}

// scrollWindow is the blocks shown in h lines with block cur in view: [start, end). A
// "↑ n more" line takes the first line when start is past the top.
func scrollWindow(blocks []block, cur, h int) (start, end int) {
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
	start, end = 0, fit(0)
	for cur >= end && start < cur {
		start++
		end = fit(start)
	}
	// A section heading never sits alone above the "more" cue.
	for end < len(blocks) && end-1 > start && !blocks[end-1].row {
		end--
	}
	return start, end
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
		state = "rejected"
	case a.Status == store.StatusWithdrawn:
		state = "withdrawn"
	}
	if t != tableDismissed {
		fit = a.Fit
		if fit == "" {
			fit = "—"
		}
	}
	return state, flag, fit
}

// stateColor is the colour of a row's state slot.
func (p pal) stateColor(a store.Application, dimC color.Color) color.Color {
	switch {
	case tableOf(a) == tableDraft:
		if _, c := p.docsLabel(a); c != p.dim {
			return c
		}
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
		return []string{line.Render(bar + st(p.bad).Bold(true).Render("⚠ "+m.malformedTitle(a))),
			line.Render(bar + st(dimC).Render("  "+xansi.Truncate(m.malformedReason(a), w-ellipsis-statusMarkWidth, "…")))}
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
	// The Status glyph leads the title; the second line lines up under the title.
	mark := st(p.statusColor(a.Status)).Render(p.statusGlyph(a.Status)) + st(p.fg).Render(" ")
	left := bar + mark + st(titleC).Bold(sel).Render(xansi.Truncate(a.Title, max(minTitle, w-xansi.StringWidth(right)-ellipsis-statusMarkWidth), "…"))
	gap := st(p.fg).Render(strings.Repeat(" ", max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))))
	meta := strings.Join(m.meta(p, a, st, dimC), st(dimC).Render(" · "))
	lines := []string{line.Render(left + gap + right), line.Render(xansi.Truncate(bar+st(p.fg).Render("  ")+meta, w, "…"))}
	if m.flash != "" && a.Folder == m.flash {
		lines = p.flashRow(lines, w)
	}
	return lines
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
		lines := wrapWords(s, inner)
		if len(lines) > maxLines {
			lines = append(lines[:maxLines-1], "…")
		}
		return p.text(c).Render(strings.Join(lines, "\n"))
	}
	var b strings.Builder
	if a.Malformed != nil {
		b.WriteString(p.text(p.bad).Bold(true).Render("⚠ "+m.malformedTitle(a)) + "\n\n" + wrap(m.malformedReason(a), p.fg, errorLines) +
			"\n\n" + p.text(p.accent).Render("e") + p.text(p.dim).Render(" fix in editor"))
	} else {
		b.WriteString(wrap(a.Title, p.fg, titleLines) + "\n" + p.text(p.dim).Render(a.Company+" · "+a.Track) + "\n\n")
		b.WriteString(keepTogether(inner, "", p.statusChip(a.Status), p.fitText(a)) + "\n")
		b.WriteString(m.fileMarks(p, a) + "\n" + p.docsMeter(a) + "\n\n")
		b.WriteString(p.heading("Timeline") + "\n" + m.timeline(p, a, true) + "\n")
		b.WriteString(p.heading("Fit") + "\n")
		if t := m.docText(a, fitFile); t != "" {
			b.WriteString(wrap(t, p.fg, docLines) + "\n")
		} else {
			b.WriteString(p.text(p.dim).Render("No fit.md yet") + "\n")
		}
		if beforeApplied(tableOf(a)) {
			if jd := m.docText(a, jdFile); jd != "" {
				b.WriteString("\n" + p.heading("JD") + "\n" + wrap(jd, p.sub, previewLines) + "\n" +
					p.key().Render("enter") + p.text(p.dim).Render(" full JD") + "\n")
			}
		}
		b.WriteString("\n" + p.heading("Notes") + "\n")
		if t := m.docText(a, notesFile); t != "" {
			b.WriteString(wrap(t, p.fg, docLines))
		} else {
			b.WriteString(p.text(p.dim).Render("No notes · ") + p.key().Render("n") + p.text(p.dim).Render(" to add"))
		}
	}
	return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).Padding(0, 1).Render(b.String())
}

// fileMarks is the one-line document set, "✓ JD  ○ Resume  ○ Cover letter  ○ PDF":
// ✓ ready, ○ missing. A source document is ready when it has text, not merely a file.
func (m model) fileMarks(p pal, a store.Application) string {
	parts := make([]string, 0, len(docChips(a)))
	for _, c := range docChips(a) {
		if c.ready {
			parts = append(parts, p.text(p.ok).Render("✓")+p.text(p.fg).Render(" "+c.name))
		} else {
			parts = append(parts, p.text(p.dim).Render("○ "+c.name))
		}
	}
	return strings.Join(parts, "  ")
}

// timeline is the Status log, newest last, plus the close date for Leads and Drafts.
// Exact dates are shown where there is room for them.
func (m model) timeline(p pal, a store.Application, dates bool) string {
	var b strings.Builder
	for i, e := range a.StatusLog {
		// Earlier entries are faint; the current one is in its Status's colour, bold.
		dot, name := p.text(p.faint).Render(p.statusGlyph(e.Status)), p.text(p.dim)
		if i == len(a.StatusLog)-1 {
			dot, name = p.statusMark(e.Status), p.text(p.fg).Bold(true)
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
		b.WriteString(dot + " " + name.Render(padRight(e.Status, labelWidth+1)) + p.text(whenC).Render(when) + "\n")
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
		b.WriteString(p.text(p.faint).Render("→ ") + p.text(p.dim).Render(padRight("closes", labelWidth+1)+date) + p.text(c).Render(r) + "\n")
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
