package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		`company: "Co\e[2J\u009b31m"`+"\ntrack: SWE\nstatus_log: []\n")
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
	root := withEmptyMeta(t, "title: \"  \"\ncompany: \"\"\ntrack: SWE\nstatus_log: []\n")
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
