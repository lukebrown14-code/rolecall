package ui

import (
	"fmt"
	"slices"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The pipeline strip: the Status counts, the follow-ups due and the Workspace's document
// readiness on one line, which is also the status filter. The cell for the view the list
// shows is drawn in the selection style; "All" highlights nothing.

// stripCell is one Status in the strip. A cell stands for one or more views: Closed and
// Dismissed share the closed cell, as the theme's Closed group does.
type stripCell struct {
	status string // the Status whose glyph and colour it takes, "" for closed
	label  string
	views  []stripView
}

// stripView is a view of a screen, counted into a cell.
type stripView struct {
	screen string
	table  string
}

var stripCells = []stripCell{
	{store.StatusLead, "Leads", []stripView{{screenLeads, tableLeads}}},
	{store.StatusDraft, "Drafts", []stripView{{screenDraft, tableDraft}}},
	{store.StatusApplied, "Applied", []stripView{{screenApplied, tableApplied}}},
	{store.StatusInterviewing, "Interviewing", []stripView{{screenApplied, tableInterviewing}}},
	{store.StatusOffer, "Offer", []stripView{{screenApplied, tableOffer}}},
	{"", "closed", []stripView{{screenApplied, tableClosed}, {screenLeads, tableDismissed}}},
}

// count is the cell's Applications under the Track and text filters, the same number the
// list's group rule shows.
func (c stripCell) count(m model) int {
	n := 0
	for _, v := range c.views {
		n += m.count(screenIndex(v.screen), v.table)
	}
	return n
}

// on reports whether the cell stands for the view the list shows.
func (c stripCell) on(m model) bool {
	if !m.hasTables() {
		return false
	}
	return slices.ContainsFunc(c.views, func(v stripView) bool {
		return v.screen == screenDefs[m.screen].name && v.table == m.views()[m.table]
	})
}

// followUpsDue is how many live Applications under the filters are Follow-up due.
func (m model) followUpsDue() int {
	n := 0
	for _, a := range m.apps {
		if !isActive(a) || !m.matches(a) {
			continue
		}
		if _, flag, _ := m.slotText(a); flag != "" {
			n++
		}
	}
	return n
}

// stripForm is one fit of the strip, widest first: the arrows or the dots between cells,
// whether the Follow-up due part is spelled out, and whether the docs share shows.
type stripForm struct {
	arrows bool
	labels bool // the cells name their Status; off, they are glyphs and counts
	words  int  // wordsFull spells "N follow-up due", wordsShort "N due", 0 just the count
	docs   bool
	closed bool // the closed cell keeps its word
}

// The Follow-up due part's wordings, longest first; 0 is the bare count.
const (
	wordsShort = 1
	wordsFull  = 2
)

// percentFull is the readiness share when every document is Ready.
const percentFull = 100

var stripForms = []stripForm{
	{true, true, wordsFull, true, true},
	{true, true, wordsShort, true, true},
	{false, true, wordsFull, true, true},
	{false, true, wordsShort, true, true},
	{false, true, wordsShort, false, true},
	{false, true, 0, false, false},
	{false, false, 0, false, false},
}

// stripSelMargin is the cells the active cell's brackets add, kept free in every fit so
// moving the highlight never changes which form fits.
const stripSelMargin = 2

// docsMeterCells is the width of the strip's readiness meter.
const docsMeterCells = 4

// pipelineStrip is the strip fitted to w cells: the widest form that fits, down to
// glyphs and counts. It never wraps.
func (m model) pipelineStrip(p pal, w int) string {
	f := m.stripFormFor(p, w)
	return xansi.Truncate(strings.Repeat(" ", inset)+m.stripLine(p, f), w-rightMargin, "…")
}

// stripFormFor is the widest form of the strip that fits w cells.
func (m model) stripFormFor(p pal, w int) stripForm {
	room := w - rightMargin - inset
	forms := stripForms
	switch m.layout().class {
	case classWide:
	case classMedium:
		forms = forms[2:]
	default:
		forms = forms[len(forms)-1:]
	}
	// The shown cell's brackets are in the line already; otherwise their cells are kept free.
	spare := stripSelMargin
	if slices.ContainsFunc(stripCells, func(c stripCell) bool { return c.on(m) }) {
		spare = 0
	}
	for _, f := range forms {
		if xansi.StringWidth(m.stripLine(p, f))+spare <= room {
			return f
		}
	}
	return forms[len(forms)-1]
}

// stripCellAt is the cell whose columns hold column x of a strip drawn w cells wide.
func (m model) stripCellAt(x, w int) (stripCell, bool) {
	p := m.pal()
	f := m.stripFormFor(p, w)
	sep, closedSep := 1, 1
	if f.labels {
		sep, closedSep = 3, 3
		if f.arrows {
			sep = 5
		}
	}
	col := inset
	for i, c := range stripCells {
		width := xansi.StringWidth(m.stripCellText(p, c, f, f.closed || i < len(stripCells)-1))
		if x >= col && x < col+width {
			return c, true
		}
		col += width + sep
		if i == len(stripCells)-2 {
			col += closedSep - sep
		}
	}
	return stripCell{}, false
}

// stripLine draws one form of the strip.
func (m model) stripLine(p pal, f stripForm) string {
	dim := p.Text(p.Dim)
	var cells []string
	for _, c := range stripCells[:len(stripCells)-1] {
		cells = append(cells, m.stripCellText(p, c, f, true))
	}
	sep, closedSep := " ", " "
	if f.labels {
		sep = dim.Render(" · ")
		if f.arrows {
			sep = dim.Render(" ──▶ ")
		}
		closedSep = dim.Render(" · ")
	}
	out := strings.Join(cells, sep) + closedSep + m.stripCellText(p, stripCells[len(stripCells)-1], f, f.closed)
	if due := m.followUpsDue(); due > 0 {
		text := fmt.Sprint(due)
		switch f.words {
		case wordsFull:
			text += " follow-up due"
		case wordsShort:
			text += " due"
		}
		if !f.labels {
			text = followUpFlag + text
		} else {
			text = followUpFlag + " " + text
		}
		out += closedSep + p.Text(p.Warn).Render(text)
	}
	if f.docs {
		out += closedSep + m.stripDocs(p)
	}
	return out
}

// stripCellText is one cell: glyph, label and count, with the label dropped narrow. The
// cell for the shown view is bracketed and drawn in the selection style, so it is marked
// without colour too.
func (m model) stripCellText(p pal, c stripCell, f stripForm, label bool) string {
	glyph, col := p.ClosedGroupGlyph(), p.Dim
	if c.status != "" {
		glyph, col = p.StatusGlyph(c.status), p.StatusColor(c.status)
	}
	n := c.count(m)
	text := fmt.Sprintf("%s%d", glyph, n)
	if f.labels {
		text = fmt.Sprintf("%s %s %d", glyph, c.label, n)
		if !label {
			text = fmt.Sprintf("%s %d", glyph, n)
		}
	}
	if c.on(m) {
		return p.Text(p.Accent).Render("[") + p.SelText(true).Render(text) + p.Text(p.Accent).Render("]")
	}
	if n == 0 {
		return p.Text(p.Dim).Render(text)
	}
	return p.Text(col).Render(text)
}

// stripDocs is the Workspace's document readiness: the meter and the share of ready
// documents, "docs ▰▰▱▱ 48%".
func (m model) stripDocs(p pal) string {
	pct := readyPercent(m.apps)
	on := (pct*docsMeterCells + percentFull/2) / percentFull
	c := p.Dim
	if pct == percentFull {
		c = p.Ok
	}
	meter := p.Text(c).Render(strings.Repeat(meterOn, on)) + p.Text(p.Faint).Render(strings.Repeat(meterOff, docsMeterCells-on))
	return p.Text(p.Dim).Render("docs ") + meter + p.Text(c).Render(fmt.Sprintf(" %d%%", pct))
}
