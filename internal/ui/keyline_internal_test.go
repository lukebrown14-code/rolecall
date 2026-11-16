package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The key line keeps only the core keys even with room to spare; the rest are in `?`.
func TestKeyLineShowsOnlyCoreKeys(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 200, 40
	m.screen = 2
	line := ansi.Strip(m.keyLine(m.pal(), m.width))
	t.Log(line)
	for _, want := range []string{"enter", "?"} {
		if !strings.Contains(line, want) {
			t.Errorf("key line %q lacks %q", line, want)
		}
	}
	for _, gone := range []string{"rescan", "quit", "1-6"} {
		if strings.Contains(line, gone) {
			t.Errorf("key line %q still shows %q", line, gone)
		}
	}
}
