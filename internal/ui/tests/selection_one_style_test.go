package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

func heldNow() time.Time { return fixtureNow }

// markedLines are the lines of a view that carry mark, after stripping style.
func markedLines(view, mark string) []string {
	var out []string
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(ansi.Strip(l), mark) {
			out = append(out, l)
		}
	}
	return out
}

// One selection style everywhere: at every size, each selectable surface shows exactly
// one marked line, and it follows the cursor.
func TestOneSelectionMarkInEverySurface(t *testing.T) {
	surfaces := map[string][]string{
		"list":     {"1"},
		"move":     {"1", "m"},
		"palette":  {"1", ":"},
		"settings": {"6", "tab"},
		"profile":  {"5"},
	}
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		for name, keys := range surfaces {
			if name == "profile" && size[0] < 80 {
				continue // the section list is hidden; the context row names the section instead
			}
			t.Run(fmt.Sprintf("%s/%dx%d", name, size[0], size[1]), func(t *testing.T) {
				m := driveStart(t, fixtureCopy(t, nil), heldNow, size[0], size[1])
				at := driveModel(t, m, keys...).View().Content
				if got := markedLines(at, "▌"); len(got) != 1 {
					t.Fatalf("want exactly one selected line, got %d:\n%s", len(got), ansi.Strip(at))
				}
				moved := driveModel(t, m, append(append([]string{}, keys...), "down")...).View().Content
				if a, b := markedLines(at, "▌")[0], markedLines(moved, "▌")[0]; ansi.Strip(a) == ansi.Strip(b) {
					t.Fatalf("the mark did not follow the cursor: %q", ansi.Strip(a))
				}
			})
		}
	}
}

func TestSelectionMarkIsAGreaterThanInASCIIAndSurvivesNoColor(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	t.Setenv("NO_COLOR", "1")
	for _, keys := range [][]string{{"1"}, {"1", "m"}, {"1", ":"}, {"6", "tab"}} {
		m := driveStart(t, fixtureCopy(t, nil), heldNow, 100, 30)
		v := driveModel(t, m, keys...).View().Content
		if strings.Contains(v, "▌") || len(markedLines(v, "> ")) == 0 {
			t.Fatalf("keys %q: ASCII selection mark missing:\n%s", keys, ansi.Strip(v))
		}
	}
}

// A pane without focus keeps the mark but drops the fill.
func TestUnfocusedListRowKeepsTheMarkWithoutTheFill(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 80, 24)
	focused := markedLines(driveModel(t, m, "1").View().Content, "▌")
	if len(focused) != 1 || !strings.Contains(focused[0], selectionBg) {
		t.Fatalf("a focused list row is filled: %q", focused)
	}
	sheet := markedLines(driveModel(t, m, "1", "enter").View().Content, "▌")
	if len(sheet) == 0 {
		t.Fatal("the list lost its selection mark under the sheet")
	}
	if strings.Contains(sheet[0], selectionBg) {
		t.Fatalf("an unfocused row kept the fill: %q", sheet[0])
	}
}
