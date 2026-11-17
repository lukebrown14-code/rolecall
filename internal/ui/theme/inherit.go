package theme

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// The "inherit terminal colours" palette: every role is an ANSI-16 index, so the Owner's
// terminal theme decides the actual colour, and no role paints a background. Any text the
// Owner must read — body, secondary and muted text, separators, Lead — uses the terminal's
// default foreground, which every theme makes legible; ANSI 8 (bright black, the
// background colour on Solarized Dark) is kept for rules only, and ANSI 7 and the bright
// yellow (near-invisible on light terminals) are not used at all. Offer is yellow and bold
// (its chip is bold while open); a close date within a week is bright red, apart from
// Offer's yellow and from danger's red, which also carries ▲ and bold. The selection and
// panel fills are ANSI backgrounds that InheritANSI turns into reverse video.
func inheritColors() Palette {
	c := func(i int) color.Color { return lipgloss.ANSIColor(i) }
	const (
		red, green, yellow, blue, magenta, cyan = 1, 2, 3, 4, 5, 6
		brightBlack, brightRed, brightBlue      = 8, 9, 12
		brightMagenta                           = 13
	)
	return Palette{
		Fg: lipgloss.NoColor{}, Sub: lipgloss.NoColor{}, OnFill: lipgloss.NoColor{},
		Dim: lipgloss.NoColor{}, Faint: lipgloss.NoColor{}, Rule: c(brightBlack),
		Accent: c(cyan), Sel: c(brightBlack), Ok: c(green), Info: c(blue), Warn: c(brightRed), Bad: c(red),
		Bg: lipgloss.NoColor{}, Panel: lipgloss.NoColor{},
		Purple: c(magenta), Gold: c(yellow), Lead: lipgloss.NoColor{}, Draft: c(brightBlue),
		Marginal: c(yellow),
		Inherit:  true,
	}
}

// InheritBase is the palette for the inherit-terminal option.
func InheritBase() Palette { return inheritColors() }

// InheritDimmed is the inherit palette behind a pop-up: everything in the muted colour.
func InheritDimmed() Palette {
	p := inheritColors()
	d := p.Dim
	p.Fg, p.Sub, p.Accent, p.Ok, p.Info, p.Warn, p.Bad = d, d, d, d, d, d, d
	p.Purple, p.Gold, p.Lead, p.Draft, p.Marginal = d, d, d, d, d
	p.Sel, p.Backdrop = lipgloss.NoColor{}, true
	return p
}

// InheritANSI reduces a rendered frame to what the inherit option allows: ANSI-16
// foregrounds, bold, underline and reverse video. A truecolor or 256-colour foreground
// (Glamour's Markdown) maps to the nearest ANSI-16 index, and any background becomes
// reverse video, so a fill that meant "selected" or "active" still shows without paint.
func InheritANSI(s string) string {
	return sgr.ReplaceAllStringFunc(s, func(seq string) string {
		body := seq[2 : len(seq)-1]
		if body == "" {
			return seq
		}
		nums := strings.Split(body, ";")
		var out []string
		reverse := false
		for i := 0; i < len(nums); i++ {
			n, _ := strconv.Atoi(nums[i])
			switch {
			case n == sgrExtFg || n == sgrExtBg:
				var r, g, b int
				i++
				switch {
				case i < len(nums) && nums[i] == "2" && i+3 < len(nums):
					r, g, b = atoi(nums[i+1]), atoi(nums[i+2]), atoi(nums[i+3])
					i += 3
				case i < len(nums) && nums[i] == "5" && i+1 < len(nums):
					r, g, b = xterm256(atoi(nums[i+1]))
					i++
				}
				if n == sgrExtBg {
					reverse = true
				} else {
					out = append(out, strconv.Itoa(ansi16Code(r, g, b)))
				}
			case n >= sgrBgFirst && n <= sgrBgLast, n >= sgrBrightBgFirst && n <= sgrBrightBgLast:
				reverse = true
			case n == sgrDefaultBg:
			default:
				out = append(out, nums[i])
			}
		}
		if reverse {
			out = append(out, "7")
		}
		if len(out) == 0 {
			return ""
		}
		return "\x1b[" + strings.Join(out, ";") + "m"
	})
}

// SGR codes and colour-cube numbers used by the reduction.
const (
	sgrExtFg, sgrExtBg                = 38, 48
	sgrBgFirst, sgrBgLast             = 40, 47
	sgrBrightBgFirst, sgrBrightBgLast = 100, 107
	sgrDefaultBg                      = 49
	sgrFgBase, sgrBrightFgBase        = 30, 90
	ansiNormal                        = 8 // colours 0-7 are normal, 8-15 bright
	cubeStart, greyStart              = 16, 232
	cubeSide, cubeBase, cubeStep      = 6, 55, 40
	greyBase, greyStep                = 8, 10
)

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

// ansi16RGB is a typical rendering of the 16 ANSI colours, used only to pick the nearest.
var ansi16RGB = [16][3]int{
	{0, 0, 0}, {205, 49, 49}, {13, 188, 121}, {229, 229, 16}, {36, 114, 200}, {188, 63, 188}, {17, 168, 205}, {229, 229, 229},
	{102, 102, 102}, {241, 76, 76}, {35, 209, 139}, {245, 245, 67}, {59, 142, 234}, {214, 112, 214}, {41, 184, 219}, {255, 255, 255},
}

// ansi16Code is the SGR foreground code of the ANSI-16 colour nearest to r, g, b.
func ansi16Code(r, g, b int) int {
	best, bestD := 0, math.MaxInt
	for i, c := range ansi16RGB {
		d := (r-c[0])*(r-c[0]) + (g-c[1])*(g-c[1]) + (b-c[2])*(b-c[2])
		if d < bestD {
			best, bestD = i, d
		}
	}
	if best < ansiNormal {
		return sgrFgBase + best
	}
	return sgrBrightFgBase + best - ansiNormal
}

// xterm256 is the RGB of colour n in the 256-colour palette.
func xterm256(n int) (int, int, int) {
	switch {
	case n < cubeStart:
		c := ansi16RGB[n]
		return c[0], c[1], c[2]
	case n < greyStart:
		n -= cubeStart
		level := func(v int) int {
			if v == 0 {
				return 0
			}
			return cubeBase + cubeStep*v
		}
		return level(n / (cubeSide * cubeSide)), level(n / cubeSide % cubeSide), level(n % cubeSide)
	}
	g := greyBase + greyStep*(n-greyStart)
	return g, g, g
}
