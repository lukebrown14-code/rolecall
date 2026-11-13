package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

var footerSizes = [][2]int{{120, 36}, {80, 24}, {60, 16}}

// plainAt is the final View at a size, stripped of styling, as lines.
func plainAt(t *testing.T, w, h int, keys ...string) []string {
	t.Helper()
	return strings.Split(ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), w, h, keys...)), "\n")
}

// The key line is the last line of the View on every screen and size, short list or not.
func TestKeyLineIsTheLastLine(t *testing.T) {
	for _, sz := range footerSizes {
		for _, screen := range []string{"1", "2", "3", "4", "5", "6"} {
			lines := plainAt(t, sz[0], sz[1], screen)
			if len(lines) != sz[1] {
				t.Errorf("%dx%d screen %s: %d lines; want %d", sz[0], sz[1], screen, len(lines), sz[1])
				continue
			}
			if last := lines[len(lines)-1]; !strings.Contains(last, "? help") {
				t.Errorf("%dx%d screen %s: last line %q is not the key line", sz[0], sz[1], screen, last)
			}
		}
	}
}

// A toast sits directly above the key line, which stays on the last row.
func TestToastSitsAboveTheKeyLine(t *testing.T) {
	lines := plainAt(t, 80, 24, "3", "m", "enter")
	if len(lines) != 24 || !strings.Contains(lines[23], "? help") {
		t.Fatalf("key line not last with a toast:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[22], "Moved") {
		t.Fatalf("toast not directly above the key line: %q", lines[22])
	}
}

func breadcrumb(lines []string) string {
	for _, l := range lines {
		if strings.Contains(l, " › ") {
			return strings.TrimSpace(l)
		}
	}
	return ""
}

func TestBreadcrumbFollowsCursorFilterAndFocus(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want string
	}{
		{"start", []string{"3"}, "Applied › All · All tracks · 1/9 · list"},
		{"down 3", []string{"3", "j", "j", "j"}, "Applied › All · All tracks · 4/9 · list"},
		{"track", []string{"3", "t"}, "Applied › All · SWE · "},
		{"empty", []string{"3", "/", "zzzzqq", "enter"}, "Applied › All · All tracks · 0/0 · list"},
	}
	for _, c := range cases {
		got := breadcrumb(plainAt(t, 120, 36, c.keys...))
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: breadcrumb %q; want it to contain %q", c.name, got, c.want)
		}
	}
}

func TestBreadcrumbShortensWhenNarrow(t *testing.T) {
	got := breadcrumb(plainAt(t, 60, 16, "3"))
	if !strings.HasPrefix(got, "Applied › All") || strings.Contains(got, "list") || !strings.Contains(got, "1/9") {
		t.Errorf("narrow breadcrumb %q: want position kept and focus dropped", got)
	}
}

// At 80x24 Enter shows the list and the detail together; esc returns with the cursor kept.
func TestMediumEnterOpensABottomSheet(t *testing.T) {
	lines := plainAt(t, 80, 24, "3", "j", "enter")
	view := strings.Join(lines, "\n")
	if len(lines) != 24 || !strings.Contains(lines[23], "? help") {
		t.Fatalf("sheet changed the frame height:\n%s", view)
	}
	if !strings.Contains(view, "2/9") || !strings.Contains(view, "▌") {
		t.Fatalf("sheet lost the selected row:\n%s", view)
	}
	if !strings.Contains(view, "[JD]") && !strings.Contains(view, "[Docs]") {
		t.Fatalf("sheet shows no detail tabs:\n%s", view)
	}
	back := strings.Join(plainAt(t, 80, 24, "3", "j", "enter", "esc"), "\n")
	if strings.Contains(back, "[Docs]") || !strings.Contains(back, "2/9") {
		t.Fatalf("esc did not return to the list with the cursor kept:\n%s", back)
	}
}

// With too little height for about 8 detail lines, Enter opens the detail full-screen.
func TestShortMediumEnterOpensFullScreen(t *testing.T) {
	view := strings.Join(plainAt(t, 80, 16, "3", "enter"), "\n")
	if strings.Contains(view, "› All") {
		t.Fatalf("detail should replace the list at 80x16:\n%s", view)
	}
}

func TestFiltersCollapseToOneLineWhenNarrow(t *testing.T) {
	lines := plainAt(t, 60, 16, "3")
	found := false
	for _, l := range lines {
		if strings.Contains(l, "s:All") && strings.Contains(l, "t:All") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no collapsed filter line:\n%s", strings.Join(lines, "\n"))
	}
	if wide := strings.Join(plainAt(t, 120, 36, "3"), "\n"); strings.Contains(wide, "s:All") {
		t.Fatalf("wide filters should not collapse")
	}
}

func TestEnterDetailHintOnlyWithoutThePane(t *testing.T) {
	for _, sz := range footerSizes {
		lines := plainAt(t, sz[0], sz[1], "3")
		last := lines[len(lines)-1]
		has := strings.Contains(last, "enter detail")
		if want := sz[0] < 100; has != want {
			t.Errorf("%dx%d: enter detail hint = %v; want %v (%q)", sz[0], sz[1], has, want, last)
		}
	}
}
