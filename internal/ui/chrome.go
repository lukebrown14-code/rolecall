package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/reporting"
)

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
// switching screens never sheds or restores a count or a name. One rule sheds by width:
// other screens' counts, the word "rolecall", the gap between tabs (two cells to one), then
// other screens' names, which keeps every screen named from 61 columns.
func (m model) tabs(p pal) (string, []int) {
	room := m.layout().w - rightMargin
	wide := m.layout().wide()
	// The Applied-per-week sparkline, the same series Stats draws over its last weeks, sits
	// beside the Stats tab at wide sizes with its "applied/wk" label, until the bar is short
	// of room for both, when it goes: the line never shows unlabelled.
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
			// A Settings problem keeps a ⚠ on the tab at every width, so it outlives the
			// feedback line it was first shown on.
			warn := ""
			if d.name == screenSettings && m.settingsBroken() {
				warn = glyphWarn
			}
			tail := ""
			if wide && level < shedSparkLabel && d.name == screenStats && spark != "" {
				// Only with its label: a bare row of bars reads as a glitch or a loading bar.
				tail = spark + sparkLabel + " "
			}
			if si == active {
				chip := digit + name + count
				if warn != "" {
					chip += " " + warn
				}
				out += p.SelChip(chip) + tail
				continue
			}
			if level >= shedCounts {
				count = ""
			}
			if level >= shedNames {
				name = ""
			}
			sep := " "
			if level >= shedGap {
				sep = "" // one cell between tabs: the leading space alone
				if si+1 == active {
					sep = " " // …but the active chip after it never touches it: "2 Drafts [3 Pipeline 9]"
				}
			}
			if name == "" && count == "" {
				sep = "" // a bare digit needs no trailing space, so the tabs do not spread
				if si+1 == active {
					sep = " " // …but the active chip after it never touches it: "3 [4 Stats]"
				}
			}
			lead := " "
			if level >= shedGap && si == 0 {
				lead = "" // the first tab starts at the edge, where the active chip's bracket does
			}
			if warn != "" {
				warn = p.Text(p.Warn).Render(" " + warn)
				sep = " "
			}
			out += lead + p.Key().Render(digit) + p.Text(p.Fg).Render(name) + p.Text(p.Dim).Render(count) + warn + sep + tail
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
// space and ? live on the key line alone, at the same place at every width.
func (m model) topBar(p pal) string {
	out, _ := m.tabs(p)
	return xansi.Truncate(out, m.layout().w-rightMargin, "")
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
// "Pipeline › All · All tracks · row 1 of 9", and at the right what just happened, a toast
// or an error with its keys. The breadcrumb gives way first: its focus, then the Track.
// When the message cannot sit whole beside it, the message takes the whole line: what
// just happened matters more than where the cursor is.
func (m model) breadcrumbLine(p pal) string {
	w := m.layout().w
	right := ""
	if m.feedbackVisible() {
		msg, keys, short := m.feedbackParts(p)
		// The breadcrumb keeps the one form it has with no message on show: the message
		// sits beside it when both fit whole, and otherwise takes the line alone. The
		// breadcrumb never switches to a shorter variant while a message shows.
		beside := w - rightMargin - gutter - xansi.StringWidth(m.breadcrumbLeft(p, w-rightMargin))
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

// breadcrumbMin is the least room a breadcrumb gets before its stub is left out.
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

// breadcrumb is "Pipeline › All · All tracks · row 1 of 9 · detail", the Track abbreviated
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
	// like its screen (Pipeline › Applied) is still a filter, so it shows.
	if v := m.views()[m.table]; m.table > 0 || v != screenDefs[m.screen].name {
		parts = append([]string{v}, parts...)
	}
	out := name + p.Text(p.Dim).Render(" › "+strings.Join(parts, " · "))
	if n := m.markCount(); n > 0 {
		out += p.Text(p.Dim).Render(" · ") + p.Text(p.Fg).Bold(true).Render(fmt.Sprintf("%s %d marked", glyphMarked, n))
	}
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
