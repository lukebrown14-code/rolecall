package ui_test

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// NO_COLOR drops colour but keeps bold, underline and every word and glyph, so every
// state still reads. Checked across the list, the detail view and the pop-ups.
func TestNoColorDropsColourAndKeepsMarkers(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"1", "m"}, {"a"}, {"?"}, {"/pl"}, {"5"},
		{"6"}, {"6", "]", "]", "]", "]", "a"}, {"6", "enter"}} {
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
	for _, want := range []string{"s [Leads 7]", "▞ rolecall", "t [All 7]"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("without colour, %q is not readable:\n%s", want, plain)
		}
	}
	// Profile keeps its pane label, selection marker and Contact mark without colour.
	profile := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	for _, want := range []string{"SECTIONS", "▌ Contact", "✓"} {
		if !strings.Contains(profile, want) {
			t.Fatalf("without colour, %q is not readable on Profile:\n%s", want, profile)
		}
	}
}

// A pop-up over Settings dims the frame behind it but leaves the values and the group
// marker where they were, so the frame still reads while the pop-up is open.
func TestSettingsPopupKeepsTheFrameReadable(t *testing.T) {
	v := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "6", "]", "]", "]", "]", "a"))
	for _, want := range []string{"Add a Track", "SWE", "Tracking", "Follow-up due after"} {
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
		{"6"}, {"6", "]", "]", "]", "enter"}, {"6", "]", "]", "]", "]", "r"}, {"6", "]", "]", "]", "]", "]", "j", "enter"}} {
		view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, keys...))
		for _, r := range view {
			if r > 127 {
				t.Fatalf("keys %q: non-ASCII glyph %q survived ROLECALL_ASCII:\n%s", keys, r, view)
			}
		}
	}
	plain := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1", "m"))
	for _, want := range []string{"[All 7]", "Move", "cancel"} {
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
	if !strings.Contains(stats, "FUNNEL") || !strings.Contains(stats, "[####--------]") {
		t.Fatalf("ASCII fallback lost the Stats screen:\n%s", stats)
	}

	// Profile's marker, Contact's ✓ and the list divider fall back to ASCII too.
	profile := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	if !strings.Contains(profile, "> Contact") {
		t.Fatalf("ASCII fallback lost Profile's selection marker:\n%s", profile)
	}
	if row := requireProfileListRow(t, profile, "Contact"); !strings.HasSuffix(row, "+") {
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
			t.Setenv("TERM", "xterm-256color") // TERM=linux would turn ASCII on regardless
			view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "6", "]", "]", "]", "]"))
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
	if !strings.Contains(view, "GLYPHS") || !strings.Contains(view, "current stop") || !strings.Contains(view, "◌ Lead") || !strings.Contains(view, "⊘ Dismissed") || !strings.Contains(view, "⚑ follow-up due (14d)") || !strings.Contains(view, "— not assessed") || !strings.Contains(view, "✓ ready") {
		t.Fatalf("help lacks the glyph legend:\n%s", view)
	}
}

// ROLECALL_THEME=light draws the light roles: a white frame, and pop-ups filled with the
// light panel rather than a dark box on a light screen. dark and the default keep the
// dark frame.
func TestThemeEnvPicksTheRoles(t *testing.T) {
	const (
		darkPanel  = "48;2;22;27;34"    // #161B22
		lightPanel = "48;2;246;248;250" // #F6F8FA
	)
	for _, tc := range []struct {
		env, bg, panel, notPanel string
	}{
		{"light", "#ffffff", lightPanel, darkPanel},
		{"dark", "#0d1117", darkPanel, lightPanel},
		{"", "#0d1117", darkPanel, lightPanel},
	} {
		t.Run("theme="+tc.env, func(t *testing.T) {
			t.Setenv("ROLECALL_THEME", tc.env)
			t.Setenv("COLORFGBG", "")
			v := driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, termHeight), "1", "m").View()
			if got := hexOf(v.BackgroundColor); got != tc.bg {
				t.Fatalf("background %s, want %s", got, tc.bg)
			}
			if !strings.Contains(v.Content, tc.panel) || strings.Contains(v.Content, tc.notPanel) {
				t.Fatalf("pop-up fill: want %q and no %q:\n%q", tc.panel, tc.notPanel, v.Content)
			}
		})
	}
}

