package ui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
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

// hasAll reports whether every one of want is in have.
func hasAll(have, want []string) bool {
	for _, w := range want {
		if !slices.Contains(have, w) {
			return false
		}
	}
	return true
}

// In every context no key is declared twice, and every registry entry appears in Help,
// bare, under the section its group belongs to.
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
			// One column, so each line belongs to the heading above it.
			var lines []string
			for _, l := range keyBlock(m.pal(), append(keys, m.dormantKeys()...), 30, false) {
				lines = append(lines, strings.TrimSpace(xansi.Strip(l.text)))
			}
			for _, k := range keys {
				h := k.Help()
				want := sectionOf(k.group)
				desc, _, _ := strings.Cut(h.Desc, " "+needsHarnessNote)
				section, found := "", false
				for _, l := range lines {
					switch l {
					case "NAVIGATE", "ACT", "FILTER & FIND":
						section = l
						continue
					}
					// The key bare, alone or with an alias (": ctrl+k"), then its words.
					at := strings.Index(l, desc)
					if at > 0 && strings.HasSuffix(l, desc) && hasAll(strings.Fields(l[:at]), strings.Fields(h.Key)) {
						found = found || section == want
					}
				}
				if !found {
					t.Errorf("Help does not list %q %q under %s", h.Key, h.Desc, want)
				}
			}
		})
	}
}

// settingsKeysEverywhere is every key the Settings key line, help and palette offer on
// any row of any group, by key.
func settingsKeysEverywhere(t *testing.T, m model) map[string]string {
	t.Helper()
	m = pressModel(t, m, tea.KeyPressMsg{Code: '6', Text: "6"})
	seen := map[string]string{}
	for range settings.Groups {
		for range 12 {
			for _, k := range m.contextKeys() {
				seen[k.Help().Key] = k.Help().Desc
			}
			m = pressModel(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
		}
		m = pressModel(t, m, tea.KeyPressMsg{Code: ']', Text: "]"})
	}
	return seen
}

// No Settings key shadows a global key with another meaning: t is the Track filter and
// r rescan on every screen, so Settings checks a Source with c and renames a Track with
// enter.
func TestSettingsKeysDoNotShadowGlobalKeys(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 140, 40
	seen := settingsKeysEverywhere(t, m)
	for _, key := range []string{"t", "T", "r", ",", "h", ":", "?", "q", "space", "1-6"} {
		if desc, ok := seen[key]; ok {
			t.Errorf("Settings offers %s %q, which shadows the global %s", key, desc, key)
		}
	}
	if seen["c"] != "check" || seen["enter"] == "" {
		t.Errorf("Settings does not offer c check and enter: %v", seen)
	}
}

// Settings names are drawn in the body text colour, not muted, and values a step quieter.
func TestSettingsNamesRenderInTheTextColour(t *testing.T) {
	m := fixtureModel(t)
	m.width, m.height = 140, 40
	m = pressModel(t, m, tea.KeyPressMsg{Code: '6', Text: "6"})
	p := m.pal()
	text, muted := p.Text(p.Fg).Render(labelEditor), p.Text(p.Dim).Render(labelEditor)
	if text == muted {
		t.Fatal("the test palette draws no colour")
	}
	m = pressModel(t, m, tea.KeyPressMsg{Code: '[', Text: "["}) // This machine; the cursor is on Editor
	m = pressModel(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"}) // move it off Editor
	view := m.settingsRowsView(p, 100, 30, false)
	if !strings.Contains(view, text) || strings.Contains(view, muted) {
		t.Fatalf("the Editor name is not in the text colour:\n%q", view)
	}
}
