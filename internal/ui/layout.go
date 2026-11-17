package ui

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// The layout system: every screen and pop-up takes its sizes from here, so the tab bar,
// the panes and the pop-ups make one decision per size instead of solving overflow on
// their own.
//
// A frame is, top to bottom: the tab bar, a rule (dropped when short), the
// context lines, a rule, the body, a rule and the status row. Width falls into one of
// three size classes; height is short or not. The detail view has two layouts: beside the
// list when wide, full-screen otherwise.

// sizeClass names a width budget. The zero value is the narrow class: below mediumWidth
// there is one pane everywhere and the tab bar shows only the active screen's name.
type sizeClass int

const (
	// classMedium is mediumWidth up to wideWidth: Profile and Settings keep their side
	// lists; the list has no preview and the detail view is full-screen.
	classMedium sizeClass = iota + 1
	// classWide is wideWidth and up: the preview beside the list, the card beside the
	// detail view, and the wordmark, sparkline and menu keys in the tab bar.
	classWide
)

const (
	wideWidth     = 100
	mediumWidth   = 72
	shortHeight   = 20 // at or below this the rule under the tab bar goes
	minTermWidth  = 56 // below this, or minTermHeight, a "needs" notice replaces the layout
	minTermHeight = 14
	edge          = 1 // the chrome column: tab bar mark, selection bar, list markers, key line
	inset         = 3 // the text column: where content starts after a marker
	rightMargin   = 1 // cells kept clear at the right edge, matching edge
	ruleLines     = 2 // the rules above and below the body
)

// detailMode is how an Application's detail is reached at a size: two layouts, side by
// side when wide and full-screen at every other size.
type detailMode int

const (
	// detailPane shows the detail beside the list (wide).
	detailPane detailMode = iota + 1
	// detailFull opens the detail as its own full-screen view on enter (80 columns and
	// below, and the medium widths up to the wide breakpoint).
	detailFull
)

const (
	filtersFull      = 2 // filter lines when the filters have room
	filtersCollapsed = 1 // and when collapsed to one line
	// columnGap is the cells between a list row's right-hand columns, and between the
	// title and the first of them, so neighbours never touch (see slotsFor).
	columnGap = 2
)

// layout is one frame's size budget: everything a view needs to place itself, from the
// terminal size alone.
type layout struct {
	w, h  int
	class sizeClass
	short bool
	// detail is how the detail is shown; listW and detailW are the list and the detail
	// pane beside it (detailW is 0 with one pane, and the list takes the width). With a
	// pane, listW + the divider + detailW is the width.
	detail         detailMode
	listW, detailW int
	// filterLines is the lines the status and Track filters take: two, or one collapsed.
	filterLines int
	// footerRow is the row the footer sits on: always the last.
	footerRow int
	// profileW and groupsW are Profile's section list and
	// Settings' group list.
	profileW, groupsW int
	// status is the lines the status area needs: one for the key line, or two while the
	// feedback line sits above it, so the key line is never wiped by a toast or an error.
	status int
}

// layout is the budget for the current terminal size.
func (m model) layout() layout {
	l := plainLayout(sizeLayout(m.viewWidth(), m.viewHeight()))
	l.status = m.frameStatusLines()
	return l
}

// sizeLayout is the pure layout for a terminal of w by h: no rendering, no model, no clock.
func sizeLayout(w, h int) layout {
	l := layout{w: w, h: h, short: h <= shortHeight, detail: detailFull, filterLines: filtersFull, listW: w,
		footerRow: max(0, h-1), status: statusAreaPlain}
	switch {
	case w >= wideWidth:
		l.class = classWide
		l.detail = detailPane
		l.detailW = l.panel(previewShare, previewMin, previewMax)
		l.listW = w - l.detailW - 1
	case w >= mediumWidth:
		l.class = classMedium
	}
	if w < mediumWidth {
		l.filterLines = filtersCollapsed
	}
	l.profileW = l.panel(profileShare, profileMin, profileMax)
	l.groupsW = l.panel(groupsShare, groupsMin, groupsMax)
	return l
}

func (l layout) wide() bool { return l.class == classWide }

// headerLines is the tab bar and, unless short, the rule under it.
func (l layout) headerLines() int {
	if l.short {
		return 1
	}
	return 2 //nolint:mnd // tab bar and the rule under it
}

