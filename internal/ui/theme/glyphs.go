package theme

import (
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The glyph table. Every glyph that carries a meaning is declared once here with its
// one meaning and its ASCII stand-in; the frame draws these constants, the `?` legend is
// generated from Legend, and the ASCII fold is built from the same rows. One glyph has one
// meaning, and no two meanings share a glyph or a stand-in, so ASCII mode and NO_COLOR
// still tell every state apart.
const (
	GlyphReady    = "✓" // a Document is ready (also a success toast and a complete Contact)
	GlyphMissing  = "○" // a Document is missing
	GlyphNone     = "—" // nothing here: no close date, Fit not assessed, a Document not needed
	GlyphMeterOn  = "■" // one ready Document in the ready meter
	GlyphMeterOff = "□" // one missing Document in the ready meter
	GlyphFollowUp = "⚑" // Follow-up due
	GlyphWarn     = "⚠" // needs attention
	GlyphUrgent   = "▲" // closes within two days
	GlyphApprox   = "~" // an approximate date
	GlyphWriting  = "⠹" // an agent is writing a Document (one frame of the spinner)
	GlyphChanged  = "Δ" // a form field changed and not yet saved
	GlyphError    = "✘" // an error message
	GlyphEllipsis = "…" // text cut to fit
	GlyphSep      = "·" // the separator between labels
)

// Glyph is one legend entry: the mark the frame draws, its ASCII stand-in, and its
// meaning in the `?` legend's words. Group is the legend row it sits in.
type Glyph struct {
	Mark, ASCII, Group, Meaning string
}

// Legend groups, in the `?` pop-up's order.
const (
	GroupStatus   = "Statuses"
	GroupTimeline = "Timeline"
	GroupDocs     = "Docs"
	GroupAgent    = "Agent"
	GroupFlags    = "Flags"
	GroupFit      = "Fit"
	GroupForms    = "Forms"
)

// statusOrder is the Statuses in the order the legend lists them: open, then closed.
var statusOrder = []string{store.StatusLead, store.StatusDraft, store.StatusApplied, store.StatusInterviewing,
	store.StatusOffer, store.StatusRejected, store.StatusWithdrawn, store.StatusDismissed}

// Legend is every glyph that means something, in legend order. The Status rows come from
// statusGlyphs, so a Status's glyph is declared once.
func Legend() []Glyph {
	out := make([]Glyph, 0, len(statusOrder)+len(markGlyphs))
	for _, s := range statusOrder {
		g := statusGlyphs[s]
		out = append(out, Glyph{g[0], g[1], GroupStatus, s})
	}
	return append(out, markGlyphs...)
}

// markGlyphs are the legend's glyphs other than the Statuses.
var markGlyphs = []Glyph{
	{GlyphApprox, "~", GroupTimeline, "approximate date"},
	{GlyphReady, "+", GroupDocs, "ready"},
	{GlyphMissing, "_", GroupDocs, "missing"},
	{GlyphNone, "-", GroupDocs, "not needed"},
	{GlyphMeterOn, "#", GroupDocs, "ready meter: ready"},
	{GlyphMeterOff, ":", GroupDocs, "ready meter: missing"},
	{GlyphWriting, "%", GroupAgent, "agent writing"},
	{GlyphWarn, "!", GroupFlags, "needs attention"},
	{GlyphUrgent, "^", GroupFlags, "closes within 2 days"},
	{GlyphFollowUp, "?", GroupFlags, "follow-up due"},
	{GlyphNone, "-", GroupFit, "not assessed"},
	{GlyphChanged, "&", GroupForms, "changed, not saved"},
}

// chromeGlyphs are the frame's marks that carry no meaning of their own: borders, rules,
// the selection gutter, arrows, the sparkline. Their stand-ins may repeat each other, but
// never a legend stand-in where that would read as a meaning.
var chromeGlyphs = []string{
	"▌", ">", "▸", ">", "▶", ">", "▏", "|", "‹", "<", "›", ">",
	GlyphSep, ".", GlyphEllipsis, ">", GlyphError, "X", "✗", "X", "×", "x",
	"↑", "^", "↓", "v", "←", "<", "→", ">",
	// the wordmark's mark, distinct from every Status stand-in and from a data block
	"▞", "R",
	// the Stats sparkline's eight steps fold onto five: _ . - = #
	"▁", "_", "▂", "_", "▃", ".", "▄", ".", "▅", "-", "▆", "=", "▇", "=", "█", "#",
	"░", ".", "▕", "|", "“", `"`, "”", `"`, "≤", "<",
	"─", "-", "│", "|", "┬", "+", "┴", "+", "╭", "+", "╮", "+", "╰", "+", "╯", "+",
	// the box rules Glamour draws around a Markdown table
	"┼", "+", "├", "+", "┤", "+", "┌", "+", "┐", "+", "└", "+", "┘", "+",
	// Glamour pads table cells with a non-breaking space
	" ", " ",
}

// spinnerFrames are the progress spinner's Braille frames; each folds to the writing
// glyph's stand-in, so ASCII draws one steady mark.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// asciiGlyphs maps every glyph the layout draws to its one-cell ASCII stand-in. The fold
// runs over the finished frame, so a stand-in is always one cell wide.
var asciiGlyphs = func() *strings.Replacer {
	var pairs []string
	seen := map[string]bool{}
	add := func(mark, ascii string) {
		if !seen[mark] {
			seen[mark] = true
			pairs = append(pairs, mark, ascii)
		}
	}
	for _, g := range Legend() {
		add(g.Mark, g.ASCII)
	}
	for _, f := range spinnerFrames {
		add(f, "%")
	}
	for i := 0; i < len(chromeGlyphs); i += 2 {
		add(chromeGlyphs[i], chromeGlyphs[i+1])
	}
	return strings.NewReplacer(pairs...)
}()

// IsChromeGlyph reports a mark the frame draws without a meaning of its own, so the
// legend need not list it.
func IsChromeGlyph(mark string) bool {
	for i := 0; i < len(chromeGlyphs); i += 2 {
		if chromeGlyphs[i] == mark {
			return true
		}
	}
	for _, f := range spinnerFrames {
		if f == mark {
			return true
		}
	}
	return false
}
