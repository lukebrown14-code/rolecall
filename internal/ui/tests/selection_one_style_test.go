package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func heldNow() time.Time { return fixtureNow }

// markedLines are the lines of a view that carry mark, after stripping style.
func markedLines(view, mark string) []string {
	var out []string
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(chipped(l), mark) {
			out = append(out, l)
		}
	}
	return out
}

// markedRows is markedLines with each run of adjacent marked lines folded to its first:
// a list row is two lines, both under the bar, but it is one selection.
func markedRows(view, mark string) []string {
	var out []string
	prev := false
	for _, l := range strings.Split(view, "\n") {
		on := strings.Contains(chipped(l), mark)
		if on && !prev {
			out = append(out, l)
		}
		prev = on
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
		"settings": {"6"},
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
				if got := markedRows(at, "▌"); len(got) != 1 {
					t.Fatalf("want exactly one selected line, got %d:\n%s", len(got), chipped(at))
				}
				moved := driveModel(t, m, append(append([]string{}, keys...), "down")...).View().Content
				if a, b := markedRows(at, "▌")[0], markedRows(moved, "▌")[0]; chipped(a) == chipped(b) {
					t.Fatalf("the mark did not follow the cursor: %q", chipped(a))
				}
			})
		}
	}
}

func TestSelectionMarkIsAGreaterThanInASCIIAndSurvivesNoColor(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	t.Setenv("NO_COLOR", "1")
	for _, keys := range [][]string{{"1"}, {"1", "m"}, {"1", ":"}, {"6"}} {
		m := driveStart(t, fixtureCopy(t, nil), heldNow, 100, 30)
		v := driveModel(t, m, keys...).View().Content
		if strings.Contains(v, "▌") || len(markedLines(v, "> ")) == 0 {
			t.Fatalf("keys %q: ASCII selection mark missing:\n%s", keys, chipped(v))
		}
	}
}

// A pane without focus keeps the mark but drops the fill.
func TestUnfocusedListRowKeepsTheMarkWithoutTheFill(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), heldNow, 80, 24)
	focused := markedRows(driveModel(t, m, "1").View().Content, "▌")
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
