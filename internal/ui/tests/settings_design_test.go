package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func settingsAt(t *testing.T, w, h int, keys ...string) []string {
	t.Helper()
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, w, h)
	d.Type("6")
	for _, k := range keys {
		d.Type(k)
	}
	return strings.Split(d.plain(), "\n")
}

func footer(lines []string) string { return lines[len(lines)-1] }

var settingsPopups = []struct {
	name string
	keys []string
	want string
}{
	{"search area", []string{"tab", "enter"}, "ctrl+s save"},
	{"gate", []string{"j", "tab", "enter"}, "enter save"},
	{"add track", []string{"j", "j", "tab", "a"}, "enter add"},
	{"rename track", []string{"j", "j", "tab", "r"}, "enter review"},
}

func TestSettingsFooterShowsPopupKeysNotTheScreenName(t *testing.T) {
	for _, tc := range settingsPopups {
		for _, w := range []int{120, 80, 60} {
			h := 24
			if w == 60 {
				h = 16
			}
			f := footer(settingsAt(t, w, h, tc.keys...))
			if !strings.Contains(f, tc.want) || !strings.Contains(f, "esc cancel") {
				t.Errorf("%s at %d: footer %q lacks pop-up keys", tc.name, w, f)
			}
		}
	}
}

func TestSettingsFooterWhileEditingAMachineSetting(t *testing.T) {
	f := footer(settingsAt(t, 120, 36, "j", "j", "j", "tab", "j", "enter"))
	if !strings.Contains(f, "enter save") || !strings.Contains(f, "esc cancel") {
		t.Fatalf("footer %q lacks edit keys", f)
	}
}

// Settings pop-ups size to their own hints, so no key is cut, and sit centred on the
// full width like every other pop-up.
func TestSettingsPopupsFitTheirHintsAndCentre(t *testing.T) {
	for _, tc := range settingsPopups {
		for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
			w := size[0]
			lines := settingsAt(t, w, size[1], tc.keys...)
			seen := false
			for _, l := range lines {
				if i := strings.Index(l, "╭"); i >= 0 {
					seen = true
					boxW := ansi.StringWidth(l[i : strings.Index(l, "╮")+len("╮")])
					if left := ansi.StringWidth(l[:i]); left != (w-boxW)/2 {
						t.Errorf("%s at %d: box starts at %d, want centred at %d", tc.name, w, left, (w-boxW)/2)
					}
				}
				if strings.Contains(l, "╰") && (!strings.Contains(l, tc.want) || !strings.Contains(l, "esc cancel ")) {
					t.Errorf("%s at %d: bottom border cuts its hints: %q", tc.name, w, l)
				}
			}
			if !seen {
				t.Errorf("%s at %d: no pop-up:\n%s", tc.name, w, strings.Join(lines, "\n"))
			}
		}
	}
}

func TestSettingsTrackFooterKeys(t *testing.T) {
	f := footer(settingsAt(t, 120, 36, "j", "j", "tab"))
	if !strings.Contains(f, "a add · r rename · x remove · J/K reorder") {
		t.Fatalf("footer %q", f)
	}
}

func TestSettingsFooterOffersEnterOnEditableRowsOnly(t *testing.T) {
	// Follow-up and a machine setting are editable; the add row adds. Read-only rows are
	// never reached (TestSettingsReadOnlyRowsAreMarked), so no footer offers enter on one.
	if f := footer(settingsAt(t, 120, 36, "j", "j", "tab", "j", "j", "j")); !strings.Contains(f, "enter edit") {
		t.Errorf("Follow-up days: %q", f)
	}
	if f := footer(settingsAt(t, 120, 36, "j", "j", "j", "tab")); !strings.Contains(f, "enter edit") {
		t.Errorf("Editor: %q", f)
	}
	if f := footer(settingsAt(t, 120, 36, "j", "j", "tab", "j", "j")); !strings.Contains(f, "enter add a Track") {
		t.Errorf("add row: %q", f)
	}
}

func rowWith(lines []string, text string) string {
	for _, l := range lines[1:] { // not the top bar, which lists Tracks too
		if strings.Contains(l, text) {
			return l
		}
	}
	return ""
}

// The focused pane carries the ▸ marker: the group while the list has focus, the rows
// while the settings pane does. Only one ▸ is ever drawn, so nothing reads as a second
// cursor.
func TestSettingsFocusMovesTheMarkerBetweenPanes(t *testing.T) {
	// The group list has focus: the group is marked, no settings row is.
	v := settingsAt(t, 120, 36, "j", "j")
	if group := settingsColumn(v, 0); !strings.Contains(group, "▸ Tracking") {
		t.Errorf("focused group list: %q", group)
	}
	if rows := settingsColumn(v, 1); strings.Contains(rows, "▸") {
		t.Errorf("unfocused settings pane marked a row:\n%s", rows)
	}
	// The settings pane has focus: its row is marked, and the group keeps its place
	// without a second ▸.
	v = settingsAt(t, 120, 36, "j", "j", "tab")
	if group := settingsColumn(v, 0); strings.Contains(group, "▸") || !strings.Contains(group, "Tracking") {
		t.Errorf("unfocused group list should be unmarked but named: %q", group)
	}
	if rows := settingsColumn(v, 1); !strings.Contains(rows, "▸ SWE") {
		t.Errorf("focused settings pane lost its marker:\n%s", rows)
	}
}

// settingsColumn is one pane of the split Settings body: 0 is the group list, 1 is the
// settings. Lines without the divider are the chrome around the body and are dropped.
func settingsColumn(lines []string, side int) string {
	var b strings.Builder
	for _, l := range lines {
		i := strings.Index(l, "│")
		if i < 0 {
			continue
		}
		if side == 0 {
			b.WriteString(l[:i])
		} else {
			b.WriteString(l[i+len("│"):])
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestSettingsSelectedRowHasMarkerAtEveryWidth(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "j", "j", "tab")
		if l := rowWith(lines, "SWE"); !strings.Contains(l, "▸ SWE") {
			t.Errorf("%dx%d first Track: %q", size[0], size[1], l)
		}
		lines = settingsAt(t, size[0], size[1], "j", "j", "tab", "j")
		if l := rowWith(lines, "Platform"); !strings.Contains(l, "▸") {
			t.Errorf("%dx%d second Track: %q\n%s", size[0], size[1], l, strings.Join(lines, "\n"))
		}
	}
	// Narrow: the Groups column is hidden, so the row is selected without pressing tab.
	if l := rowWith(settingsAt(t, 60, 16, "j", "j"), "SWE"); !strings.Contains(l, "▸ SWE") {
		t.Errorf("60x16 before tab: %q", l)
	}
}
