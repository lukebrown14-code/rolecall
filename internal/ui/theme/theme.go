// Package theme owns colour roles, palettes, and terminal appearance.
package theme

import (
	"fmt"
	"image/color"
	"os"
	"regexp"
	"strings"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Every colour and style lives here (CODING_STANDARDS). Colours are named by role, and
// every role has a dark and a light value; one frame is drawn with one Mode's Roles.
// Colour always pairs with a word or glyph, so meaning never rests on colour alone.
//
// Floors (contrast_internal_test.go): text Roles 4.5:1 on bg, panel and the selection
// bar; rules, separators and glyphs 3:1. Accent means focus and selection only.
type Roles struct {
	Bg     string // painted as the terminal background
	Panel  string // pop-up fill and the feedback line
	Text   string // body text
	Sub    string // secondary text on the selection bar
	Muted  string // the floor for any text; also closed Statuses
	Faint  string // separators between labels and past timeline dots, never body text
	Rule   string // rules and pane dividers
	Accent string // focus and selection only
	OnFill string // text on an accent fill
	Sel    string // the selection bar: the accent tinted over bg
	Ok     string // Strong fit, built docs, success
	Info   string // links, Good fit
	Warn   string // closes within a week, quiet follow-up, written docs, warning pop-ups
	Danger string // closes today, errors
	Purple string // Interviewing
	Gold   string // Offer
	Lead   string // Lead: cool grey-blue
	Draft  string // Draft: a lighter blue than Applied, apart from the accent
	// dimmed is everything behind a pop-up, still at the text floor; dimmedOnFill is the
	// active tab's text on the dimmed fill.
	Dimmed, DimmedOnFill string
}

// The two backgrounds; text on an accent fill reuses them.
const (
	darkBg  = "#0D1117"
	lightBg = "#FFFFFF"
)

var darkRoles = Roles{
	Bg: darkBg, Panel: "#161B22", Text: "#E6EDF3", Sub: "#C9D1D9", Muted: "#8B949E",
	Faint: "#6E7681", Rule: "#6E7681", Accent: "#5EEAD4", OnFill: darkBg, Sel: "#1F3D3D",
	Ok: "#3FB950", Info: "#58A6FF", Warn: "#F0883E", Danger: "#FF7B72", Purple: "#BC8CFF",
	Gold: "#F2CC60", Lead: "#8FA1B3", Draft: "#A5D6FF", Dimmed: "#768390", DimmedOnFill: darkBg,
}

// lightRoles are the same Roles on white. Warn is orange rather than amber, so it never
// reads as Offer's gold.
var lightRoles = Roles{
	Bg: lightBg, Panel: "#F6F8FA", Text: "#1F2328", Sub: "#424A53", Muted: "#59636E",
	Faint: "#818B98", Rule: "#818B98", Accent: "#0F766E", OnFill: lightBg, Sel: "#EEF6F5",
	Ok: "#1A7F37", Info: "#0969DA", Warn: "#B04400", Danger: "#CF222E", Purple: "#8250DF",
	Gold: "#7D5F00", Lead: "#5B6F86", Draft: "#3F6D99", Dimmed: "#656D76", DimmedOnFill: lightBg,
}

// Mode picks a set of Roles.
type Mode int

const (
	Dark Mode = iota
	Light
)

func (t Mode) Roles() Roles {
	if t == Light {
		return lightRoles
	}
	return darkRoles
}

// themeEnv overrides terminal detection: auto (the default), dark or light.
const themeEnv = "ROLECALL_THEME"

// colorFGBGLight are COLORFGBG background values that mean a light terminal.
var colorFGBGLight = map[string]bool{"7": true, "15": true}

// FromEnv is the Mode the first frame is drawn with, and whether to ask the
// terminal for its background colour (auto). Until the terminal answers, COLORFGBG
// ("fg;bg", set by rxvt, Konsole and others) is the best guess, else dark.
func FromEnv() (Mode, bool) {
	switch strings.ToLower(os.Getenv(themeEnv)) {
	case "dark":
		return Dark, false
	case "light":
		return Light, false
	}
	if v := os.Getenv("COLORFGBG"); v != "" {
		parts := strings.Split(v, ";")
		if colorFGBGLight[parts[len(parts)-1]] {
			return Light, true
		}
	}
	return Dark, true
}

// IsDark reports a colour whose relative luminance is under the midpoint.
func IsDark(c color.Color) bool {
	const (
		shift = 8 // RGBA is 16-bit per channel
		mid   = 128
	)
	r, g, b, _ := c.RGBA()
	// Rec. 601 luma, close enough to tell a dark background from a light one.
	return (299*(r>>shift)+587*(g>>shift)+114*(b>>shift))/1000 < mid
}

// ForBackground is the Mode for a terminal whose background is dark or not.
func ForBackground(dark bool) Mode {
	if dark {
		return Dark
	}
	return Light
}

// Palette is the set of colours one frame is drawn with; behind a pop-up the whole frame is
// drawn with the Mode's dimmed palette. Field names are the Roles: dim is muted, bad is
// danger. ascii draws Status glyphs as their own ASCII stand-ins, which keep every Status
// apart where the frame-wide fold would merge them.
type Palette struct {
	Fg, Dim, Sub, Faint, Rule, Accent, OnFill, Sel, Ok, Info, Warn, Bad color.Color
	Bg, Panel, Purple, Gold, Lead, Draft                                color.Color
	Ascii                                                               bool
}

func (t Mode) Base() Palette {
	r, c := t.Roles(), lipgloss.Color
	return Palette{Fg: c(r.Text), Dim: c(r.Muted), Sub: c(r.Sub), Faint: c(r.Faint), Rule: c(r.Rule),
		Accent: c(r.Accent), OnFill: c(r.OnFill), Sel: c(r.Sel), Ok: c(r.Ok), Info: c(r.Info), Warn: c(r.Warn),
		Bad: c(r.Danger), Bg: c(r.Bg), Panel: c(r.Panel), Purple: c(r.Purple), Gold: c(r.Gold), Lead: c(r.Lead),
		Draft: c(r.Draft)}
}

// dimmed draws everything behind a pop-up in one readable grey; rules keep their colour.
func (t Mode) Dimmed() Palette {
	r, c := t.Roles(), lipgloss.Color
	d := c(r.Dimmed)
	return Palette{Fg: d, Dim: d, Sub: d, Faint: c(r.Faint), Rule: c(r.Rule), Accent: d, OnFill: c(r.DimmedOnFill),
		Sel: c(r.Panel), Ok: d, Info: d, Warn: d, Bad: d, Bg: c(r.Bg), Panel: c(r.Panel), Purple: d, Gold: d,
		Lead: d, Draft: d}
}

// MarkdownStyle is Glamour's style drawn with the Mode's Roles rather than Glamour's own
// 256-colour palette: body text, bold headings without markers, links and inline code in
// the info colour with no fill or padding, rules in the rule colour, and no margin.
func MarkdownStyle(t Mode) ansi.StyleConfig {
	c := styles.DarkStyleConfig
	if t == Light {
		c = styles.LightStyleConfig
	}
	r := t.Roles()
	text, muted, info, rule := r.Text, r.Muted, r.Info, r.Rule
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

// NoColor reports whether NO_COLOR is set (https://no-color.org): colour is dropped but
// bold and underline stay, so every state still reads.
func NoColor() bool { return os.Getenv("NO_COLOR") != "" }

// ASCIIMode reports whether glyphs should fall back to ASCII stand-ins, for terminals and
// fonts where ◆ ▌ ⚑ and friends render badly. ROLECALL_ASCII=1 forces it on; otherwise it
// turns on by itself when the locale cannot draw UTF-8, or on the Linux console
// (TERM=linux), whose font lacks most of them.
func ASCIIMode() bool {
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

// StripColor drops colour from a rendered frame, keeping bold and underline. It runs over
// the whole view, so Glamour output and the pop-up fill lose colour too.
func StripColor(s string) string {
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
	"●", "*", "○", "0", "✓", "v", "⚠", "!", "✕", "x", "✗", "X", "✘", "X", "◌", "o", "█", "#",
	"↑", "^", "↓", "v", "←", "<", "→", ">", "×", "x", "—", "-", "·", ".",
	// Status glyphs; a palette in ASCII mode draws each Status's own stand-in first
	"◐", "=", "★", "$", "⊘", "-",
	// the wordmark's mark, distinct from every Status stand-in
	"▞", "#",
	// the documents' ready meter and an urgent close date
	"▰", "#", "▱", "-", "▲", "^",
	"─", "-", "│", "|", "┬", "+", "┴", "+", "╭", "+", "╮", "+", "╰", "+", "╯", "+", "…", "~",
	// the box rules Glamour draws around a Markdown table
	"┼", "+", "├", "+", "┤", "+", "┌", "+", "┐", "+", "└", "+", "┘", "+",
	// Glamour pads table cells with a non-breaking space
	"\u00a0", " ",
	// the progress spinner's Braille frames
	"⠋", "*", "⠙", "*", "⠹", "*", "⠸", "*", "⠼", "*", "⠴", "*",
	"⠦", "*", "⠧", "*", "⠇", "*", "⠏", "*",
)

// ASCIIFold replaces glyphs with ASCII stand-ins when ROLECALL_ASCII is set or force
// (the ascii setting) is on.
func ASCIIFold(s string, force bool) string {
	if !force && !ASCIIMode() {
		return s
	}
	return asciiGlyphs.Replace(s)
}

func (p Palette) Text(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }

// key is a key name in a hint: bold body text. The accent is kept for focus.
func (p Palette) Key() lipgloss.Style { return p.Text(p.Fg).Bold(true) }

// activeTab is the selected tab's fill: the accent and bold, so it is marked by more
// than the fill; behind a pop-up its text stays at the text floor on the dimmed fill.
func (p Palette) ActiveTab() lipgloss.Style {
	return lipgloss.NewStyle().Background(p.Accent).Foreground(p.OnFill).Bold(true)
}

// paneRule colours a pane's rule or divider: the accent while the pane has focus, the
// rule colour otherwise (lazygit style).
func (p Palette) PaneRule(focused bool) lipgloss.Style {
	if focused {
		return p.Text(p.Accent)
	}
	return p.Text(p.Rule)
}

// focusRule is a rule w wide over two panes split after left cells, with the focused
// pane's part in the accent, so focus shows without reading the context row.
func (p Palette) FocusRule(w, left int, focusLeft bool) string {
	left = min(max(0, left), w)
	return p.PaneRule(focusLeft).Render(strings.Repeat("─", left)) + p.PaneRule(!focusLeft).Render(strings.Repeat("─", w-left))
}

// spinnerStyle colours the progress spinner: it shows work, not focus.
func (p Palette) SpinnerStyle() lipgloss.Style { return lipgloss.NewStyle().Foreground(p.Info) }

// heading is a dim, bold, upper-case section label.
func (p Palette) Heading(s string) string { return p.Text(p.Dim).Bold(true).Render(strings.ToUpper(s)) }

// fitColor keeps amber and red for time: Fit runs green → blue → grey.
func (p Palette) FitColor(fit string) color.Color {
	switch fit {
	case store.FitStrong:
		return p.Ok
	case store.FitGood:
		return p.Info
	}
	return p.Dim
}

// statusColor is the colour of a Status word and glyph.
func (p Palette) StatusColor(status string) color.Color {
	switch status {
	case store.StatusLead:
		return p.Lead
	case store.StatusDraft:
		return p.Draft
	case store.StatusApplied:
		return p.Info
	case store.StatusInterviewing:
		return p.Purple
	case store.StatusOffer:
		return p.Gold
	}
	return p.Dim
}

// statusGlyph is a Status's glyph, or its ASCII stand-in; "" for no Status.
func (p Palette) StatusGlyph(status string) string {
	g, ok := statusGlyphs[status]
	if !ok {
		return ""
	}
	if p.Ascii {
		return g[1]
	}
	return g[0]
}

// statusMark is a Status's glyph in its colour, one cell wide.
func (p Palette) StatusMark(status string) string {
	return p.Text(p.StatusColor(status)).Render(p.StatusGlyph(status))
}

// statusChip is a Status's glyph and word in its colour: bold while open, plain once
// closed.
func (p Palette) StatusChip(status string) string {
	return p.Text(p.StatusColor(status)).Bold(!ClosedStatus(status)).Render(p.StatusGlyph(status) + " " + status)
}

// keys renders a key list: bold key, dim description, " · " between pairs. The key and
// description are one run of text apart from their colour, so "e edit" reads whole.
func (p Palette) Keys(pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, kv := range pairs {
		parts[i] = p.Key().Render(kv[0]) + p.Text(p.Dim).Render(" "+kv[1])
	}
	return strings.Join(parts, p.Text(p.Faint).Render(" · "))
}

// panelFill paints s on the palette's pop-up fill, re-applying it after inner resets so
// the fill survives styled text inside it.
func (p Palette) PanelFill(s string) string {
	panelSeq := bgSeq(p.Panel)
	s = strings.ReplaceAll(s, "\x1b[0m", "\x1b[0m"+panelSeq)
	s = strings.ReplaceAll(s, "\x1b[m", "\x1b[m"+panelSeq)
	return panelSeq + s + "\x1b[m"
}

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

// ClosedStatus reports a Status that ends an Application: drawn muted, not bold.
func ClosedStatus(status string) bool {
	return status == store.StatusRejected || status == store.StatusWithdrawn || status == store.StatusDismissed
}

// bgSeq is the 24-bit background escape sequence for c.
func bgSeq(c color.Color) string {
	const shift = 8 // RGBA is 16-bit per channel
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>shift, g>>shift, b>>shift)
}
