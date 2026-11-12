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

// Every colour and style lives here (CODING_STANDARDS). Colours are named by role, and
// every role has a dark and a light value; one frame is drawn with one theme's roles.
// Colour always pairs with a word or glyph, so meaning never rests on colour alone.
//
// Floors (contrast_internal_test.go): text roles 4.5:1 on bg, panel and the selection
// bar; rules, separators and glyphs 3:1. Accent means focus and selection only.
type roles struct {
	bg     string // painted as the terminal background
	panel  string // pop-up fill and the feedback line
	text   string // body text
	sub    string // secondary text on the selection bar
	muted  string // the floor for any text; also closed Statuses
	faint  string // separators between labels and past timeline dots, never body text
	rule   string // rules and pane dividers
	accent string // focus and selection only
	onFill string // text on an accent fill
	sel    string // the selection bar: the accent tinted over bg
	ok     string // Strong fit, built docs, success
	info   string // links, Good fit
	warn   string // closes within a week, quiet follow-up, written docs, warning pop-ups
	danger string // closes today, errors
	purple string // Interviewing
	gold   string // Offer
	lead   string // Lead: cool grey-blue
	draft  string // Draft: a lighter blue than Applied, apart from the accent
	// dimmed is everything behind a pop-up, still at the text floor; dimmedOnFill is the
	// active tab's text on the dimmed fill.
	dimmed, dimmedOnFill string
}

// The two backgrounds; text on an accent fill reuses them.
const (
	darkBg  = "#0D1117"
	lightBg = "#FFFFFF"
)

var darkRoles = roles{
	bg: darkBg, panel: "#161B22", text: "#E6EDF3", sub: "#C9D1D9", muted: "#8B949E",
	faint: "#6E7681", rule: "#6E7681", accent: "#5EEAD4", onFill: darkBg, sel: "#1F3D3D",
	ok: "#3FB950", info: "#58A6FF", warn: "#F0883E", danger: "#FF7B72", purple: "#BC8CFF",
	gold: "#F2CC60", lead: "#8FA1B3", draft: "#A5D6FF", dimmed: "#768390", dimmedOnFill: darkBg,
}

// lightRoles are the same roles on white. Warn is orange rather than amber, so it never
// reads as Offer's gold.
var lightRoles = roles{
	bg: lightBg, panel: "#F6F8FA", text: "#1F2328", sub: "#424A53", muted: "#59636E",
	faint: "#818B98", rule: "#818B98", accent: "#0F766E", onFill: lightBg, sel: "#EEF6F5",
	ok: "#1A7F37", info: "#0969DA", warn: "#B04400", danger: "#CF222E", purple: "#8250DF",
	gold: "#7D5F00", lead: "#5B6F86", draft: "#3F6D99", dimmed: "#656D76", dimmedOnFill: lightBg,
}

// theme picks a set of roles.
type theme int

const (
	themeDark theme = iota
	themeLight
)

func (t theme) roles() roles {
	if t == themeLight {
		return lightRoles
	}
	return darkRoles
}

// themeEnv overrides terminal detection: auto (the default), dark or light.
const themeEnv = "ROLECALL_THEME"

// colorFGBGLight are COLORFGBG background values that mean a light terminal.
var colorFGBGLight = map[string]bool{"7": true, "15": true}

// themeFromEnv is the theme the first frame is drawn with, and whether to ask the
// terminal for its background colour (auto). Until the terminal answers, COLORFGBG
// ("fg;bg", set by rxvt, Konsole and others) is the best guess, else dark.
func themeFromEnv() (theme, bool) {
	switch strings.ToLower(os.Getenv(themeEnv)) {
	case "dark":
		return themeDark, false
	case "light":
		return themeLight, false
	}
	if v := os.Getenv("COLORFGBG"); v != "" {
		parts := strings.Split(v, ";")
		if colorFGBGLight[parts[len(parts)-1]] {
			return themeLight, true
		}
	}
	return themeDark, true
}

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

// isDark reports a colour whose relative luminance is under the midpoint.
func isDark(c color.Color) bool {
	const (
		shift = 8 // RGBA is 16-bit per channel
		mid   = 128
	)
	r, g, b, _ := c.RGBA()
	// Rec. 601 luma, close enough to tell a dark background from a light one.
	return (299*(r>>shift)+587*(g>>shift)+114*(b>>shift))/1000 < mid
}

// themeFor is the theme for a terminal whose background is dark or not.
func themeFor(dark bool) theme {
	if dark {
		return themeDark
	}
	return themeLight
}

// pal is the set of colours one frame is drawn with; behind a pop-up the whole frame is
// drawn with the theme's dimmed palette. Field names are the roles: dim is muted, bad is
// danger. ascii draws Status glyphs as their own ASCII stand-ins, which keep every Status
// apart where the frame-wide fold would merge them.
type pal struct {
	fg, dim, sub, faint, rule, accent, onFill, sel, ok, info, warn, bad color.Color
	bg, panel, purple, gold, lead, draft                                color.Color
	ascii                                                               bool
}

