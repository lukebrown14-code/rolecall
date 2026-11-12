package ui

import (
	"fmt"
	"image/color"
	"os"
	"regexp"
	"strings"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// Every colour and style lives here (CODING_STANDARDS). v0.1.1: dark only, Mint accent.
// Colour always pairs with a word or glyph, so meaning never rests on colour alone.
// accentHex is Mint, shared by Lip Gloss and Glamour.
const accentHex = "#5EEAD4"

var (
	colorBg     = lipgloss.Color("#0D1117") // painted as the terminal background
	colorFg     = lipgloss.Color("#E6EDF3")
	colorDim    = lipgloss.Color("#8B949E") // floor for any text (≥ 4.5:1 on colorBg)
	colorSub    = lipgloss.Color("#C9D1D9") // secondary text on the selection bar
	colorFaint  = lipgloss.Color("#30363D") // faint separators between labels, never text
	colorRule   = lipgloss.Color("#484F58") // rules and pane dividers, one step up from faint
	colorPanel  = lipgloss.Color("#161B22") // pop-up fill and the feedback line
	colorAccent = lipgloss.Color(accentHex)
	colorOnFill = lipgloss.Color("#0D1117") // text on an accent fill
	colorSelect = lipgloss.Color("#1F3D3D") // selection bar: Mint at 20% over colorBg
	colorOK     = lipgloss.Color("#3FB950") // Strong fit, built docs
	colorInfo   = lipgloss.Color("#58A6FF") // Good fit
	colorWarn   = lipgloss.Color("#D29922") // closes within a week, quiet follow-up, written docs
	colorBad    = lipgloss.Color("#F85149") // closes today, rejected, errors
	colorLilac  = lipgloss.Color("#B4A7FF") // Interviewing
	colorDimmed = lipgloss.Color("#3D444D") // everything behind a pop-up; dim text there drops to colorFaint
)

// pal is the set of colours one frame is drawn with; behind a pop-up the whole frame
// is drawn with dimmedPal.
type pal struct {
	fg, dim, sub, faint, rule, accent, onFill, sel, ok, info, warn, bad, lilac color.Color
}

var basePal = pal{fg: colorFg, dim: colorDim, sub: colorSub, faint: colorFaint, rule: colorRule, accent: colorAccent, onFill: colorOnFill,
	sel: colorSelect, ok: colorOK, info: colorInfo, warn: colorWarn, bad: colorBad, lilac: colorLilac}

var dimmedPal = pal{fg: colorDimmed, dim: colorFaint, sub: colorDimmed, faint: colorFaint, rule: colorFaint, accent: colorDimmed, onFill: colorDim,
	sel: colorPanel, ok: colorDimmed, info: colorDimmed, warn: colorDimmed, bad: colorDimmed, lilac: colorDimmed}

func (p pal) text(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// spinnerStyle colours the progress spinner in the feedback line.
func (p pal) spinnerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(p.accent) }

// heading is a dim, bold, upper-case section label.
func (p pal) heading(s string) string { return p.text(p.dim).Bold(true).Render(strings.ToUpper(s)) }

// fitColor keeps amber and red for time: Fit runs green → blue → grey.
func (p pal) fitColor(fit string) color.Color {
	switch fit {
	case store.FitStrong:
		return p.ok
	case store.FitGood:
		return p.info
	}
	return p.dim
}

// statusColor is the colour of a Status word.
func (p pal) statusColor(status string) color.Color {
	switch status {
	case store.StatusInterviewing:
		return p.lilac
	case store.StatusOffer:
		return p.ok
	case store.StatusRejected:
		return p.bad
	case store.StatusLead, store.StatusDismissed, store.StatusWithdrawn:
		return p.dim
	}
	return p.fg
}

// fitNone is how a missing Fit reads in prose; list columns show "—".
const fitNone = "not assessed"

// fitText is "   fit Strong", the Fit beside a status chip.
func (p pal) fitText(a store.Application) string {
	fit := a.Fit
	if fit == "" {
		fit = fitNone
	}
	return p.text(p.dim).Render("   fit ") + p.text(p.fitColor(a.Fit)).Render(fit)
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
	popSmall  = 52
	popForm   = 62
	popLarge  = 88
	popEdges  = 2 // the border's left and right cells
	popInset  = 6 // border plus two cells of padding, each side
	gutter    = 2 // the usual left indent and gap
	ellipsis  = 4 // room kept for "…" and a space when text is cut
	bothSides = 2
	popClear  = 2 // blank cells kept either side of a pop-up, so nothing behind it touches the border
)

// popWidth is the width a pop-up of preferred width w gets on a terminal termWidth wide.
func popWidth(w, termWidth int) int { return min(w, termWidth-popClear*bothSides) }

// panelSeq is colorPanel as a background sequence, re-applied after inner resets so
// the pop-up fill survives styled text inside it.
var panelSeq = bgSeq(colorPanel)

// bgSeq is the 24-bit background escape sequence for c.
func bgSeq(c color.Color) string {
	const shift = 8 // RGBA is 16-bit per channel
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>shift, g>>shift, b>>shift)
}

func panelFill(s string) string {
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+panelSeq)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+panelSeq)
	return panelSeq + s + "\x1b[m"
}

