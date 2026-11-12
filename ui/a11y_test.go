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
	for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"1", "m"}, {"a"}, {"?"}, {"/pl"}, {"5"},
		{"6"}, {"6", "j", "j", "tab", "a"}, {"6", "tab", "enter"}} {
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
	// Profile keeps its pane label, selection marker and Contact mark without colour.
	profile := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	for _, want := range []string{"SECTIONS", "▸ Contact", "✓"} {
		if !strings.Contains(profile, want) {
			t.Fatalf("without colour, %q is not readable on Profile:\n%s", want, profile)
		}
	}
}

// A pop-up over Settings dims the frame behind it but leaves the values and the group
// marker where they were, so the frame still reads while the pop-up is open.
func TestSettingsPopupKeepsTheFrameReadable(t *testing.T) {
	v := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "6", "j", "j", "tab", "a"))
	for _, want := range []string{"Add Track", "▸ Tracks", "Tracking", "Follow-up days"} {
		if !strings.Contains(v, want) {
			t.Fatalf("pop-up over Settings lost %q:\n%s", want, v)
		}
	}
}

// ROLECALL_ASCII replaces every drawn glyph with an ASCII stand-in, at the same width, so
// the frame is unchanged. Every state leaves only ASCII.
func TestAsciiGlyphsFallBack(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"1", "m"}, {"a"}, {"?"}, {"/pl"}, {"G", "m", "d"}, {"5"},
		{"6"}, {"6", "j", "tab", "enter"}, {"6", "j", "j", "tab", "r"}, {"6", "j", "j", "j", "tab", "j", "enter"}} {
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

	// Profile's marker, Contact's ✓ and the list divider fall back to ASCII too.
	profile := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	if !strings.Contains(profile, "> Contact") {
		t.Fatalf("ASCII fallback lost Profile's selection marker:\n%s", profile)
	}
	if row := requireProfileListRow(t, profile, "Contact"); !strings.HasSuffix(row, "v") {
		t.Fatalf("ASCII fallback lost Contact's mark: row %q:\n%s", row, profile)
	}
	if !hasProfileDivider(profile) {
		t.Fatalf("ASCII fallback lost the list divider:\n%s", profile)
	}
}

// A locale that is not UTF-8 turns ASCII on automatically, so a terminal that cannot
// draw ◆ ▌ ⚑ still reads without anyone setting ROLECALL_ASCII. A UTF-8 locale keeps the
// glyphs.
func TestAsciiFallsBackWhenTheLocaleIsNotUTF8(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "")
	for _, tc := range []struct {
		name     string
		lang     string
		nonASCII bool
	}{
		{"C locale", "C", false},
		{"UTF-8 locale", "en_US.UTF-8", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("LC_ALL", "")
			t.Setenv("LC_CTYPE", "")
			t.Setenv("LANG", tc.lang)
			view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "6", "j", "j", "tab"))
			got := false
			for _, r := range view {
				if r > 127 {
					got = true
				}
			}
			if got != tc.nonASCII {
				t.Fatalf("non-ASCII present = %v, want %v under LANG=%s:\n%s", got, tc.nonASCII, tc.lang, view)
			}
		})
	}
}

// The ? pop-up explains the glyphs.
func TestHelpExplainsTheGlyphs(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "?"))
	if !strings.Contains(view, "GLYPHS") || !strings.Contains(view, "● current status") || !strings.Contains(view, "⚑ quiet: no Status change in 14+ days") || !strings.Contains(view, "— not assessed") {
		t.Fatalf("help lacks the glyph legend:\n%s", view)
	}
}
