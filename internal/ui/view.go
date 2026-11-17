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
	appearance "github.com/lukebrown14-code/rolecall/internal/ui/theme"
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
	shedSparkLabel = 1 // topBar detail levels: drop the sparkline's "applied/wk" label…
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
	plain := plainMode()
	// ROLECALL_PLAIN strips each layer before a pop-up is set over the frame, so a word
	// put in place of a glyph never shifts the pop-up's border.
	strip := func(s string, box bool) string {
		if plain {
			return plainFrame(s, m.viewWidth(), box)
		}
		return s
	}
	var body string
	switch box := m.overlayView(p); {
	case m.tooSmall():
		body = m.tooSmallView(p)
	case box != "" && m.helpScreenOpen():
		body = strip(box, false) // the full-screen help is the whole frame
	case box != "":
		behind := m.screenView(m.dimmedPal())
		if noColor() {
			behind = appearance.StripBold(behind) // without the dimmed colours, weight sets the pop-up apart
		}
		body = m.overlayBody(p, strip(behind, false), strip(box, true))
	default:
		body = strip(m.screenView(p), false)
	}
	v := tea.NewView(asciiFold(body, m.ascii))
	if !plain {
		// Clicks and the wheel (mouse.go). ROLECALL_PLAIN leaves the mouse to the
		// terminal, so its own selection and a screen reader's review cursor work.
		v.MouseMode = tea.MouseModeCellMotion
	} else if x, y, ok := selectionCell(body); ok {
		// A screen reader follows the terminal cursor: it sits on the selection.
		v.Cursor = tea.NewCursor(x, y)
	}
	// Paint the theme's background so its roles keep their contrast whatever the
	// terminal's own colours: as the terminal background (OSC 11) and on every cell, as
	// a terminal may ignore OSC 11. NO_COLOR drops colour but keeps bold and underline.
	switch {
	case noColor(): // NO_COLOR wins over the inherit option
		v.Content = stripColor(v.Content)
	case m.inherit:
		v.Content = inheritANSI(v.Content)
	default:
		v.BackgroundColor, v.ForegroundColor = p.Bg, p.Fg
		v.Content = appearance.PaintBackground(v.Content, p.Bg, m.viewWidth())
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

// statusAreaLines is the status area's height: one row for the key line alone (sizeLayout's
// default, before a model fills it in), or two with the breadcrumb line above it.
const (
	statusAreaPlain    = 1
	statusAreaFeedback = 2
)

// frameStatusLines is the lines the status area takes: on every screen the breadcrumb
// line, which holds the feedback, then the key line, so the bottom rule never moves
// between screens.
func (m model) frameStatusLines() int { return statusAreaFeedback }

// statusRow is the frame's last rows: the breadcrumb line, which holds a toast or error on
// its right, over the line the caller prepared (the search input while typing, else the
// key line), so feedback never wipes the key hints. The rule above is never replaced.
func (m model) statusRow(p pal, fallback string) string {
	return m.breadcrumbLine(p) + "\n" + fallback
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

// movedOutView is the empty view after a move: where the Application is now and the one
// key that goes there. The toast below already says what moved, so the centre does not
// repeat it.
func (m model) movedOutView(p pal) string {
	screen, table := StatusLocation(m.movedTo)
	where, hint := screen, p.Key().Render("← →")+p.Text(p.Dim).Render(" to follow it")
	if m.viewHoldsStatus(m.movedTo) {
		where += " › " + table
	} else {
		for i, d := range screenDefs {
			if d.name == screen {
				hint = p.Key().Render(string(rune('1'+i))) + p.Text(p.Dim).Render(" to go there")
			}
		}
	}
	colour := p.Fg
	if m.movedTo == store.StatusOffer {
		colour = p.Gold
	}
	return p.Text(colour).Bold(true).Render("Now in "+where) + p.Text(p.Dim).Render(" · ") + hint
}

func (m model) screenCount(si int) int {
	n := 0
	for _, t := range screenDefs[si].tables {
		if t != allView && t != tableDismissed && t != tableFiltered && t != tableDeadlines {
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
		if t != allView && t != tableDismissed && t != tableFiltered && t != tableDeadlines {
			total += m.totalCount(si, t)
		}
	}
	if !m.filtered() {
		return fmt.Sprintf("%d", total)
	}
	return fmt.Sprintf("%d of %d", m.screenCount(si), total)
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
	// The Applied-per-week sparkline, the same series Stats draws over its last weeks, sits
	// beside the Stats tab at wide sizes, until the bar is short of room.
	// It sits outside the tab's own cell, a space either side, so it stays in the same
	// column whether or not Stats is the active tab.
	spark, sparkLabel := "", ""
	if weeks := reporting.AppliedWeeks(m.apps, m.now(), activityWeeks); anyApplied(weeks) && !plainMode() {
		// No weeks with data draw no line; a screen reader would read it out cell by cell.
		// Empty weeks are the faint baseline, so the line reads as a chart, not a glitch.
		spark = " " + sparkline(p, weeks, 1, p.Dim)
		sparkLabel = p.Text(p.Dim).Render(" applied/wk")
	}
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
			if wide && level < shedCounts && d.name == screenStats && spark != "" {
				tail = spark + " "
				if level < shedSparkLabel {
					tail = spark + sparkLabel + " "
				}
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
				if si+1 == active {
					sep = " " // …but the active chip after it never touches it: "3 [4 Stats]"
				}
			}
			out += " " + p.Key().Render(digit) + p.Text(p.Fg).Render(name) + p.Text(p.Dim).Render(count) + sep + tail
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

// trackTotal is the Applications on a Track (0 is All; see filterTracks). On a table
// screen it is the rows the open view would show with that Track chosen, so the chosen
// option matches the list; on Stats it is the Applications Stats counts, so malformed
// ones are left out and the options add up to All.
func (m model) trackTotal(track int) int {
	with := m
	with.track = track
	if m.hasTables() {
		return len(with.rows())
	}
	return len(with.statsApps())
}

// trackName is the Track filter's name: All, or the Track.
func (m model) trackName() string {
	if t, ok := m.selectedTrack(); ok {
		return trackLabel(t)
	}
	return allView
}

// breadcrumbLine is the footer's first line on every screen: where the Owner is, as
// "Applied › All · All tracks · row 1 of 9", and at the right what just happened, a toast
// or an error with its keys. The breadcrumb gives way first: its focus, then the Track.
// When the message cannot sit whole beside it, the message takes the whole line: what
// just happened matters more than where the cursor is.
func (m model) breadcrumbLine(p pal) string {
	w := m.layout().w
	right := ""
	if m.feedbackVisible() {
		msg, keys, short := m.feedbackParts(p)
		beside := w - rightMargin - gutter - inset - breadcrumbMin
		if r, ok := feedbackRight(msg, keys, short, beside, false); ok {
			right = r
		} else {
			r, _ := feedbackRight(msg, keys, short, w-rightMargin-inset+1, true)
			return strings.Repeat(" ", inset-1) + r // the chip's own leading space sets its glyph at the text column
		}
	} else if warn := m.legacyWarning(); warn != "" {
		room := max(1, w-rightMargin-gutter-inset-breadcrumbMin)
		right = p.Text(p.Warn).Render(xansi.Truncate(warn, room, "…"))
	}
	room := w - rightMargin
	if right != "" {
		room -= gutter + xansi.StringWidth(right)
	}
	return contextRow(m.breadcrumbLeft(p, room), right, w)
}

// legacyWarning names a dashboard-era file in the Workspace, such as jobs/SHORTLIST.md,
// that nothing reads: a Harness writing it has old instructions, so its roles never
// appear here. It stays on the footer while no toast or error needs the space.
func (m model) legacyWarning() string {
	if len(m.legacy) == 0 {
		return ""
	}
	return "⚠ " + filepath.ToSlash(m.legacy[0]) + " isn't read by rolecall · run rolecall doctor"
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

// breadcrumb is "Applied › All · All tracks · row 1 of 9 · detail", the Track abbreviated
// when short and the focus, shown only while a detail is open, left off when noFocus. A
// screen without tables names its own place: "Stats › All tracks", "Settings › Tracking".
func (m model) breadcrumb(p pal, short, noFocus bool) string {
	name := p.Text(p.Fg).Bold(true).Render(screenDefs[m.screen].name)
	if !m.hasTables() {
		if sub := m.screenCrumb(); sub != "" {
			return name + p.Text(p.Dim).Render(" › "+sub)
		}
		return name
	}
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
	track := "All tracks"
	if m.track > 0 {
		track = m.trackName()
	}
	where := fmt.Sprintf("row %d of %d", pos, len(rows))
	if short {
		track = "All"
		if m.track > 0 {
			track = m.trackName()
			if r := []rune(track); len(r) > abbrevTrack {
				track = string(r[:abbrevTrack])
			}
		}
		where = fmt.Sprintf("%d of %d", pos, len(rows))
	}
	if len(rows) == 0 {
		where = "no rows"
	}
	parts := []string{track, where}
	// The screen's own first view (Leads › Leads) says nothing twice; a status filter named
	// like its screen (Applied › Applied) is still a filter, so it shows.
	if v := m.views()[m.table]; m.table > 0 || v != screenDefs[m.screen].name {
		parts = append([]string{v}, parts...)
	}
	out := name + p.Text(p.Dim).Render(" › "+strings.Join(parts, " · "))
	if !noFocus && m.detail != nil {
		out += p.Text(p.Dim).Render(" · ") + p.Text(p.Accent).Bold(true).Render("detail")
	}
	return out
}

// screenCrumb is where a screen without tables is: the Track Stats counts, the Profile
// entry shown, or the Settings group. Empty when there is nothing to name.
func (m model) screenCrumb() string {
	switch screenDefs[m.screen].name {
	case screenStats:
		if m.track > 0 {
			return m.trackName()
		}
		return "All tracks"
	case screenProfile:
		if m.profileLoaded() {
			return m.profile.entryName()
		}
	case screenSettings:
		if m.settings != nil && m.settings.Ws.Err == nil {
			return settingsGroups[m.settings.Group]
		}
	}
	return ""
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

// sheetLines is the detail sheet: a title line, the tabs, the next step, then the tab's
// content, h lines.
func (m model) sheetLines(p pal, a store.Application, w, h int) []string {
	// The sheet keeps the list's columns: text from the inset to where the rows' Fit ends.
	pad := strings.Repeat(" ", inset)
	right := w - rightMargin - rowPad
	title := pad + p.Text(p.Fg).Bold(true).Render(appName(a.Title)) + p.Text(p.Dim).Render(companySuffix(a.CompanyLabel()))
	if a.Malformed != nil {
		title = pad + p.Text(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a))
	} else {
		title = spread(xansi.Truncate(title, max(0, right-gutter-xansi.StringWidth(a.Status)-markWidth), "…"), p.StatusChip(a.Status), right)
	}
	out := []string{title}
	var content []string
	// Text sits at the inset; the Docs rows bring their own indent, so their selection bar
	// sits at the edge, as a list's does.
	indent := pad
	if a.Malformed != nil {
		for _, l := range wrapWords(m.malformedReason(a), right-inset) {
			content = append(content, p.Text(p.Fg).Render(l))
		}
		content = append(content, "", p.Key().Render("e")+p.Text(p.Dim).Render(" fix in editor"))
		out = append(out, "", "")
	} else {
		// The tab names sit one cell in from their brackets, so the brackets go one left. The
		// next step takes the line under the tabs on every tab, as in the full-screen detail,
		// so it is in view however little of the sheet shows; Overview leaves it out.
		out = append(out, pad[1:]+m.detailTabsRow(p, a, right-inset+1), pad+m.nextLine(p, a, right-inset, false))
		content = strings.Split(m.detailContent(p, a, w), "\n")
		if m.detail.tab == tabDocs {
			indent = strings.Repeat(" ", edge)
		}
	}
	for i, line := range content {
		if plain := xansi.Strip(line); indent != pad && plain != "" && !strings.HasPrefix(plain, " ") && !strings.HasPrefix(plain, "▌") {
			content[i] = pad + line // a Docs group heading, at the text column
			continue
		}
		content[i] = indent + line
	}
	from := 0
	for i, line := range content {
		if strings.Contains(xansi.Strip(line), "▌") {
			from = i - (h-len(out))/bothSides
		}
	}
	out = append(out, p.ClipLines(content, from, max(0, h-len(out)), pad)...)
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
	// The heading stands where the views stand on a table screen, and the Track filter
	// follows it after the same │, so t's options are in one place on every screen.
	heading := strings.Repeat(" ", inset) + p.Heading(screenDefs[m.screen].name)
	right := m.withChip(p, "")
	var left string
	for _, form := range []trackForm{trackFull, trackCompact, trackCollapsed} {
		left = heading
		if part := m.trackPart(p, form); part.text != "" {
			left += p.Text(p.Rule).Render(" │ ") + part.text
		}
		need := xansi.StringWidth(left)
		if right != "" {
			need += gutter + xansi.StringWidth(right)
		}
		if need <= w-rightMargin {
			break
		}
	}
	return contextRow(left, right, w)
}

// scopeLine is the shown-of-total count when a filter is on. The search chip beside it
// names the text and the track filter names the track.
func (m model) scopeLine(p pal) string {
	if !m.hasTables() || (m.track == 0 && m.text == "") {
		return ""
	}
	return p.Text(p.Dim).Render(fmt.Sprintf("%d of %d", len(m.rows()), m.totalCount(m.screen, m.views()[m.table])))
}

// listBody lays out the rows with section dividers. Rows always run dense: only a
// section's header has a blank line above it (see listBlocks).
func (m model) listBody(p pal, rows []store.Application, w, h int) string {
	blocks, cur := m.listBlocks(rows)
	return scroll(p, m, rows, m.slotsFor(rows), w, blocks, cur, h)
}

// rowLines is a list row's height: the title line and the meta line. Rows in a group sit
// together; only a group's rule has air above it.
const rowLines = 2

// rowHeight is a list row's height: rowLines, or one line when compact rows are on.
func (m model) rowHeight() int {
	if m.compactShown() {
		return 1
	}
	return rowLines
}

// compactShown reports one-line rows: compact rows are on (z, compact_rows in
// config.toml) and the terminal is wide, where one line has room for every column.
func (m model) compactShown() bool { return m.compactRows && m.layout().wide() }

// listBlocks is the list as blocks — dividers and two-line rows — and the cursor's block.
// Heights are measured without rendering: a row is two lines and a divider one or two.
// This is the one place a block's height is computed; renderBlock draws content and
// scroll pads it.
func (m model) listBlocks(rows []store.Application) ([]block, int) {
	dividers := m.dividers(rows)
	sheet := m.sheetShown()
	dividerHeight := func(i int) int {
		n := 1
		if i > 0 && !sheet {
			n++ // the air above a section, which the few lines above a sheet cannot spare
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
		blocks = append(blocks, block{height: m.rowHeight(), row: true, idx: i})
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
			if _, t := appLocation(a); i == 0 || t != tableOf(rows[i-1]) {
				div[i] = divider{label: t, n: m.count(m.screen, t)}
			}
		}
	case tableDeadlines:
		in := func(g deadlineGroup) func(store.Application) bool {
			return func(a store.Application) bool { got, _ := m.deadlineGroupOf(a); return got == g }
		}
		for i, a := range rows {
			if in(deadlinePast)(a) {
				div[i] = divider{label: groupExpired, n: deadlineEnd(rows, i, in(deadlinePast))}
				break
			}
		}
		for i, a := range rows {
			if in(deadlineUnclear)(a) {
				div[i] = divider{label: "Date unclear", n: len(rows) - i}
				break
			}
		}
	case tableLeads, tableDraft:
		if m.leadsNewest && m.views()[m.table] == tableLeads {
			break // newest first mixes expired Leads in with the rest
		}
		for i, a := range rows {
			if m.expired(a) {
				div[i] = divider{label: groupExpired, n: len(rows) - i}
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
	// The Leads header names the sort o switches, "◌ Leads · 23 · by deadline", so the
	// order reads after the toast has gone.
	if d, ok := div[0]; ok && m.views()[m.table] == tableLeads {
		sortText := sortByDeadline
		if m.leadsNewest {
			sortText = sortNewestText
		}
		d.rollup = " · " + sortText + d.rollup
		div[0] = d
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
	if label == tableClosed || label == groupExpired {
		return ""
	}
	due, ready := 0, 0
	for _, a := range group {
		if flag, _ := m.slotText(a); flag != "" {
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

// deadlineEnd counts the rows from i on that keep in.
func deadlineEnd(rows []store.Application, i int, in func(store.Application) bool) int {
	n := 0
	for ; i < len(rows) && in(rows[i]); i++ {
		n++
	}
	return n
}

func tableOf(a store.Application) string { _, t := appLocation(a); return t }

// expired is a Lead or Draft whose deadline has passed: its close date, or a Lead's ad
// past max_posting_age_days (the row reads "past the age limit").
func (m model) expired(a store.Application) bool {
	if !beforeApplied(tableOf(a)) {
		return false
	}
	t, ok := deadline(a, m.rules)
	return ok && t.Before(day(m.now()))
}

// scroll fills h lines with blocks, keeping block cur in view. "↑/↓ n more" appear only
// when rows are really hidden, and count rows, not dividers. Only the visible blocks are
// rendered.
func scroll(p pal, m model, rows []store.Application, show slotSet, w int, blocks []block, cur, h int) string {
	folded := m.sheetShown()
	start, end := scrollWindow(blocks, cur, h, folded)
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
	if start > 0 && !folded {
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
	var cues []string
	if n := count(blocks[:start]); n > 0 && folded {
		cues = append(cues, p.MoreCue(true, n))
	}
	if end < len(blocks) {
		cues = append(cues, p.MoreCue(false, count(blocks[end:])))
	}
	if len(cues) > 0 {
		out = append(out, cueIndent+strings.Join(cues, "  "))
	}
	for len(out) < h {
		out = append(out, "")
	}
	return strings.Join(out[:h], "\n")
}

// scrollWindow is the blocks shown in h lines with block cur in view: [start, end). A
// "↑ n more" line takes the first line when start is past the top. folded (the list part
// above a detail sheet, where every line counts) puts both cues on the one last line.
func scrollWindow(blocks []block, cur, h int, folded bool) (start, end int) {
	fit := func(start int) int {
		avail := h
		if start > 0 {
			avail-- // the ↑ cue's line; folded, the line both cues share
		}
		used, end := 0, start
		for end < len(blocks) && used+blocks[end].height <= avail {
			used += blocks[end].height
			end++
		}
		if folded && start > 0 {
			return end // the ↓ cue shares the line already kept
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

// slotSet is the width of each right-hand column, gap included: the Follow-up due flag and
// Fit. Every row in a view uses the same set, so each column starts at the same cell on
// every row and nothing wraps. A width of 0 hides the column. Compact rows also size
// their title, company and track columns from it: each value's widest in the view.
type slotSet struct {
	flagW, fitW              int
	titleW, companyW, trackW int
}

// fitSlotWidth is the Fit column's width: its longest word, so every Fit starts at the
// same cell and a Strong row's word lines up under a Marginal one's.
const fitSlotWidth = len(store.FitMarginal)

// Compact rows: the most the company and track columns take, and the least room and
// largest share the title keeps before them.
const (
	compactCompanyMax = 16
	compactTrackMax   = 12
	compactTitleMin   = 16
	compactTitleShare = 45 // percent of the row's room
)

// slotsFor sizes each right-hand column to its widest value in rows, the whole view rather
// than the rows in sight, so the columns hold still while scrolling. A column empty on
// every row is hidden, and the title takes the room. columnGap cells part each column
// from the one before it; the first shown column is parted from the title by the title's
// own one cell and columnGap-1 more, so the title keeps the same gap.
func (m model) slotsFor(rows []store.Application) slotSet {
	var s slotSet
	var flagW, fitW int
	for _, a := range rows {
		flag, fit := m.slotText(a)
		flagW = max(flagW, xansi.StringWidth(flag))
		if fit != "" {
			fitW = fitSlotWidth
		}
		if a.Malformed == nil {
			s.titleW = max(s.titleW, xansi.StringWidth(titleLabel(a.Title)))
			s.companyW = max(s.companyW, min(compactCompanyMax, xansi.StringWidth(a.CompanyLabel())))
			s.trackW = max(s.trackW, min(compactTrackMax, xansi.StringWidth(a.Track)))
		}
	}
	lead := columnGap - 1
	col := func(widest int) int {
		if widest == 0 {
			return 0
		}
		w := lead + widest
		lead = columnGap
		return w
	}
	s.flagW = col(flagW)
	s.fitW = col(fitW)
	return s
}

// slotText is an Application's right-hand column text: the Follow-up due flag with how
// long it has been quiet ("⚑ 6w"), and the Fit word alone ("Good"), blank until assessed.
// The row has no age column: how long it has been in its Status is the preview's to say.
func (m model) slotText(a store.Application) (flag, fit string) {
	if a.Malformed != nil {
		return "", ""
	}
	if _, due := followUpDaysPast(a, m.now(), m.followUpDays); due {
		flag = followUpFlag + " " + m.followUpQuiet(a)
	}
	return flag, a.Fit
}

// row is one Application as two lines: title and right-hand columns, then company ·
// track · dates; or as one line, in aligned columns, when compact rows are on. The
// selected row is a full-width bar; text on it is never dimmed.
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
		title := st(p.Bad).Bold(true).Render("⚠ ") + st(p.Bad).Bold(true).Render(m.malformedTitle(a))
		if fill {
			title = st(p.Bad).Bold(true).Render("⚠ ") + st(p.Fg).Bold(true).Render(m.malformedTitle(a)) // the words clear 4.5:1 on the fill
		}
		reason := m.malformedReason(a)
		if m.compactShown() {
			return []string{line.Render(xansi.Truncate(bar+title+st(dimC).Render("  "+reason), w, "…"))}
		}
		return []string{line.Render(bar + title),
			line.Render(bar + st(dimC).Render("  "+xansi.Truncate(reason, w-ellipsis-statusMarkWidth, "…")))} // the bar spans both lines, like any row
	}
	flag, fit := m.slotText(a)
	fitC, heatC := p.FitColor(a.Fit), p.Warn
	if fitC == p.Dim {
		fitC = dimC
	}
	if fill {
		// Coloured text on the selection fill falls below 4.5:1, so the selected row's
		// words take the secondary colour; its Status glyph keeps the colour.
		fitC, heatC = p.Sub, p.Sub
	}
	// The right-hand columns are the view's (slotsFor): a row without a value holds the
	// column blank, so every column starts at the same cell on every row. The flag sits
	// flush right in its column; the Fit word starts at its column's first cell.
	buildRight := func() string {
		heat := st(heatC).Width(show.flagW).Align(lipgloss.Right)
		if past, due := followUpDaysPast(a, m.now(), m.followUpDays); due && followUpLate(past) {
			heat = heat.Bold(true)
		}
		out := heat.Render(flag)
		if show.fitW > 0 {
			out += st(p.Fg).Render(strings.Repeat(" ", show.fitW-fitSlotWidth)) +
				st(fitC).Width(fitSlotWidth).Render(xansi.Truncate(fit, fitSlotWidth, ""))
		}
		return out + st(p.Fg).Render(strings.Repeat(" ", rowPad))
	}
	// The row carries no ready meter: the detail's DOCS line and a Draft's second line say
	// how far its Documents are.
	right := buildRight()
	// The Status glyph leads the title; the second line lines up under the title.
	// On the selection fill the row's coloured words and its muted glyph take the
	// secondary colour (OnSel), so they clear the floors there.
	rp := p
	if fill {
		rp = pal{p.OnSel()}
	}
	mark := st(rp.StatusColor(a.Status)).Render(p.StatusGlyph(a.Status)) + st(p.Fg).Render(" ")
	room := max(1, w-xansi.StringWidth(bar)-xansi.StringWidth(mark)-xansi.StringWidth(right)-1)
	sep := st(dimC).Render(" · ")
	if m.compactShown() {
		left := bar + mark + m.compactColumns(a, show, room, st, titleC, dimC, sel, strings.Join(m.metaDates(rp, a, st, dimC), sep))
		gap := st(p.Fg).Render(strings.Repeat(" ", max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))))
		lines := []string{line.Render(left + gap + right)}
		if m.flash != "" && a.Folder == m.flash {
			lines = p.FlashRow(lines, w)
		}
		return lines
	}
	left := bar + mark + st(titleC).Bold(sel).Render(xansi.Truncate(titleLabel(a.Title), room, "…"))
	gap := st(p.Fg).Render(strings.Repeat(" ", max(1, w-xansi.StringWidth(left)-xansi.StringWidth(right))))
	meta := strings.Join(m.meta(rp, a, st, dimC), sep)
	// The bar spans both lines, so the selected row reads as one block.
	lines := []string{line.Render(left + gap + right), line.Render(xansi.Truncate(bar+st(p.Fg).Render("  ")+meta, w, "…"))}
	if m.flash != "" && a.Folder == m.flash {
		lines = p.FlashRow(lines, w)
	}
	return lines
}

// compactColumns is a compact row between its Status glyph and its right-hand columns, in
// room cells: the title, company and track in columns as wide as the view's widest (each
// capped), then the dates and badges, cut at the end. A column that would squeeze the
// title below compactTitleMin is left out, track first.
func (m model) compactColumns(a store.Application, show slotSet, room int, st func(color.Color) lipgloss.Style,
	titleC, dimC color.Color, sel bool, dates string) string {
	titleW := min(show.titleW, max(compactTitleMin, room*compactTitleShare/percent), room)
	companyW, trackW := show.companyW, show.trackW
	if titleW+columnGap+companyW > room {
		companyW = 0
	}
	if titleW+columnGap+companyW+columnGap+trackW > room {
		trackW = 0
	}
	cell := func(s string, n int) string {
		if n == 0 {
			return ""
		}
		return st(dimC).Render(strings.Repeat(" ", columnGap)) + st(dimC).Width(n).Render(xansi.Truncate(s, n, "…"))
	}
	out := st(titleC).Bold(sel).Width(titleW).Render(xansi.Truncate(titleLabel(a.Title), titleW, "…")) +
		cell(a.CompanyLabel(), companyW) + cell(a.Track, trackW)
	if rest := room - xansi.StringWidth(out) - columnGap; rest > 0 && dates != "" {
		out += st(dimC).Render(strings.Repeat(" ", columnGap)) + xansi.Truncate(dates, rest, "…")
	}
	return out
}

// meta is a row's second line: company, track, and the dates that matter for its table.
func (m model) meta(p pal, a store.Application, st func(color.Color) lipgloss.Style, dimC color.Color) []string {
	dim := st(dimC)
	var out []string
	if strings.TrimSpace(a.CompanyLabel()) != "" {
		out = append(out, dim.Render(a.CompanyLabel()))
	}
	out = append(out, dim.Render(a.Track))
	return append(out, m.metaDates(p, a, st, dimC)...)
}

// metaDates is the part of a row's meta after company and track: the dates that matter
// for its table, and on a Lead or Draft the ad's badges. Meaning never rests on colour: a
// close date within a week is bold (and ▲ within two days), and a blocker badge leads
// with ⚠.
func (m model) metaDates(p pal, a store.Application, st func(color.Color) lipgloss.Style, dimC color.Color) []string {
	dim := st(dimC)
	// when renders a Status date, flagging one in the future as a data error.
	when := func(e *store.StatusEntry, prefix string) string {
		if m.futureDate(e) {
			return st(p.Warn).Render(prefix + futureFlag)
		}
		return dim.Render(prefix + m.agoEntry(e))
	}
	var out []string
	switch t := tableOf(a); t {
	case tableLeads, tableDraft:
		// No close date shows nothing: the slot is optional, not a dash on every row.
		if text, s := closeInfo(a.Closes, m.now()); s == closeNone && a.ClosesUnclear {
			out = append(out, st(p.Warn).Render("closes: "+closesUnclearText))
		} else if s != closeNone {
			out = append(out, p.CloseStyle(s, st, dimC).Render(text))
		}
		// After the close date, which matters more: how old the ad is, and how near the
		// max posting age it has come.
		if age := postedAge(a, m.now()); age != "" && t == tableLeads {
			posted := dim.Render("posted " + age)
			if warn := ageWarning(a, m.rules, m.now()); warn != "" {
				posted += st(p.Warn).Render(" " + glyphWarn + " " + warn)
			}
			out = append(out, posted)
		}
		if t == tableDraft {
			label, c := p.DocsLabel(a)
			if c == p.Dim {
				c = dimC
			}
			out = append(out, st(c).Render(label))
		}
		// Badges come last, so a narrow row drops them before anything else, and blockers
		// lead them (adBadges). Amber and red stay for time: a blocker is ⚠ and bold in the
		// text colour, a visa fit green, a note muted.
		for _, b := range m.adBadges(a) {
			switch b.tone {
			case toneBlock:
				out = append(out, st(p.Fg).Bold(true).Render(glyphWarn+" "+b.text))
			case toneOK:
				out = append(out, st(p.Ok).Render(b.text))
			default:
				out = append(out, dim.Render(b.text))
			}
		}
	case tableDismissed:
		if lastChange(a) != nil {
			out = append(out, when(lastChange(a), "dismissed "))
		}
	case tableFiltered:
		out = append(out, dim.Render(filterReasonLabel(a.FilterReason)))
		if lastChange(a) != nil {
			out = append(out, when(lastChange(a), "dismissed "))
		}
	case tableClosed:
		if lastChange(a) != nil {
			// The outcome by name, "rejected ~7d ago": "closed" alone could be the ad.
			out = append(out, when(lastChange(a), strings.ToLower(lastChange(a).Status)+" "))
		}
	default:
		if appliedDate(a) != nil {
			out = append(out, when(appliedDate(a), "applied "))
		}
		if lastChange(a) != nil && lastChange(a).At != entryAt(appliedDate(a)) {
			out = append(out, when(lastChange(a), strings.ToLower(lastChange(a).Status)+" "))
		}
		if t == tableInterviewing || t == tableOffer {
			if text, s := m.nextStepInfo(a.NextStep); s != closeNone {
				out = append(out, p.CloseStyle(s, st, dimC).Render("next "+text))
			}
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

// preview is the selected Application beside the list, h lines: the card, then as much
// of fit.md and jd.md as the room under it holds.
func (m model) preview(p pal, a store.Application, w, h int) string {
	return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).Padding(0, 1).Render(m.previewText(p, a, w-previewInset, h, cardPreview))
}

// previewText is the card's lines inside inner cells, at most h: the title, Company ·
// Track, then the fields. Beside the list (cardPreview) the room left fills with the
// start of fit.md, then of jd.md; beside the detail tabs (cardDetail), which show those
// whole, the link closes it instead. Opening the detail leaves the card's rows in place.
func (m model) previewText(p pal, a store.Application, inner, h int, mode cardMode) string {
	wrap := func(s string, c color.Color, maxLines int) string {
		return p.Text(c).Render(strings.Join(clampLines(wrapWords(s, inner), maxLines, inner), "\n"))
	}
	if a.Malformed != nil {
		return p.Text(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a)) + "\n\n" + wrap(m.malformedReason(a), p.Fg, errorLines) +
			"\n\n" + p.Text(p.Accent).Render("e") + p.Text(p.Dim).Render(" fix in editor")
	}
	lines := strings.Split(wrap(titleLabel(a.Title), p.Fg, titleLines), "\n")
	lines = append(lines, m.previewMeta(p, a, inner), "")
	for _, r := range m.fieldRows(p, a, inner, mode) {
		lines = append(lines, strings.Split(r, "\n")...)
	}
	if mode == cardDetail && a.URL != "" {
		// One inline row, as every other field: the link and the key that opens it.
		key, action := "", ""
		if _, ok := webLink(a); ok {
			key, action = "w", descOpen
		}
		lines = append(lines, "", p.cardRow("LINK", p.Text(p.Info).Render(a.URL), key, action, inner))
	}
	if mode == cardPreview {
		// The Fit's reasoning first, as it decides dismiss or draft; the JD takes what is left.
		fit := m.excerpt(p, "FIT", m.docLines(a, fitFile), min(fitExcerptLines, h-len(lines)), inner)
		lines = append(lines, fit...)
		lines = append(lines, m.excerpt(p, docJD, m.docLines(a, jdFile), h-len(lines), inner)...)
	}
	return strings.Join(lines, "\n")
}

// fitExcerptLines is the most of the preview the FIT block takes, its blank line and
// heading included, so the JD's opening still shows under a long fit.md.
const fitExcerptLines = 10

// excerpt is a document's opening under a block label, inside room lines counting the
// blank line before it: as many lines as fit, then, when some do not, "↓ 5 more · enter
// open". It is empty when the document has no text or room holds no line of it.
func (m model) excerpt(p pal, label string, text []string, room, w int) []string {
	var body []string
	for _, l := range text {
		if l == "" {
			body = append(body, "")
			continue
		}
		for _, wl := range wrapWords(l, w) {
			body = append(body, p.Text(p.Fg).Render(wl))
		}
	}
	avail := room - 2 //nolint:mnd // the blank line and the label
	switch {
	case len(body) == 0 || avail < 1:
		return nil
	case len(body) > avail && avail < 2: //nolint:mnd // one line of text and the cue
		return nil
	case len(body) > avail:
		shown := body[:avail-1]
		for len(shown) > 1 && shown[len(shown)-1] == "" {
			shown = shown[:len(shown)-1] // the cue never follows a blank line
		}
		cue := p.MoreCue(false, len(body)-len(shown)) + p.Text(p.Dim).Render(" · ") + p.Key().Render("enter") + p.Text(p.Dim).Render(" open")
		body = append(shown, cue)
	}
	return append([]string{"", p.blockLabel(label)}, body...)
}

// docLines is a document's text as plain lines for a short excerpt: headings, tables, the
// fit.md verdict line (the card says the Fit) and Markdown's marks dropped, one blank line
// between paragraphs. Missing files read as none.
func (m model) docLines(a store.Application, name string) []string {
	text := m.md.body(m.gen, 0, a.Folder+"\x00"+name+"\x00lines", func() string {
		b, err := m.store.ReadDocument(a.Folder, name)
		if err != nil {
			return ""
		}
		var keep []string
		for _, l := range strings.Split(string(b), "\n") {
			l = strings.TrimSpace(l)
			switch {
			case l == "":
				if len(keep) > 0 && keep[len(keep)-1] != "" {
					keep = append(keep, "")
				}
			case strings.HasPrefix(l, "#"), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "|"),
				strings.HasPrefix(strings.TrimLeft(l, "*_"), "Verdict:"):
			default:
				if l = strings.TrimSpace(markdownMarks.Replace(l)); l != "" {
					keep = append(keep, l)
				}
			}
		}
		return strings.TrimRight(strings.Join(keep, "\n"), "\n")
	})
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// clampLines keeps at most n of lines, cutting the last one kept with "…" when more
// follow, so a long title fills its lines rather than ending on a lone "…".
func clampLines(lines []string, n, w int) []string {
	if len(lines) <= n || n <= 0 {
		return lines
	}
	out := append([]string{}, lines[:n]...)
	out[n-1] = xansi.Truncate(out[n-1]+" …", w, "…")
	return out
}

// previewMeta is the line under the card's title: "Orbit Labs · SWE". The row's dates,
// close date and badges are left to the card's own labelled rows, so none shows twice.
func (m model) previewMeta(p pal, a store.Application, w int) string {
	var parts []string
	if c := strings.TrimSpace(a.CompanyLabel()); c != "" {
		parts = append(parts, c)
	}
	if a.Track != "" {
		parts = append(parts, a.Track)
	}
	return p.Text(p.Dim).Render(xansi.Truncate(strings.Join(parts, " · "), w, "…"))
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

// groupExpired is the divider over Leads and Drafts whose close date has passed.
const groupExpired = "Expired"