// COLORFGBG with a light background (7 or 15) picks the light theme when nothing says
// otherwise; an explicit ROLECALL_THEME wins.
func TestColorFGBGGuessesTheTheme(t *testing.T) {
	for _, tc := range []struct{ env, fgbg, bg string }{
		{"", "0;15", "#ffffff"}, {"auto", "0;7", "#ffffff"}, {"", "15;0", "#0d1117"}, {"dark", "0;15", "#0d1117"},
	} {
		t.Setenv("ROLECALL_THEME", tc.env)
		t.Setenv("COLORFGBG", tc.fgbg)
		v := driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, termHeight), "1").View()
		if got := hexOf(v.BackgroundColor); got != tc.bg {
			t.Fatalf("ROLECALL_THEME=%q COLORFGBG=%q: background %s, want %s", tc.env, tc.fgbg, got, tc.bg)
		}
	}
}

func hexOf(c color.Color) string {
	if c == nil {
		return "none"
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// The Linux console's font lacks most glyphs, so TERM=linux turns ASCII on by itself.
func TestAsciiOnTheLinuxConsole(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("TERM", "linux")
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	for _, r := range view {
		if r > 127 {
			t.Fatalf("non-ASCII glyph %q survived TERM=linux:\n%s", r, view)
		}
	}
}

// Each Status keeps its own glyph in ASCII, so the list still tells them apart.
func TestAsciiKeepsStatusesApart(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	for _, want := range []string{"* Applied . 1", "@ Interviewing . 2", "$ Offer . 2", "x docs-ready", "< docs-resume-only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("ASCII lost %q:\n%s", want, view)
		}
	}
}

// Every Status shows its glyph wherever the Status shows: the view row, group headers,
// rows, the preview, the timeline and the Move pop-up. The filter row names views without
// glyphs, as the design draws it.
func TestStatusGlyphsShowEverywhere(t *testing.T) {
	list := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	for _, want := range []string{"● Applied · 1", "◆ Interviewing · 2", "★ Offer · 2", "▌ ● Software Engineer",
		"● Applied", "◌ Lead", "★ docs-partial", "✕ docs-ready", "↩ docs-resume-only"} {
		if !strings.Contains(list, want) {
			t.Fatalf("Applied screen lacks %q:\n%s", want, list)
		}
	}
	move := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1", "m"))
	for _, want := range []string{"now ◌ Lead", "✎ Draft", "⊘ Dismissed"} {
		if !strings.Contains(move, want) {
			t.Fatalf("Move pop-up lacks %q:\n%s", want, move)
		}
	}
}

// The Closed group carries the ✕ group glyph (x in ASCII), and Withdrawn rows are told
// from Rejected ones by their own ↩ glyph.
func TestClosedGroupHeaderHasAGlyph(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	for _, want := range []string{"✕ Closed · 4", "↩ docs-resume-only"} {
		if !strings.Contains(view, want) {
			t.Fatalf("Applied screen lacks %q:\n%s", want, view)
		}
	}
	t.Setenv("ROLECALL_ASCII", "1")
	view = ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	if !strings.Contains(view, "x Closed . 4") {
		t.Fatalf("ASCII Closed header lost its glyph:\n%s", view)
	}
}

// With NO_COLOR set, every Status still shows its glyph, so no meaning rests on colour.
func TestNoColorShowsAGlyphForEveryStatus(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, tc := range []struct {
		keys  []string
		wants []string
	}{
		{[]string{"1"}, []string{"◌ "}},
		{[]string{"1", "s"}, []string{"⊘ "}},
		{[]string{"2"}, []string{"✎ "}},
		{[]string{"3"}, []string{"● Applied", "◆ Interviewing", "★ Offer", "✕ Closed", "✕ docs-ready", "↩ docs-resume-only"}},
	} {
		view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, tc.keys...))
		for _, want := range tc.wants {
			if !strings.Contains(view, want) {
				t.Errorf("NO_COLOR keys %q lacks %q:\n%s", tc.keys, want, view)
			}
		}
	}
}
