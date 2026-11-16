package theme_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// PaintBackground pads each line to the width on the background and puts the background
// back after every reset, so no cell falls through to the terminal's own colour.
func TestPaintBackgroundCoversEveryCell(t *testing.T) {
	const bg = "\x1b[48;2;255;255;255m"
	got := theme.PaintBackground("a\x1b[1mb\x1b[m c\n\x1b[0;31mx", lipgloss.Color("#FFFFFF"), 6)
	lines := strings.Split(got, "\n")
	if want := bg + "a\x1b[1mb\x1b[m" + bg + " c  \x1b[m"; lines[0] != want {
		t.Errorf("line 0 = %q, want %q", lines[0], want)
	}
	if want := bg + "\x1b[0;31m" + bg + "x     \x1b[m"; lines[1] != want {
		t.Errorf("line 1 = %q, want %q", lines[1], want)
	}
}

// StripBold drops bold and keeps every other attribute, colours whole.
func TestStripBoldKeepsColour(t *testing.T) {
	in := "\x1b[1;38;2;1;2;3mx\x1b[1m\x1b[3;1;48;5;1my\x1b[m"
	want := "\x1b[38;2;1;2;3mx\x1b[3;48;5;1my\x1b[m"
	if got := theme.StripBold(in); got != want {
		t.Errorf("StripBold = %q, want %q", got, want)
	}
}
