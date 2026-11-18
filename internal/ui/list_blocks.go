package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

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

// rollup is what a group's rule adds after its count when non-zero: the Follow-ups due,
// the Applications whose documents are Ready, and the unreadable rows the count includes,
// " · 1 follow-up due · 2 Ready · 1 unreadable", so the count never reads as Applications
// of the group's Status alone. Closed and Expired groups have nothing left to nudge or
// send, so they carry only the unreadable count.
func (m model) rollup(label string, group []store.Application) string {
	unreadable := ""
	if n := countUnreadable(group); n > 0 {
		unreadable = fmt.Sprintf(" · %d unreadable", n)
	}
	if label == tableClosed || label == groupExpired {
		return unreadable
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
	return out + unreadable
}

// countUnreadable is how many of apps are malformed: listed, but with no Status to count.
func countUnreadable(apps []store.Application) int {
	n := 0
	for _, a := range apps {
		if a.Malformed != nil {
			n++
		}
	}
	return n
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
	return schedule.Passed(a, m.rules, m.now())
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
	var cues []string
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
// "↑ n more" line takes the first line when start is past the top.
func scrollWindow(blocks []block, cur, h int) (start, end int) {
	fit := func(start int) int {
		avail := h
		if start > 0 {
			avail-- // the ↑ cue's line
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

// slotSet is the width of each right-hand column, gap included: the Follow-up due flag and
// Fit. Every row in a view uses the same set, so each column starts at the same cell on
// every row and nothing wraps. A width of 0 hides the column. Compact rows also size
// their title, company and track columns from it: each value's widest in the view.
type slotSet struct {
	flagW, fitW              int
	titleW, companyW, trackW int
	datesW                   int // a compact row's first date, kept whole (compactColumns)
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
	compactCompanyMin = 8  // the narrowest the company column gets before it goes
	compactTitleFloor = 10 // the narrowest the title gets to keep the company and the date
	// compactDatesMax is the most of a compact row's first date ("21 Nov 10:30 · Phone
	// screen", "applied ~7w ago") kept whole before the title gives way.
	compactDatesMax = 28
)

// slotsFor sizes each right-hand column to its widest value in rows, the whole view rather
// than the rows in sight, so the columns hold still while scrolling. A column empty on
// every row is hidden, and the title takes the room. columnGap cells part each column
// from the one before it; the first shown column is parted from the title by the title's
// own one cell and columnGap-1 more, so the title keeps the same gap.
func (m model) slotsFor(rows []store.Application) slotSet {
	var s slotSet
	var flagW, fitW int
	compact, p := m.compactShown(), m.pal()
	plain := func(color.Color) lipgloss.Style { return lipgloss.NewStyle() }
	for _, a := range rows {
		if compact && a.Malformed == nil {
			if d := m.compactDates(p, a, plain, p.Dim); len(d) > 0 {
				s.datesW = max(s.datesW, min(compactDatesMax, xansi.StringWidth(d[0])))
			}
		}
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
	if _, due := schedule.FollowUpDaysPast(a, m.now(), m.followUpDays); due {
		flag = followUpFlag + " " + m.followUpQuiet(a)
	}
	return flag, a.Fit
}

// groupExpired is the divider over Leads and Drafts whose close date has passed.
const groupExpired = "Expired"
