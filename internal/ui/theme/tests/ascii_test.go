package theme_test

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// Every glyph whose meaning matters folds to its own ASCII stand-in, so a terminal that
// cannot draw the Unicode set still tells a Lead from a missing document, a warning from
// a follow-up flag, and a failure from a closed Status.
func TestASCIIStandInsAreDistinct(t *testing.T) {
	for _, tc := range []struct{ name, glyph string }{
		{"Lead", "◌"}, {"Draft", "◐"}, {"Applied", "●"}, {"Interviewing", "◆"},
		{"Offer", "★"}, {"Rejected", "✕"}, {"Withdrawn", "↩"}, {"Dismissed", "⊘"},
		{"warning", "⚠"}, {"follow-up flag", "⚑"}, {"missing document", "○"},
		{"ready document", "✓"}, {"error", "✘"}, {"wordmark", "▞"},
	} {
		got := theme.ASCIIFold(tc.glyph, true)
		if got == tc.glyph {
			t.Errorf("%s %q has no ASCII stand-in", tc.name, tc.glyph)
		}
		if got == "" {
			t.Errorf("%s %q folded away", tc.name, tc.glyph)
		}
		for _, r := range got {
			if r > 127 {
				t.Errorf("%s %q folded to non-ASCII %q", tc.name, tc.glyph, got)
				break
			}
		}
	}
	// No two of these share a stand-in.
	seen := map[string]string{}
	for _, tc := range []struct{ name, glyph string }{
		{"Lead", "◌"}, {"Draft", "◐"}, {"Applied", "●"}, {"Interviewing", "◆"},
		{"Offer", "★"}, {"Rejected", "✕"}, {"Withdrawn", "↩"}, {"Dismissed", "⊘"},
		{"warning", "⚠"}, {"follow-up flag", "⚑"}, {"missing document", "○"},
		{"ready document", "✓"}, {"error", "✘"},
	} {
		got := theme.ASCIIFold(tc.glyph, true)
		if other, ok := seen[got]; ok {
			t.Errorf("%s and %s both fold to %q", other, tc.name, got)
		}
		seen[got] = tc.name
	}
}