// bodyTop is the first body line under ctx context lines.
func (l layout) bodyTop(ctx int) int { return l.headerLines() + ctx + 1 }

// bodyHeight is the lines between the two rules under ctx context lines.
func (l layout) bodyHeight(ctx int) int {
	return max(minBody, l.h-l.headerLines()-ctx-ruleLines-l.status)
}

// panel is a side panel's width: share percent of the width, held between lo and hi. Only
// sizeLayout calls it; views read the finished widths.
func (l layout) panel(share, lo, hi int) int { return min(max(l.w*share/percent, lo), hi) }

// Side panels: share of the width, then the least and most cells.
const (
	previewShare, previewMin, previewMax = 45, 44, 72 // the preview beside the list
	profileShare, profileMin, profileMax = 28, 26, 36 // Profile's section list: short names whole from 80 columns
	groupsShare, groupsMin, groupsMax    = 18, 18, 24 // Settings' group list
)

// contextLines is how many context lines the current screen draws under the tab bar.
func (m model) contextLines() int {
	if m.detail != nil {
		return m.detailContextLines()
	}
	if m.hasTables() {
		return tableContext
	}
	return 1
}

// tableContext is a table screen's context lines: the filter row. The breadcrumb sits in
// the footer, over the key line.
const tableContext = 1

// listHeight is the lines the list gets: the whole body.
func (m model) listHeight() int { return m.layout().bodyHeight(tableContext) }

// frame stacks one screen: tab bar, rule, context lines, rule, body, rule, status row.
// split is the column of the body's vertical divider, 0 for none; the rules meet it
// with ┬ and ┴.
func (m model) frame(p pal, ctx []string, body string, split int, focus paneFocus, status string) string {
	l := m.layout()
	rule := p.Text(p.Rule)
	top, bot := rule.Render(strings.Repeat("─", l.w)), rule.Render(strings.Repeat("─", l.w))
	if split > 0 && split < l.w {
		left, right := strings.Repeat("─", split), strings.Repeat("─", l.w-split-1)
		top = p.PaneRule(focus == focusLeft).Render(left) + rule.Render("┬") + p.PaneRule(focus == focusRight).Render(right)
		bot = rule.Render(left + "┴" + right)
	}
	parts := []string{m.topBar(p)}
	if !l.short {
		parts = append(parts, rule.Render(strings.Repeat("─", l.w)))
	}
	parts = append(parts, ctx...)
	parts = append(parts, top, fitLines(body, l.bodyHeight(len(ctx))), bot, status)
	return strings.Join(parts, "\n")
}

