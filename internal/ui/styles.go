package ui

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
	appearance "github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

type theme appearance.Mode
type pal struct{ appearance.Palette }

const (
	themeDark  = theme(appearance.Dark)
	themeLight = theme(appearance.Light)
)

func (t theme) Base() pal         { return pal{appearance.Mode(t).Base()} }
func (t theme) Dimmed() pal       { return pal{appearance.Mode(t).Dimmed()} }
func themeFromEnv() (theme, bool) { t, auto := appearance.FromEnv(); return theme(t), auto }
func themeFor(dark bool) theme    { return theme(appearance.ForBackground(dark)) }

var isDark = appearance.IsDark
var noColor = appearance.NoColor
var asciiMode = appearance.ASCIIMode
var plainMode = appearance.PlainMode
var inheritANSI = appearance.InheritANSI
var stripColor = appearance.StripColor
var asciiFold = appearance.ASCIIFold

// DetectTheme picks the theme before the program starts: ROLECALL_THEME when it says
// dark or light, else the background colour the terminal reports when asked on in and
// out (OSC 11), else the COLORFGBG guess. Asking happens before Bubble Tea owns the
// terminal, because the frame then paints its own background over the answer.
func DetectTheme(in, out *os.File) Option {
	return func(m *model) {
		t, auto := themeFromEnv()
		if auto && in != nil && out != nil {
			if bg, err := lipgloss.BackgroundColor(in, out); err == nil && bg != nil {
				t = themeFor(isDark(bg))
			}
		}
		m.theme = t
	}
}

// basePal is the dark theme's palette, for drawing outside a model.
var basePal = themeDark.Base()

// pal is the palette for the model's theme and glyph setting.
func (m model) pal() pal {
	p := m.theme.Base()
	if m.inherit {
		p = pal{appearance.InheritBase()}
	}
	p.Ascii = m.ascii || asciiMode()
	return p
}

// dimmedPal is the palette for the frame behind a pop-up.
func (m model) dimmedPal() pal {
	p := m.theme.Dimmed()
	if m.inherit {
		p = pal{appearance.InheritDimmed()}
	}
	p.Ascii = m.ascii || asciiMode()
	return p
}

// glyph is s with its ASCII stand-in when the frame draws ASCII glyphs, so a match on a
// drawn character — hit-testing a rule or the search chip — uses what the renderer wrote.
func (m model) glyph(s string) string { return asciiFold(s, m.ascii) }

// The glyph table's marks (theme/glyphs.go), each with one meaning.
const (
	glyphReady    = appearance.GlyphReady
	glyphMissing  = appearance.GlyphMissing
	glyphNone     = appearance.GlyphNone
	glyphUrgent   = appearance.GlyphUrgent
	glyphMarked   = appearance.GlyphMarked
	glyphFollowUp = appearance.GlyphFollowUp
	glyphMeterOn  = appearance.GlyphMeterOn
	glyphMeterOff = appearance.GlyphMeterOff
	glyphChanged  = appearance.GlyphChanged
	glyphWarn     = appearance.GlyphWarn
)

// fitLabel leads a Fit wherever it is shown: "fit Good", "fit —".
const fitLabel = "fit "

// wordmark is rolecall's own mark, kept apart from every Status glyph (ASCII "R").
const wordmark = "▞"

// statusMarkWidth is a Status glyph and the space after it, before a row's title.
const statusMarkWidth = 2

// tableStatus is the Status a list view holds, or "" for a view of several (All, Closed)
// or none.
func tableStatus(table string) string {
	switch table {
	case tableLeads:
		return store.StatusLead
	case tableDismissed, tableFiltered:
		return store.StatusDismissed
	case tableDraft:
		return store.StatusDraft
	case tableApplied, tableInterviewing, tableOffer:
		return table
	}
	return ""
}

// groupHeader is a list section divider, "● Applied · 2 ────", label and rule in the
// Status's colour, then its rollups when non-zero ("· 1 follow-up due"). A section of several Statuses (Closed, Expired) is muted and keeps the
// glyph's column blank so the labels line up.
func (p pal) GroupHeader(label, status string, n int, rollup string, w int) string {
	c, glyph := p.Dim, " "
	switch {
	case status != "":
		c, glyph = p.StatusColor(status), p.StatusGlyph(status)
	case label == tableClosed:
		glyph = p.ClosedGroupGlyph()
	}
	s := p.Text(c)
	head := xansi.Truncate("  "+glyph+" "+label+" · "+fmt.Sprint(n)+rollup, max(0, w-1), "…") + " "
	return s.Bold(!closedStatus(status) && status != "").Render(head) + s.Render(strings.Repeat("─", max(0, w-xansi.StringWidth(head))))
}