func (t theme) base() pal {
	r, c := t.roles(), lipgloss.Color
	return pal{fg: c(r.text), dim: c(r.muted), sub: c(r.sub), faint: c(r.faint), rule: c(r.rule),
		accent: c(r.accent), onFill: c(r.onFill), sel: c(r.sel), ok: c(r.ok), info: c(r.info), warn: c(r.warn),
		bad: c(r.danger), bg: c(r.bg), panel: c(r.panel), purple: c(r.purple), gold: c(r.gold), lead: c(r.lead),
		draft: c(r.draft)}
}

// dimmed draws everything behind a pop-up in one readable grey; rules keep their colour.
func (t theme) dimmed() pal {
	r, c := t.roles(), lipgloss.Color
	d := c(r.dimmed)
	return pal{fg: d, dim: d, sub: d, faint: c(r.faint), rule: c(r.rule), accent: d, onFill: c(r.dimmedOnFill),
		sel: c(r.panel), ok: d, info: d, warn: d, bad: d, bg: c(r.bg), panel: c(r.panel), purple: d, gold: d,
		lead: d, draft: d}
}

// basePal and dimmedPal are the dark theme's palettes, for drawing outside a model.
var (
	basePal   = themeDark.base()
	dimmedPal = themeDark.dimmed()
)

// pal is the palette for the model's theme and glyph setting.
func (m model) pal() pal {
	p := m.theme.base()
	p.ascii = m.ascii || asciiMode()
	return p
}

// dimmedPal is the palette for the frame behind a pop-up.
func (m model) dimmedPal() pal {
	p := m.theme.dimmed()
	p.ascii = m.ascii || asciiMode()
	return p
}

func (p pal) text(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// key is a key name in a hint: bold body text. The accent is kept for focus.
func (p pal) key() lipgloss.Style { return p.text(p.fg).Bold(true) }

// activeTab is the selected tab's fill: the accent and bold, so it is marked by more
// than the fill; behind a pop-up its text stays at the text floor on the dimmed fill.
func (p pal) activeTab() lipgloss.Style {
	return lipgloss.NewStyle().Background(p.accent).Foreground(p.onFill).Bold(true)
}

// paneRule colours a pane's rule or divider: the accent while the pane has focus, the
// rule colour otherwise (lazygit style).
func (p pal) paneRule(focused bool) lipgloss.Style {
	if focused {
		return p.text(p.accent)
	}
	return p.text(p.rule)
}

// focusRule is a rule w wide over two panes split after left cells, with the focused
// pane's part in the accent, so focus shows without reading the context row.
func (p pal) focusRule(w, left int, focusLeft bool) string {
	left = min(max(0, left), w)
	return p.paneRule(focusLeft).Render(strings.Repeat("─", left)) + p.paneRule(!focusLeft).Render(strings.Repeat("─", w-left))
}

// spinnerStyle colours the progress spinner: it shows work, not focus.
func (p pal) spinnerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(p.info) }

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

// statusMarkWidth is a Status glyph and the space after it, before a row's title.
const statusMarkWidth = 2

// statusGlyphs are each Status's glyph and its ASCII stand-in. The open Statuses run a
// cold-to-gold ramp; the closed ones are muted, never alarming.
var statusGlyphs = map[string][2]string{
	store.StatusLead:         {"◌", "o"},
	store.StatusDraft:        {"◐", "="},
	store.StatusApplied:      {"●", "*"},
	store.StatusInterviewing: {"◆", "@"},
	store.StatusOffer:        {"★", "$"},
	store.StatusRejected:     {"✕", "x"},
	store.StatusWithdrawn:    {"↩", "<"},
	store.StatusDismissed:    {"⊘", "-"},
}

// closedStatus reports a Status that ends an Application: drawn muted, not bold.
func closedStatus(status string) bool {
	return status == store.StatusRejected || status == store.StatusWithdrawn || status == store.StatusDismissed
}

// statusColor is the colour of a Status word and glyph.
func (p pal) statusColor(status string) color.Color {
	switch status {
	case store.StatusLead:
		return p.lead
	case store.StatusDraft:
		return p.draft
	case store.StatusApplied:
		return p.info
	case store.StatusInterviewing:
		return p.purple
	case store.StatusOffer:
		return p.gold
	}
	return p.dim
}

// statusGlyph is a Status's glyph, or its ASCII stand-in; "" for no Status.
func (p pal) statusGlyph(status string) string {
	g, ok := statusGlyphs[status]
	if !ok {
		return ""
	}
	if p.ascii {
		return g[1]
	}
	return g[0]
}

// statusMark is a Status's glyph in its colour, one cell wide.
func (p pal) statusMark(status string) string {
	return p.text(p.statusColor(status)).Render(p.statusGlyph(status))
}

// tableStatus is the Status a list view holds, or "" for a view of several (All, Closed)
// or none.
func tableStatus(table string) string {
	switch table {
	case tableLeads:
		return store.StatusLead
	case tableDismissed:
		return store.StatusDismissed
	case tableDraft:
		return store.StatusDraft
	case tableApplied, tableInterviewing, tableOffer:
		return table
	}
	return ""
}

