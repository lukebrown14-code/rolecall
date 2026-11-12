package ui

import (
	"image/color"
	"math"
	"testing"
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

func TestPaletteMeetsContrastFloors(t *testing.T) {
	const (
		textFloor    = 4.5 // WCAG AA for body text
		graphicFloor = 3.0 // WCAG AA for rules and dividers
	)
	for _, c := range []struct {
		name string
		fg   color.Color
	}{
		{"fg", basePal.fg}, {"dim", basePal.dim}, {"sub", basePal.sub},
	} {
		if got := contrastRatio(c.fg, colorBg); got < textFloor {
			t.Errorf("base %s on background is %.2f:1, below %.1f:1", c.name, got, textFloor)
		}
	}
	if got := contrastRatio(basePal.rule, colorBg); got < graphicFloor {
		t.Errorf("pane divider on background is %.2f:1, below %.1f:1", got, graphicFloor)
	}
	if got := contrastRatio(dimmedPal.rule, colorBg); got < graphicFloor {
		t.Errorf("pane divider behind a pop-up is %.2f:1, below %.1f:1", got, graphicFloor)
	}
	// Behind a pop-up the frame dims but stays readable: the values and the group marker
	// must clear the text floor, not just look "unimportant".
	for _, c := range []struct {
		name string
		fg   color.Color
	}{
		{"fg", dimmedPal.fg}, {"dim", dimmedPal.dim},
		{"sub", dimmedPal.sub}, {"accent", dimmedPal.accent},
	} {
		if got := contrastRatio(c.fg, colorBg); got < textFloor {
			t.Errorf("dimmed %s on background is %.2f:1, below %.1f:1", c.name, got, textFloor)
		}
	}
}
