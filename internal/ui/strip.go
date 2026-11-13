package ui

import (
	tea "charm.land/bubbletea/v2"
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

// shortLabel is a cell's label in the short forms: the list's own abbreviation where it
// has one ("Interv."), otherwise the label.
func (c stripCell) shortLabel() string {
	if s, ok := stageNames[c.status]; ok {
		return s
	}
	return c.label
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
// whether the Follow-up due part is spelled out, and how the docs share shows.
type stripForm struct {
	arrows  bool
	labels  bool // the cells name their Status; off, they are glyphs and counts
	short   bool // the labels are abbreviated, "Interv."
	compact bool // two spaces between the labelled cells; the closed, due and docs parts glyph-tight after them
	words   int  // wordsFull spells "N follow-up due", wordsShort "N due", 0 just the count
	docs    int  // docsMeter "docs ▰▱▱▱ 8%", docsShare "docs 8%", 0 none
	closed  bool // the closed cell keeps its word
}

// How a strip form shows the docs share.
const (
	docsShare = 1
	docsMeter = 2
)

// The Follow-up due part's wordings, longest first; 0 is the bare count.
const (
	wordsShort = 1
	wordsFull  = 2
)

// percentFull is the readiness share when every document is Ready.
const percentFull = 100

// stripForms is every fit, widest first. The medium size class starts at the third; the
// narrow one has only the last narrowForms, which drop the labels but keep the docs share
// while it fits.
var stripForms = []stripForm{
	{arrows: true, labels: true, words: wordsFull, docs: docsMeter, closed: true},
	{arrows: true, labels: true, words: wordsShort, docs: docsMeter, closed: true},
	{labels: true, words: wordsFull, docs: docsMeter, closed: true},
	{labels: true, words: wordsShort, docs: docsMeter, closed: true},
	{labels: true, words: wordsShort, closed: true},
	{labels: true, short: true, docs: docsShare},
	{labels: true, short: true, compact: true, docs: docsShare},
	{labels: true, short: true, compact: true},
	{docs: docsShare},
	{},
}

// narrowForms is how many of the last stripForms the narrow size class may use.
const narrowForms = 2

// stripSelMargin is the cells the active cell's brackets add, kept free in every fit so
// moving the highlight never changes which form fits.
const stripSelMargin = 2

// docsMeterCells is the width of the strip's readiness meter.
const docsMeterCells = 4

// pipelineStrip is the strip fitted to w cells: the widest form that fits, down to
// glyphs and counts. It never wraps.
func (m model) pipelineStrip(p pal, w int) string {
	f := m.stripFormFor(p, w)
	lead := strings.Repeat(" ", inset)
	if m.stripFocus {
		lead = p.SelGutter(false) + " " // the gutter mark: the strip has the keys
	}
	return xansi.Truncate(lead+m.stripLine(p, f), w-rightMargin, "…")
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
		forms = forms[len(forms)-narrowForms:]
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
	sep, closedSep := f.seps()
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
	switch {
	case f.compact:
		sep = "  "
	case f.arrows:
		sep, closedSep = p.Text(p.Rule).Render(" ──▶ "), dim.Render(" · ")
	case f.labels:
		sep, closedSep = dim.Render(" · "), dim.Render(" · ")
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
		if f.labels && !f.compact {
			text = followUpFlag + " " + text
		} else {
			text = followUpFlag + text
		}
		out += closedSep + p.Text(p.Warn).Render(text)
	}
	if f.docs > 0 {
		out += closedSep + m.stripDocs(p, f.docs == docsMeter)
	}
	return out
}

// seps is the cells between the Status cells, and before the closed cell and each part
// after it, as stripLine draws them.
func (f stripForm) seps() (sep, closedSep int) {
	switch {
	case f.compact:
		return 2, 1 //nolint:mnd // "  " between the cells, " " after them
	case f.arrows:
		return 5, 3 //nolint:mnd // " ──▶ ", " · "
	case f.labels:
		return 3, 3 //nolint:mnd // " · "
	}
	return 1, 1
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
	switch {
	case f.labels && label && f.short:
		text = fmt.Sprintf("%s %s %d", glyph, c.shortLabel(), n)
	case f.labels && label:
		text = fmt.Sprintf("%s %s %d", glyph, c.label, n)
	case f.labels && !f.compact:
		text = fmt.Sprintf("%s %d", glyph, n)
	}
	if c.on(m) {
		return p.SelChip(text)
	}
	if n == 0 {
		return p.Text(p.Dim).Render(text)
	}
	// The glyph and label in the Status colour, the count in the bright text colour.
	count := fmt.Sprint(n)
	return p.Text(col).Render(strings.TrimSuffix(text, count)) + p.Text(p.Fg).Render(count)
}

// stripDocs is the Workspace's document readiness: the meter and the share of ready
// documents, "docs ▰▰▱▱ 48%", or the share alone, "docs 48%".
func (m model) stripDocs(p pal, meter bool) string {
	pct := readyPercent(m.apps)
	on := (pct*docsMeterCells + percentFull/2) / percentFull
	c := p.Dim
	if pct == percentFull {
		c = p.Ok
	}
	share := p.Text(c).Render(fmt.Sprintf("%d%%", pct))
	if !meter {
		return p.Text(p.Dim).Render("docs ") + share
	}
	cells := p.Text(c).Render(strings.Repeat(meterOn, on)) + p.Text(p.Faint).Render(strings.Repeat(meterOff, docsMeterCells-on))
	return p.Text(p.Dim).Render("docs ") + cells + " " + share
}

// stripKey handles a key while the strip has focus: ← → move the status filter one cell and
// show it, enter keeps it, esc too. Any other key hands the keyboard back to the list and
// is handled there.
func (m model) stripKey(k tea.KeyPressMsg) (model, bool) {
	if !m.hasTables() || m.detail != nil {
		m.stripFocus = false
		return m, false
	}
	switch k.String() {
	case "left", "right":
		// With no cell shown (All), → lands on the first and ← on the last.
		next := 0
		if at := slices.IndexFunc(stripCells, func(c stripCell) bool { return c.on(m) }); at >= 0 {
			next = (at + 1) % len(stripCells)
			if k.String() == "left" {
				next = (at + len(stripCells) - 1) % len(stripCells)
			}
		} else if k.String() == "left" {
			next = len(stripCells) - 1
		}
		m = m.showStripCell(stripCells[next])
		return m.syncDetail(), true
	case "enter", "esc", "S":
		m.stripFocus = false
		return m, true
	}
	m.stripFocus = false
	return m, false
}
