package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// sectionOf is the Help section a registry group is drawn under.
func sectionOf(g helpGroup) string {
	switch g {
	case groupAction:
		return "ACT"
	case groupFilter:
		return "FILTER & FIND"
	default:
		return "NAVIGATE"
	}
}

// In every context no key is declared twice, and every registry entry appears in Help as
// a chip under the section its group belongs to.
func TestKeymapHasNoDuplicatesAndHelpListsEveryEntryUnderItsSection(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys []string
	}{
		{"leads", []string{"1"}}, {"dismissed", []string{"1", "s"}}, {"drafts", []string{"2"}},
		{"applied", []string{"3"}}, {"detail", []string{"3", "enter"}}, {"stats", []string{"4"}},
		{"profile", []string{"5"}}, {"settings", []string{"6"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureModel(t)
			m.width, m.height = 60, 300
			for _, k := range tc.keys {
				code := []rune(k)[0]
				msg := tea.KeyPressMsg{Code: code, Text: k}
				if k == "enter" {
					msg = tea.KeyPressMsg{Code: tea.KeyEnter}
				}
				m = pressModel(t, m, msg)
			}
			keys := m.screenKeys()
			seen := map[string]bool{}
			for _, k := range keys {
				label := k.Help().Key
				if seen[label] {
					t.Errorf("key %q is declared twice in this context", label)
				}
				seen[label] = true
			}
			m.overlay = &helpMode{}
			lines := strings.Split(xansi.Strip(m.helpBox(m.pal())), "\n")
			for _, k := range keys {
				h := k.Help()
				chip := "[" + h.Key + "]"
				want := sectionOf(k.group)
				section, found := "", false
				for _, l := range lines {
					switch {
					case strings.Contains(l, "NAVIGATE"), strings.Contains(l, "ACT") && strings.Contains(l, "│  ACT"),
						strings.Contains(l, "FILTER & FIND"):
						section = strings.TrimSpace(strings.Trim(l, "│ "))
					case strings.Contains(l, chip):
						found = found || section == want
					}
				}
				if !found {
					t.Errorf("Help does not list %s %q under %s", chip, h.Desc, want)
				}
			}
		})
	}
}
