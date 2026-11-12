package ui_test

import (
	"strings"
	"testing"
)

// The Track removal refusal is a warning pop-up next to the rows, not a footer error, and
// it says what to do about the Applications still on the Track.
func TestSettingsRemoveTrackRefusalSaysWhatToDoNext(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "j", "j", "tab", "x")
		v := strings.Join(lines, "\n")
		requireAll(t, v, "Remove Track", "SWE is used by 10 Applications", "Rename it with r", "move its Applications")
		if !strings.Contains(v, "╭") {
			t.Errorf("%dx%d: the refusal is not a pop-up:\n%s", size[0], size[1], v)
		}
	}
}

// The refusal names rename, so r acts on it directly instead of leaving the Owner stuck.
func TestSettingsRefusalOffersRenameDirectly(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE"]}`)
	h.tm.Type("x")
	requireAll(t, h.tm.plain(), "⚠ Remove Track", "r rename")
	h.tm.Type("r")
	v := h.tm.plain()
	if strings.Contains(v, "Remove Track") {
		t.Fatalf("the refusal stayed open:\n%s", v)
	}
	requireAll(t, v, "Rename Track", "enter review")
}

// Read-only rows wear a marker, so they are not mistaken for the editable ones beside them.
func TestSettingsReadOnlyRowsAreMarked(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "tab", "j", "j")
		if l := rowWith(lines, "Sources"); !strings.Contains(l, "(read-only)") {
			t.Errorf("%dx%d Sources: %q", size[0], size[1], l)
		}
		if l := rowWith(lines, "Blockers"); !strings.Contains(l, "(read-only)") {
			t.Errorf("%dx%d Blockers: %q", size[0], size[1], l)
		}
		lines = settingsAt(t, size[0], size[1], "j", "j", "j", "tab")
		if l := rowWith(lines, "Workspace"); !strings.Contains(l, "(read-only)") {
			t.Errorf("%dx%d Workspace: %q", size[0], size[1], l)
		}
	}
}

// An unset editable value shows an example of what to type, not a bare "not set".
func TestSettingsUnsetRowsHintWhatToEnter(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "j", "j", "j", "tab")
		v := strings.Join(lines, "\n")
		for _, want := range []string{"nvim, code --wait", "~/Downloads", "claude", "path to rolecall/plugin"} {
			if !strings.Contains(v, want) {
				t.Errorf("%dx%d: machine rows lack the %q hint:\n%s", size[0], size[1], want, v)
			}
		}
	}
	// Places is unset in an empty workspace.json, so it carries its own example.
	h := newSettingsHarness(t, 120, `{}`, nil)
	h.tm.Type("6")
	requireAll(t, h.tm.plain(), "Australia, APAC")
}

// The rename name step says what the next screen does: review the reach, then confirm.
func TestSettingsRenamePopupSaysReviewNotNext(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		f := footer(settingsAt(t, size[0], size[1], "j", "j", "tab", "r"))
		if !strings.Contains(f, "enter review") {
			t.Fatalf("%dx%d rename footer %q", size[0], size[1], f)
		}
		if strings.Contains(f, "enter next") {
			t.Fatalf("%dx%d rename still says next: %q", size[0], size[1], f)
		}
	}
}

// Every Settings save says so on the feedback line, so the Owner knows the write landed.
func TestSettingsSavesConfirmOnTheFeedbackLine(t *testing.T) {
	t.Run("gate", func(t *testing.T) {
		h := newSettingsHarness(t, 120, `{"letterWordCap":300}`, nil)
		h.tm.Type("6")
		h.tm.Type("j")
		h.tm.Type("tab")
		h.tm.Type("enter")
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), "Saved Cover letter word cap: 300")
	})
	t.Run("gate names the stored number", func(t *testing.T) {
		h := newSettingsHarness(t, 120, `{"letterWordCap":300}`, nil)
		h.tm.Type("6")
		h.tm.Type("j")
		h.tm.Type("tab")
		h.tm.Type("enter")
		for range 3 {
			h.tm.Type("backspace")
		}
		h.tm.Type("0300")
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), "Saved Cover letter word cap: 300")
		requireAll(t, h.tm.plain(), "300")
	})
	t.Run("search area", func(t *testing.T) {
		h := searchHarness(t, `{"search_area":{"places":["Australia"],"remote":"no"}}`)
		h.tm.Type("enter")
		h.tm.Type("enter")
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), "Saved Search area")
	})
	t.Run("machine", func(t *testing.T) {
		d, _ := machineDriver(t, 120)
		selectRow(d, 3, 1)
		d.Type("enter")
		for range 3 {
			d.Type("backspace")
		}
		d.Type("nano")
		d.Type("enter")
		requireAll(t, d.plain(), "Saved Editor: nano")
	})
	t.Run("add track", func(t *testing.T) {
		h := tracksHarness(t, `{"tracks":["SWE"]}`)
		h.tm.Type("a")
		h.tm.Type("Data")
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), "Added Track Data")
	})
	t.Run("remove track", func(t *testing.T) {
		h := tracksHarness(t, `{"tracks":["Spare"]}`)
		h.tm.Type("x")
		requireAll(t, h.tm.plain(), "Removed Track Spare")
	})
	t.Run("reorder tracks", func(t *testing.T) {
		h := tracksHarness(t, `{"tracks":["A","B"]}`)
		h.tm.Type("J")
		requireAll(t, h.tm.plain(), "Moved Track A")
	})
	t.Run("rename track", func(t *testing.T) {
		h := tracksHarness(t, `{"tracks":["SWE"]}`)
		h.tm.Type("r")
		for range 3 {
			h.tm.Type("backspace")
		}
		h.tm.Type("Software")
		h.tm.Type("enter")
		h.tm.Type("y")
		requireAll(t, h.tm.plain(), "Renamed SWE to Software")
	})
}