// A Fit not yet assessed is blank in the table columns and the frame's rows; the card
// says "fit not assessed".

// fitWords is an assessed Fit as the card writes it: "fit Good" (cardFit words an unassessed one).
func (p pal) fitWords(fit string) string {
	value := fit
	if value == "" {
		value = fitUnassessed
	}
	return p.Text(p.Dim).Render(fitLabel) + p.Text(p.FitColor(fit)).Render(value)
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
	// popHintChrome is the bottom border's cells around its hints: "╰─ " … " ─╯" and a
	// spare cell, so the hints never touch the corner.
	popHintChrome = 7
)

// popWidth is the width a pop-up of preferred width w gets on a terminal termWidth wide.
func popWidth(w, termWidth int) int { return min(w, termWidth-popClear*bothSides) }

// popup is the one pop-up template: rounded border, the title set into the top border,
// key hints set into the bottom border, two cells of padding and a solid fill. Warnings
// get an amber border and a ⚠ in the title. w is capped to the terminal width, less
// the clear margin overlay adds.
func popup(p pal, title string, warn bool, w, termWidth int, body, hints string) string {
	return popupNote(p, title, "", warn, w, termWidth, body, hints)
}

// popupNote is popup with a short note at the right of the top border, such as a form's
// "* required". An empty note, or one with no room beside the title, is left out.
func popupNote(p pal, title, note string, warn bool, w, termWidth int, body, hints string) string {
	// Grow to fit the hints in the bottom border, so a key is never cut. Where the terminal
	// is too narrow for them all, the hints before the last (esc, the way out) give way
	// whole, latest first, rather than any being cut.
	if termWidth > 0 {
		sep := p.Text(p.Faint).Render(" · ")
		parts := strings.Split(hints, sep)
		for len(parts) > 2 && xansi.StringWidth(strings.Join(parts, sep))+popHintChrome > popWidth(termWidth, termWidth) {
			parts = append(parts[:len(parts)-2], parts[len(parts)-1])
		}
		hints = strings.Join(parts, sep)
	}
	w = max(w, xansi.StringWidth(hints)+popHintChrome)
	if termWidth > 0 {
		w = min(w, popWidth(w, termWidth))
	}
	border := p.Sub
	if warn {
		border, title = p.Warn, "⚠ "+title
	}
	b := p.Text(border)
	inner := w - popEdges
	t := p.Text(border).Bold(true).Background(p.Panel).Render(" " + xansi.Truncate(title, inner-ellipsis-gutter, "…") + " ")
	n := ""
	if note != "" && xansi.StringWidth(t)+xansi.StringWidth(note)+popHintChrome <= inner {
		n = p.Text(p.Dim).Background(p.Panel).Render(" "+note+" ") + b.Render("─")
	}
	top := b.Render("╭─") + t + b.Render(strings.Repeat("─", max(0, inner-1-xansi.StringWidth(t)-xansi.StringWidth(n)))) + n + b.Render("╮")
	h := xansi.Truncate(" "+hints+" ", inner-popEdges-1, "…")
	bottom := b.Render("╰─") + p.PanelFill(h) + b.Render(strings.Repeat("─", max(0, inner-1-xansi.StringWidth(h)))+"╯")
	lines := []string{top}
	for _, l := range append(append([]string{""}, strings.Split(body, "\n")...), "") {
		// Text too long for the box wraps at spaces, so the end of a sentence (where a
		// file lives, what is being left) is never cut; a selected row is fitted already
		// and keeps its one line.
		parts := []string{l}
		if room := inner - gutter - 1; xansi.StringWidth(l) > room && !strings.Contains(xansi.Strip(l), "▌") {
			parts = strings.Split(xansi.Wrap(l, room, ""), "\n")
		}
		for _, part := range parts {
			content := xansi.Truncate("  "+part, inner, "…")
			lines = append(lines, b.Render("│")+p.PanelFill(content+strings.Repeat(" ", max(0, inner-xansi.StringWidth(content))))+b.Render("│"))
		}
	}
	return strings.Join(append(lines, bottom), "\n")
}

var closedStatus = appearance.ClosedStatus
