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
// A frame is, top to bottom: the tab bar, a blank line (dropped when short), the
// context lines, a rule, the body, a rule and the status row. Width falls into one of
// three size classes; height is short or not.

// sizeClass names a width budget.
type sizeClass int

const (
	// classNarrow is below mediumWidth: one pane everywhere; the tab bar shows only the
	// active screen's name.
	classNarrow sizeClass = iota
	// classMedium is mediumWidth up to wideWidth: Profile and Settings keep their side
	// lists; the list has no preview and the detail view's card folds into a strip.
	classMedium
	// classWide is wideWidth and up: the preview beside the list, the card beside the
	// detail view and the pipeline summary in the tab bar.
	classWide
)

const (
	wideWidth     = 110
	mediumWidth   = 80
	shortHeight   = 20 // at or below this the blank line under the tab bar goes
	minTermWidth  = 56 // below this, or minTermHeight, a "needs" notice replaces the layout
	minTermHeight = 14
	edge          = 1 // the chrome column: tab bar mark, selection bar, list markers, key line
	inset         = 3 // the text column: where content starts after a marker
	rightMargin   = 1 // cells kept clear at the right edge, matching edge
	ruleLines     = 2 // the rules above and below the body
	statusLines   = 1 // the status row
)

// layout is one frame's size budget.
type layout struct {
	w, h  int
	class sizeClass
	short bool
}

// layout is the budget for the current terminal size.
func (m model) layout() layout { return sizeLayout(m.viewWidth(), m.viewHeight()) }

func sizeLayout(w, h int) layout {
	l := layout{w: w, h: h, short: h <= shortHeight}
	switch {
	case w >= wideWidth:
		l.class = classWide
	case w >= mediumWidth:
		l.class = classMedium
	}
	return l
}

func (l layout) wide() bool   { return l.class == classWide }
func (l layout) narrow() bool { return l.class == classNarrow }

// headerLines is the tab bar and, unless short, the blank line under it.
func (l layout) headerLines() int {
	if l.short {
		return 1
	}
	return 2 //nolint:mnd // tab bar and the blank line
}

// bodyTop is the first body line under ctx context lines.
func (l layout) bodyTop(ctx int) int { return l.headerLines() + ctx + 1 }

// bodyHeight is the lines between the two rules under ctx context lines.
func (l layout) bodyHeight(ctx int) int {
	return max(minBody, l.h-l.headerLines()-ctx-ruleLines-statusLines)
}

// panel is a side panel's width: share percent of the width, held between lo and hi.
func (l layout) panel(share, lo, hi int) int { return min(max(l.w*share/percent, lo), hi) }

// Side panels: share of the width, then the least and most cells.
const (
	previewShare, previewMin, previewMax = 45, 44, 72 // the preview beside the list
	cardShare, cardMin, cardMax          = 28, 30, 40 // the detail view's card
	profileShare, profileMin, profileMax = 26, 22, 32 // Profile's section list
	groupsShare, groupsMin, groupsMax    = 18, 18, 24 // Settings' group list
)

// contextLines is how many context lines the current screen draws under the tab bar.
func (m model) contextLines() int {
	if m.detail != nil {
		return m.detailContextLines()
	}
	return 1
}

