package theme_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// The five active Statuses must stay apart for the Owner whatever their colour vision:
// under simulated deuteranopia and protanopia every pair keeps a CIEDE2000 distance of at
// least cvdFloor, so the glyph is a second signal rather than the only one.
const cvdFloor = 10.0

// machado are Machado, Oliveira and Fernandes (2009) severity-1.0 matrices, applied to
// linear RGB.
var machado = map[string][3][3]float64{
	"deuteranopia": {{0.367322, 0.860646, -0.227968}, {0.280085, 0.672501, 0.047413}, {-0.011820, 0.042940, 0.968881}},
	"protanopia":   {{0.152286, 1.052583, -0.204868}, {0.114503, 0.786281, 0.099216}, {-0.003882, -0.048116, 1.051998}},
}

func linear(v uint32) float64 {
	s := float64(v>>8) / 255
	if s <= 0.04045 {
		return s / 12.92
	}
	return math.Pow((s+0.055)/1.055, 2.4)
}

// simulate is c as seen through a matrix, in CIELAB (D65).
func simulate(c color.Color, m [3][3]float64) [3]float64 {
	r, g, b, _ := c.RGBA()
	in := [3]float64{linear(r), linear(g), linear(b)}
	var out [3]float64
	for i := range out {
		out[i] = math.Min(1, math.Max(0, m[i][0]*in[0]+m[i][1]*in[1]+m[i][2]*in[2]))
	}
	x := 0.4124564*out[0] + 0.3575761*out[1] + 0.1804375*out[2]
	y := 0.2126729*out[0] + 0.7151522*out[1] + 0.0721750*out[2]
	z := 0.0193339*out[0] + 0.1191920*out[1] + 0.9503041*out[2]
	f := func(t float64) float64 {
		if t > 216.0/24389 {
			return math.Cbrt(t)
		}
		return (24389.0/27*t + 16) / 116
	}
	fx, fy, fz := f(x/0.95047), f(y), f(z/1.08883)
	return [3]float64{116*fy - 16, 500 * (fx - fy), 200 * (fy - fz)}
}

func sq(v float64) float64  { return v * v }
func deg(r float64) float64 { return r * 180 / math.Pi }
func rad(d float64) float64 { return d * math.Pi / 180 }

// deltaE2000 is the CIEDE2000 colour difference between two CIELAB colours.
func deltaE2000(a, b [3]float64) float64 {
	c1, c2 := math.Hypot(a[1], a[2]), math.Hypot(b[1], b[2])
	cb7 := math.Pow((c1+c2)/2, 7)
	g := 0.5 * (1 - math.Sqrt(cb7/(cb7+math.Pow(25, 7))))
	a1, a2 := (1+g)*a[1], (1+g)*b[1]
	c1p, c2p := math.Hypot(a1, a[2]), math.Hypot(a2, b[2])
	hue := func(y, x float64) float64 {
		if x == 0 && y == 0 {
			return 0
		}
		h := deg(math.Atan2(y, x))
		if h < 0 {
			h += 360
		}
		return h
	}
	h1, h2 := hue(a[2], a1), hue(b[2], a2)
	dL, dC := b[0]-a[0], c2p-c1p
	dh := 0.0
	if c1p*c2p != 0 {
		dh = h2 - h1
		if dh > 180 {
			dh -= 360
		} else if dh < -180 {
			dh += 360
		}
	}
	dH := 2 * math.Sqrt(c1p*c2p) * math.Sin(rad(dh/2))
	lb, cpb := (a[0]+b[0])/2, (c1p+c2p)/2
	hb := h1 + h2
	if c1p*c2p != 0 {
		switch {
		case math.Abs(h1-h2) <= 180:
			hb = (h1 + h2) / 2
		case h1+h2 < 360:
			hb = (h1 + h2 + 360) / 2
		default:
			hb = (h1 + h2 - 360) / 2
		}
	}
	t := 1 - 0.17*math.Cos(rad(hb-30)) + 0.24*math.Cos(rad(2*hb)) + 0.32*math.Cos(rad(3*hb+6)) - 0.20*math.Cos(rad(4*hb-63))
	dTheta := 30 * math.Exp(-sq((hb-275)/25))
	rc := 2 * math.Sqrt(math.Pow(cpb, 7)/(math.Pow(cpb, 7)+math.Pow(25, 7)))
	sl := 1 + 0.015*sq(lb-50)/math.Sqrt(20+sq(lb-50))
	sc, sh := 1+0.045*cpb, 1+0.015*cpb*t
	rt := -math.Sin(rad(2*dTheta)) * rc
	return math.Sqrt(sq(dL/sl) + sq(dC/sc) + sq(dH/sh) + rt*(dC/sc)*(dH/sh))
}

// TestActiveStatusColoursSurviveColourBlindness checks every pair of active Status
// colours, on both palettes, under both simulations.
func TestActiveStatusColoursSurviveColourBlindness(t *testing.T) {
	active := []string{store.StatusLead, store.StatusDraft, store.StatusApplied, store.StatusInterviewing, store.StatusOffer}
	for _, th := range []struct {
		name string
		m    theme.Mode
	}{{"dark", theme.Dark}, {"light", theme.Light}} {
		p := th.m.Base()
		for kind, matrix := range machado {
			for i, s1 := range active {
				for _, s2 := range active[i+1:] {
					d := deltaE2000(simulate(p.StatusColor(s1), matrix), simulate(p.StatusColor(s2), matrix))
					if d < cvdFloor {
						t.Errorf("%s palette, %s: %s and %s are only ΔE %.1f apart, want ≥ %.0f", th.name, kind, s1, s2, d, cvdFloor)
					}
				}
			}
		}
	}
}
