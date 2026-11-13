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
		"Rejected": "✕", "Withdrawn": "–", "Dismissed": "⊘",
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
		"■": "#", "□": "-", "◌": "o", "⚑": "!", "↩": "<", "▌": ">", "▶": ">", "✕": "x", "◦": "o",
		"●": "*", "★": "*", "◆": "@", "▁": "_", "▃": ".", "▅": "-", "▆": "=", "█": "#", "–": "-", "✎": "=",
	} {
		if got := theme.ASCIIFold(glyph, true); got != want {
			t.Errorf("%q folds to %q, want %q", glyph, got, want)
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
