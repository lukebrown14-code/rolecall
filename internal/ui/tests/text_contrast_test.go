package ui_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// sgr is one SGR escape: its parameters.
var sgr = regexp.MustCompile(`\x1b\[([0-9;:]*)m`)

// rgbParam is a hex colour as SGR truecolour parameters, "110;118;129".
func rgbParam(hex string) string {
	v, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return fmt.Sprintf("%d;%d;%d", v>>16, v>>8&0xff, v&0xff)
}

// dimText is what view draws with terminal faint (SGR 2), which a terminal renders
// at about half intensity, or in the Faint role, which is for separators and marks: both
// fall below the 4.5:1 text floor. Glyphs and rules may use Faint (3:1); letters and
// digits may not.
func dimText(view, faintRGB string) []string {
	var out []string
	fg, faint := "", false
	drawn := strings.Builder{}
	flush := func() {
		if drawn.Len() > 0 {
			out = append(out, drawn.String())
			drawn.Reset()
		}
	}
	rest := view
	for rest != "" {
		loc := sgr.FindStringSubmatchIndex(rest)
		text := rest
		if loc != nil {
			text = rest[:loc[0]]
		}
		for _, r := range text {
			word := unicode.IsLetter(r) || unicode.IsDigit(r)
			if (faint && !unicode.IsSpace(r)) || (fg == faintRGB && word) {
				drawn.WriteRune(r)
			} else {
				flush()
			}
		}
		if loc == nil {
			break
		}
		flush()
		params := strings.FieldsFunc(rest[loc[2]:loc[3]], func(r rune) bool { return r == ';' || r == ':' })
		if len(params) == 0 {
			fg, faint = "", false
		}
		for i := 0; i < len(params); i++ {
			switch params[i] {
			case "0":
				fg, faint = "", false
			case "2":
				faint = true
			case "22":
				faint = false
			case "39":
				fg = ""
			case "38", "48", "58":
				if i+1 < len(params) && params[i+1] == "2" && i+4 < len(params) {
					if params[i] == "38" {
						fg = strings.Join(params[i+2:i+5], ";")
					}
					i += 4
				} else if i+1 < len(params) && params[i+1] == "5" {
					i += 2
				}
			}
		}
		rest = rest[loc[1]:]
	}
	flush()
	return out
}

// TestNoTextIsDrawnFaint walks the screens, the detail tabs and the pop-ups in both themes
// and fails on any letter or digit drawn with terminal faint or the Faint role: the
// palette's contrast test checks the roles' hex values, so it cannot see either.
func TestNoTextIsDrawnFaint(t *testing.T) {
	places := map[string][]string{
		"leads":     {"1"},
		"applied":   {"3"},
		"interview": {"3", "G"},
		"detail":    {"3", "G", "enter"},
		"docs":      {"3", "G", "enter", "left"},
		"stats":     {"4"},
		"profile":   {"5"},
		"settings":  {"6"},
		"help":      {"1", "?"},
		"palette":   {"1", ":"},
		"menu":      {"1", " "},
	}
	for _, mode := range []struct {
		env  string
		mode theme.Mode
	}{{"dark", theme.Dark}, {"light", theme.Light}} {
		t.Run(mode.env, func(t *testing.T) {
			t.Setenv("ROLECALL_THEME", mode.env)
			t.Setenv("COLORFGBG", "")
			faint := rgbParam(mode.mode.Roles().Faint)
			root := fixtureCopy(t, nil)
			for name, keys := range places {
				m := driveModel(t, driveStart(t, root, heldNow, 120, 36), keys...)
				if got := dimText(m.View().Content, faint); len(got) > 0 {
					t.Errorf("%s: text drawn faint: %q\n%s", name, got, chipped(m.View().Content))
				}
			}
		})
	}
}
