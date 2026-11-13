package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var columnSizes = [][2]int{{120, 36}, {100, 30}, {88, 24}, {80, 24}, {72, 24}, {60, 16}}

// listView is the Applied list at a size.
func listView(t *testing.T, w, h int, keys ...string) string {
	t.Helper()
	return ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), w, h, append([]string{"3"}, keys...)...))
}

// listPane is a line's list part: what is left of the pane divider.
func listPane(line string) string {
	if i := strings.Index(line, "│"); i >= 0 {
		return line[:i]
	}
	return line
}

// rowLines are the first lines of the list's rows: those that start with a Status glyph
// after the selection gutter.
func rowLines(view string) []string {
	var out []string
	for _, line := range strings.Split(view, "\n") {
		pane := strings.TrimRight(listPane(line), " ")
		trim := strings.TrimLeft(pane, " ▌")
		if pane == "" || len(pane)-len(trim) > 5 || len(pane)-len(trim) < 2 {
			continue
		}
		for _, g := range []string{"●", "◆", "★", "✕", "–", "◌", "✎", "⊘"} {
			if strings.HasPrefix(trim, g+" ") && !strings.Contains(pane, "──") {
				out = append(out, listPane(line))
			}
		}
	}
	return out
}

// No list line is longer than the terminal, and the right columns start at the same
// cell on every row, at every size.
func TestListRowsAlignAndNeverWrap(t *testing.T) {
	for _, s := range columnSizes {
		w, h := s[0], s[1]
		view := listView(t, w, h)
		for _, line := range strings.Split(view, "\n") {
			if got := ansi.StringWidth(line); got > w {
				t.Errorf("%dx%d: line is %d cells: %q", w, h, got, line)
			}
		}
		rows := rowLines(view)
		if len(rows) < 3 {
			t.Fatalf("%dx%d: found %d rows:\n%s", w, h, len(rows), view)
		}
		end := ansi.StringWidth(strings.TrimRight(rows[0], " "))
		for _, r := range rows {
			if got := ansi.StringWidth(strings.TrimRight(r, " ")); got != end {
				t.Errorf("%dx%d: Fit column ends at cell %d, want %d: %q", w, h, got, end, r)
			}
		}
	}
}

// The latest stage date is fully visible on its row, at every size.
func TestListStageDateIsAlwaysWhole(t *testing.T) {
	for _, s := range columnSizes {
		view := listView(t, s[0], s[1])
		var found bool
		for _, r := range rowLines(view) {
			if strings.Contains(r, "Software Engineer") {
				found = true
				if !strings.Contains(r, "Applied 6d") {
					t.Errorf("%dx%d: stage date cut: %q", s[0], s[1], r)
				}
			}
		}
		if !found {
			t.Errorf("%dx%d: Software Engineer row not shown:\n%s", s[0], s[1], view)
		}
	}
}

// The Fit column shows the value, or · when there is none, and no label.
func TestListFitColumn(t *testing.T) {
	view := listView(t, 120, 36)
	for _, r := range rowLines(view) {
		switch {
		case strings.Contains(r, "Software Engineer"):
			if !strings.HasSuffix(strings.TrimRight(r, " "), " ·") {
				t.Errorf("unassessed row lacks ·: %q", r)
			}
		case strings.Contains(r, "Engineer #1") && strings.Contains(r, "quiet"), strings.Contains(r, "Engineer #1") && strings.Contains(r, "Interv"):
			if !strings.HasSuffix(strings.TrimRight(r, " "), "Good") {
				t.Errorf("assessed row lacks Good: %q", r)
			}
		}
		if strings.Contains(r, "fit ") || strings.Contains(r, "Fit") {
			t.Errorf("row carries a Fit label: %q", r)
		}
	}
}

// A long title is cut with an ellipsis, not wrapped, and the fixed columns survive.
func TestListLongTitleTruncates(t *testing.T) {
	root := fixtureCopy(t, nil)
	long := "Principal Staff Distributed Systems Reliability Engineer"
	meta := filepath.Join(root, "jobs", "SWE", "orbit", "meta.yml")
	b, err := os.ReadFile(meta)
	if err != nil {
		t.Skip("fixture layout changed")
	}
	title := regexp.MustCompile(`(?m)^title: .*$`)
	if err := os.WriteFile(meta, title.ReplaceAll(b, []byte("title: "+long)), 0o600); err != nil {
		t.Fatal(err)
	}
	view := ansi.Strip(runKeysSized(t, root, 60, 16, "3"))
	for _, r := range rowLines(view) {
		if strings.Contains(r, "Principal") {
			if !strings.Contains(r, "…") || !strings.Contains(r, "Applied 6d") {
				t.Errorf("long title row: %q", r)
			}
			return
		}
	}
	t.Fatalf("long title row not found:\n%s", view)
}

// The old Fit phrasings appear nowhere in the screens that talk about Fit.
func TestFitWordingIsConsistent(t *testing.T) {
	banned := []string{"fit —", "fit not assessed", "no fit assessment", "score"}
	screens := [][]string{{"1"}, {"2"}, {"3"}, {"3", "enter"}, {"3", "enter", "right"}, {"3", "enter", "right", "right"}, {"?"}, {"3", "/docs-written", "enter", "enter", "right", "right"}}
	for _, w := range []int{120, 80, 60} {
		for _, keys := range screens {
			view := strings.ToLower(ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), w, 40, keys...)))
			for _, b := range banned {
				if strings.Contains(view, b) {
					t.Errorf("%dw %v: old wording %q:\n%s", w, keys, b, view)
				}
			}
		}
	}
}
