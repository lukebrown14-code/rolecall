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
	{"search area", []string{"enter"}, "ctrl+s save"},
	{"gate", []string{"]", "]", "]", "enter"}, "enter save"},
	{"add track", []string{"]", "]", "]", "]", "a"}, "enter add"},
	{"rename track", []string{"]", "]", "]", "]", "enter"}, "enter review"},
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
	f := footer(settingsAt(t, 120, 36, "]", "]", "]", "]", "]", "j", "enter"))
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
	f := helpText(strings.Join(settingsAt(t, 120, 40, "]", "]", "]", "]", "?"), "\n"))
	if !strings.Contains(f, "a add") || !strings.Contains(f, "enter rename") || !strings.Contains(f, "x remove") || !strings.Contains(f, "J/K reorder") {
		t.Fatalf("footer %q", f)
	}
}

func TestSettingsFooterOffersEnterOnEditableRowsOnly(t *testing.T) {
	// Follow-up and a machine setting are editable; the add row adds. Read-only rows are
	// never reached (TestSettingsReadOnlyRowsAreMarked), so no footer offers Enter on one.
	if f := footer(settingsAt(t, 120, 36, "]", "]", "]", "]", "G")); !strings.Contains(f, "enter edit") {
		t.Errorf("Follow-up days: %q", f)
	}
	if f := footer(settingsAt(t, 120, 36, "]", "]", "]", "]", "]")); !strings.Contains(f, "enter edit") {
		t.Errorf("Editor: %q", f)
	}
	if f := footer(settingsAt(t, 120, 36, "]", "]", "]", "]", "G", "k")); !strings.Contains(f, "enter add a Track") {
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

// The sections are tabs: the active one is bracketed, and the field cursor (or the header,
// above the first field) carries the one selection mark.
func TestSettingsCursorMarksOneFieldOrTheHeader(t *testing.T) {
	v := strings.Join(settingsAt(t, 120, 36, "]", "]", "]", "]"), "\n")
	if !strings.Contains(v, "[Tracking]") || strings.Count(v, "▌") != 1 || !strings.Contains(v, "▌ SWE") {
		t.Errorf("field cursor:\n%s", v)
	}
	lines := settingsAt(t, 120, 36, "]", "]", "]", "]", "k")
	if l := rowWith(lines, "SETTINGS"); !strings.Contains(l, "▌") {
		t.Errorf("header cursor: %q", l)
	}
	if v := strings.Join(lines, "\n"); strings.Contains(v, "▌ SWE") {
		t.Errorf("two cursors:\n%s", v)
	}
}

func TestSettingsSelectedRowHasMarkerAtEveryWidth(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "]", "]", "]", "]")
		if l := rowWith(lines, "▌ SWE"); l == "" {
			t.Errorf("%dx%d first Track: %q", size[0], size[1], l)
		}
		lines = settingsAt(t, size[0], size[1], "]", "]", "]", "]", "j")
		if l := rowWith(lines, "Platform"); !strings.Contains(l, "▌") {
			t.Errorf("%dx%d second Track: %q\n%s", size[0], size[1], l, strings.Join(lines, "\n"))
		}
	}
}
