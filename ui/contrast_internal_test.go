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
	check := func(t *testing.T, what string, fg, bg color.Color, floor float64) {
		t.Helper()
		if got := contrastRatio(fg, bg); got < floor {
			t.Errorf("%s is %.2f:1, below %.1f:1", what, got, floor)
		}
	}
	for _, th := range []struct {
		name string
		t    theme
	}{{"dark", themeDark}, {"light", themeLight}} {
		t.Run(th.name, func(t *testing.T) {
			p, d := th.t.base(), th.t.dimmed()
			text := []role{{"text", p.fg}, {"muted", p.dim}, {"sub", p.sub}, {"accent", p.accent}, {"ok", p.ok},
				{"info", p.info}, {"warn", p.warn}, {"danger", p.bad}, {"purple", p.purple}, {"gold", p.gold},
				{"lead", p.lead}, {"draft", p.draft}}
			for _, r := range text {
				check(t, r.name+" on background", r.c, p.bg, textFloor)
				check(t, r.name+" on the pop-up fill", r.c, p.panel, textFloor)
			}
			// The selection bar carries the title, secondary text, the slots and dates.
			for _, r := range []role{{"text", p.fg}, {"sub", p.sub}, {"accent", p.accent}, {"ok", p.ok}, {"info", p.info},
				{"warn", p.warn}, {"danger", p.bad}} {
				check(t, r.name+" on the selection bar", r.c, p.sel, textFloor)
			}
			// Status glyphs lead a row, so they also sit on the selection bar.
			for _, s := range []string{"Lead", "Draft", "Applied", "Interviewing", "Offer", "Rejected", "Withdrawn", "Dismissed"} {
				check(t, s+" glyph on the selection bar", p.statusColor(s), p.sel, graphicFloor)
			}
			for _, r := range []role{{"rule", p.rule}, {"faint", p.faint}} {
				check(t, r.name+" on background", r.c, p.bg, graphicFloor)
				check(t, r.name+" on the pop-up fill", r.c, p.panel, graphicFloor)
			}
			check(t, "text on an accent fill", p.onFill, p.accent, textFloor)
			// Behind a pop-up the frame dims but stays readable: values, markers and the
			// active tab must clear the text floor, not just look "unimportant".
			for _, r := range []role{{"text", d.fg}, {"muted", d.dim}, {"sub", d.sub}, {"accent", d.accent}} {
				check(t, "dimmed "+r.name+" on background", r.c, d.bg, textFloor)
			}
			check(t, "dimmed rule on background", d.rule, d.bg, graphicFloor)
			check(t, "dimmed active tab text on its fill", d.onFill, d.accent, textFloor)
		})
	}
}

// The Statuses are told apart by colour as well as glyph: no two share either, Draft is
// not the accent, Offer is not grey, and no closed Status is red.
func TestStatusesAreDistinct(t *testing.T) {
	statuses := []string{"Lead", "Draft", "Applied", "Interviewing", "Offer", "Rejected", "Withdrawn", "Dismissed"}
	for _, th := range []theme{themeDark, themeLight} {
		p := th.base()
		glyphs, ascii, colours := map[string]string{}, map[string]string{}, map[color.Color]string{}
		for _, s := range statuses {
			g, a := p.statusGlyph(s), statusGlyphs[s][1]
			if g == "" || glyphs[g] != "" || ascii[a] != "" || len(a) != 1 {
				t.Errorf("%s: glyph %q / ASCII %q missing or shared", s, g, a)
			}
			glyphs[g], ascii[a] = s, s
			if closedStatus(s) {
				if p.statusColor(s) != p.dim {
					t.Errorf("closed %s is not muted", s)
				}
				continue
			}
			if c := p.statusColor(s); colours[c] != "" || c == p.dim || c == p.accent || c == p.bad {
				t.Errorf("%s colour clashes with %q, muted, the accent or danger", s, colours[c])
			}
			colours[p.statusColor(s)] = s
		}
	}
}