// groupHeader is a list section divider, "● Applied · 2 ────", label and rule in the
// Status's colour. A section of several Statuses (Closed, Expired) is muted and keeps the
// glyph's column blank so the labels line up.
func (p pal) groupHeader(label, status string, n, w int) string {
	c, glyph := p.dim, " "
	if status != "" {
		c, glyph = p.statusColor(status), p.statusGlyph(status)
	}
	s := p.text(c)
	head := "  " + glyph + " " + label + " · " + fmt.Sprint(n) + " "
	return s.Bold(!closedStatus(status) && status != "").Render(head) + s.Render(strings.Repeat("─", max(0, w-xansi.StringWidth(head))))
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

// statusChip is a Status's glyph and word in its colour: bold while open, plain once
// closed.
func (p pal) statusChip(status string) string {
	return p.text(p.statusColor(status)).Bold(!closedStatus(status)).Render(p.statusGlyph(status) + " " + status)
}

// keys renders a key list: bold key, dim description, " · " between pairs. The key and
// description are one run of text apart from their colour, so "e edit" reads whole.
func (p pal) keys(pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, kv := range pairs {
		parts[i] = p.key().Render(kv[0]) + p.text(p.dim).Render(" "+kv[1])
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

// bgSeq is the 24-bit background escape sequence for c.
func bgSeq(c color.Color) string {
	const shift = 8 // RGBA is 16-bit per channel
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>shift, g>>shift, b>>shift)
}

// panelFill paints s on the palette's pop-up fill, re-applying it after inner resets so
// the fill survives styled text inside it.
func (p pal) panelFill(s string) string {
	panelSeq := bgSeq(p.panel)
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
	t := p.text(border).Bold(true).Background(p.panel).Render(" " + xansi.Truncate(title, inner-ellipsis-gutter, "…") + " ")
	top := b.Render("╭─") + t + b.Render(strings.Repeat("─", max(0, inner-1-xansi.StringWidth(t)))+"╮")
	h := xansi.Truncate(" "+hints+" ", inner-popEdges-1, "…")
	bottom := b.Render("╰─") + p.panelFill(h) + b.Render(strings.Repeat("─", max(0, inner-1-xansi.StringWidth(h)))+"╯")
	lines := []string{top}
	for _, l := range append(append([]string{""}, strings.Split(body, "\n")...), "") {
		content := xansi.Truncate("  "+l, inner, "…")
		lines = append(lines, b.Render("│")+p.panelFill(content+strings.Repeat(" ", max(0, inner-xansi.StringWidth(content))))+b.Render("│"))
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

// markdownStyle is Glamour's style drawn with the theme's roles rather than Glamour's own
// 256-colour palette: body text, bold headings without markers, links and inline code in
// the info colour with no fill or padding, rules in the rule colour, and no margin.
func markdownStyle(t theme) ansi.StyleConfig {
	c := styles.DarkStyleConfig
	if t == themeLight {
		c = styles.LightStyleConfig
	}
	r := t.roles()
	text, muted, info, rule := r.text, r.muted, r.info, r.rule
	zero := uint(0)
	c.Document.Margin = &zero
	c.Document.Color = &text
	c.Heading.Color = &text
	c.H1.Color, c.H1.BackgroundColor = &text, nil
	c.H1.Prefix, c.H1.Suffix = "", ""
	c.H2.Prefix, c.H3.Prefix, c.H4.Prefix = "", "", ""
	c.H6.Color = &muted
	c.HorizontalRule.Color = &rule
	c.Link.Color, c.LinkText.Color = &info, &info
	c.Image.Color, c.ImageText.Color = &info, &muted
	c.Code.Color, c.Code.BackgroundColor = &info, nil
	c.Code.Prefix, c.Code.Suffix = "", ""
	c.CodeBlock.Color, c.CodeBlock.Chroma = &text, nil
	return c
}

// noColor reports whether NO_COLOR is set (https://no-color.org): colour is dropped but
// bold and underline stay, so every state still reads.
func noColor() bool { return os.Getenv("NO_COLOR") != "" }

// asciiMode reports whether glyphs should fall back to ASCII stand-ins, for terminals and
// fonts where ◆ ▌ ⚑ and friends render badly. ROLECALL_ASCII=1 forces it on; otherwise it
// turns on by itself when the locale cannot draw UTF-8, or on the Linux console
// (TERM=linux), whose font lacks most of them.
func asciiMode() bool {
	return os.Getenv("ROLECALL_ASCII") == "1" || os.Getenv("TERM") == "linux" || !utf8Capable()
}

// utf8Capable reports whether the environment's locale can draw UTF-8. POSIX gives LC_ALL
// priority over LC_CTYPE over LANG; the first one set decides. Nothing set is treated as
// capable, so a bare environment keeps the glyphs.
func utf8Capable() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		v := os.Getenv(key)
		if v == "" {
			continue
		}
		return strings.Contains(strings.ReplaceAll(strings.ToLower(v), "-", ""), "utf8")
	}
	return true
}

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
	// Status glyphs; a palette in ASCII mode draws each Status's own stand-in first
	"◐", "=", "★", "$", "⊘", "-",
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