// popup is the one pop-up template: rounded border, the title set into the top border,
// key hints set into the bottom border, two cells of padding and a solid fill. Warnings
// get an amber border and a ⚠ in the title. w is capped to the terminal width, less
// the clear margin overlay adds.
func popup(p pal, title string, warn bool, w, termWidth int, body, hints string) string {
	if termWidth > 0 {
		w = min(w, popWidth(w, termWidth))
	}
	border := p.accent
	if warn {
		border, title = p.warn, "⚠ "+title
	}
	b := p.text(border)
	inner := w - popEdges
	t := p.text(border).Bold(true).Background(colorPanel).Render(" " + xansi.Truncate(title, inner-ellipsis-gutter, "…") + " ")
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

// overlay draws box centred over base, with popClear blank cells either side so the
// dimmed text behind never butts against the border. A box taller than the screen
// loses middle lines and keeps its bottom border, where the key hints are.
func overlay(base, box string, width int) string { return overlayIn(base, box, 0, width) }

// overlayIn is overlay centred in the columns from x0 to x0+width, for a screen whose
// left column the box must not cover.
func overlayIn(base, box string, x0, width int) string {
	const half = 2
	h := lipgloss.Height(base)
	lines := strings.Split(box, "\n")
	if len(lines) > h && h > 1 {
		lines = append(lines[:h-1], lines[len(lines)-1])
	}
	pad := strings.Repeat(" ", popClear)
	for i, l := range lines {
		lines[i] = pad + l + pad
	}
	box = strings.Join(lines, "\n")
	x := x0 + max(0, (width-lipgloss.Width(box))/half)
	y := max(0, (h-len(lines))/half)
	return lipgloss.NewCompositor(lipgloss.NewLayer(base), lipgloss.NewLayer(box).X(x).Y(y).Z(1)).Render()
}

// markdownStyle is Glamour's dark style with the accent on headings, no heading markers
// and no document margin.
func markdownStyle() ansi.StyleConfig {
	c := styles.DarkStyleConfig
	accent := accentHex
	zero := uint(0)
	c.Document.Margin = &zero
	c.Heading.Color = &accent
	c.H1.Color, c.H1.BackgroundColor = &accent, nil
	c.H1.Prefix, c.H1.Suffix = "", ""
	c.H2.Prefix, c.H3.Prefix, c.H4.Prefix = "", "", ""
	return c
}

// noColor reports whether NO_COLOR is set (https://no-color.org): colour is dropped but
// bold and underline stay, so every state still reads.
func noColor() bool { return os.Getenv("NO_COLOR") != "" }

// asciiMode reports whether glyphs should fall back to ASCII stand-ins, for terminals and
// fonts where ◆ ▌ ⚑ and friends render badly. Set ROLECALL_ASCII=1.
func asciiMode() bool { return os.Getenv("ROLECALL_ASCII") == "1" }

// sgr matches one SGR escape sequence.
var sgr = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// stripColor drops colour from a rendered frame, keeping bold and underline. It runs over
// the whole view, so Glamour output and the pop-up fill lose colour too.
func stripColor(s string) string {
	return sgr.ReplaceAllStringFunc(s, func(seq string) string {
		body := seq[2 : len(seq)-1]
		if body == "" {
			return seq // "\x1b[m" is a plain reset
		}
		nums := strings.Split(body, ";")
		var keep []string
		for i := 0; i < len(nums); i++ {
			switch nums[i] {
			case "38", "48": // an extended colour: skip its parameters
				i++
				if i < len(nums) && nums[i] == "2" {
					i += 3
				}
			case "0", "1", "4", "22", "24":
				keep = append(keep, nums[i])
			}
		}
		if len(keep) == 0 {
			return ""
		}
		return "\x1b[" + strings.Join(keep, ";") + "m"
	})
}

// asciiGlyphs maps every glyph the layout draws to an ASCII stand-in.
var asciiGlyphs = strings.NewReplacer(
	"◆", "*", "▌", ">", "▸", ">", "▏", "|", "‹", "<", "›", ">", "⚑", "!", "↩", "<",
	"●", "*", "○", "o", "✓", "v", "⚠", "!", "✕", "x", "✗", "x", "◌", "o", "█", "#",
	"↑", "^", "↓", "v", "←", "<", "→", ">", "×", "x", "—", "-", "·", ".",
	"─", "-", "│", "|", "┬", "+", "┴", "+", "╭", "+", "╮", "+", "╰", "+", "╯", "+", "…", "~",
	// the box rules Glamour draws around a Markdown table
	"┼", "+", "├", "+", "┤", "+", "┌", "+", "┐", "+", "└", "+", "┘", "+",
	// Glamour pads table cells with a non-breaking space
	"\u00a0", " ",
	// the progress spinner's Braille frames
	"⠋", "*", "⠙", "*", "⠹", "*", "⠸", "*", "⠼", "*", "⠴", "*",
	"⠦", "*", "⠧", "*", "⠇", "*", "⠏", "*",
)

// asciiFold replaces glyphs with ASCII stand-ins when ROLECALL_ASCII is set or force
// (the ascii setting) is on.
func asciiFold(s string, force bool) string {
	if !force && !asciiMode() {
		return s
	}
	return asciiGlyphs.Replace(s)
}
