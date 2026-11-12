package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// clickOn clicks the first cell of text on the given line of the current frame.
func clickOn(t *testing.T, m tea.Model, line int, text string) tea.Model {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	i := strings.Index(lines[line], text)
	if i < 0 {
		t.Fatalf("line %d lacks %q: %q", line, text, lines[line])
	}
	return runMsg(t, m, tea.MouseClickMsg{X: ansi.StringWidth(lines[line][:i]), Y: line, Button: tea.MouseLeft})
}

// lineOf is the index of the first frame line holding text.
func lineOf(t *testing.T, m tea.Model, text string) int {
	t.Helper()
	for i, l := range strings.Split(ansi.Strip(m.View().Content), "\n") {
		if strings.Contains(l, text) {
			return i
		}
	}
	t.Fatalf("frame lacks %q", text)
	return -1
}

func TestClickSwitchesScreensViewsAndRows(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {60, 16}} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, size[0], size[1])
		label := "3 Applied"
		if size[0] < 80 {
			label = " 3 " // a narrow tab bar keeps only the other screens' digits
		}
		m = clickOn(t, m, 0, label)
		if view := ansi.Strip(m.View().Content); !strings.Contains(view, "[All") {
			t.Fatalf("%v: tab click did not open Applied:\n%s", size, view)
		}
		m = clickOn(t, m, lineOf(t, m, "Interviewing"), "Interviewing")
		if view := ansi.Strip(m.View().Content); !strings.Contains(view, "[Interviewing") {
			t.Fatalf("%v: view click did not switch:\n%s", size, view)
		}
	}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = clickOn(t, m, lineOf(t, m, "Empty log"), "Empty log")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "▌ ◌ Empty log") {
		t.Fatalf("row click did not move the cursor:\n%s", view)
	}
}

func TestWheelMovesTheCursorAndClickClearsTheSearchChip(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = runMsg(t, m, tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "▌ ◌ Empty log") {
		t.Fatalf("wheel down did not move the cursor:\n%s", view)
	}
	m = runMsg(t, m, tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelUp})
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "▌ ◌ Empty\n") && !strings.Contains(view, "▌ ◌ Empty ") {
		t.Fatalf("wheel up did not move the cursor back:\n%s", view)
	}
	m = driveModel(t, m, "/pl", "enter")
	m = clickOn(t, m, lineOf(t, m, "/pl ×"), "×")
	if view := ansi.Strip(m.View().Content); strings.Contains(view, "/pl ×") {
		t.Fatalf("clicking × left the search on:\n%s", view)
	}
}
