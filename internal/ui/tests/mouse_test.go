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
	lines := strings.Split(chipped(m.View().Content), "\n")
	i := strings.Index(lines[line], text)
	if i < 0 {
		t.Fatalf("line %d lacks %q: %q", line, text, lines[line])
	}
	return runMsg(t, m, tea.MouseClickMsg{X: ansi.StringWidth(lines[line][:i]), Y: line, Button: tea.MouseLeft})
}

// lineOf is the index of the first frame line holding text.
func lineOf(t *testing.T, m tea.Model, text string) int {
	t.Helper()
	for i, l := range strings.Split(chipped(m.View().Content), "\n") {
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
		if view := chipped(m.View().Content); !strings.Contains(view, "Applied › All") {
			t.Fatalf("%v: tab click did not open Applied:\n%s", size, view)
		}
		if size[0] < 80 {
			continue // narrow, the filters collapse to s:/t: and the view tabs go with tab
		}
		m = clickOn(t, m, lineOf(t, m, "Interviewing 2"), "Interviewing")
		if view := chipped(m.View().Content); !strings.Contains(view, "Applied › Interviewing") {
			t.Fatalf("%v: view click did not switch:\n%s", size, view)
		}
		m = clickOn(t, m, lineOf(t, m, "TRACK t"), "SWE")
		if view := chipped(m.View().Content); !strings.Contains(view, "Applied › Interviewing · SWE") {
			t.Fatalf("%v: track click did not filter:\n%s", size, view)
		}
	}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = clickOn(t, m, lineOf(t, m, "Empty log"), "Empty log")
	if view := chipped(m.View().Content); !strings.Contains(view, "▌ ◌ Empty log") {
		t.Fatalf("row click did not move the cursor:\n%s", view)
	}
}

// In ASCII mode the frame draws - and x where the Unicode frame drew ─ and ×, so a
// click must target the same characters the renderer drew.
func TestClickTargetsAsciiGlyphs(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "3")
	m = clickOn(t, m, lineOf(t, m, "Interviewing 2"), "Interviewing")
	if view := chipped(m.View().Content); !strings.Contains(view, "Applied > Interviewing") {
		t.Fatalf("ASCII view click did not switch:\n%s", view)
	}
	m = driveModel(t, m, "/pl", "enter")
	m = clickOn(t, m, lineOf(t, m, "/pl x"), "x")
	if view := chipped(m.View().Content); strings.Contains(view, "/pl x") {
		t.Fatalf("ASCII chip click left the search on:\n%s", view)
	}
}

func TestWheelMovesTheCursorAndClickClearsTheSearchChip(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = runMsg(t, m, tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelDown})
	if view := chipped(m.View().Content); !strings.Contains(view, "▌ ◌ Empty log") {
		t.Fatalf("wheel down did not move the cursor:\n%s", view)
	}
	m = runMsg(t, m, tea.MouseWheelMsg{X: 5, Y: 6, Button: tea.MouseWheelUp})
	if view := chipped(m.View().Content); !strings.Contains(view, "▌ ◌ Empty\n") && !strings.Contains(view, "▌ ◌ Empty ") {
		t.Fatalf("wheel up did not move the cursor back:\n%s", view)
	}
	m = driveModel(t, m, "/pl", "enter")
	m = clickOn(t, m, lineOf(t, m, "/pl ×"), "×")
	if view := chipped(m.View().Content); strings.Contains(view, "/pl ×") {
		t.Fatalf("clicking × left the search on:\n%s", view)
	}
}
