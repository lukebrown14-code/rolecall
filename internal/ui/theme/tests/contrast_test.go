package theme_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// The palette must keep its footing on the dark background: body text at the WCAG AA
// floor of 4.5:1, and the rules and pane dividers at the non-text floor of 3:1. This is
// the check behind "colour always pairs with a word or glyph, so meaning never rests on
// colour alone" (CODING_STANDARDS): the words and glyphs have to be legible.

// relLum is a colour's relative luminance, per WCAG 2.x.
func relLum(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	channel := func(v uint32) float64 {
		s := float64(v>>8) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// contrastRatio is the WCAG contrast ratio between two colours.
func contrastRatio(a, b color.Color) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

type role struct {
	name string
	c    color.Color
}

// TestPaletteMeetsContrastFloors checks every role pair both themes draw: text roles at
// 4.5:1 on the background, the pop-up fill and the selection bar; rules, separators and
// Status glyphs at 3:1; and the frame behind a pop-up still at the text floor.
func TestPaletteMeetsContrastFloors(t *testing.T) {
	const (
		textFloor    = 4.5 // WCAG AA for body text
		graphicFloor = 3.0 // WCAG AA for rules, dividers and glyphs
	)
	// The selection fill is a tint, so it cannot reach 3:1 against the page and still keep
	// the title and accent at 4.5:1 on it: on white the accent's 4.5:1 caps the fill near
	// 1.2:1. Each fill is as deep as the text floor allows; the accent gutter mark, at
	// 3:1 and more on both, carries the selection's non-text contrast.
	selFillFloor := map[string]float64{"dark": 2.3, "light": 1.17}
	check := func(t *testing.T, what string, fg, bg color.Color, floor float64) {
		t.Helper()
		if got := contrastRatio(fg, bg); got < floor {
			t.Errorf("%s is %.2f:1, below %.1f:1", what, got, floor)
		}
	}
	for _, th := range []struct {
		name string
		t    theme.Mode
	}{{"dark", theme.Dark}, {"light", theme.Light}} {
		t.Run(th.name, func(t *testing.T) {
			p, d := th.t.Base(), th.t.Dimmed()
			text := []role{{"text", p.Fg}, {"muted", p.Dim}, {"sub", p.Sub}, {"accent", p.Accent}, {"ok", p.Ok},
				{"info", p.Info}, {"warn", p.Warn}, {"danger", p.Bad}, {"purple", p.Purple}, {"gold", p.Gold},
				{"lead", p.Lead}, {"draft", p.Draft}, {"marginal", p.Marginal}}
			for _, r := range text {
				check(t, r.name+" on background", r.c, p.Bg, textFloor)
				check(t, r.name+" on the pop-up fill", r.c, p.Panel, textFloor)
			}
			// The selection bar carries the title and secondary text, which reach the text
			// floor. Coloured words drawn on it (close dates, warnings, document states) are
			// drawn with OnSel, whose word roles must reach the text floor too; every Status
			// glyph on it, open or closed, reaches the graphic floor.
			on := p.OnSel()
			for _, r := range []role{{"text", p.Fg}, {"sub", p.Sub}, {"accent", p.Accent},
				{"ok word", on.Ok}, {"warn word", on.Warn}, {"danger word", on.Bad}, {"muted word", on.Dim}} {
				check(t, r.name+" on the selection bar", r.c, p.Sel, textFloor)
			}
			for _, s := range []string{"Lead", "Draft", "Applied", "Interviewing", "Offer", "Rejected", "Withdrawn", "Dismissed"} {
				check(t, s+" glyph on the selection bar", on.StatusColor(s), p.Sel, graphicFloor)
			}
			// The fill itself is told from the page by the gutter mark as well as its tint.
			check(t, "selection fill on background", p.Sel, p.Bg, selFillFloor[th.name])
			for _, r := range []role{{"rule", p.Rule}, {"faint", p.Faint}} {
				check(t, r.name+" on background", r.c, p.Bg, graphicFloor)
				check(t, r.name+" on the pop-up fill", r.c, p.Panel, graphicFloor)
			}
			check(t, "text on an accent fill", p.OnFill, p.Accent, textFloor)
			// Behind a pop-up the frame dims but stays readable: values, markers and the
			// active tab must clear the text floor, not just look "unimportant".
			for _, r := range []role{{"text", d.Fg}, {"muted", d.Dim}, {"sub", d.Sub}, {"accent", d.Accent}} {
				check(t, "dimmed "+r.name+" on background", r.c, d.Bg, textFloor)
				// The selected row behind a pop-up is filled with the dimmed Sel (the panel).
				check(t, "dimmed "+r.name+" on the dimmed selection", r.c, d.Sel, textFloor)
			}
			check(t, "dimmed rule on background", d.Rule, d.Bg, graphicFloor)
			check(t, "dimmed active tab text on its fill", d.OnFill, d.Accent, textFloor)
		})
	}
}

// The Statuses are told apart by colour as well as glyph: no two share either, Draft is
// not the accent, Offer is not grey, and no closed Status is red.
func TestStatusesAreDistinct(t *testing.T) {
	statuses := []string{"Lead", "Draft", "Applied", "Interviewing", "Offer", "Rejected", "Withdrawn", "Dismissed"}
	for _, th := range []theme.Mode{theme.Dark, theme.Light} {
		p := th.Base()
		glyphs, ascii, colours := map[string]string{}, map[string]string{}, map[color.Color]string{}
		for _, s := range statuses {
			asciiPalette := p
			asciiPalette.Ascii = true
			g, a := p.StatusGlyph(s), asciiPalette.StatusGlyph(s)
			if g == "" || glyphs[g] != "" || ascii[a] != "" || len(a) != 1 {
				t.Errorf("%s: glyph %q / ASCII %q missing or shared", s, g, a)
			}
			glyphs[g], ascii[a] = s, s
			if theme.ClosedStatus(s) {
				if p.StatusColor(s) != p.Dim {
					t.Errorf("closed %s is not muted", s)
				}
				continue
			}
			if c := p.StatusColor(s); colours[c] != "" || c == p.Dim || c == p.Accent || c == p.Bad {
				t.Errorf("%s colour clashes with %q, muted, the accent or danger", s, colours[c])
			}
			colours[p.StatusColor(s)] = s
		}
	}
}

// Each Fit word has a colour of its own, Strong green, Good blue, Marginal yellow and Poor
// muted, and none borrows the amber or red that a row keeps for time, so a Fit never reads
// as a deadline.
func TestFitColoursAreApartFromUrgency(t *testing.T) {
	for _, th := range []theme.Mode{theme.Dark, theme.Light} {
		p := th.Base()
		seen := map[color.Color]string{}
		for _, fit := range []string{"Strong", "Good", "Marginal", "Poor"} {
			c := p.FitColor(fit)
			if seen[c] != "" {
				t.Errorf("fit %s shares its colour with %s", fit, seen[c])
			}
			seen[c] = fit
			if c == p.Warn || c == p.Bad || c == p.Gold || c == p.Accent {
				t.Errorf("fit %s is drawn in an urgency, Offer or accent colour", fit)
			}
		}
		if p.FitColor("Poor") != p.Dim || p.FitColor("") != p.Dim {
			t.Error("Poor and not assessed stay muted")
		}
	}
}

// The light selection fill is a tint of the teal accent, not a blue: green leads, then
// blue, then red, as in the accent, and it sits nearer the page than the accent.
func TestLightSelectionIsAnAccentTint(t *testing.T) {
	p := theme.Light.Base()
	r, g, b, _ := p.Sel.RGBA()
	if g < b || b <= r {
		t.Errorf("light Sel %v is not a teal tint", p.Sel)
	}
	if contrastRatio(p.Sel, p.Accent) < contrastRatio(p.Bg, p.Accent)*0.8 {
		t.Errorf("light Sel is too close to the accent to read as a tint of the page")
	}
}
