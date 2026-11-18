package ui

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

// The Profile reader keeps a fact per line: a line under a bold label, a hard break, or
// a fact line inside a list item, each starts its own rendered line, and nothing it
// draws is wider than the wrap width. Prose lines still join into a paragraph.
func TestProfileFactLinesKeepTheirOwnLines(t *testing.T) {
	m := model{md: &mdCache{}}
	md := "### Acme\n\n**Dates:** 2023 – Ongoing\n**Canonical title:** Developer\n\n" +
		"- Lead role\n  **Stack:** Go\n  **Team:** four\n\nhard break  \nnext line\n\nsoft\nwrapped prose\n"
	const width = 60
	out := xansi.Strip(m.renderProfile(md, width))
	lines := strings.Split(out, "\n")
	trimmed := make([]string, len(lines))
	for i, l := range lines {
		if w := xansi.StringWidth(l); w > width {
			t.Errorf("line %d is %d wide, over %d: %q", i, w, width, l)
		}
		trimmed[i] = strings.TrimRight(l, " ")
	}
	for _, want := range []string{"Dates: 2023 – Ongoing", "Canonical title: Developer", "• Lead role", "  Stack: Go", "  Team: four",
		"hard break", "next line", "soft wrapped prose"} {
		found := false
		for _, l := range trimmed {
			if l == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no line reads %q:\n%s", want, strings.Join(trimmed, "\n"))
		}
	}
	if strings.Contains(out, lineBreak) {
		t.Errorf("the break mark leaked into the view:\n%q", out)
	}
}
