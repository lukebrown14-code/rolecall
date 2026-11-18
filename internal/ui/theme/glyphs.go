package theme

import (
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The glyph table. Every glyph that carries a meaning is declared once here with its
// one meaning and its ASCII stand-in; the frame draws these constants, the `?` legend is
// generated from Legend, and the ASCII fold is built from the same rows. One glyph has one
// meaning, and no two meanings share a glyph or a stand-in (SharedMeaning names the
// glyphs that repeat another's meaning), so ASCII mode and NO_COLOR still tell every
// state apart.
//
// A stand-in is never a key: not a letter or a digit, and not the punctuation keys
// (: , * / ? [ ] and the arrows' < > ^ v), so in the Move pop-up and the `?` help, which
// list keys and glyphs together, a stand-in never reads as one. So the Statuses take
// punctuation, not their initials ("l  L Lead" would read as two keys, and Draft and
// Dismissed share a D). Nor is a stand-in any chrome's (the separator, the ellipsis,
// rules and corners, the sparkline and bars), so punctuation never reads as a meaning; the
// one exception is the not-needed dash, which is a dash in ASCII as everywhere.
//
// Each stand-in follows its glyph's shape or idea where ASCII has one: Lead ( (half a
// ring), Draft = (lines of writing), Applied # (the solid one), Interviewing & (you and
// them), Offer $ (pay), Rejected \ (a stroke of the ✕; x is the dismiss key), Withdrawn `
// (a hook turned back), Dismissed % (a struck circle); ready +, missing _ (an empty slot),
// not needed -; needs attention !, which closes-within-2-days shares (it is that, with a
// date: "closes today !"); follow-up due @ (contact them); marked {; changed ' (a prime,
// as in x'); agent writing ;.
const (
	GlyphReady    = "✓" // a Document is ready (also a success toast and a complete Contact)
	GlyphMissing  = "○" // a Document is missing
	GlyphNone     = "—" // nothing here: a Document not needed
	GlyphMeterOn  = "■" // one ready Document in the ready meter
	GlyphMeterOff = "□" // one missing Document in the ready meter
	GlyphFollowUp = "⚑" // Follow-up due
	GlyphWarn     = "⚠" // needs attention
	GlyphUrgent   = "▲" // closes within two days
	GlyphMarked   = "◉" // a row marked for a bulk action
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

// markGlyphs are the legend's glyphs other than the Statuses. The ready meter's cells
// mean what ready and missing mean, one Document each, so they share those stand-ins
// (SharedMeaning): ASCII draws a meter as "++_". A close within two days is a thing that
// needs attention, and always sits beside its words, so it shares "!".
var markGlyphs = []Glyph{
	{GlyphApprox, "~", GroupTimeline, "approximate date"},
	{GlyphReady, "+", GroupDocs, "ready"},
	{GlyphMissing, "_", GroupDocs, "missing"},
	{GlyphNone, "-", GroupDocs, "not needed"},
	{GlyphMeterOn, "+", GroupDocs, "ready meter: ready"},
	{GlyphMeterOff, "_", GroupDocs, "ready meter: missing"},
	{GlyphWriting, ";", GroupAgent, "agent writing"},
	{GlyphWarn, "!", GroupFlags, "needs attention"},
	{GlyphUrgent, "!", GroupFlags, "closes within 2 days"},
	{GlyphFollowUp, "@", GroupFlags, "follow-up due"},
	{GlyphMarked, "{", GroupFlags, "marked for x or m"},
	{GlyphChanged, "'", GroupForms, "changed, not saved"},
}

// sharedMeaning maps a legend glyph to the glyph whose meaning it repeats, and so whose
// ASCII stand-in it takes.
var sharedMeaning = map[string]string{GlyphMeterOn: GlyphReady, GlyphMeterOff: GlyphMissing, GlyphUrgent: GlyphWarn}

// SharedMeaning is the legend glyph whose meaning mark repeats, and whose stand-in it
// shares, or "" when mark's meaning is its own.
func SharedMeaning(mark string) string { return sharedMeaning[mark] }

// ChromeASCII is every stand-in a chrome glyph folds to.
func ChromeASCII() []string {
	var out []string
	for i := 1; i < len(chromeGlyphs); i += 2 {
		out = append(out, chromeGlyphs[i])
	}
	return out
}

// chromeGlyphs are the frame's marks that carry no meaning of their own: borders, rules,
// the selection gutter, arrows, the sparkline. Their stand-ins may repeat each other, but
// never a legend stand-in, so no punctuation reads as a meaning.
var chromeGlyphs = []string{
	"▌", ">", "▸", ">", "▶", ">", "▏", "|", "‹", "<", "›", ">",
	// the separator folds to "|" and the ellipsis to ".", so the two never look alike and
	// cut text never ends in an arrow
	GlyphSep, "|", GlyphEllipsis, ".", GlyphError, "X", "✗", "X", "×", "x",
	"↑", "^", "↓", "v", "←", "<", "→", ">",
	// the wordmark's mark, distinct from every Status stand-in and from a data block
	"▞", "R",
	// the sparkline's baseline is "."; its seven bars fold onto - and |, apart from it;
	// Stats' bars are | on a . track
	"▁", ".", "▂", "-", "▃", "-", "▄", "-", "▅", "|", "▆", "|", "▇", "|", "█", "|",
	"░", ".", "▕", "|", "“", `"`, "”", `"`, "≤", "<", "–", "-",
	// rules are - and |; corners fold onto -, junctions onto |, rather than "+", which is
	// ready
	"─", "-", "│", "|", "┬", "|", "┴", "|", "╭", "-", "╮", "-", "╰", "-", "╯", "-",
	// the box rules Glamour draws around a Markdown table
	"┼", "|", "├", "|", "┤", "|", "┌", "-", "┐", "-", "└", "-", "┘", "-",
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
	writing := ""
	for _, g := range markGlyphs {
		if g.Mark == GlyphWriting {
			writing = g.ASCII
		}
	}
	for _, f := range spinnerFrames {
		add(f, writing)
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
