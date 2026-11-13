package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The Applied screen opens on All: every Application past Draft, one section per table.
func TestAllViewGroupsApplicationsByTable(t *testing.T) {
	view := ansi.Strip(finalView(t, "3"))
	var at []int
	for _, section := range []string{"Applied · 1", "Interviewing · 2", "Offer · 2", "Closed · 4"} {
		i := strings.Index(view, section)
		if i < 0 {
			t.Fatalf("All view lacks section %q:\n%s", section, view)
		}
		at = append(at, i)
	}
	for i := 1; i < len(at); i++ {
		if at[i] < at[i-1] {
			t.Fatalf("sections out of order:\n%s", view)
		}
	}
}

// A Lead whose close date has passed moves to an Expired section at the end of Leads.
func TestExpiredLeadsSortLastUnderTheirOwnSection(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	meta := filepath.Join(root, "jobs", "SWE", "fern", "meta.yml")
	b, _ := os.ReadFile(meta)
	if err := os.WriteFile(meta, bytes.Replace(b, []byte("track: Platform\n"), []byte("track: Platform\ncloses: 2026-09-01\n"), 1), 0o600); err != nil {
		t.Fatal(err)
	}
	view := ansi.Strip(runKeys(t, root, 120, "1"))
	expired, fern, empty := strings.Index(view, "Expired · 1"), strings.Index(view, "Platform Engineer"), strings.Index(view, "Empty log")
	if expired < 0 || fern < expired || empty > expired || !strings.Contains(view, "closed 1 Sep") {
		t.Fatalf("expired Lead not last under Expired:\n%s", view)
	}
}

// An error stays on the status row through other keys until esc dismisses it.
func TestErrorsStayUntilEsc(t *testing.T) {
	const want = "Can't read SWE/malformed ·"
	if view := ansi.Strip(finalView(t, "G", "m", "d", "j")); !strings.Contains(view, want) {
		t.Fatalf("error cleared by another key:\n%s", view)
	}
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), selectionWidth, "G", "m", "d", "esc"))
	if status := view[strings.LastIndex(view, "\n")+1:]; strings.Contains(status, want) {
		t.Fatalf("esc did not dismiss the error:\n%s", view)
	}
}

// a in the overwrite prompt overwrites this file and every later clash without asking again.
func TestDownloadOverwriteAll(t *testing.T) {
	h, downloads := newDownloadHarness(t, clash)
	h.open("3", 4, "docs-ready")
	h.tm.Type("d")
	h.wait("File exists")
	h.tm.Type("a")
	h.lastRun()
	for _, name := range []string{"Owner Resume Role.pdf", "Owner Role Cover Letter.pdf"} {
		want := readFile(t, filepath.Join(h.folder("docs-ready"), "out", name))
		if got := readFile(t, filepath.Join(downloads, name)); got != want {
			t.Fatalf("%s not overwritten: %q", name, got)
		}
	}
	h.show("Saved 2 PDFs → ")
}

// On a narrow terminal nothing runs past the right edge: the screens row sheds
// detail, the preview pane is hidden and pop-ups shrink to fit.
func TestNarrowTerminalKeepsEveryLineInside(t *testing.T) {
	const width = 76
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{{"1"}, {"3"}, {"m"}, {"a"}, {"?"}, {"3", "enter"}} {
		view := runKeys(t, root, width, keys...)
		for _, line := range strings.Split(view, "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Fatalf("keys %q: line is %d cells wide, terminal is %d:\n%s", keys, w, width, ansi.Strip(view))
			}
		}
	}
}

// runKeys runs the program on a Workspace at the given width and returns the final
// screen. Keys are typed as text, except the names drive understands.
func runKeys(t *testing.T, root string, width int, keys ...string) string {
	t.Helper()
	return runKeysSized(t, root, width, termHeight, keys...)
}

const termHeight = 40

func runKeysSized(t *testing.T, root string, width, height int, keys ...string) string {
	t.Helper()
	m := driveStart(t, root, fixtureClock, width, height)
	return driveModel(t, m, keys...).View().Content
}

// fixtureCopy is a fresh copy of the fixture Workspace, with edit applied to it.
func fixtureCopy(t *testing.T, edit func(root string)) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(root)
	}
	return root
}

