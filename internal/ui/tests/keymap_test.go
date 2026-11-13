package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// keyLineOf is the frame's last line.
func keyLineOf(view string) string {
	plain := chipped(view)
	return plain[strings.LastIndex(plain, "\n")+1:]
}

// A narrow key line drops navigation hints before the main actions, and never cuts a
// key it keeps.
func TestKeyLineDropsLowPriorityKeysFirst(t *testing.T) {
	for _, tc := range []struct {
		name           string
		w, h           int
		keys           []string
		want, unwanted []string
	}{
		{"leads 80", 80, 24, []string{"1"}, []string{"enter detail", "m move", "a add", "/ search", "? help", "space menu"}, []string{"1-6 screen"}},
		{"leads 60", 60, 16, []string{"1"}, []string{"enter detail", "m move", "a add", "? help"}, []string{"1-6 screen", "←→ view"}},
		{"docs tab 60", 60, 16, []string{"3", "enter", "right", "right", "right", "right"}, []string{"esc back", "enter open", "space menu"}, []string{"1-6 screen"}},
		{"settings 60", 60, 16, []string{"6"}, []string{"enter edit", "? help", "space menu"}, []string{"1-6 screen"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := keyLineOf(runKeysSized(t, fixtureCopy(t, nil), tc.w, tc.h, tc.keys...))
			if ansi.StringWidth(line) > tc.w {
				t.Fatalf("key line is %d wide, terminal %d: %q", ansi.StringWidth(line), tc.w, line)
			}
			if strings.Contains(line, "…") {
				t.Fatalf("key line cuts a key: %q", line)
			}
			for _, w := range tc.want {
				if !strings.Contains(line, w) {
					t.Errorf("key line lacks %q: %q", w, line)
				}
			}
			for _, u := range tc.unwanted {
				if strings.Contains(line, u) {
					t.Errorf("key line keeps %q over the main actions: %q", u, line)
				}
			}
		})
	}
}

// ←→ switch views on a table screen; T cycles the track filter backwards.
func TestArrowsSwitchViewsAndShiftTCyclesTracksBack(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "right"))
	if !strings.Contains(view, "Leads › Dismissed") {
		t.Fatalf("→ did not switch to Dismissed:\n%s", view)
	}
	view = chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "right", "left"))
	if !strings.Contains(view, "Leads › Leads") {
		t.Fatalf("← did not switch back to Leads:\n%s", view)
	}
	view = chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "T"))
	if !strings.Contains(view, "[Platform 1]") {
		t.Fatalf("T did not cycle back to the last track:\n%s", view)
	}
}

// The `?` pop-up lists the context's keys, then the global ones, and offers paging only
// when there is a second page.
func TestHelpIsContextAwareAndPagesOnlyWhenNeeded(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "3", "enter", "?"))
	for _, want := range []string{"Keys · Applied · JD", "NAVIGATE", "←→ switches tab"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail help lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "1/1") || strings.Contains(view, "↑↓ page") {
		t.Fatalf("one-page help offers paging:\n%s", view)
	}
	if strings.Contains(view, "/ search") || helpLists(view, "a add") {
		t.Fatalf("detail help lists list-only keys:\n%s", view)
	}
}
