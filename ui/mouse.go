package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// wheelKey is the key a wheel turn stands for: it moves whatever ↑↓ moves here — the
// cursor, a reader's scroll, a pop-up's selection or the help's page.
func wheelKey(b tea.MouseButton) (string, bool) {
	switch b {
	case tea.MouseWheelDown:
		return "down", true
	case tea.MouseWheelUp:
		return "up", true
	}
	return "", false
}

// mouse handles a click or a wheel turn. A click on a screen tab switches to it, on a
// view switches the view, on the search chip's × clears the search, and on a list row
// moves the cursor there. handled is false for anything else.
func (m model) mouse(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		if key, ok := wheelKey(msg.Button); ok && !m.settingsEditing() && !m.tooSmall() {
			next, cmd := m.Update(keyPress(key))
			return next, cmd, true
		}
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft || m.overlay != nil || m.settingsEditing() || m.tooSmall() {
			return m, nil, false
		}
		return m.click(msg.X, msg.Y)
	}
	return m, nil, false
}

// click acts on the frame at column x, line y.
func (m model) click(x, y int) (tea.Model, tea.Cmd, bool) {
	view := strings.Split(xansi.Strip(m.View().Content), "\n")
	if y < 0 || y >= len(view) {
		return m, nil, false
	}
	line := view[y]
	if y == 0 {
		if si, ok := m.tabAt(x); ok {
			next, cmd := m.Update(keyPress(fmt.Sprint(si + 1)))
			return next, cmd, true
		}
		return m, nil, false
	}
	if m.text != "" && chipCloseAt(line, x) {
		m.text, m.cursor = "", 0
		return m, nil, true
	}
	if !m.hasTables() || m.detail != nil {
		return m, nil, false
	}
	rule := ruleLine(view)
	if y == rule-1 {
		if v, ok := labelAt(line, x, m.views()); ok {
			m.table, m.cursor = v, 0
			return m, nil, true
		}
		return m, nil, false
	}
	if i, ok := m.rowAt(x, y-rule-1); ok {
		m.cursor = i
		return m.syncDetail(), nil, true
	}
	return m, nil, false
}

// ruleLine is the index of the frame's top rule: the first line drawn with ─.
func ruleLine(view []string) int {
	for i, l := range view {
		if strings.HasPrefix(l, "─") {
			return i
		}
	}
	return -1
}

// tabAt is the screen whose tab holds column x: the last tab starting at or before it.
func (m model) tabAt(x int) (int, bool) {
	bar, starts := m.tabs(m.pal())
	if x >= xansi.StringWidth(bar) {
		return 0, false
	}
	best := -1
	for si, col := range starts {
		if col-1 <= x {
			best = si
		}
	}
	return best, best >= 0
}

// labelAt is the index of the label holding column x on a line that lists them in order.
func labelAt(line string, x int, labels []string) (int, bool) {
	// Each label owns the columns from its bracket to the next label's.
	from, hit := 0, -1
	last := 0
	for i, l := range labels {
		col := columnOf(line[from:], l)
		if col < 0 {
			break
		}
		col += xansi.StringWidth(line[:from])
		if x >= col-1 {
			hit = i
		}
		last = col + xansi.StringWidth(l)
		from = byteIndexOfColumn(line, last)
	}
	// The last label's count and bracket follow it; the scope line further right does not count.
	if hit < 0 || (hit == len(labels)-1 && x > last+len(" 00/00]")) {
		return 0, false
	}
	return hit, true
}

// chipCloseAt reports whether column x is the search chip's ×.
func chipCloseAt(line string, x int) bool {
	col := columnOf(line, "×")
	return col >= 0 && x >= col-1 && x <= col+1
}

// columnOf is the terminal column where sub starts in line, or -1.
func columnOf(line, sub string) int {
	i := strings.Index(line, sub)
	if i < 0 {
		return -1
	}
	return xansi.StringWidth(line[:i])
}

// byteIndexOfColumn is the byte offset in line of terminal column col.
func byteIndexOfColumn(line string, col int) int {
	w := 0
	for i, r := range line {
		if w >= col {
			return i
		}
		w += xansi.StringWidth(string(r))
	}
	return len(line)
}

// rowAt is the row drawn at column x and body line y of the list, if any.
func (m model) rowAt(x, y int) (int, bool) {
	rows := m.rows()
	listW, _ := m.listPane(len(rows))
	if len(rows) == 0 || x >= listW || y < 0 {
		return 0, false
	}
	h := m.layout().bodyHeight(1)
	blocks, cur := m.listBlocks(m.pal(), rows, listW-gutter, h)
	start, end := scrollWindow(blocks, cur, h)
	line := 0
	if start > 0 {
		line = 1 // "↑ n more"
	}
	for _, b := range blocks[start:end] {
		if y < line+len(b.lines) {
			return b.idx, b.row
		}
		line += len(b.lines)
	}
	return 0, false
}
