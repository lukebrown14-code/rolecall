package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// withEmptyMeta rewrites the fixture's first Lead row ("Empty") with the given meta.yml.
func withEmptyMeta(t *testing.T, meta string) string {
	t.Helper()
	return fixtureCopy(t, func(root string) {
		if err := os.WriteFile(filepath.Join(root, "jobs", "SWE", "empty", "meta.yml"), []byte(meta), 0o600); err != nil {
			t.Fatal(err)
		}
	})
}

// A scraped Title or Company can carry an OSC (retitle the window, write the clipboard),
// a CSI (clear the screen), a bell, tabs and line breaks. None reaches the terminal, and
// the frame keeps its height in the list and in Detail.
func TestHostileMetaStringsNeverReachTheTerminal(t *testing.T) {
	root := withEmptyMeta(t, `title: "Evil\u001b]0;PWNED\u0007 title\there and\nnewline"`+"\n"+
		`company: "Co\e[2J\u009b31m"`+"\ntrack: SWE\ncloses: 2026-10-02\nstatus_log: []\n") // closes first, so it is the first row
	for _, size := range [][2]int{{80, 24}, {120, 40}} {
		d := newDriver(t, root, fixtureClock, size[0], size[1])
		for _, step := range []string{"list", "enter"} {
			if step != "list" {
				d.Type(step)
			}
			view := d.View()
			for _, bad := range []string{"\x1b]", "\x07", "\x1b[2J", "\u009b", "\t"} {
				if strings.Contains(view, bad) {
					t.Fatalf("%dx%d %s: view carries %q", size[0], size[1], step, bad)
				}
			}
			if lines := strings.Count(view, "\n") + 1; lines != size[1] {
				t.Fatalf("%dx%d %s: %d lines, want %d:\n%s", size[0], size[1], step, lines, size[1], d.plain())
			}
			requireAll(t, d.plain(), "Evil]0;PWNED title here and")
		}
	}
}

// A blank Title reads "(untitled)" in the row, the preview and the Detail header alike,
// and a blank Company leaves no separator hanging.
func TestBlankTitleAndCompanyReadTheSameEverywhere(t *testing.T) {
	root := withEmptyMeta(t, "title: \"  \"\ncompany: \"\"\ntrack: SWE\ncloses: 2026-10-02\nstatus_log: []\n") // the first row
	d := newDriver(t, root, fixtureClock, 120, 40)
	row := rowWith(strings.Split(d.plain(), "\n"), "▌")
	if !strings.Contains(row, "(untitled)") {
		t.Fatalf("row: %q\n%s", row, d.plain())
	}
	if next := rowWith(strings.Split(d.plain(), "\n"), "▌   SWE"); next == "" {
		t.Fatalf("second line should start with the Track, no leading separator:\n%s", d.plain())
	}
	d.Type("enter")
	header := rowWith(strings.Split(d.plain(), "\n"), "‹ esc")
	if !strings.Contains(header, "(untitled)") || strings.Contains(header, "(untitled) ·") {
		t.Fatalf("header: %q", header)
	}
}

// hostileMarkdown carries an OSC 0 (retitle the window), OSC 52 (write the clipboard), an
// OSC 8 hyperlink, a CSI that clears the screen, a raw C1 CSI, a bell, an RTL override and
// a zero-width space.
const hostileMarkdown = "# Role\n\nEvil \x1b]0;PWNED\x07 and \x1b]52;c;SGVsbG8=\x07 clip " +
	"\x1b]8;;https://evil.test\x1b\\link\x1b]8;;\x1b\\ \x1b[2Jclear \u009b31m\u202eRTL\u200bZW\n"

// hostileSequences are what hostileMarkdown must never put on the screen.
var hostileSequences = []string{"\x1b]0;", "\x1b]52;", "\x1b]8;", "\x1b[2J", "\u009b", "\x07", "\u202e", "\u200b"}

func requireNoHostileSequences(t *testing.T, where, view string) {
	t.Helper()
	for _, bad := range hostileSequences {
		if strings.Contains(view, bad) {
			t.Fatalf("%s: view carries %q", where, bad)
		}
	}
}

// jd.md, fit.md and notes.md are drawn through Glamour, notes.md also in the preview, and
// the Profile files on the Profile screen; an escape sequence in any of them is dropped
// before it is rendered, and the text around it stays.
func TestHostileMarkdownNeverReachesTheTerminal(t *testing.T) {
	setup := func(root string) {
		fern := filepath.Join(root, "jobs", "SWE", "fern")
		for _, name := range []string{"jd.md", "fit.md", "notes.md"} {
			if err := os.WriteFile(filepath.Join(fern, name), []byte(hostileMarkdown), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		for _, name := range []string{"master-profile.md", "projects.md"} {
			path := filepath.Join(root, "profile", name)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(b, []byte("\n"+hostileMarkdown)...), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	h := newDetailHarness(t, setup)
	h.tm.Type("1")
	h.tm.Type("/Fern")
	h.key(tea.KeyEnter)
	requireNoHostileSequences(t, "list preview", h.tm.View())
	h.key(tea.KeyEnter)
	for _, tab := range []string{"jd", "fit", "notes"} {
		requireNoHostileSequences(t, tab, h.tm.View())
		h.show("Evil", "clip", "RTLZW")
		h.key(tea.KeyRight)
	}
	root := fixtureCopy(t, setup)
	for _, keys := range [][]string{{"5"}, {"5", "G"}, profileKeysToFile} {
		requireNoHostileSequences(t, strings.Join(keys, " "), runKeys(t, root, 120, keys...))
	}
	if view := chipped(runKeys(t, root, 120, profileKeysToFile...)); !strings.Contains(view, "RTLZW") {
		t.Fatalf("projects.md lost its text:\n%s", view)
	}
}
