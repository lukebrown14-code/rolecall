package theme_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// The Statuses each keep a glyph and an ASCII stand-in of their own, so a terminal that
// cannot draw the Unicode set (or NO_COLOR, which drops colour) still tells them apart.
func TestStatusGlyphsAndASCIIStandIns(t *testing.T) {
	wantGlyph := map[string]string{
		"Lead": "◌", "Draft": "✎", "Applied": "●", "Interviewing": "◆", "Offer": "★",
		"Rejected": "✕", "Withdrawn": "↩", "Dismissed": "⊘",
	}
	for _, mode := range []theme.Mode{theme.Dark, theme.Light} {
		p := mode.Base()
		ascii := p
		ascii.Ascii = true
		seen := map[string]string{}
		for status, want := range wantGlyph {
			if got := p.StatusGlyph(status); got != want {
				t.Errorf("%s glyph is %q, want %q", status, got, want)
			}
			a := ascii.StatusGlyph(status)
			if len(a) != 1 || a[0] > 127 {
				t.Errorf("%s ASCII stand-in %q is not one ASCII cell", status, a)
			}
			if other, ok := seen[a]; ok {
				t.Errorf("%s and %s share the ASCII stand-in %q", other, status, a)
			}
			seen[a] = status
		}
	}
}

// The frame-wide fold maps each glyph to the stand-in the design names.
func TestASCIIFoldTable(t *testing.T) {
	for glyph, want := range map[string]string{
		"■": "+", "□": "_", "◌": "(", "⚑": "@", "⚠": "!", "↩": "`", "▌": ">", "▶": ">", "✕": `\`,
		"●": "#", "★": "$", "◆": "&", "⊘": "%", "▁": ".", "▂": "-", "▃": "-", "▅": "|", "▆": "|", "█": "|",
		"✎": "=", "✓": "+", "○": "_", "—": "-", "…": ".", "·": "|", "⠹": ";", "⠋": ";", "▲": "!", "Δ": "'",
		"◉": "{", "╭": "-", "┼": "|", "↑": "^",
	} {
		if got := theme.ASCIIFold(glyph, true); got != want {
			t.Errorf("%q folds to %q, want %q", glyph, got, want)
		}
	}
}

// One meaning, one stand-in: every glyph in the `?` legend folds to an ASCII character no
// other legend glyph folds to, and none folds to the separator's or the ellipsis's
// stand-in, so ASCII mode never draws two meanings alike.
func TestLegendGlyphsFoldToDistinctASCII(t *testing.T) {
	reserved := map[string]string{
		theme.ASCIIFold(theme.GlyphSep, true):      "the separator",
		theme.ASCIIFold(theme.GlyphEllipsis, true): "the ellipsis",
	}
	byASCII := map[string]string{}
	for _, g := range theme.Legend() {
		a := theme.ASCIIFold(g.Mark, true)
		if a != g.ASCII || len(a) != 1 {
			t.Errorf("%s %q folds to %q, want its one-cell stand-in %q", g.Meaning, g.Mark, a, g.ASCII)
		}
		if what, ok := reserved[a]; ok {
			t.Errorf("%s %q folds to %q, the stand-in of %s", g.Meaning, g.Mark, a, what)
		}
		// A glyph that repeats another's meaning (the ready meter's cells) takes its
		// stand-in; no other two share one.
		meaning := g.Mark
		if shared := theme.SharedMeaning(g.Mark); shared != "" {
			meaning = shared
		}
		if other, ok := byASCII[a]; ok && other != meaning {
			t.Errorf("%q and %q both fold to %q", other, g.Mark, a)
		}
		byASCII[a] = meaning
	}
	// No legend glyph is also the separator.
	for _, g := range theme.Legend() {
		if g.Mark == theme.GlyphSep {
			t.Errorf("%s is drawn with the separator %q", g.Meaning, g.Mark)
		}
	}
}

// Every non-ASCII character in a string literal the theme or the UI draws has an ASCII
// fold, so ROLECALL_ASCII output never carries a rune a limited font could show as tofu.
func TestEveryDrawnGlyphHasAnASCIIFold(t *testing.T) {
	missing := map[rune]string{}
	for _, dir := range []string{"..", "../.."} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				for _, r := range s {
					if r < 128 {
						continue
					}
					folded := theme.ASCIIFold(string(r), true)
					if folded == "" || strings.IndexFunc(folded, func(c rune) bool { return c > 127 }) >= 0 {
						missing[r] = file
					}
				}
				return true
			})
		}
	}
	for r, file := range missing {
		t.Errorf("%q (U+%04X) in %s has no ASCII fold", r, r, filepath.Base(file))
	}
}

// No legend stand-in is a letter, a digit or a punctuation key (/ search, ? help,
// : commands, , settings, * mark, [ ] Settings sections), so in the Move pop-up
// ("r  \ Rejected"), the `?` help (which lists keys and glyphs together) and beside the
// key line a stand-in never reads as a key; and no stand-in is an arrow's: "<" and ">"
// belong to ← → ‹ › and the gutter, "^" and "v" to ↑ ↓.
func TestLegendStandInsAreNeverKeys(t *testing.T) {
	keys := "/?:,*[]<>^"
	for _, g := range theme.Legend() {
		c := rune(g.ASCII[0])
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune(keys, c) {
			t.Errorf("%s stands in as %q, which reads as a key or an arrow", g.Meaning, g.ASCII)
		}
	}
	for _, s := range []string{"cut…", "a · b"} {
		if got := theme.ASCIIFold(s, true); strings.HasSuffix(got, ">") {
			t.Errorf("%q folds to %q: cut text ends in an arrow", s, got)
		}
	}
}

// No legend stand-in is also chrome's: a rule's corner, the separator, an arrow key (↑ is
// ^ in "^v select"), the sparkline or a bar never draws a character that means something.
// The not-needed dash is the one exception: an em dash is a dash in ASCII.
func TestLegendStandInsAreNeverChrome(t *testing.T) {
	chrome := map[string]bool{}
	for _, a := range theme.ChromeASCII() {
		chrome[a] = true
	}
	for _, g := range theme.Legend() {
		if g.Mark == theme.GlyphNone {
			continue
		}
		if chrome[g.ASCII] {
			t.Errorf("%s stands in as %q, which chrome also draws", g.Meaning, g.ASCII)
		}
	}
}
