package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// NO_COLOR drops colour but keeps bold, underline and every word and glyph, so every
// state still reads. Checked across the list, the detail view and the pop-ups.
func TestNoColorDropsColourAndKeepsMarkers(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"1", "m"}, {"a"}, {"?"}, {"/pl"}} {
		view := runKeys(t, fixtureCopy(t, nil), 120, keys...)
		for _, seq := range []string{"38;2;", "48;2;"} {
			if strings.Contains(view, seq) {
				t.Fatalf("keys %q: NO_COLOR left a colour sequence %q:\n%q", keys, seq, view)
			}
		}
	}
	view := runKeys(t, fixtureCopy(t, nil), 120, "1")
	if !strings.Contains(view, "\x1b[1m") && !strings.Contains(view, "\x1b[4m") {
		t.Fatalf("NO_COLOR dropped bold and underline as well:\n%q", view)
	}
	plain := ansi.Strip(view)
	for _, want := range []string{"[All]", "[Leads 4]", "◆ rolecall", "t track"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("without colour, %q is not readable:\n%s", want, plain)
		}
	}
}

// ROLECALL_ASCII replaces every drawn glyph with an ASCII stand-in, at the same width, so
// the frame is unchanged. Every state leaves only ASCII.
func TestAsciiGlyphsFallBack(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"1", "m"}, {"a"}, {"?"}, {"/pl"}, {"G", "e"}} {
		view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, keys...))
		for _, r := range view {
			if r > 127 {
				t.Fatalf("keys %q: non-ASCII glyph %q survived ROLECALL_ASCII:\n%s", keys, r, view)
			}
		}
	}
	plain := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1", "m"))
	for _, want := range []string{"[All]", "Move", "cancel"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("ASCII fallback lost %q:\n%s", want, plain)
		}
	}

	stats := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "4"))
	for _, g := range []string{"█", "─", "│", "…"} {
		if strings.Contains(stats, g) {
			t.Fatalf("glyph %q survived ROLECALL_ASCII on Stats:\n%s", g, stats)
		}
	}
	if !strings.Contains(stats, "Funnel") {
		t.Fatalf("ASCII fallback lost the Stats screen:\n%s", stats)
	}
}

// The ? pop-up explains the glyphs.
func TestHelpExplainsTheGlyphs(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "?"))
	if !strings.Contains(view, "GLYPHS") || !strings.Contains(view, "follow-up due") {
		t.Fatalf("help lacks the glyph legend:\n%s", view)
	}
}
