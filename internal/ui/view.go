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
	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Layout constants for the list view.
const (
	fallbackWidth  = 100
	fallbackHeight = 30
	rowPad         = 1 // keeps the last label off the edge of the selection bar
	soonDays       = 7 // "closes in Nd" turns amber at or below this
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

// joinColumns sets blocks side by side inside room: each w wide, or wider for a longer
// line; failing that, each as wide as its widest line and a gutter. When neither fits it
// stacks them. The last block needs no gap after it.
func joinColumns(cols []string, w, room int) string {
	fit := func(least int) ([]int, bool) {
		widths := make([]int, len(cols))
		total := 0
		for i, c := range cols {
			widths[i] = lipgloss.Width(c)
			if i < len(cols)-1 {
				widths[i] = max(least, widths[i]+gutter)
			}
			total += widths[i]
		}
		return widths, total <= room
	}
	widths, ok := fit(w)
	if !ok {
		widths, ok = fit(0)
	}
	if !ok {
		return strings.Join(cols, "\n\n")
	}
	styled := make([]string, len(cols))
	for i, c := range cols {
		styled[i] = lipgloss.NewStyle().Width(widths[i]).Render(c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, styled...)
}

// ago renders a date relative to today: "today", "in 4d", "3d ago", "2w ago", "2mo ago",
// and past about three months the month itself, "Jan 2025", which reads better than
// "21mo ago".
func ago(date string, now time.Time) (string, int) {
	t, days, ok := daysFrom(date, now)
	if !ok {
		return date, 0
	}
	n := max(days, -days)
	switch {
	case days == 0:
		return "today", 0
	case n >= monthYearAfter:
		return t.Format("Jan 2006"), days
	case days > 0:
		return "in " + span(n), days
	}
	return span(n) + " ago", days
}

// since is how long an Application has gone without a change, "9mo", for the follow-up
// flag: always a span, since "quiet Feb 2026" would read as a date.
func (m model) since(e *store.StatusEntry) string {
	_, days, ok := daysFrom(e.At, m.now())
	if !ok {
		return ""
	}
	return span(max(-days, 0))
}

// futureFlag marks a Status date after today: almost always a data error, so it is
// flagged rather than shown as a normal "in 8mo" relative date.
const futureFlag = "⚠ future date"

// futureDate reports whether a Status entry's date is after today.
func (m model) futureDate(e *store.StatusEntry) bool {
	if e == nil {
		return false
	}
	t, ok := parseDay(e.At)
	return ok && t.After(day(m.now()))
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
	switch {
	case noColor(): // NO_COLOR wins over the inherit option
		v.Content = stripColor(v.Content)
	case m.inherit:
		v.Content = inheritANSI(v.Content)
	default:
		v.BackgroundColor, v.ForegroundColor = p.Bg, p.Fg
	}
	return v
}

// screenView is the open detail view, or the current screen.
func (m model) screenView(p pal) string {
	if m.detail != nil && !m.sheetShown() {
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
	w, bodyH := l.w, m.listHeight()
	rows := m.rows()
	listW, split := m.listPane(len(rows))
	divider := 0
	if split {
		divider = listW
	}
	previewW := w - listW - 1
	var body string
	switch {
	case len(m.apps) == 0:
		body = lipgloss.Place(w, bodyH, lipgloss.Center, lipgloss.Center, m.welcome(p))
	case len(rows) == 0:
		switch {
		case m.text != "":
			msg, hints := m.emptySearchLine()
			body = m.emptyBody(p, w, bodyH, msg, hints)
		case m.movedOut():
			// A move just took this view's last row elsewhere: say where it went and keep
			// a celebration whole, rather than showing "nothing here" under a toast.
			body = lipgloss.Place(w, bodyH, lipgloss.Center, lipgloss.Center, m.movedOutView(p))
		default:
			msg, hints := m.emptyViewLine(m.views()[m.table])
			body = m.emptyBody(p, w, bodyH, msg, hints)
		}
	default:
		list := lipgloss.NewStyle().PaddingLeft(1).Width(listW).Render(m.listBody(p, rows, listW-gutter, bodyH))
		body = list
		if split {
			sel, _ := m.selected()
			body = lipgloss.JoinHorizontal(lipgloss.Top, list, p.Divider(bodyH), m.preview(p, sel, previewW, bodyH))
		}
	}
	keys := m.keyLine(p, w)
	if m.searching() {
		in := p.Text(p.Fg).Bold(true).Render(" / ") + p.Text(p.Fg).Render(m.text) + p.Text(p.Fg).Render("▏")
		keys = spread(in, p.Keys(pairs(m.helpKeys(), groupAction, groupFilter, groupNav)), w-1)
	}
	// The bottom rule is always drawn; the status row below it holds a toast, an error,
	// the search input or the key line, one at a time.
	// The list holds the cursor, so with a preview beside it the list's rule is the accent.
	focus := focusNone
	if divider > 0 {
		focus = focusLeft
	}
	if m.sheetShown() {
		body = m.withSheet(p, body)
		focus = focusNone
	}
	return m.frame(p, []string{m.viewRow(p)}, body, divider, focus, m.breadcrumbLine(p)+"\n"+keys)
}

// listPane is the list's width and whether the preview pane sits beside it, for a view
// of n rows. A mouse click reads the same geometry the frame is drawn with.
func (m model) listPane(n int) (int, bool) {
	l := m.layout()
	if l.detail == detailPane && n > 0 {
		return l.listW, true
	}
	return l.w, false
}

// feedbackVisible reports whether the status area is showing what just happened: a toast,
// an error or a build in flight, and not the search input, which takes the whole row.
func (m model) feedbackVisible() bool {
	return !m.searching() && (m.err != nil || m.toast != "" || m.building != nil)
}

// statusAreaLines is the status area's height: one row for the key line, or two while the
// feedback line sits above it.
const (
	statusAreaPlain    = 1
	statusAreaFeedback = 2
)

// frameStatusLines is the lines the status area needs: the feedback line and the key line
// when something just happened, otherwise the key line alone.
func (m model) frameStatusLines() int {
	if m.hasTables() && (m.detail == nil || m.sheetShown()) {
		return statusAreaFeedback // the breadcrumb line, which holds the feedback, then the keys
	}
	if m.feedbackVisible() {
		return statusAreaFeedback
	}
	return statusAreaPlain
}

// statusRow is the frame's last rows: the feedback line over the key line when there is
// one, so a toast or error never wipes the key hints; otherwise the line the caller
// prepared (the search input while typing, else the key line). The rule above is never
// replaced.
func (m model) statusRow(p pal, w int, fallback string) string {
	if m.searching() {
		return fallback // the search input shows while typing, even over an error
	}
	if m.feedbackVisible() {
		return m.feedbackLine(p, w) + "\n" + fallback
	}
	return fallback
}

// welcome is the first-run screen, for a Workspace with no Applications at all: the
// first three moves instead of empty panes.
func (m model) welcome(p pal) string {
	title := p.Text(p.Fg).Bold(true).Render(wordmark + " rolecall")
	msg, _ := m.emptyViewLine(m.views()[m.table])
	hints := m.availableHints(addLeadHint)
	step := func(k, desc string) string {
		return p.Key().Render(padRight(k, gutter+1)) + p.Text(p.Fg).Render(desc)
	}
	steps := lipgloss.JoinVertical(lipgloss.Left,
		step(fmt.Sprint(settingsScreen+1), "pick an Editor"), step("?", "see every key"))
	line := strings.Join(m.emptyLines(p, m.layout().w-2*gutter, msg, hints), "\n")
	return lipgloss.JoinVertical(lipgloss.Left, title, "", line, "", "  "+strings.ReplaceAll(steps, "\n", "\n  "))
}

// movedOut reports that a move just took the last row out of this view.
func (m model) movedOut() bool { return m.movedTo != "" }

// viewHoldsStatus reports whether a Status lives in a view of the current screen, so the
// empty state can say which key follows the moved row.
func (m model) viewHoldsStatus(status string) bool {
	screen, table := StatusLocation(status)
	if screen != screenDefs[m.screen].name {
		return false
	}
	for _, t := range m.views() {
		if t == table {
			return true
		}
	}
	return false
}

// movedOutView is the empty view after a move: the Offer celebration in full, or a quiet
// line naming where the Application went, and how to follow it.
func (m model) movedOutView(p pal) string {
	line := p.Text(p.Fg).Bold(true).Render(m.toast)
	if m.movedTo == store.StatusOffer {
		line = p.Text(p.Gold).Bold(true).Render(m.toast)
	}
	hint := "press ← → to follow it"
	if !m.viewHoldsStatus(m.movedTo) {
		hint = "press 1-6 to find it"
	}
	return lipgloss.JoinVertical(lipgloss.Center, line, "", p.Text(p.Dim).Render(hint))
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
	if r := m.barKeys(p); r != "" {
		room -= gutter + xansi.StringWidth(r)
	}
	wide := m.layout().wide()
	// The activity sparkline sits beside the Stats tab at wide sizes, until the bar is
	// short of room.
	spark := " " + p.Text(p.Accent).Faint(true).Render(activitySparkline(p, reporting.WeeklyActivity(m.apps, m.now(), activityWeeks)))
	build := func(level, active int) (string, []int) {
		// The brand mark leads the bar at wide sizes only, bold in the accent and one cell
		// in from the edge (twelve cells with its trailing space); below that the bar is the
		// tabs alone.
		mark := ""
		if wide {
			switch {
			case level >= shedMark:
			case level >= shedWordmark:
				mark = " " + wordmark + " "
			default:
				mark = " " + wordmark + " rolecall "
			}
		}
		out := p.Text(p.Accent).Bold(true).Render(mark)
		starts := make([]int, len(screenDefs))
		for si, d := range screenDefs {
			starts[si] = xansi.StringWidth(out) + 1
			digit, name, count := fmt.Sprintf("%d", si+1), " "+d.name, ""
			if d.hasTables {
				count = " " + m.screenCountLabel(si)
			}
			tail := ""
			if wide && level < shedCounts && d.name == screenStats {
				tail = spark
			}
			if si == active {
				out += p.SelChip(digit+name+count) + tail
				continue
			}
			if level >= shedCounts {
				count = ""
			}
			if level >= shedNames {
				name = ""
			}
			sep := " "
			if name == "" && count == "" {
				sep = "" // a bare digit needs no trailing space, so the tabs do not spread
			}
			out += " " + p.Key().Render(digit) + p.Text(p.Fg).Render(name) + p.Text(p.Dim).Render(count) + tail + sep
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

// topBar is the tab bar: the wordmark and the screens. Every tab keeps the same padding
// whether active or not, so nothing moves between screens. As it narrows it sheds, in
// order, other screens' counts, the word "rolecall", other screens' names (their digits
// stay) and the wordmark's mark; the active screen keeps its name and count at every width.
func (m model) topBar(p pal) string {
	out, _ := m.tabs(p)
	w := m.layout().w
	if r := m.barKeys(p); r != "" {
		return spread(out, r, w-rightMargin)
	}
	return xansi.Truncate(out, w-rightMargin, "")
}

// barKeys is the tab bar's right end, "space menu  ? help", at wide sizes: the two keys
// that work everywhere. An open Settings field takes space as text, so then it is empty.
func (m model) barKeys(p pal) string {
	if !m.layout().wide() || m.fieldOpen() {
		return ""
	}
	return p.KeyRow([][2]string{{spaceKey, "menu"}, {"?", "help"}})
}

// trackApplies reports whether the track filter changes the current screen: the
// Application screens and Stats. Profile and Settings ignore it, so they do not show it.
func (m model) trackApplies() bool {
	return len(m.tracks) > 0 && (m.hasTables() || screenDefs[m.screen].name == screenStats)
}

// trackForm is how much of the Track filter a row has room for.
type trackForm int

const (
	trackFull      trackForm = iota // every option with its count
	trackCompact                    // the active option only
	trackCollapsed                  // "t:All"
)

// trackSelector is the track filter on the context row, "t  [All 15]  SWE 10  Platform 5",
// or compact, "t [All 15]▸", or collapsed, "t:All", where the full one does not fit. The
// key reads once, as "t". Each option carries how many Applications are on it.
func (m model) trackSelector(p pal, form trackForm) string {
	if !m.trackApplies() {
		return ""
	}
	key := p.Key().Render("t")
	if form == trackCollapsed {
		return key + p.Text(p.Dim).Render(":"+m.trackName())
	}
	mark := p.SelChip
	label := func(i int) string {
		name := allView
		if i > 0 {
			name = m.tracks[i-1]
		}
		return fmt.Sprintf("%s %d", name, m.trackTotal(i))
	}
	if form == trackCompact {
		return key + " " + mark(label(m.track)) + p.Text(p.Dim).Render("▸")
	}
	parts := make([]string, len(m.tracks)+1)
	for i := range parts {
		parts[i] = p.Text(p.Dim).Render(label(i))
		if i == m.track {
			parts[i] = mark(label(i))
		}
	}
	return p.Text(p.Dim).Render("TRACK ") + key + " " + strings.Join(parts, " ")
}

// trackTotal is the Applications on a Track (0 is All). On a table screen it is the rows
// the open view would show with that Track chosen, so the chosen option matches the list;
// elsewhere it is every one the Workspace lists.
func (m model) trackTotal(track int) int {
	if m.hasTables() {
		with := m
		with.track = track
		return len(with.rows())
	}
	if track == 0 {
		return len(m.apps)
	}
	n := 0
	for _, a := range m.apps {
		if a.Track == m.tracks[track-1] {
			n++
		}
	}
	return n
}

// trackName is the Track filter's name: All, or the Track.
func (m model) trackName() string {
	if m.track > 0 && m.track <= len(m.tracks) {
		return m.tracks[m.track-1]
	}
	return allView
}

// breadcrumbLine is the footer's first line on a table screen: where the Owner is, as
// "Applied › All · All tracks · 1/9 · list", and at the right what just happened, a toast or
// an error with its keys. The breadcrumb gives way first: its focus, then the Track.
func (m model) breadcrumbLine(p pal) string {
	w := m.layout().w
	right := ""
	if m.feedbackVisible() {
		msg, keys := m.feedbackParts(p)
		// The message gives way, never the keys that recover from it.
		// An error matters more than where the cursor is, so it may take the whole line.
		room := max(0, w-rightMargin-gutter-inset-breadcrumbMin)
		if m.err != nil {
			room = max(0, w-rightMargin-inset)
		}
		if keys != "" {
			room -= len(keyGap) + xansi.StringWidth(keys)
		}
		right = joinNonEmpty(keyGap, xansi.Truncate(msg, max(1, room), "…"), keys)
	}
	room := w - rightMargin
	if right != "" {
		room -= gutter + xansi.StringWidth(right)
	}
	return contextRow(m.breadcrumbLeft(p, room), right, w)
}

// keyGap separates the feedback from its keys, as the key line separates its keys.
const keyGap = "  "

// spaceKey is the space bar as the key line names it.
const spaceKey = "space"

// breadcrumbMin is the cells a toast leaves the breadcrumb: its screen and position.
const breadcrumbMin = 16

// joinNonEmpty joins the parts that have text.
func joinNonEmpty(sep string, parts ...string) string {
	var keep []string
	for _, s := range parts {
		if s != "" {
			keep = append(keep, s)
		}
	}
	return strings.Join(keep, sep)
}

// breadcrumb is "Applied › All · All tracks · 1/9 · list", the Track abbreviated when short
// and the focus left off when noFocus.
func (m model) breadcrumb(p pal, short, noFocus bool) string {
	rows := m.rows()
	pos := 0
	if len(rows) > 0 {
		pos = m.cursor + 1
	}
	if m.detail != nil {
		for i, r := range rows {
			if r.Folder == m.detail.folder {
				pos = i + 1
			}
		}
	}
	focus := "list"
	if m.detail != nil {
		focus = "detail"
	}
	track := "All tracks"
	if m.track > 0 {
		track = m.trackName()
	}
	if short {
		track = "All"
		if m.track > 0 {
			track = m.trackName()
			if r := []rune(track); len(r) > abbrevTrack {
				track = string(r[:abbrevTrack])
			}
		}
	}
	parts := []string{track, fmt.Sprintf("%d/%d", pos, len(rows))}
	// A view named like its screen (Leads › Leads) says nothing twice.
	if v := m.views()[m.table]; v != screenDefs[m.screen].name {
		parts = append([]string{v}, parts...)
	}
	out := p.Text(p.Fg).Bold(true).Render(screenDefs[m.screen].name) + p.Text(p.Dim).Render(" › "+strings.Join(parts, " · "))
	if !noFocus {
		out += p.Text(p.Dim).Render(" · ") + p.Text(p.Accent).Bold(true).Render(focus)
	}
	return out
}

// breadcrumbLeft is the fullest breadcrumb that fits room cells after the inset.
func (m model) breadcrumbLeft(p pal, room int) string {
	l := m.layout()
	pad := strings.Repeat(" ", inset)
	narrow := l.class != classMedium && l.class != classWide
	for _, c := range []struct{ short, noFocus bool }{{false, narrow}, {false, true}, {true, true}} {
		if s := pad + m.breadcrumb(p, c.short, c.noFocus); xansi.StringWidth(s) <= room {
			return s
		}
	}
	if room < inset+breadcrumbMin {
		return "" // a stub would say nothing; the line is the error's
	}
	return xansi.Truncate(pad+m.breadcrumb(p, true, true), room, "…")
}

// abbrevTrack is how many letters of a Track name a narrow breadcrumb keeps.
const abbrevTrack = 3

// withSheet stacks the detail sheet under the list part: a rule in the accent, then the
// detail, together exactly the body's height.
func (m model) withSheet(p pal, list string) string {
	a, ok := m.application(m.detail.folder)
	if !ok {
		return list
	}
	l := m.layout()
	listH, detailH, _ := l.sheetHeights(tableContext)
	rule := p.PaneRule(true).Render(strings.Repeat("─", l.w))
	return strings.Join([]string{fitLines(list, listH), rule, strings.Join(m.sheetLines(p, a, l.w, detailH), "\n")}, "\n")
}

// sheetLines is the detail sheet: a title line, the tabs, then the tab's content, h lines.
func (m model) sheetLines(p pal, a store.Application, w, h int) []string {
	pad := strings.Repeat(" ", inset)
	title := pad + p.Text(p.Fg).Bold(true).Render(appName(a.Title)) + p.Text(p.Dim).Render(" · "+a.Company)
	if a.Malformed != nil {
		title = pad + p.Text(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a))
	} else {
		title = spread(xansi.Truncate(title, max(0, w-rightMargin-gutter-xansi.StringWidth(a.Status)-markWidth), "…"), p.StatusChip(a.Status), w-rightMargin)
	}
	out := []string{title}
	var content []string
	if a.Malformed != nil {
		for _, l := range wrapWords(m.malformedReason(a), w-1-rightMargin) {
			content = append(content, p.Text(p.Fg).Render(l))
		}
		content = append(content, "", p.Key().Render("e")+p.Text(p.Dim).Render(" fix in editor"))
		out = append(out, "")
	} else {
		out = append(out, " "+m.detailTabsRow(p, a))
		content = strings.Split(m.detailContent(p, a, w), "\n")
	}
	for i, line := range content {
		content[i] = " " + line
	}
	from := 0
	for i, line := range content {
		if strings.Contains(xansi.Strip(line), "▌") {
			from = i - (h-len(out))/bothSides
		}
	}
	out = append(out, p.ClipLines(content, from, max(0, h-len(out)), " ")...)
	return append(out[:min(len(out), h)], make([]string, max(0, h-len(out)))...)
}

// markWidth is the cells a Status chip takes beyond its name: the glyph and its space.
const markWidth = 2

// viewRow is the frame's first context row. On an Application screen it is the pipeline
// strip, which is also the status filter (s, or ← → to move it). Elsewhere it is the
// screen's heading, with the search chip and the Track filter on the right where they
// apply. The row is always present, so the layout never jumps.
func (m model) viewRow(p pal) string {
	w := m.viewWidth()
	if m.hasTables() {
		return m.filterRow(p)
	}
	left := strings.Repeat(" ", inset) + p.Heading(screenDefs[m.screen].name)
	for _, form := range []trackForm{trackFull, trackCompact, trackCollapsed} {
		right := m.withChip(p, m.trackSelector(p, form))
		if xansi.StringWidth(left)+gutter+xansi.StringWidth(right) <= w-rightMargin || form == trackCollapsed {
			return contextRow(left, right, w)
		}
	}
	return left
}

// scopeLine is the shown-of-total count when a filter is on. The search chip beside it
// names the text and the track filter names the track.
func (m model) scopeLine(p pal) string {
	if !m.hasTables() || (m.track == 0 && m.text == "") {
		return ""
	}
	return p.Text(p.Dim).Render(fmt.Sprintf("%d of %d", len(m.rows()), m.totalCount(m.screen, m.views()[m.table])))
}

// listBody lays out the rows with section dividers. Rows get a blank line between them
// when everything fits, and run dense when the list is longer than the screen.
func (m model) listBody(p pal, rows []store.Application, w, h int) string {
	blocks, cur := m.listBlocks(rows)
	return scroll(p, m, rows, fixedSlots, w, blocks, cur, h)
}

// rowLines is a list row's height: the title line and the meta line. Rows in a group sit
// together; only a group's rule has air above it.
const rowLines = 2

// listBlocks is the list as blocks — dividers and two-line rows — and the cursor's block.
// Heights are measured without rendering: a row is two lines and a divider one or two.
// This is the one place a block's height is computed; renderBlock draws content and
// scroll pads it.
func (m model) listBlocks(rows []store.Application) ([]block, int) {
	dividers := m.dividers(rows)
	dividerHeight := func(i int) int {
		n := 1
		if i > 0 {
			n++ // the air above a section
		}
		return n
	}
	var blocks []block
	cur := 0
	for i := range rows {
		if d, ok := dividers[i]; ok {
			blocks = append(blocks, block{height: dividerHeight(i), padTop: i > 0, label: d.label, n: d.n, rollup: d.rollup})
		}
		if i == m.cursor {
			cur = len(blocks)
		}
		blocks = append(blocks, block{height: rowLines, row: true, idx: i})
	}
	return blocks, cur
}

// renderBlock renders one block's content lines, without the height padding: scroll pads
// every block to the height listBlocks measured. It is a var so a test can count the calls
// and assert only the visible blocks are rendered.
var renderBlock = func(m model, p pal, b block, rows []store.Application, show slotSet, w int) []string {
	if b.row {
		return m.row(p, rows[b.idx], b.idx == m.cursor, show, w)
	}
	return []string{p.GroupHeader(b.label, tableStatus(b.label), b.n, b.rollup, w)}
}

type block struct {
	height int    // lines the block occupies, without rendering
	row    bool   // an Application, as opposed to a section divider
	padTop bool   // the blank line that spaces a section goes above the header
	idx    int    // the row's index in the view, when row is set
	label  string // the divider's label, when row is false
	n      int    // the divider's count, when row is false
	rollup string // the divider's rollups, " · 1 follow-up due", when row is false
}

type divider struct {
	label  string
	n      int
	rollup string
}

// dividers are the section headings drawn before a row: one per Status in the Applied
// screen's All view, one over a single view's rows, and Expired at the end of Leads
// and Draft.
func (m model) dividers(rows []store.Application) map[int]divider {
	div := map[int]divider{}
	switch m.views()[m.table] {
	case allView:
		for i, a := range rows {
			if _, t := StatusLocation(a.Status); i == 0 || t != tableOf(rows[i-1]) {
				div[i] = divider{label: t, n: m.count(m.screen, t)}
			}
		}
	case tableLeads, tableDraft:
		if m.leadsNewest && m.views()[m.table] == tableLeads {
			break // newest first mixes expired Leads in with the rest
		}
		for i, a := range rows {
			if m.expired(a) {
				div[i] = divider{label: "Expired", n: len(rows) - i}
				break
			}
		}
	}
	// A view of one group still opens with its rule, "◌ Leads · 3 ────", up to the
	// Expired section when there is one.
	if v := m.views()[m.table]; v != allView && len(rows) > 0 {
		if _, ok := div[0]; !ok {
			div[0] = divider{label: v, n: groupEnd(div, 0, len(rows))}
		}
	}
	for i, d := range div {
		d.rollup = m.rollup(d.label, rows[i:groupEnd(div, i, len(rows))])
		div[i] = d
	}
	return div
}

// groupEnd is the index after the group that starts at row i: the next divider's row, or the end.
func groupEnd(div map[int]divider, i, n int) int {
	end := n
	for j := range div {
		if j > i && j < end {
			end = j
		}
	}
	return end
}

// rollup is what a group's rule adds after its count when non-zero: the Follow-ups due and
// the Applications whose documents are Ready, " · 1 follow-up due · 2 Ready". Closed and
// Expired groups have nothing left to nudge or send, so they carry none.
func (m model) rollup(label string, group []store.Application) string {
	if label == tableClosed || label == "Expired" {
		return ""
	}
	due, ready := 0, 0
	for _, a := range group {
		if _, flag, _ := m.slotText(a); flag != "" {
			due++
		}
		if a.Malformed == nil && a.Documents == store.DocsReady {
			ready++
		}
	}
	out := ""
	if due > 0 {
		out += fmt.Sprintf(" · %d follow-up due", due)
	}
	if ready > 0 {
		out += fmt.Sprintf(" · %d Ready", ready)
	}
	return out
}

func tableOf(a store.Application) string { _, t := StatusLocation(a.Status); return t }

// expired is a Lead or Draft whose close date has passed.
func (m model) expired(a store.Application) bool {
	if !beforeApplied(tableOf(a)) {
		return false
	}
	t, ok := parseDay(a.Closes)
	return ok && t.Before(day(m.now()))
}

// scroll fills h lines with blocks, keeping block cur in view. "↑/↓ n more" appear only
// when rows are really hidden, and count rows, not dividers. Only the visible blocks are
// rendered.
func scroll(p pal, m model, rows []store.Application, show slotSet, w int, blocks []block, cur, h int) string {
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
		out = append(out, cueIndent+p.MoreCue(true, count(blocks[:start])))
	}
	for _, b := range blocks[start:end] {
		lines := renderBlock(m, p, b, rows, show, w)
		for len(lines) < b.height {
			if b.padTop {
				lines = append([]string{""}, lines...)
			} else {
				lines = append(lines, "")
			}
		}
		out = append(out, lines...)
	}
	// The ↓ cue sits on the body's last line, where it is on every screen.
	for len(out) < h-1 {
		out = append(out, "")
	}
	if end < len(blocks) {
		out = append(out, cueIndent+p.MoreCue(false, count(blocks[end:])))
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
		for end < len(blocks) && used+blocks[end].height <= avail {
			used += blocks[end].height
			end++
		}
		for end < len(blocks) && used+1 > avail && end > start {
			end--
			used -= blocks[end].height
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

// slotSet is the width of each right-hand column. The columns are fixed (see the layout
// constants), so every row's columns start at the same cell and nothing wraps.
type slotSet struct{ stateW, flagW, fitW int }

// fixedSlots are the list row's right-hand columns: stage and date, Follow-up due flag, Fit.
var fixedSlots = slotSet{stateW: stageColumn, flagW: flagColumn, fitW: fitColumn}

// stageNames are the Status names short enough to leave room for a date in the stage column.
var stageNames = map[string]string{store.StatusInterviewing: "Interv.", store.StatusRejected: "Rej.", store.StatusWithdrawn: "Withdr.", store.StatusDismissed: "Dism."}

// stageText is the latest stage and its date, "Applied 3d", cut back to the date alone
// (or the stage alone) when both do not fit the column: it is never truncated.
func (m model) stageText(a store.Application) string {
	e := a.LastChange
	if e == nil {
		return ""
	}
	date, _ := ago(e.At, m.now())
	date = strings.TrimSuffix(strings.TrimPrefix(date, "in "), " ago")
	if e.Approx {
		date = "~" + date
	}
	name := e.Status
	if short, ok := stageNames[name]; ok && xansi.StringWidth(name+" "+date) > stageColumn {
		name = short
	}
	if text := name + " " + date; xansi.StringWidth(text) <= stageColumn {
		return text
	}
	if xansi.StringWidth(date) <= stageColumn {
		return date
	}
	return name
}

// fitDot is the empty Fit in the list.
const fitDot = "·"

// fitCell is a Fit in the six-cell column: the value, Marginal abbreviated, or · when it
// has not been assessed.
func fitCell(fit string) string {
	switch fit {
	case "":
		return fitDot
	case store.FitMarginal:
		return "Marg."
	}
	return fit
}

// slotText is an Application's right-hand column text: stage and date, follow-up flag
// and Fit.
func (m model) slotText(a store.Application) (state, flag, fit string) {
	if a.Malformed != nil {
		return "", "", ""
	}
	if past, due := followUpDaysPast(a, m.now(), m.followUpDays); due {
		flag = followUpHeat(past) + " " + m.since(a.LastChange)
	}
	return m.stageText(a), flag, fitCell(a.Fit)
}

// row is one Application as two lines: title and right-hand columns, then company ·
// track · dates. The selected row is a full-width bar; text on it is never dimmed.
func (m model) row(p pal, a store.Application, sel bool, show slotSet, w int) []string {
	line := lipgloss.NewStyle().Width(w).MaxWidth(w)
	bar := "  "
	titleC, dimC := p.Fg, p.Dim
	if m.expired(a) {
		titleC = p.Dim
	}
	// The list loses the fill, not the mark, while a sheet over it holds the focus.
	fill := sel && !m.sheetShown()
	st := func(c color.Color) lipgloss.Style {
		s := lipgloss.NewStyle().Foreground(c)
		if fill {
			s = s.Background(p.Sel)
		}
		return s
	}
	if sel {
		if fill {
			line = line.Background(p.Sel)
		}
		bar = p.SelGutter(fill)
		titleC, dimC = p.Fg, p.Sub
	}
	if a.Malformed != nil {
		return []string{line.Render(bar + st(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a))),
			line.Render(bar + st(dimC).Render("  "+xansi.Truncate(m.malformedReason(a), w-ellipsis-statusMarkWidth, "…")))} // the bar spans both lines, like any row
	}
	state, flag, fit := m.slotText(a)
	fitC := p.FitColor(a.Fit)
	if fitC == p.Dim {
		fitC = dimC
	}
	slot := func(width int, c color.Color, s string) string {
		return st(c).Width(width).Align(lipgloss.Right).Render(s)
	}
	// The right-hand columns are fixed: a row without a value holds the column blank, so
	// every column starts at the same cell on every row.
	buildRight := func() string {
		heat := slot(show.flagW, p.Warn, flag)
		if strings.HasPrefix(flag, heatHigh) {
			heat = st(p.Warn).Bold(true).Width(show.flagW).Align(lipgloss.Right).Render(flag)
		}
		return heat + slot(show.stateW, dimC, state) + slot(show.fitW, fitC, fit) +
			st(p.Fg).Render(strings.Repeat(" ", rowPad))
	}
	// The row carries no ready meter: the detail's DOCS line and a Draft's second line say
	// how far its Documents are.
	right := buildRight()
	// The Status glyph leads the title; the second line lines up under the title.
	mark := st(p.StatusColor(a.Status)).Render(p.StatusGlyph(a.Status)) + st(p.Fg).Render(" ")
	left := bar + mark + st(titleC).Bold(sel).Render(xansi.Truncate(a.Title, max(1, w-xansi.StringWidth(bar)-xansi.StringWidth(mark)-xansi.StringWidth(right)-1), "…"))
	gap := st(p.Fg).Render(strings.Repeat(" ", max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))))
	meta := strings.Join(m.meta(p, a, st, dimC), st(dimC).Render(" · "))
	// The bar spans both lines, so the selected row reads as one block.
	lines := []string{line.Render(left + gap + right), line.Render(xansi.Truncate(bar+st(p.Fg).Render("  ")+meta, w, "…"))}
	if m.flash != "" && a.Folder == m.flash {
		lines = p.FlashRow(lines, w)
	}
	return lines
}

// meta is a row's second line: company, track, and the dates that matter for its table.
func (m model) meta(p pal, a store.Application, st func(color.Color) lipgloss.Style, dimC color.Color) []string {
	dim := st(dimC)
	// when renders a Status date, flagging one in the future as a data error.
	when := func(e *store.StatusEntry, prefix string) string {
		if m.futureDate(e) {
			return st(p.Warn).Render(prefix + futureFlag)
		}
		return dim.Render(prefix + m.agoEntry(e))
	}
	out := []string{dim.Render(a.Company), dim.Render(a.Track)}
	switch t := tableOf(a); t {
	case tableLeads, tableDraft:
		// No close date is a quiet dash, not the same words on every row.
		if text, s := closeInfo(a.Closes, m.now()); s == closeNone {
			out = append(out, dim.Render("—"))
		} else {
			out = append(out, p.CloseStyle(s, st, dimC).Render(text))
		}
		if t == tableDraft {
			label, c := p.DocsLabel(a)
			if c == p.Dim {
				c = dimC
			}
			out = append(out, st(c).Render(label))
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
		return p.Text(c).Render(strings.Join(lines, "\n"))
	}
	var b strings.Builder
	if a.Malformed != nil {
		b.WriteString(p.Text(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a)) + "\n\n" + wrap(m.malformedReason(a), p.Fg, errorLines) +
			"\n\n" + p.Text(p.Accent).Render("e") + p.Text(p.Dim).Render(" fix in editor"))
	} else {
		b.WriteString(wrap(a.Title, p.Fg, titleLines) + "\n" + m.previewMeta(p, a, inner) + "\n\n")
		b.WriteString(strings.Join(m.fieldBlock(p, a, inner), "\n"))
	}
	return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).Padding(0, 1).Render(b.String())
}

// previewMeta is the line under the preview's title, the row's own second line: "Orbit
// Labs · SWE · applied 6d ago". A Lead with no close date leaves off the row's dash.
func (m model) previewMeta(p pal, a store.Application, w int) string {
	var parts []string
	for _, s := range m.meta(p, a, p.Text, p.Dim) {
		if xansi.Strip(s) != "—" {
			parts = append(parts, s)
		}
	}
	return xansi.Truncate(strings.Join(parts, p.Text(p.Dim).Render(" · ")), w, "…")
}

// fileMarks is the one-line document set, "✓ JD  ○ Resume  ○ Cover letter  ○ PDF":
// ✓ ready, ○ missing. A source document is ready when it has text, not merely a file.
func (m model) fileMarks(p pal, a store.Application) string {
	parts := make([]string, 0, len(docChips(a)))
	for _, c := range docChips(a) {
		parts = append(parts, m.chipText(p, a, c))
	}
	return strings.Join(parts, "  ")
}

// docText is a document's text with Markdown headings and list markers dropped, for
// short previews. Missing files read as "".
func (m model) docText(a store.Application, name string) string {
	// docText is width-independent, so it shares the render cache at width 0; the body
	// key is the Document, so the three preview Documents do not evict one another.
	return m.md.body(m.gen, 0, a.Folder+"\x00"+name, func() string {
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
	})
}
