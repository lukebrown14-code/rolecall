package ui_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// keyCell is a key name at the start of a help or Move pop-up entry: a run of non-space
// characters after the border and padding, before two spaces and a word.
var keyCell = regexp.MustCompile(`(?:^|\s{2})(\S+(?: \S+)*?)\s{2,}[a-z]`)

// popUpBox is the inside of the ASCII pop-up whose bottom border starts with bottom: each
// line from the top border down, cut to the box's columns, so the frame behind it is left
// out.
func popUpBox(t *testing.T, view, bottom string) []string {
	t.Helper()
	lines := strings.Split(view, "\n")
	end, col := -1, -1
	for i, l := range lines {
		if c := strings.Index(l, bottom); c >= 0 {
			end, col = i, c
		}
	}
	if end < 0 {
		t.Fatalf("no pop-up ending %q:\n%s", bottom, view)
	}
	width := len(strings.TrimRight(lines[end][col:], " |"))
	var out []string
	for i := end - 1; i >= 0; i-- {
		l := lines[i]
		if len(l) < col+width {
			break
		}
		inner := l[col : col+width]
		if !strings.HasPrefix(inner, "|") {
			break // above the top border
		}
		out = append([]string{inner}, out...)
	}
	return out
}

// In ASCII mode no glyph's stand-in is also a key in the same pop-up: the Move pop-up
// sets each Status's stand-in beside its key letter, and the `?` help lists keys beside
// the glyph legend, so a stand-in that were a key would read as one.
func TestAsciiStandInsAreNeverKeysInThePopUp(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	standIns := map[string]string{}
	for _, g := range theme.Legend() {
		standIns[g.ASCII] = g.Meaning
	}
	help := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 120, 70, "?"))
	keys := map[string]bool{}
	box := popUpBox(t, help, "-- esc close")
	var section []string
	for _, line := range box {
		if strings.Contains(line, "GLYPHS") {
			break // the key sections sit above the legend
		}
		section = append(section, line)
	}
	for _, line := range section {
		for _, m := range keyCell.FindAllStringSubmatch(line, -1) {
			for _, k := range strings.Fields(m[1]) {
				keys[k] = true
			}
		}
	}
	for _, want := range []string{"*", ","} { // the parse found the punctuation keys
		if !keys[want] {
			t.Fatalf("help's key list lacks %q; parsed %v:\n%s", want, keys, help)
		}
	}
	for k := range keys {
		if meaning, ok := standIns[k]; ok {
			t.Errorf("%q is a key in the help and the stand-in for %s:\n%s", k, meaning, strings.Join(section, "\n"))
		}
	}
	move := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1", "m"))
	for _, line := range strings.Split(move, "\n") {
		f := strings.Fields(strings.Trim(strings.TrimSpace(line), "|>"))
		if len(f) >= 3 && len(f[0]) == 1 && f[0] >= "a" && f[0] <= "z" {
			if _, ok := standIns[f[0]]; ok {
				t.Errorf("Move key %q is also a stand-in: %q", f[0], line)
			}
		}
	}
	// Each Status's stand-in sits beside its key letter, and no key letter is one.
	for _, want := range []string{"d  = Draft", "a  # Applied", "x  % Dismissed", "i  & Interviewing", "w  ` Withdrawn"} {
		if !strings.Contains(move, want) {
			t.Errorf("Move pop-up lacks %q:\n%s", want, move)
		}
	}
}

// ASCII draws the separator and the ellipsis apart, a follow-up as @, and a marked row
// with its own mark in the gutter, apart from the selection's >.
func TestAsciiStandInsReadApart(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	list := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	for _, want := range []string{"# Applied | 1", "& Interviewing | 2", "@ 9mo", "Orbit Labs | SWE | applied"} {
		if !strings.Contains(list, want) {
			t.Errorf("ASCII Pipeline lacks %q:\n%s", want, list)
		}
	}
	marked := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "2", "*"))
	if !strings.Contains(marked, "{ = Engineer #1") || !strings.Contains(marked, "{ 1 marked") {
		t.Errorf("ASCII lost the marked row's mark:\n%s", marked)
	}
}

// Without colour a row's title is bold and its meta line is not, so the two-line rhythm
// (title, then company · track · dates) survives on every row, not only the selected one.
func TestNoColorRowTitleOutweighsItsMetaLine(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	view := runKeys(t, fixtureCopy(t, nil), 120, "3")
	lines := strings.Split(view, "\n")
	bold := regexp.MustCompile(`\x1b\[(?:[0-9;]*;)?1(?:;[0-9;]*)?m`)
	checked := 0
	for i, l := range lines {
		plain := ansi.Strip(l)
		// An unselected row's title: "docs-written" under Interviewing, off the cursor.
		if !strings.Contains(plain, "docs-written") || strings.HasPrefix(plain, " >") || strings.HasPrefix(plain, " ▌") {
			continue
		}
		title := l[:strings.Index(l, "│")]
		meta := lines[i+1][:strings.Index(lines[i+1], "│")]
		if !bold.MatchString(title) {
			t.Errorf("NO_COLOR row title is not bold: %q", title)
		}
		if bold.MatchString(meta) {
			t.Errorf("NO_COLOR meta line is bold like its title: %q", meta)
		}
		checked++
	}
	if checked == 0 {
		t.Fatalf("no unselected docs-written row found:\n%s", ansi.Strip(view))
	}
}

// Missing optional setup is not a failure: "No Harness set" (and "No Plugin set") draw
// with the needs-attention ⚠ in the warning colour, never the ✘ error mark, which stays
// for things that went wrong.
func TestNoHarnessIsNotStyledAsAnError(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3", "f"))
	if !strings.Contains(view, "⚠ No Harness set") || strings.Contains(view, "✘") {
		t.Fatalf("no Harness reads as an error:\n%s", view)
	}
	// Real failures keep ✘ (form_test, add_test, email_check_test).
}
