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
		{"leads 80", 80, 24, []string{"1"}, []string{"enter open", "m move", "a add", "/ search", "? help", "space commands"}, []string{"1-6 screen"}},
		{"leads 60", 60, 16, []string{"1"}, []string{"enter open", "m move", "a add", "? help"}, []string{"1-6 screen", "←→ view"}},
		{"docs tab 60", 60, 16, []string{"3", "enter", "right", "right", "right", "right"}, []string{"esc back", "enter open", "space commands"}, []string{"1-6 screen"}},
		{"settings 60", 60, 16, []string{"6"}, []string{"enter edit", "? help", "space commands"}, []string{"1-6 screen"}},
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

// ←→ switch views on a table screen; T cycles the track filter backwards, past empty Tracks.
func TestArrowsSwitchViewsAndShiftTCyclesTracksBack(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "right"))
	if !strings.Contains(view, "Leads › Dismissed") {
		t.Fatalf("→ did not switch to Dismissed:\n%s", view)
	}
	view = chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "right", "left"))
	if !strings.Contains(view, "Leads › All tracks") {
		t.Fatalf("← did not switch back to Leads:\n%s", view)
	}
	// T from All goes back to the last Track with rows here: the off-list Unknown has none
	// on Leads, so it is passed over for Platform.
	view = chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "T"))
	if !strings.Contains(view, "[Platform 1]") {
		t.Fatalf("T did not cycle back to the last Track with rows, Platform:\n%s", view)
	}
}

// The `?` pop-up lists the context's keys, then the global ones, and offers paging only
// when there is a second page.
func TestHelpIsContextAwareAndPagesOnlyWhenNeeded(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 80, "3", "enter", "?"))
	for _, want := range []string{"Keys · Applied · JD", "NAVIGATE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("detail help lacks %q:\n%s", want, view)
		}
	}
	if !helpLists(view, "←→ tab") {
		t.Fatalf("detail help lacks ←→ tab:\n%s", view)
	}
	if strings.Contains(view, "1/1") || strings.Contains(view, "page 1 of") || strings.Contains(view, "↑↓ page") {
		t.Fatalf("one-page help offers paging:\n%s", view)
	}
	if short := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 30, "3", "enter", "?")); !strings.Contains(short, "page 1 of") ||
		!strings.Contains(short, "↑↓ page") {
		t.Fatalf("a help taller than its box does not page:\n%s", short)
	}
	if strings.Contains(view, "/ search") || helpLists(view, "a add") {
		t.Fatalf("detail help lists list-only keys:\n%s", view)
	}
}
