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
	return chipped(runKeysSized(t, fixtureCopy(t, nil), w, h, append([]string{"3"}, keys...)...))
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
		if pane == "" || strings.Contains(pane, "⚑1") || strings.Contains(pane, "Drafts ") || strings.Contains(pane, "✎2 ") || len(pane)-len(trim) > 5 || len(pane)-len(trim) < 2 {
			continue
		}
		for _, g := range []string{"●", "◆", "★", "✕", "↩", "◌", "✎", "⊘"} {
			if strings.HasPrefix(trim, g+" ") && !strings.Contains(pane, "──") {
				out = append(out, listPane(line))
			}
		}
	}
	return out
}

// fitWord finds a row's Fit, alone at the end of its title line.
var fitWord = regexp.MustCompile(`  (Strong|Good|Marginal|Poor) *$`)

// No list line is longer than the terminal, and the right columns start at the same
// cell on every row, at every size: every Fit word starts at one cell, left-aligned in a
// column as wide as "Marginal", and every Follow-up flag ends at one cell.
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
		fitAt, flagEnd := -1, -1
		for _, r := range rows {
			if m := fitWord.FindStringSubmatchIndex(r); m != nil {
				at := ansi.StringWidth(r[:m[2]])
				if fitAt >= 0 && at != fitAt {
					t.Errorf("%dx%d: Fit starts at cell %d, want %d: %q", w, h, at, fitAt, r)
				}
				fitAt = at
			}
			if i := strings.Index(r, "⚑"); i >= 0 {
				end := ansi.StringWidth(r[:i]) + ansi.StringWidth(strings.Fields(r[i:])[0]+" "+strings.Fields(r[i:])[1])
				if flagEnd >= 0 && end != flagEnd {
					t.Errorf("%dx%d: flag ends at cell %d, want %d: %q", w, h, end, flagEnd, r)
				}
				flagEnd = end
			}
		}
		if fitAt < 0 || flagEnd < 0 {
			t.Errorf("%dx%d: no Fit or no flag found:\n%s", w, h, view)
		}
	}
}

// A Follow-up due flag is whole on its row, at every size, however much of the title is
// cut to make room.
func TestListFollowUpFlagIsAlwaysWhole(t *testing.T) {
	for _, s := range columnSizes {
		view := listView(t, s[0], s[1])
		var found bool
		for _, r := range rowLines(view) {
			if strings.Contains(r, "◆ Engineer #1") {
				found = true
				if !strings.Contains(r, "⚑ 9mo  Good") {
					t.Errorf("%dx%d: flag or Fit cut: %q", s[0], s[1], r)
				}
			}
		}
		if !found {
			t.Errorf("%dx%d: Engineer #1 row not shown:\n%s", s[0], s[1], view)
		}
	}
}

// The Fit column is the Fit word alone, "Good", and blank when there is none: no "fit"
// label repeated down the list, no age, and never a bare · or — that reads as a value.
func TestListFitColumn(t *testing.T) {
	view := listView(t, 120, 36)
	for _, r := range rowLines(view) {
		end := strings.TrimRight(r, " ")
		switch {
		case strings.Contains(r, "● Software Eng"):
			if !strings.HasSuffix(end, "Software Engineer") {
				t.Errorf("unassessed row draws something after its title: %q", r)
			}
		case strings.Contains(r, "Engineer #1"):
			if !strings.HasSuffix(end, "  Good") {
				t.Errorf("assessed row lacks Good: %q", r)
			}
		}
		if strings.Contains(r, "fit") || strings.HasSuffix(end, "·") || strings.HasSuffix(end, "—") {
			t.Errorf("row shows a fit label or ends in a bare mark: %q", r)
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
	view := chipped(runKeysSized(t, root, 60, 16, "3"))
	for _, r := range rowLines(view) {
		if strings.Contains(r, "Principal") {
			if !strings.Contains(r, "…") {
				t.Errorf("long title row: %q", r)
			}
			return
		}
	}
	t.Fatalf("long title row not found:\n%s", view)
}

// The old Fit phrasings appear nowhere in the screens that talk about Fit: a Fit reads
// "fit Good" everywhere, the list, the preview, the detail and the Fit tab; one not assessed
// reads "fit not assessed" in the card and on the Fit tab.
func TestFitWordingIsConsistent(t *testing.T) {
	banned := []string{"fit: ", "no fit assessment", "score"}
	screens := [][]string{{"1"}, {"2"}, {"3"}, {"3", "enter"}, {"3", "enter", "right"}, {"3", "enter", "right", "right"}, {"?"}, {"3", "/docs-written", "enter", "enter", "right", "right"}}
	for _, w := range []int{120, 80, 60} {
		for _, keys := range screens {
			view := strings.ToLower(chipped(runKeysSized(t, fixtureCopy(t, nil), w, 40, keys...)))
			for _, b := range banned {
				if strings.Contains(view, b) {
					t.Errorf("%dw %v: old wording %q:\n%s", w, keys, b, view)
				}
			}
		}
	}
}
