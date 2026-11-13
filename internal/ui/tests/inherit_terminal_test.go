package ui_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

var sgrSeq = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// ansi16Only fails on any SGR parameter other than reset, bold/faint, underline, reverse,
// an ANSI-16 foreground or the default foreground.
func ansi16Only(t *testing.T, view string) {
	t.Helper()
	for _, m := range sgrSeq.FindAllStringSubmatch(view, -1) {
		if m[1] == "" {
			continue
		}
		for _, p := range strings.Split(m[1], ";") {
			switch {
			case p == "0", p == "1", p == "2", p == "3", p == "4", p == "7", p == "22", p == "24", p == "27", p == "39":
			case len(p) == 2 && (p[0] == '3' && p[1] <= '7' || p[0] == '9' && p[1] <= '7'):
			default:
				t.Fatalf("inherit mode drew SGR parameter %q in %q", p, m[0])
			}
		}
	}
}

func hasReverse(view string) bool {
	for _, m := range sgrSeq.FindAllStringSubmatch(view, -1) {
		for _, p := range strings.Split(m[1], ";") {
			if p == "7" {
				return true
			}
		}
	}
	return false
}

func TestInheritTerminalOptionPaintsOnlyANSI16(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, w := range []int{120, 80} {
		d := newDriver(t, root, fixtureClock, w, 36, ui.WithInheritTerminal(true))
		ansi16Only(t, d.View())
		if !hasReverse(d.View()) {
			t.Fatalf("selection shows no reverse video at %d:\n%q", w, d.View())
		}
		if !strings.Contains(d.plain(), "▌") {
			t.Fatalf("selection gutter missing at %d:\n%s", w, d.plain())
		}
		d.Type("?")
		ansi16Only(t, d.View())
		d.Type("esc")
		d.Type("space")
		ansi16Only(t, d.View())
	}
}

func TestInheritTerminalOffKeepsTruecolor(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	if !strings.Contains(d.View(), "38;2;") || !strings.Contains(d.View(), "48;2;") {
		t.Fatalf("default output lost its truecolor:\n%q", d.View())
	}
}

func TestInheritTerminalNoColorWins(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 36, ui.WithInheritTerminal(true))
	for _, m := range sgrSeq.FindAllStringSubmatch(d.View(), -1) {
		for _, p := range strings.Split(m[1], ";") {
			if len(p) == 2 && (p[0] == '3' || p[0] == '9') && p[1] <= '7' {
				t.Fatalf("NO_COLOR left colour %q", m[0])
			}
		}
	}
}

func TestSettingsInheritToggleSavesAndAppliesAtOnce(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, 3, 5)
	requireAll(t, d.plain(), "Inherit colours", "No")
	d.Type("enter")
	d.Type("ctrl+s")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "inherit_terminal = true\n") {
		t.Fatalf("config.toml:\n%s", b)
	}
	ansi16Only(t, d.View())
	d.Type("enter")
	d.Type("ctrl+s")
	if !strings.Contains(d.View(), "38;2;") {
		t.Fatalf("turning it off did not restore the palette")
	}
}
