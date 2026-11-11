package ui

import (
	"image/color"
	"strings"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// Every colour and style lives here (CODING_STANDARDS). v0.1.1: dark only, Mint accent.
// Colour always pairs with a word or glyph, so meaning never rests on colour alone.
var (
	colorBg      = lipgloss.Color("#0D1117") // painted as the terminal background
	colorFg      = lipgloss.Color("#E6EDF3")
	colorDim     = lipgloss.Color("#8B949E") // floor for any text (≥ 4.5:1 on colorBg)
	colorSub     = lipgloss.Color("#C9D1D9") // secondary text on the selection bar
	colorFaint   = lipgloss.Color("#30363D") // rules and borders only, never text
	colorPanel   = lipgloss.Color("#161B22") // pop-up fill and the feedback line
	colorAccent  = lipgloss.Color("#5EEAD4") // Mint
	colorOnFill  = lipgloss.Color("#0D1117") // text on an accent fill
	colorSelect  = lipgloss.Color("#1F3D3D") // selection bar: Mint at 20% over colorBg
	colorOK      = lipgloss.Color("#3FB950") // Strong fit, built docs
	colorInfo    = lipgloss.Color("#58A6FF") // Good fit
	colorWarn    = lipgloss.Color("#D29922") // closes within a week, quiet follow-up, written docs
	colorBad     = lipgloss.Color("#F85149") // closes today, rejected, errors
	colorLilac   = lipgloss.Color("#B4A7FF") // Interviewing
	colorDimmed  = lipgloss.Color("#3D444D") // everything behind a pop-up
	colorDimmest = lipgloss.Color("#30363D")
)

// pal is the set of colours one frame is drawn with; behind a pop-up the whole frame
// is drawn with dimmedPal.
type pal struct {
	fg, dim, sub, faint, accent, onFill, sel, ok, info, warn, bad, lilac color.Color
}

var basePal = pal{fg: colorFg, dim: colorDim, sub: colorSub, faint: colorFaint, accent: colorAccent, onFill: colorOnFill,
	sel: colorSelect, ok: colorOK, info: colorInfo, warn: colorWarn, bad: colorBad, lilac: colorLilac}

var dimmedPal = pal{fg: colorDimmed, dim: colorDimmest, sub: colorDimmed, faint: colorFaint, accent: colorDimmed, onFill: colorDim,
	sel: colorPanel, ok: colorDimmed, info: colorDimmed, warn: colorDimmed, bad: colorDimmed, lilac: colorDimmed}

func (p pal) text(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// heading is a dim, bold, upper-case section label.
func (p pal) heading(s string) string { return p.text(p.dim).Bold(true).Render(strings.ToUpper(s)) }

// fitColor keeps amber and red for time: Fit runs green → blue → grey.
func (p pal) fitColor(fit string) color.Color {
	switch fit {
	case "Strong":
		return p.ok
	case "Good":
		return p.info
	}
	return p.dim
}

// statusColor is the colour of a Status word.
func (p pal) statusColor(status string) color.Color {
	switch status {
	case "Interviewing":
		return p.lilac
	case "Offer":
		return p.ok
	case "Rejected":
		return p.bad
	case "Lead", "Dismissed", "Withdrawn":
		return p.dim
	}
	return p.fg
}

func (p pal) statusChip(status string) string {
	return p.text(p.statusColor(status)).Bold(true).Render("● " + status)
}

// keys renders a key list: accent key, dim description, " · " between pairs. The key
// and description are one run of text apart from their colour, so "e edit" reads whole.
func (p pal) keys(pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, kv := range pairs {
		parts[i] = p.text(p.accent).Bold(true).Render(kv[0]) + p.text(p.dim).Render(" "+kv[1])
	}
	return strings.Join(parts, p.text(p.faint).Render(" · "))
}

// spread places left and right on one line of width w.
func spread(left, right string, w int) string {
	return left + strings.Repeat(" ", max(gutter, w-xansi.StringWidth(left)-xansi.StringWidth(right))) + right
}

// Pop-up widths: small (menus, prompts), form, large (help).
const (
	popSmall = 52
	popForm  = 62
	popLarge = 88
	popEdges = 2 // the border's left and right cells
	popInset = 6 // border plus two cells of padding, each side
	gutter   = 2 // the usual left indent and gap
	ellipsis = 4 // room kept for "…" and a space when text is cut
)

// panelSeq is colorPanel as a raw background sequence, re-applied after inner resets
// so the pop-up fill survives styled text inside it.
const panelSeq = "\x1b[48;2;22;27;34m"

func panelFill(s string) string {
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+panelSeq)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+panelSeq)
	return panelSeq + s + "\x1b[m"
}

// popup is the one pop-up template: rounded border, the title set into the top border,
// key hints set into the bottom border, two cells of padding and a solid fill. Warnings
// get an amber border and a ⚠ in the title. w is capped to the terminal width.
func popup(p pal, title string, warn bool, w, termWidth int, body, hints string) string {
	if termWidth > 0 {
		w = min(w, termWidth-popEdges)
	}
	border := p.accent
	if warn {
		border, title = p.warn, "⚠ "+title
	}
	b := p.text(border)
	inner := w - popEdges
	t := p.text(border).Bold(true).Background(colorPanel).Render(" " + title + " ")
	top := b.Render("╭─") + t + b.Render(strings.Repeat("─", max(0, inner-1-xansi.StringWidth(t)))+"╮")
	h := xansi.Truncate(" "+hints+" ", inner-popEdges-1, "…")
	bottom := b.Render("╰─") + panelFill(h) + b.Render(strings.Repeat("─", max(0, inner-1-xansi.StringWidth(h)))+"╯")
	lines := []string{top}
	for _, l := range append(append([]string{""}, strings.Split(body, "\n")...), "") {
		content := xansi.Truncate("  "+l, inner, "…")
		lines = append(lines, b.Render("│")+panelFill(content+strings.Repeat(" ", max(0, inner-xansi.StringWidth(content))))+b.Render("│"))
	}
	return strings.Join(append(lines, bottom), "\n")
}

// overlay draws box centred over base.
func overlay(base, box string, width int) string {
	const half = 2
	x := max(0, (width-lipgloss.Width(box))/half)
	y := max(0, (lipgloss.Height(base)-lipgloss.Height(box))/half)
	return lipgloss.NewCompositor(lipgloss.NewLayer(base), lipgloss.NewLayer(box).X(x).Y(y).Z(1)).Render()
}

// markdownStyle is Glamour's dark style with the accent on headings, no heading markers
// and no document margin.
func markdownStyle() ansi.StyleConfig {
	c := styles.DarkStyleConfig
	accent := "#5EEAD4"
	zero := uint(0)
	c.Document.Margin = &zero
	c.Heading.Color = &accent
	c.H1.Color, c.H1.BackgroundColor = &accent, nil
	c.H1.Prefix, c.H1.Suffix = "", ""
	c.H2.Prefix, c.H3.Prefix, c.H4.Prefix = "", "", ""
	return c
}