// At any size, from a sliver to the minimum and just above it, every screen and pop-up
// stays inside the terminal and keeps the last column clear; below the minimum a notice
// says so instead.
func TestEverySizeFitsTheTerminal(t *testing.T) {
	root := fixtureCopy(t, nil)
	sizes := [][2]int{{1, 1}, {10, 3}, {55, 30}, {120, 13}, {56, 14}, {64, 15}, {79, 20}, {80, 21}, {109, 24}, {110, 24}}
	states := [][]string{{"1"}, {"3"}, {"4"}, {"5"}, {"6"}, {"m"}, {"a"}, {"?"}, {":"}, {"3", "enter"}, {"G", "m", "d"}, {"6", "tab", "enter"}}
	for _, size := range sizes {
		for _, keys := range states {
			w, h := size[0], size[1]
			view := runKeysSized(t, root, w, h, keys...)
			lines := strings.Split(view, "\n")
			small := w < 56 || h < 14
			if !small && len(lines) != h {
				t.Fatalf("%dx%d keys %q: %d lines:\n%s", w, h, keys, len(lines), ansi.Strip(view))
			}
			if len(lines) > h {
				t.Fatalf("%dx%d keys %q: %d lines:\n%s", w, h, keys, len(lines), ansi.Strip(view))
			}
			for _, line := range lines {
				if ansi.StringWidth(line) > w {
					t.Fatalf("%dx%d keys %q: a line is %d cells wide:\n%s", w, h, keys, ansi.StringWidth(line), ansi.Strip(view))
				}
			}
			// The notice needs a few cells to be readable at all.
			if w >= 30 && h >= 2 && small != strings.Contains(ansi.Strip(view), "resize to continue") {
				t.Fatalf("%dx%d: too-small notice shown = %v:\n%s", w, h, !small, ansi.Strip(view))
			}
		}
	}
}

// Below the minimum the notice names the size rolecall needs and the size it has, centred.
func TestTooSmallNoticeSaysWhatItNeeds(t *testing.T) {
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 48, 12, "1"))
	lines := strings.Split(view, "\n")
	if len(lines) != 12 {
		t.Fatalf("notice is %d lines, want 12:\n%s", len(lines), view)
	}
	for _, want := range []string{"rolecall needs 56×14", "now 48×12 · resize to continue"} {
		found := false
		for i, l := range lines {
			if c := strings.Index(l, want); c >= 0 {
				found = true
				left := ansi.StringWidth(l[:c])
				if right := 48 - left - ansi.StringWidth(want); left < 1 || right < 0 || abs(left-right) > 4 || i < 3 || i > 8 {
					t.Errorf("%q not centred: line %d, %d cells left, %d right", want, i, left, right)
				}
			}
		}
		if !found {
			t.Errorf("notice lacks %q:\n%s", want, view)
		}
	}
}

func abs(n int) int { return max(n, -n) }

// Each size class makes its own layout choices, and every screen shares them: the
// preview and detail card are wide-only, the side lists stay down to medium, the short
// class drops the gap under the tab bar, and rules join every pane divider.
func TestSizeClassesShareOneLayout(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, tc := range []struct {
		w, h           int
		preview, lists bool
	}{{120, 36, true, true}, {100, 30, true, true}, {99, 30, false, true}, {72, 24, false, true}, {71, 24, false, false}, {60, 16, false, false}} {
		short := tc.h <= 20
		for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"5"}, {"6"}, {"4"}} {
			lines := strings.Split(ansi.Strip(runKeysSized(t, root, tc.w, tc.h, keys...)), "\n")
			if gap := strings.TrimSpace(lines[1]) == ""; gap == short {
				t.Errorf("%dx%d keys %q: gap under the tab bar = %v, short = %v", tc.w, tc.h, keys, gap, short)
			}
			// The body sits between the first two rules; the status row is last.
			var rules []int
			for i, l := range lines {
				if strings.HasPrefix(l, "──") {
					rules = append(rules, i)
				}
			}
			// A bottom sheet adds a third rule between the list and the detail.
			if sheet := tc.w < 100 && tc.w >= 72 && len(keys) > 1; len(rules) == 3 && sheet {
				rules = []int{rules[0], rules[2]}
			}
			if len(rules) != 2 || rules[1] != len(lines)-2 {
				t.Fatalf("%dx%d keys %q: rules at %v:\n%s", tc.w, tc.h, keys, rules, strings.Join(lines, "\n"))
			}
			split := strings.Contains(lines[rules[0]], "┬")
			want := tc.lists
			switch keys[0] {
			case "1", "3":
				want = tc.preview
			case "4":
				want = false
			}
			if split != want || split != strings.Contains(lines[rules[1]], "┴") {
				t.Errorf("%dx%d keys %q: divider = %v, want %v:\n%s", tc.w, tc.h, keys, split, want, strings.Join(lines, "\n"))
			}
			for i, l := range lines {
				// Every line keeps the last column clear, and content starts past column 0.
				if ansi.StringWidth(strings.TrimRight(l, " ")) > tc.w-1 && !strings.HasPrefix(l, "──") {
					t.Errorf("%dx%d keys %q: line %d reaches the right edge: %q", tc.w, tc.h, keys, i, l)
				}
				if i > rules[0] && i < rules[1] && l != "" && l[0] != ' ' && !strings.HasPrefix(l, "──") {
					t.Errorf("%dx%d keys %q: line %d starts at column 0: %q", tc.w, tc.h, keys, i, l)
				}
			}
		}
	}
}

