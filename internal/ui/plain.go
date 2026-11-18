package ui

import (
	"fmt"
	"regexp"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	appearance "github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// ROLECALL_PLAIN is for screen readers and braille displays, which read the frame a line
// at a time and follow the terminal cursor. So under it the frame is one region (no
// preview beside the list: see plainLayout), decoration that reads as noise goes (rules
// and dividers), a Status glyph standing without its word becomes the word, as do the
// flags and the ready meter (markWords), a pop-up is a screen of its own (plainPopUp),
// the help and Stats read down one column, and the terminal cursor sits on the
// selection. See docs/spec.md, Accessibility, for its limits.

// plainLayout is l as ROLECALL_PLAIN draws it: the narrow class's single pane at any
// width, so no line holds two panes. The detail opens full-screen on enter.
func plainLayout(l layout) layout {
	if !plainMode() {
		return l
	}
	l.class = 0
	l.detail, l.listW, l.detailW = detailFull, l.w, 0
	return l
}

// ruleRun is a run of rule cells: a frame rule, a section header's trailing rule, a
// pop-up border's span.
var ruleRun = regexp.MustCompile("─{2,}")

// boxOnly is a line left with nothing but box-drawing marks and spaces.
var boxOnly = regexp.MustCompile(`^[\s─│┬┴┼├┤╭╮╰╯┌┐└┘]*$`)

// plainFrame is a finished frame (before the ASCII fold) without decoration, each line
// at most w cells: rule runs become spaces, so the columns beside them stay put; a line
// of nothing but box-drawing marks is blank; a Status glyph not followed by its word is
// the word; and the marks in plainMarks and the ready meter are words.
func plainFrame(frame string, w int) string {
	lines := strings.Split(frame, "\n")
	for i, line := range lines {
		line = ruleRun.ReplaceAllStringFunc(line, func(run string) string {
			return strings.Repeat(" ", len([]rune(run)))
		})
		if boxOnly.MatchString(xansi.Strip(line)) {
			lines[i] = ""
			continue
		}
		lines[i] = xansi.Truncate(markWords(statusWords(line)), w, "")
	}
	return strings.Join(lines, "\n")
}

// statusWords replaces each Status glyph in a styled line that does not stand before its
// own word ("● Applied") with that word, so a screen reader says "Applied", not "black
// circle".
func statusWords(line string) string {
	for _, g := range appearance.Legend() {
		if g.Group != appearance.GroupStatus || !strings.Contains(line, g.Mark) {
			continue
		}
		var b strings.Builder
		rest := line
		for {
			i := strings.Index(rest, g.Mark)
			if i < 0 {
				b.WriteString(rest)
				break
			}
			b.WriteString(rest[:i])
			rest = rest[i+len(g.Mark):]
			// The Closed group's header shares Rejected's glyph: "✕ Closed" keeps it.
			if next := strings.TrimLeft(xansi.Strip(rest), " "); strings.HasPrefix(next, g.Meaning) || strings.HasPrefix(next, tableClosed) {
				b.WriteString(g.Mark)
			} else {
				b.WriteString(g.Meaning)
				rest = absorb(rest, xansi.StringWidth(g.Meaning)-1)
			}
		}
		line = b.String()
	}
	return line
}

// plainMarks are the marks ROLECALL_PLAIN says as words wherever they stand, as a screen
// reader would otherwise read "black flag" or "white circle". The ready meter is said
// whole (meterRun); ✓, — and ~ stay, as they read sensibly and mean more than one thing
// in running text.
var plainMarks = func() []plainMark {
	words := [][2]string{
		{glyphFollowUp, "follow-up due"},
		{appearance.GlyphMissing, "missing"},
		{glyphWarn, "warning"},
		{appearance.GlyphUrgent, "closes soon"},
		{appearance.GlyphMarked, "marked"},
		{appearance.GlyphChanged, "changed"},
	}
	out := make([]plainMark, len(words))
	for i, w := range words {
		out[i] = plainMark{regexp.MustCompile(regexp.QuoteMeta(w[0])), w[1]}
	}
	return out
}()

// plainMark is one mark ROLECALL_PLAIN says as a word.
type plainMark struct {
	re   *regexp.Regexp
	word string
}

// meterRun is a ready meter's cells, any styling between them included.
var meterRun = regexp.MustCompile(`(?:[` + meterOn + meterOff + `](?:\x1b\[[0-9;]*m)*)+`)

// meterCount is the "2/3" the help's legend sets beside its sample meter.
var meterCount = regexp.MustCompile(`^\s*\d+/\d+`)

// sgr is one styling sequence.
var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// markWords says each of plainMarks and the ready meter as words: "⚑ 6w" is "follow-up
// due 6w", "■■□" is "2 of 3 ready". On a line that says the word already (the help's
// legend, "⚑ follow-up due"; a Docs row's "○ JD  missing") the mark just goes.
func markWords(line string) string {
	line = replaceMark(line, meterRun, func(run, rest string) string {
		if meterCount.MatchString(xansi.Strip(rest)) {
			return ""
		}
		on := strings.Count(run, meterOn)
		return fmt.Sprintf("%d of %d ready", on, on+strings.Count(run, meterOff))
	})
	for _, pm := range plainMarks {
		word := pm.word
		if strings.Contains(xansi.Strip(line), word) {
			word = "" // the line says it already: the legend's "○ missing", a Docs row's state
		}
		line = replaceMark(line, pm.re, func(string, string) string { return word })
	}
	return line
}

// replaceMark puts word(match, rest of line) in place of each match of re in a styled
// line, keeping the match's styling sequences so no colour is left open or lost, and
// absorbing the extra width into a gap after it where the row has room.
func replaceMark(line string, re *regexp.Regexp, word func(match, rest string) string) string {
	done := ""
	for {
		loc := re.FindStringIndex(line)
		if loc == nil {
			return done + line
		}
		match, rest := line[loc[0]:loc[1]], line[loc[1]:]
		before, wd := done+line[:loc[0]], word(match, rest)
		if grow := xansi.StringWidth(wd) - xansi.StringWidth(match); grow > 0 {
			// A flag at a row's right edge has its gap before it, not after.
			if after := absorb(rest, grow); after != rest {
				rest = after
			} else {
				before = absorbLast(before, grow)
			}
		}
		done, line = before+wd+strings.Join(sgr.FindAllString(match, -1), ""), rest
	}
}

// absorbLast is absorb from the last wide-enough gap in s.
func absorbLast(s string, n int) string {
	if i := strings.LastIndex(s, strings.Repeat(" ", n+gutter)); i >= 0 {
		return s[:i] + s[i+n:]
	}
	return s
}

// absorb takes n cells out of the first gap of spaces in s wide enough to keep two, so a
// word put in place of a glyph pushes nothing off the line's end where the row has room.
func absorb(s string, n int) string {
	gap := strings.Repeat(" ", n+gutter)
	if i := strings.Index(s, gap); i >= 0 {
		return s[:i] + s[i+n:]
	}
	return s
}

// plainPopUp is a pop-up as ROLECALL_PLAIN draws it: a screen of its own, as the help
// is, with nothing of the frame behind it, so no line holds both. Its border goes (the
// title and key hints set in it stay), and a pop-up taller than the screen scrolls to
// keep its selection in view, as it does over the frame.
func (m model) plainPopUp(p pal, box string) string {
	h := m.viewHeight()
	pin := 0
	if pm, ok := m.overlay.(pinnedMode); ok {
		pin = pm.pinned()
	}
	lines := clipBox(p, strings.Split(box, "\n"), h, pin)
	for i, line := range lines {
		edge := 1 // the side borders, │ … │
		if i == 0 || i == len(lines)-1 {
			edge = 2 // the corners and the rule's first cell: ╭─ … ╮, ╰─ … ╯
		}
		w := xansi.StringWidth(line)
		line = xansi.Cut(line, edge, max(edge, w-1))
		if edge > 1 {
			line = strings.ReplaceAll(line, "─", " ") // a lone rule cell beside a form's note
		}
		lines[i] = " " + line
	}
	return plainFrame(fitLines(strings.Join(lines, "\n"), h), m.viewWidth())
}

// selectionCell is where the terminal cursor goes under ROLECALL_PLAIN: the first drawn
// cell after the selected item's ▌ gutter mark (a list row's ◉ when it is marked, else its
// Status). With a pop-up open the frame behind draws no selection, so the cursor lands in
// the pop-up. ok is false when nothing is selected (Stats, an empty view).
func selectionCell(frame string) (x, y int, ok bool) {
	for y, line := range strings.Split(xansi.Strip(frame), "\n") {
		if i := strings.Index(line, "▌"); i >= 0 {
			rest := line[i+len("▌"):]
			text := strings.TrimLeft(rest, " ")
			if text == "" {
				return xansi.StringWidth(line[:i]) + gutter, y, true
			}
			return xansi.StringWidth(line[:i]) + 1 + len(rest) - len(text), y, true
		}
	}
	return 0, 0, false
}
