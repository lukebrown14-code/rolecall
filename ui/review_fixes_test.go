package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// The Move pop-up moves the Application it shows, even when a live refresh moved the
// cursor to another row while it was open.
func TestMoveActsOnTheApplicationItShows(t *testing.T) {
	h := newLiveHarness(t, nil)
	h.tm.Type("j") // Empty log
	h.tm.Type("m")
	h.wait("Move")
	meta := filepath.Join(h.folder("empty-log"), "meta.yml")
	b, err := os.ReadFile(meta)
	if err != nil {
		t.Fatal(err)
	}
	h.write(meta, strings.Replace(string(b), "status_log: []", "status_log:\n  - {status: Draft, at: 2026-10-01T01:00:00Z}", 1))
	h.wait("Draft 3") // Empty log left Leads; the cursor now sits on another row
	h.tm.Type("x")
	h.wait("→ Dismissed")
	s, err := store.Open(h.root)
	if err != nil {
		t.Fatal(err)
	}
	status := func(name string) string {
		a, err := s.Rescan(h.folder(name))
		if err != nil || a == nil {
			t.Fatalf("rescan %s: %v", name, err)
		}
		return a.Status
	}
	if got := status("empty-log"); got != "Dismissed" {
		t.Fatalf("Empty log is %s, want Dismissed", got)
	}
	if got := status("fern"); got != "Lead" {
		t.Fatalf("the row under the cursor was moved instead: fern is %s", got)
	}
}

// Pasted text lands in the focused form field and in the search, one line.
func TestPasteFillsFormAndSearch(t *testing.T) {
	root := fixtureCopy(t, nil)
	if view := ansi.Strip(runKeys(t, root, 120, "a", "paste:Pasted Title\n")); !strings.Contains(view, "Pasted Title▏") {
		t.Fatalf("paste missing from the Title field:\n%s", view)
	}
	if view := ansi.Strip(runKeys(t, root, 120, "/", "paste:Orbit\n")); !strings.Contains(view, "search Orbit") {
		t.Fatalf("paste missing from the search:\n%s", view)
	}
}

// Backspace removes a whole character: no half emoji, no stray joiner.
func TestBackspaceRemovesWholeCharacters(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "a", "paste:Rocket👩‍💻", "backspace"))
	if !strings.Contains(view, "Rocket▏") || strings.Contains(view, "\u200d") {
		t.Fatalf("backspace left part of the emoji:\n%s", view)
	}
}

// With the help pop-up open over an error, esc closes the pop-up and leaves the error.
func TestEscClosesHelpOverAnError(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "G", "e", "?", "esc"))
	if strings.Contains(view, "─ Keys ") {
		t.Fatalf("esc left the help pop-up open:\n%s", view)
	}
	if !strings.Contains(view, "invalid status_log entry") {
		t.Fatalf("esc cleared the error instead of closing help:\n%s", view)
	}
}

// A long title is cut in the pop-up's top border, which stays as wide as the box.
func TestLongTitleKeepsThePopupBorder(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		meta := filepath.Join(root, "jobs", "SWE", "empty", "meta.yml")
		b, _ := os.ReadFile(meta)
		long := "title: Principal Distributed Systems Engineer, Storage Infrastructure and Reliability Platform\n"
		if err := os.WriteFile(meta, []byte(strings.Replace(string(b), "title: Empty\n", long, 1)), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	for _, width := range []int{76, 120} {
		view := ansi.Strip(runKeys(t, root, width, "e"))
		var top, bottom string
		for _, line := range strings.Split(view, "\n") {
			if i := strings.Index(line, "╭"); i >= 0 {
				top = line[i:]
			}
			if i := strings.Index(line, "╰"); i >= 0 {
				bottom = line[i:]
			}
		}
		topW, botW := ansi.StringWidth(top[:strings.Index(top, "╮")+len("╮")]), ansi.StringWidth(bottom[:strings.Index(bottom, "╯")+len("╯")])
		if top == "" || topW != botW {
			t.Fatalf("width %d: top border %d cells, bottom %d:\n%s", width, topW, botW, view)
		}
	}
}

// A long form value shows its end, where typing happens, and its error in full.
func TestLongFormValueShowsItsEndAndError(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "a", "tab", "tab", "tab", "tab",
		"2026-01-01-and-some-more-text-here-xx", "enter"))
	if !strings.Contains(view, "here-xx▏") {
		t.Fatalf("the end of the value is hidden:\n%s", view)
	}
	if !strings.Contains(view, "✕ Closes must be YYYY-MM-DD") {
		t.Fatalf("the error is cut off:\n%s", view)
	}
}

