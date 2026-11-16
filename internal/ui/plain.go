package ui

import (
	"regexp"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	appearance "github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// ROLECALL_PLAIN is for screen readers and braille displays, which read the frame a line
// at a time and follow the terminal cursor. So under it the frame is one region (no
// preview beside the list: see plainLayout), decoration that reads as noise goes (rules
// and dividers), a Status glyph standing without its word becomes the word, and the
// terminal cursor sits on the selection. See docs/spec.md, Accessibility, for its limits.

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
// of nothing but box-drawing marks is blank, unless it is a pop-up's (box), whose blank
// rows keep its fill; and a Status glyph not followed by its word is the word.
func plainFrame(frame string, w int, box bool) string {
	lines := strings.Split(frame, "\n")
	for i, line := range lines {
		line = ruleRun.ReplaceAllStringFunc(line, func(run string) string {
			return strings.Repeat(" ", len([]rune(run)))
		})
		if !box && boxOnly.MatchString(xansi.Strip(line)) {
			lines[i] = ""
			continue
		}
		lines[i] = xansi.Truncate(statusWords(line), w, "")
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

// absorb takes n cells out of the first gap of spaces in s wide enough to keep two, so a
// word put in place of a glyph pushes nothing off the line's end where the row has room.
func absorb(s string, n int) string {
	gap := strings.Repeat(" ", n+gutter)
	if i := strings.Index(s, gap); i >= 0 {
		return s[:i] + s[i+n:]
	}
	return s
}

// selectionCell is where the terminal cursor goes under ROLECALL_PLAIN: the first cell of
// the selected item's text, after its ▌ gutter mark. With a pop-up open the frame
// behind draws no selection, so the cursor lands in the pop-up. ok is false when nothing
// is selected (Stats, an empty view).
func selectionCell(frame string) (x, y int, ok bool) {
	for y, line := range strings.Split(xansi.Strip(frame), "\n") {
		if i := strings.Index(line, "▌"); i >= 0 {
			return xansi.StringWidth(line[:i]) + gutter, y, true
		}
	}
	return 0, 0, false
}
