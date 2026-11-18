package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// The help's AI skills section names every skill the Plugin ships, and nothing else.
func TestAISkillsNameEveryPluginSkill(t *testing.T) {
	dirs, err := os.ReadDir(filepath.Join("..", "..", "plugin", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	var shipped, named []string
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join("..", "..", "plugin", "skills", d.Name(), "SKILL.md")); err == nil {
			shipped = append(shipped, d.Name())
		}
	}
	for _, s := range aiSkills {
		named = append(named, s.name)
		if s.key != "" && !slices.ContainsFunc(listActions, func(a listAction) bool { return a.keyLabel == s.key && a.needsHarness }) {
			t.Errorf("%s is handed off by %q, which is no Harness key", s.name, s.key)
		}
	}
	slices.Sort(shipped)
	slices.Sort(named)
	if !slices.Equal(shipped, named) {
		t.Fatalf("help names skills %q, the Plugin ships %q", named, shipped)
	}
}

// One name per key: a key the help and the command finder both offer has the same words in
// each.
func TestHelpAndPaletteNameAKeyAlike(t *testing.T) {
	for _, keys := range [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}} {
		m := fixtureModel(t)
		m.width, m.height = 120, 40
		for _, k := range keys {
			m = pressModel(t, m, tea.KeyPressMsg{Code: []rune(k)[0], Text: k})
		}
		names := map[string]string{}
		for _, k := range m.screenKeys() {
			if _, ok := names[k.Help().Key]; !ok {
				names[k.Help().Key] = k.Help().Desc
			}
		}
		for _, k := range m.openPalette().items {
			if want, ok := names[k.Help().Key]; ok && k.Help().Desc != want {
				t.Errorf("%v: palette names %q %q, help %q", keys, k.Help().Key, k.Help().Desc, want)
			}
		}
	}
}

// helpContexts are the keys that open each context the help is measured in: every
// screen, and a detail view on a list, on Applied, on Profile and on Settings.
var helpContexts = []string{"1", "2", "3", "4", "5", "6", "1\r", "3\r", "5\r", "6\r"}

// helpAt opens the help at w by h in the context keys reach ("\r" is enter).
func helpAt(t *testing.T, w, h int, keys string) (model, *helpMode) {
	t.Helper()
	m := fixtureModel(t)
	m.width, m.height = w, h
	for _, k := range keys {
		msg := tea.KeyPressMsg{Code: k, Text: string(k)}
		if k == '\r' {
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		}
		m = pressModel(t, m, msg)
	}
	hm := &helpMode{}
	m.overlay = hm
	m.helpView(m.pal())
	return m, hm
}

// helpBody is the open help page's lines as drawn: the full-screen help's lines above its
// footer, or the pop-up's lines between its borders.
func helpBody(m model) []string {
	lines := strings.Split(xansi.Strip(m.helpView(m.pal())), "\n")
	if m.helpFullScreen() {
		return lines[:len(lines)-1]
	}
	var body []string
	for _, l := range lines[1 : len(lines)-1] {
		body = append(body, strings.Trim(l, "│"))
	}
	return body
}

// Paging never leaves a heading as a page's last line, at every common size and in every
// context.
func TestHelpPagesNeverEndOnAHeading(t *testing.T) {
	for _, sz := range [][2]int{{166, 44}, {120, 36}, {100, 30}, {90, 30}, {80, 24}, {72, 24}, {64, 24}, {60, 16}, {56, 14}} {
		for _, keys := range helpContexts {
			m, hm := helpAt(t, sz[0], sz[1], keys)
			for page := range hm.pages {
				hm.page = page
				var last string
				for _, l := range helpBody(m) {
					if body := strings.TrimSpace(l); body != "" {
						last = body
					}
				}
				if last != "" && last == strings.ToUpper(last) && strings.ContainsAny(last, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
					t.Errorf("%dx%d keys %q page %d ends on the heading %q", sz[0], sz[1], keys, page+1, last)
				}
			}
		}
	}
}

// The help fits in a page or two at common sizes, 80x24 included (the skills with no key
// share one line, so the ten Plugin skills cost three rows), and in six on the smallest
// terminal rolecall runs in: below wide it is a screen of its own rather than a pop-up
// between the frame's rules. The core Words come last and may take one page more where
// they do not fit beside the rest, so the keys, glyphs and row words keep their pages.
func TestHelpPageCounts(t *testing.T) {
	for _, tc := range []struct{ w, h, most int }{{166, 44, 2}, {120, 36, 2}, {90, 30, 2}, {80, 24, 2}, {72, 24, 2}, {56, 14, 6}} {
		for _, keys := range helpContexts {
			m, hm := helpAt(t, tc.w, tc.h, keys)
			most := tc.most
			hm.page = hm.pages - 1
			if last := strings.Join(helpBody(m), "\n"); strings.Contains(last, "WORDS") && !strings.Contains(last, "ROW WORDS") &&
				!strings.Contains(last, "GLYPHS") {
				most++ // a page of the core Words alone
			}
			if hm.pages > most {
				t.Errorf("%dx%d keys %q: help is %d pages, want at most %d", tc.w, tc.h, keys, hm.pages, most)
			}
		}
	}
}

// ? defines each core word, in every context, on one of its pages.
func TestHelpDefinesTheCoreWords(t *testing.T) {
	for _, keys := range helpContexts {
		m, hm := helpAt(t, 120, 36, keys)
		var all []string
		for page := range hm.pages {
			hm.page = page
			all = append(all, helpBody(m)...)
		}
		text := strings.Join(all, "\n")
		for _, w := range []string{"Lead", "Draft", "Track", "Harness", "Ready", "Filtered"} {
			if !regexp.MustCompile(`(?m)\b` + w + `\s{2,}\S`).MatchString(text) {
				t.Errorf("keys %q: the help does not define %s:\n%s", keys, w, text)
			}
		}
	}
}

// Every page of the full-screen help is the terminal's height and no line reaches its
// last column, so the footer stays on the last line as it pages.
func TestFullScreenHelpFitsTheTerminal(t *testing.T) {
	for _, sz := range [][2]int{{90, 30}, {72, 24}, {56, 14}} {
		for _, keys := range helpContexts {
			m, hm := helpAt(t, sz[0], sz[1], keys)
			for page := range hm.pages {
				hm.page = page
				lines := strings.Split(xansi.Strip(m.helpView(m.pal())), "\n")
				if len(lines) != sz[1] {
					t.Errorf("%dx%d keys %q page %d is %d lines", sz[0], sz[1], keys, page+1, len(lines))
				}
				for i, l := range lines {
					if xansi.StringWidth(strings.TrimRight(l, " ")) > sz[0]-1 {
						t.Errorf("%dx%d keys %q page %d line %d reaches the last column: %q", sz[0], sz[1], keys, page+1, i, l)
					}
				}
			}
		}
	}
}
