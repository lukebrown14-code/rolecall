package ui

import (
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
	appearance "github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// Layout constants for the list view.
const (
	fallbackWidth  = 100
	fallbackHeight = 30
	rowPad         = 1                 // keeps the last label off the edge of the selection bar
	soonDays       = schedule.SoonDays // "closes in Nd" turns amber at or below this
	percent        = 100
	minTitle       = 8 // a title never shrinks below this
	previewInset   = 3 // padding and divider around the preview pane
	errorLines     = 8 // lines of a parse error shown in the preview
	docLines       = 4 // lines of fit.md / notes.md shown in the preview
	titleLines     = 2 // a long title wraps to at most this in the preview
	minBody        = 4
	shedSparkLabel = 1 // topBar detail levels: drop the sparkline with its "applied/wk" label…
	shedCounts     = 2 // …then other screens' counts…
	shedWordmark   = 3 // …then the word "rolecall"…
	shedGap        = 4 // …then the gap between tabs narrows to one cell…
	shedNames      = 5 // …then other screens' names, keeping their digits…
	shedMark       = 6 // …then the wordmark's mark
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
	t, days, ok := schedule.DaysFrom(date, now)
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

// rel shows paths inside the Workspace relative to it, so messages fit their line and
// read the same on every machine.
func (m model) rel(s string) string {
	return strings.ReplaceAll(s, m.store.Root()+string(filepath.Separator), "")
}

// futureFlag marks a Status date after today: almost always a data error, so it is
// flagged rather than shown as a normal "in 8mo" relative date.
const futureFlag = "⚠ future date"

// futureDate reports whether a Status entry's date is after today.
func (m model) futureDate(e *store.StatusEntry) bool {
	if e == nil {
		return false
	}
	t, ok := store.LocalDay(e.At, m.now().Location())
	return ok && t.After(store.Today(m.now()))
}

func (m model) View() tea.View {
	p := m.pal()
	plain := plainMode()
	strip := func(s string) string {
		if plain {
			return plainFrame(s, m.viewWidth())
		}
		return s
	}
	var body string
	switch box := m.overlayView(p); {
	case m.tooSmall():
		body = m.tooSmallView(p)
	case box != "" && m.helpScreenOpen():
		body = strip(box) // the full-screen help is the whole frame
	case box != "" && plain:
		body = m.plainPopUp(p, box) // read alone: nothing of the frame shares its lines
	case box != "":
		behind := m.screenView(m.dimmedPal())
		if noColor() {
			behind = appearance.StripBold(behind) // without the dimmed colours, weight sets the pop-up apart
		}
		body = m.overlayBody(p, behind, box)
	default:
		body = strip(m.screenView(p))
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
		query := p.Text(p.Fg).Render(m.text)
		if sm, _ := m.overlay.(*searchMode); sm.selected {
			query = p.SelText(true).Render(m.text) // typing replaces it
		}
		in := p.Text(p.Fg).Bold(true).Render(" / ") + query + p.Text(p.Fg).Render("▏")
		keys = spread(in, p.Keys(pairs(m.helpKeys(), groupAction, groupFilter, groupNav)), w-1)
	}
	// The bottom rule is always drawn; the status row below it holds a toast, an error,
	// the search input or the key line, one at a time.
	// The list holds the cursor, so with a preview beside it the list's rule is the accent.
	focus := focusNone
	if divider > 0 {
		focus = focusLeft
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