// frame stacks one screen: tab bar, gap, context lines, rule, body, rule, status row.
// split is the column of the body's vertical divider, 0 for none; the rules meet it
// with ┬ and ┴.
func (m model) frame(p pal, ctx []string, body string, split int, focus paneFocus, status string) string {
	l := m.layout()
	rule := p.text(p.rule)
	top, bot := rule.Render(strings.Repeat("─", l.w)), rule.Render(strings.Repeat("─", l.w))
	if split > 0 && split < l.w {
		left, right := strings.Repeat("─", split), strings.Repeat("─", l.w-split-1)
		top = p.paneRule(focus == focusLeft).Render(left) + rule.Render("┬") + p.paneRule(focus == focusRight).Render(right)
		bot = rule.Render(left + "┴" + right)
	}
	parts := []string{m.topBar(p)}
	if !l.short {
		parts = append(parts, "")
	}
	parts = append(parts, ctx...)
	parts = append(parts, top, body, bot, status)
	return strings.Join(parts, "\n")
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
func (p pal) divider(h int) string {
	return p.text(p.rule).Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
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
func (p pal) moreCue(up bool, n int) string {
	arrow := "↓"
	if up {
		arrow = "↑"
	}
	return p.text(p.dim).Render(fmt.Sprintf("%s %d more", arrow, n))
}

// clipLines keeps h of lines starting near from, replacing the first and last kept line
// with a moreCue when lines are hidden that way. indent sets the cue at the text column.
// The result is exactly h lines.
func (p pal) clipLines(lines []string, from, h int, indent string) []string {
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
		out[0] = indent + p.moreCue(true, start+1)
	}
	if hidden := len(lines) - (start + h) + 1; hidden > 1 {
		// A section's first line, after a blank, never sits alone above the cue.
		blank := func(s string) bool { return strings.TrimSpace(xansi.Strip(s)) == "" }
		const minRun = 3
		if h >= minRun && blank(out[h-3]) && !blank(out[h-2]) {
			out[h-2] = ""
			hidden++
		}
		out[h-1] = indent + p.moreCue(false, hidden)
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
// line, so a phrase such as "fit not assessed" is never split across lines.
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
	msg := p.text(p.warn).Bold(true).Render("⚠ ") + p.text(p.fg).Bold(true).Render(fmt.Sprintf("rolecall needs %d×%d", minTermWidth, minTermHeight)) + "\n" +
		p.text(p.dim).Render(fmt.Sprintf("now %d×%d · resize to continue", m.width, m.height))
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
// than the body scrolls inside its border, keeping the selection bar (▌) in view. The
// rows the box covers are blanked edge to edge, so no cut-off text shows beside it.
func (m model) overlayBody(p pal, base, box string) string {
	l, ctx := m.layout(), m.contextLines()
	top, h := l.bodyTop(ctx), l.bodyHeight(ctx)
	lines := clipBox(p, strings.Split(box, "\n"), h)
	boxW := 0
	for _, line := range lines {
		boxW = max(boxW, xansi.StringWidth(line))
	}
	y := top + max(0, (h-len(lines))/bothSides)
	x := max(0, (l.w-boxW)/bothSides)
	rows := strings.Split(base, "\n")
	blank := strings.Repeat(" ", l.w)
	for i := range lines {
		if y+i < len(rows) {
			rows[y+i] = blank
		}
	}
	return lipgloss.NewCompositor(lipgloss.NewLayer(strings.Join(rows, "\n")),
		lipgloss.NewLayer(strings.Join(lines, "\n")).X(x).Y(y).Z(1)).Render()
}

// clipBox fits a pop-up into h lines. Its borders stay; the lines between them scroll to
// keep the first selection bar in view, with a moreCue where lines are hidden.
func clipBox(p pal, lines []string, h int) []string {
	const borders = 2
	if len(lines) <= h || len(lines) <= borders {
		return lines
	}
	inner := lines[1 : len(lines)-1]
	avail := max(1, h-borders)
	focus := 0
	for i, l := range inner {
		if strings.Contains(xansi.Strip(l), "▌") {
			focus = i
			break
		}
	}
	start := min(max(0, focus-avail/bothSides), len(inner)-avail)
	kept := append([]string{}, inner[start:start+avail]...)
	blank := inner[0] // popup pads its body with a blank line top and bottom
	cue := func(text string) string {
		w := xansi.StringWidth(blank)
		pad := popEdges/bothSides + gutter // the border and the padding
		text = xansi.Truncate(text, max(0, w-pad*bothSides), "")
		return xansi.Cut(blank, 0, pad) + p.panelFill(text) + xansi.Cut(blank, pad+xansi.StringWidth(text), w)
	}
	if start > 0 {
		kept[0] = cue(p.moreCue(true, start+1))
	}
	if hidden := len(inner) - start - avail; hidden > 0 && avail > 1 {
		kept[avail-1] = cue(p.moreCue(false, hidden+1))
	}
	return append(append([]string{lines[0]}, kept...), lines[len(lines)-1])
}
