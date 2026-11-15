package theme_test

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// The inherit palette never draws text the Owner must read in ANSI 8 (the background on
// Solarized Dark), 7 (near-white, invisible on light terminals) or bright yellow: muted
// text, separators and Lead take the terminal's default foreground, and only rules and
// the selection fill (drawn as reverse video) use ANSI 8.
func TestInheritPaletteKeepsTextLegible(t *testing.T) {
	const white, brightBlack, brightYellow = 7, 8, 11
	for name, p := range map[string]theme.Palette{"base": theme.InheritBase(), "dimmed": theme.InheritDimmed()} {
		for role, c := range map[string]color.Color{"text": p.Fg, "sub": p.Sub, "muted": p.Dim, "faint": p.Faint,
			"accent": p.Accent, "ok": p.Ok, "info": p.Info, "warn": p.Warn, "danger": p.Bad, "purple": p.Purple,
			"gold": p.Gold, "lead": p.Lead, "draft": p.Draft} {
			if a, ok := c.(lipgloss.ANSIColor); ok && (a == white || a == brightBlack || a == brightYellow) {
				t.Errorf("%s inherit palette draws %s in ANSI %d", name, role, a)
			}
		}
		for role, c := range map[string]color.Color{"muted": p.Dim, "faint": p.Faint, "lead": p.Lead} {
			if _, ok := c.(lipgloss.NoColor); !ok {
				t.Errorf("%s inherit palette draws %s in %v, not the default foreground", name, role, c)
			}
		}
	}
}

// A truecolor foreground becomes the nearest ANSI-16 foreground and any background becomes
// reverse video, so nothing is painted and bold survives.
func TestInheritANSIReducesToANSI16(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"\x1b[1;38;2;255;80;80m", "\x1b[1;91m"},
		{"\x1b[38;5;34m", "\x1b[32m"},
		{"\x1b[48;2;47;85;96m", "\x1b[7m"},
		{"\x1b[38;2;10;10;10;48;2;1;2;3m", "\x1b[30;7m"},
		{"\x1b[44m", "\x1b[7m"},
		{"\x1b[0m", "\x1b[0m"},
		{"\x1b[m", "\x1b[m"},
	} {
		if got := theme.InheritANSI(c.in); got != c.want {
			t.Errorf("InheritANSI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