// fitLines makes body exactly h lines: padded with blanks or cut, so the status rows
// always land on the terminal's last rows.
func fitLines(body string, h int) string {
	lines := strings.Split(body, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// paneFocus is which of two panes has focus, for the frame's top rule: the focused
// pane's part is the accent, the other the rule colour.
type paneFocus int

const (
	focusNone paneFocus = iota
	focusLeft
	focusRight
)

// divider is a vertical rule h lines tall.
func (p pal) Divider(h int) string {
	return p.Text(p.Rule).Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
}

// contextRow places left and right on one context line, the right flush with the right
// margin and cut to what left leaves, so the row never spills.
func contextRow(left, right string, w int) string {
	left = xansi.Truncate(left, max(0, w-rightMargin), "…")
	if right == "" {
		return left
	}
	avail := max(0, w-rightMargin-xansi.StringWidth(left)-gutter)
	if avail == 0 {
		return left
	}
	return spread(left, xansi.Truncate(right, avail, "…"), w-rightMargin)
}

// moreCue is the one overflow cue: "↓ 3 more" or "↑ 3 more", set at the caller's text
// column.
func (p pal) MoreCue(up bool, n int) string {
	arrow := "↓"
	if up {
		arrow = "↑"
	}
	return p.Text(p.Dim).Render(fmt.Sprintf("%s %d more", arrow, n))
}

// clipLines keeps h of lines starting near from, replacing the first and last kept line
// with a moreCue when lines are hidden that way. indent sets the cue at the text column.
// The result is exactly h lines.
func (p pal) ClipLines(lines []string, from, h int, indent string) []string {
	if h <= 0 {
		return nil
	}
	if len(lines) <= h {
		out := append([]string{}, lines...)
		for len(out) < h {
			out = append(out, "")
		}
		return out
	}
	start := min(max(0, from), len(lines)-h)
	out := append([]string{}, lines[start:start+h]...)
	if start > 0 {
		out[0] = indent + p.MoreCue(true, start+1)
	}
	if hidden := len(lines) - (start + h) + 1; hidden > 1 {
		// A section's first line, after a blank, never sits alone above the cue.
		blank := func(s string) bool { return strings.TrimSpace(xansi.Strip(s)) == "" }
		const minRun = 3
		if h >= minRun && blank(out[h-3]) && !blank(out[h-2]) {
			out[h-2] = ""
			hidden++
		}
		out[h-1] = indent + p.MoreCue(false, hidden)
	}
	return out
}

// wrapWords wraps plain text to width w, breaking only at spaces: "subl --wait" and "fit
// not assessed" stay whole when they fit a line. A word longer than a line is the only
// thing ever cut, and only at the line's end.
func wrapWords(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	var out []string
	for _, para := range strings.Split(s, "\n") {
		line := ""
		for _, word := range strings.FieldsFunc(para, unicode.IsSpace) {
			for xansi.StringWidth(word) > w {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				out = append(out, xansi.Truncate(word, w, ""))
				word = xansi.TruncateLeft(word, w, "")
			}
			switch {
			case line == "":
				line = word
			case xansi.StringWidth(line)+1+xansi.StringWidth(word) <= w:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}
		out = append(out, line)
	}
	return out
}

// keepTogether joins phrases with spaces on one line when they fit w, otherwise one per
// line, so a phrase such as "Fit: not assessed" is never split across lines.
func keepTogether(w int, sep string, phrases ...string) string {
	var lines []string
	line := ""
	for _, ph := range phrases {
		switch {
		case ph == "":
		case line == "":
			line = ph
		case xansi.StringWidth(line)+xansi.StringWidth(sep)+xansi.StringWidth(ph) <= w:
			line += sep + ph
		default:
			// A phrase starting its own line loses the spaces that set it apart.
			lines = append(lines, line)
			lead := len(xansi.Strip(ph)) - len(strings.TrimLeft(xansi.Strip(ph), " "))
			line = xansi.TruncateLeft(ph, lead, "")
		}
	}
	return strings.Join(append(lines, line), "\n")
}

// tooSmall reports a terminal below the smallest size the layout honestly renders.
// Before the first size message the fallback size applies, which is never too small.
func (m model) tooSmall() bool {
	return m.width > 0 && (m.width < minTermWidth || m.height < minTermHeight)
}

// tooSmallView replaces the layout on a terminal too small for it, centred.
func (m model) tooSmallView(p pal) string {
	// Say how to leave too: with a large font the Owner may not be able to resize.
	head := p.Text(p.Warn).Bold(true).Render("⚠ ") + p.Text(p.Fg).Bold(true).Render(fmt.Sprintf("rolecall needs %d×%d", minTermWidth, minTermHeight))
	now := p.Text(p.Dim).Render(fmt.Sprintf("now %d×%d · resize to continue", m.width, m.height))
	quit := p.Key().Render("q") + p.Text(p.Dim).Render(" quit")
	msg := head + "\n" + now + p.Text(p.Dim).Render(" · ") + quit
	if xansi.StringWidth(now)+len(" · q quit") > m.width {
		msg = head + "\n" + now + "\n" + quit
	}
	block := strings.Split(msg, "\n")
	for i, l := range block {
		block[i] = xansi.Truncate(l, m.width, "")
	}
	placed := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, strings.Join(block, "\n"))
	lines := strings.Split(placed, "\n")
	return strings.Join(lines[:min(len(lines), m.height)], "\n")
}

// overlayBody draws box over base inside the body: centred on the full width and
// between the rules, never over the tab bar, context lines or status row. A box taller
// than the body scrolls inside its border, keeping the selection bar (▌) in view. Only
// the box's own footprint and a margin around it are cleared (see popMargin), so the
// dimmed frame stays in view beside it and no word is cut off against its border.
func (m model) overlayBody(p pal, base, box string) string {
	l, ctx := m.layout(), m.contextLines()
	top, h := l.bodyTop(ctx), l.bodyHeight(ctx)
	pin := 0
	if pm, ok := m.overlay.(pinnedMode); ok {
		pin = pm.pinned()
	}
	lines := clipBox(p, strings.Split(box, "\n"), h, pin)
	boxW := 0
	for _, line := range lines {
		boxW = max(boxW, xansi.StringWidth(line))
	}
	y := top + max(0, (h-len(lines))/bothSides)
	x := max(0, (l.w-boxW)/bothSides)
	side, rows := popMargin()
	layers := []*lipgloss.Layer{lipgloss.NewLayer(base)}
	baseRows := strings.Split(base, "\n")
	for row := max(top, y-rows); row < min(top+h, y+len(lines)+rows, len(baseRows)); row++ {
		from, to := wordEdges(xansi.Strip(baseRows[row]), max(0, x-side), min(l.w, x+boxW+side))
		layers = append(layers, lipgloss.NewLayer(strings.Repeat(" ", to-from)).X(from).Y(row).Z(layerClear))
	}
	layers = append(layers, lipgloss.NewLayer(strings.Join(lines, "\n")).X(x).Y(y).Z(layerPopUp))
	return lipgloss.NewCompositor(layers...).Render()
}

// wordEdges widens the cleared cells [from, to) of a backdrop row to the ends of any word
// they cut, so the frame beside a pop-up shows whole words or nothing. Rules and
// dividers are never words: the pane divider stays whole beside the pop-up.
func wordEdges(row string, from, to int) (int, int) {
	word := func(c int) bool {
		cell := xansi.Cut(row, c, c+1)
		return cell != "" && cell != " " && !strings.ContainsAny(cell, "│┬┴─|")
	}
	for from > 0 && word(from) && word(from-1) {
		from--
	}
	for to < xansi.StringWidth(row) && word(to-1) && word(to) {
		to++
	}
	return from, to
}

// The overlay's layers above the frame: the cleared margin, then the pop-up.
const (
	layerClear = 1
	layerPopUp = 2
)

// popMargin is the cells cleared around a pop-up: one at each side, so no backdrop word
// is cut against its border. Without colour there is no dimmed frame to set the pop-up
// apart, so the margin widens to two cells and a blank row above and below, and the
// frame behind loses its bold (see View): the pop-up is the only bold, ringed by space.
func popMargin() (side, rows int) {
	if noColor() {
		return gutter, 1
	}
	return 1, 0
}

// clipBox fits a pop-up into h lines. Its borders stay; the lines between them scroll to
// keep the first selection bar in view, with a moreCue where lines are hidden. The first
// pin lines of the body (a Move pop-up's subject) stay above the scrolling part, and the
// blank spacers around them go, as the room is short.
func clipBox(p pal, lines []string, h, pin int) []string {
	const borders = 2
	if len(lines) <= h || len(lines) <= borders {
		return lines
	}
	inner := lines[1 : len(lines)-1]
	blank := inner[0] // popup pads its body with a blank line top and bottom
	var head []string
	if pin > 0 && len(inner) > pin+1 {
		head, inner = inner[1:1+pin], inner[1+pin:]
		for len(inner) > 1 && strings.Trim(xansi.Strip(inner[0]), "│ ") == "" { // a blank row between its borders
			inner = inner[1:]
		}
	}
	top := append([]string{lines[0]}, head...)
	avail := max(1, h-borders-len(head))
	if len(inner) <= avail {
		return append(append(top, inner...), lines[len(lines)-1])
	}
	focus := 0
	for i, l := range inner {
		if strings.Contains(xansi.Strip(l), "▌") {
			focus = i
			break
		}
	}
	start := min(max(0, focus-avail/bothSides), len(inner)-avail)
	kept := append([]string{}, inner[start:start+avail]...)
	cue := func(text string) string {
		w := xansi.StringWidth(blank)
		pad := popEdges/bothSides + gutter // the border and the padding
		text = xansi.Truncate(text, max(0, w-pad*bothSides), "")
		return xansi.Cut(blank, 0, pad) + p.PanelFill(text) + xansi.Cut(blank, pad+xansi.StringWidth(text), w)
	}
	if start > 0 {
		kept[0] = cue(p.MoreCue(true, start+1))
	}
	// Count only the hidden lines that carry content: a pop-up's blank padding must not
	// read as hidden rows.
	hidden := 0
	for _, l := range inner[start+avail-1:] {
		if strings.TrimSpace(xansi.Strip(l)) != "" {
			hidden++
		}
	}
	if hidden > 0 && avail > 1 {
		kept[avail-1] = cue(p.MoreCue(false, hidden))
	}
	return append(append(top, kept...), lines[len(lines)-1])
}
