package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// glyphScreens are the frames the legend must explain: every screen, the detail tabs,
// the pop-ups, and a Lead about to close.
var glyphScreens = [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}, {"3", "enter"},
	{"3", "/docs-written", "enter"}, {"3", "/docs-resume-only", "enter"}, {"1", "m"}, {"a"}, {"?"}, {" "}}

// Every glyph the frame draws is either in the `?` legend (theme.Legend, which the
// legend is generated from) or chrome with no meaning of its own: borders, rules, the
// selection gutter, arrows, the separator. A new glyph cannot be drawn without saying
// what it means.
func TestEveryDrawnGlyphIsInTheLegend(t *testing.T) {
	legend := map[string]bool{}
	for _, g := range theme.Legend() {
		legend[g.Mark] = true
	}
	// Marks that belong to the words around them rather than to the legend: the search
	// chip's clear key and the wordmark are chrome too, but quotes and letters in the
	// fixture's own text are not glyphs at all.
	text := map[rune]bool{'’': true, '‘': true, '“': true, '”': true, '•': true, 'é': true}
	for _, keys := range glyphScreens {
		view := ansi.Strip(runKeysSized(t, fixtureCopy(t, fernCloses("2026-10-02")), 120, 50, keys...))
		for _, r := range view {
			s := string(r)
			if r < 128 || legend[s] || theme.IsChromeGlyph(s) || text[r] {
				continue
			}
			t.Errorf("keys %q draw %q (U+%04X), which the legend does not explain", keys, s, r)
		}
	}
}

// The legend lists every glyph in the table, each with its one meaning.
func TestLegendListsTheGlyphTable(t *testing.T) {
	help := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 60, "?"))
	for _, g := range theme.Legend() {
		want := g.Mark + " " + g.Meaning
		switch {
		case g.Group == theme.GroupStatus:
			want = g.Mark + " " + g.Meaning
		case g.Mark == theme.GlyphMeterOn || g.Mark == theme.GlyphMeterOff:
			want = theme.GlyphMeterOn + theme.GlyphMeterOn + theme.GlyphMeterOff + " 2/3 ready"
		case g.Mark == theme.GlyphFollowUp:
			want = g.Mark + " " + g.Meaning + " ("
		}
		if !strings.Contains(help, want) {
			t.Errorf("legend lacks %q:\n%s", want, help)
		}
	}
}

// NO_COLOR keeps an expired close date's strikethrough: it is not colour.
func TestNoColorKeepsTheExpiredStrike(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	view := runKeys(t, fixtureCopy(t, fernCloses("2026-09-01")), 120, "1")
	if !strings.Contains(view, "\x1b[9m") && !strings.Contains(view, ";9m") {
		t.Fatalf("NO_COLOR dropped the expired date's strike:\n%q", view)
	}
}

// ROLECALL_PLAIN=1 leaves the mouse to the terminal and drops the tab bar's sparkline.
func TestPlainModeDropsMouseAndSparkline(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	if v := m.View(); v.MouseMode == tea.MouseModeNone {
		t.Fatal("the default frame does not capture the mouse")
	}
	t.Setenv("ROLECALL_PLAIN", "1")
	v := m.View()
	if v.MouseMode != tea.MouseModeNone {
		t.Errorf("plain mode still captures the mouse")
	}
	if bar := strings.SplitN(ansi.Strip(v.Content), "\n", 2)[0]; strings.ContainsAny(bar, "▁▂▃▄▅▆▇█") {
		t.Errorf("plain mode keeps the sparkline: %q", bar)
	}
}