// A pop-up taller than the body scrolls inside its border and never covers the tab bar,
// the context row or the status row.
func TestPopupsStayInsideTheBody(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, size := range [][2]int{{60, 16}, {56, 14}, {80, 24}} {
		for _, keys := range [][]string{{"1", "m"}, {"a"}, {"?"}, {"6", "tab", "enter"}} {
			lines := strings.Split(ansi.Strip(runKeysSized(t, root, size[0], size[1], keys...)), "\n")
			top, bottom := -1, -1
			for i, l := range lines {
				if strings.Contains(l, "╭") && top < 0 {
					top = i
				}
				if strings.Contains(l, "╰") {
					bottom = i
				}
			}
			rule := 3
			if size[1] <= 20 {
				rule = 2
			}
			if top <= rule || bottom >= len(lines)-2 || top < 0 {
				t.Errorf("%dx%d keys %q: box spans lines %d-%d, body is %d-%d:\n%s", size[0], size[1], keys, top, bottom, rule+1, len(lines)-3, strings.Join(lines, "\n"))
			}
			if keys[len(keys)-1] == "m" && !strings.Contains(strings.Join(lines, "\n"), "▌ d  ✎ Draft") {
				t.Errorf("%dx%d: Move lost its selection while scrolled:\n%s", size[0], size[1], strings.Join(lines, "\n"))
			}
		}
	}
}

// Text wraps only at spaces: a flag such as "--wait" and a phrase such as "Fit: not
// assessed" stay whole.
func TestWrapsOnlyAtSpaces(t *testing.T) {
	machine := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 60, 16, "6", "j", "j", "j"))
	if !strings.Contains(machine, "--wait") || strings.Contains(machine, "-\n") {
		t.Errorf("editor examples split a flag:\n%s", machine)
	}
	for _, l := range strings.Split(machine, "\n") {
		if strings.HasSuffix(strings.TrimRight(l, " "), " -") || strings.HasPrefix(strings.TrimSpace(l), "-wait") {
			t.Errorf("editor examples split a flag: %q", l)
		}
	}
	reader := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "3", "/docs-written", "enter", "enter", "right", "right"))
	if !strings.Contains(reader, "Fit: not assessed") {
		t.Errorf("card split the Fit phrase:\n%s", reader)
	}
}

// The overflow cue is one style everywhere, set at the text column, and a section heading
// never sits alone above it.
func TestOverflowCueIsOneStyle(t *testing.T) {
	root := fixtureCopy(t, nil)
	// Stats fits 80x24, so it overflows only on a smaller terminal.
	for _, c := range []struct {
		keys []string
		w, h int
	}{{[]string{"3"}, 80, 24}, {[]string{"4"}, 60, 16}} {
		keys := c.keys
		lines := strings.Split(ansi.Strip(runKeysSized(t, root, c.w, c.h, keys...)), "\n")
		cue := lines[len(lines)-3]
		if !strings.HasPrefix(cue, "   ↓ ") || !strings.HasSuffix(strings.TrimRight(cue, " "), " more") {
			t.Errorf("keys %q: cue %q, want \"   ↓ N more\" on the body's last line", keys, cue)
		}
		above := strings.TrimSpace(lines[len(lines)-4])
		if strings.Contains(above, "· ") && strings.Contains(above, "──") {
			t.Errorf("keys %q: a section heading sits alone above the cue: %q", keys, above)
		}
	}
}