// Rows keep SCHEMA's ~ on approximate dates.
func TestApproxDatesKeepTheirTilde(t *testing.T) {
	if view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3")); !strings.Contains(view, "applied ~") {
		t.Fatalf("approximate applied dates lost their ~:\n%s", view)
	}
}

// On a narrow terminal the key line keeps add and search, the way in for a newcomer.
func TestNarrowKeyLineKeepsAddAndSearch(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 76, "1"))
	lines := strings.Split(view, "\n")
	if last := lines[len(lines)-1]; !strings.Contains(last, "a add") || !strings.Contains(last, "/ search") {
		t.Fatalf("narrow key line dropped add or search: %q", last)
	}
}

// The frame is fixed: every screen and pop-up state is exactly the terminal height, and
// the ┬/┴ joins where the pane divider meets the rules stay intact — a toast or error on
// the status row never replaces the bottom rule.
func TestEveryScreenFillsTheTerminal(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"m"}, {"a"}, {"?"}, {"/pl"}, {"1", "t"}, {"G", "e"}, {"1", "x"}} {
		view := runKeys(t, root, 120, keys...)
		if n := strings.Count(view, "\n") + 1; n != termHeight {
			t.Fatalf("keys %q: %d lines on a %d-line terminal", keys, n, termHeight)
		}
		plain := ansi.Strip(view)
		if strings.Contains(plain, "│") && (!strings.Contains(plain, "┬") || !strings.Contains(plain, "┴")) {
			t.Fatalf("keys %q: pane joins missing:\n%s", keys, plain)
		}
	}
}

// One place names every filter — the view, the track, the search and the shown-of-total
// count — and the header count reads n/total while a filter hides rows.
func TestScopeLineNamesEveryFilter(t *testing.T) {
	root := fixtureCopy(t, nil)
	search := ansi.Strip(runKeys(t, root, 120, "1", "/pl", "enter"))
	for _, want := range []string{"search pl", "3 of 4", "Leads 3/4"} {
		if !strings.Contains(search, want) {
			t.Fatalf("search scope lacks %q:\n%s", want, search)
		}
	}
	track := ansi.Strip(runKeys(t, root, 120, "1", "t"))
	if !strings.Contains(track, "track SWE") || !strings.Contains(track, "3 of 4") {
		t.Fatalf("track scope missing:\n%s", track)
	}
}

// A success toast shows on the status row, over the key line, with the bottom rule kept.
func TestToastKeepsTheBottomRule(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1", "x"))
	if !strings.Contains(view, "→ Dismissed") || !strings.Contains(view, "┴") {
		t.Fatalf("toast replaced the bottom rule:\n%s", view)
	}
}

// The help pop-up fits a short terminal and keeps its bottom border with "any key close".
func TestHelpFitsAShortTerminal(t *testing.T) {
	const height = 14
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 80, height, "?"))
	if n := strings.Count(view, "\n") + 1; n > height {
		t.Fatalf("%d lines on a %d-line terminal", n, height)
	}
	if !strings.Contains(view, "any key close") {
		t.Fatalf("help lost its bottom border:\n%s", view)
	}
}
