package ui_test

import (
	"strings"
	"testing"
)

// The Track removal refusal is a warning pop-up next to the rows, not a footer error, and
// it says what to do about the Applications still on the Track.
func TestSettingsRemoveTrackRefusalSaysWhatToDoNext(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "]", "]", "x")
		v := strings.Join(lines, "\n")
		requireAll(t, v, "Can't remove Track SWE", "SWE is used by 10 Applications", "incl. 1 Dismissed", "Move its Applications to another Track")
		if strings.Contains(v, "rename") {
			t.Errorf("%dx%d: the refusal offers rename, which does not remove anything:\n%s", size[0], size[1], v)
		}
		if !strings.Contains(v, "╭") {
			t.Errorf("%dx%d: the refusal is not a pop-up:\n%s", size[0], size[1], v)
		}
	}
}

// The refusal counts what the lists make easy to miss, so its number adds up: SWE's ten
// Applications include one Dismissed and one whose meta.yml cannot be read.
func TestSettingsRefusalCountAddsUp(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE"]}`)
	h.tm.Type("x")
	requireAll(t, h.tm.plain(), "⚠ Can't remove Track SWE", "10 Applications,", "incl. 1 Dismissed, 1 unreadable.", "esc close")
	h.tm.Type("esc")
	if strings.Contains(h.tm.plain(), "Can't remove") {
		t.Fatalf("esc left the refusal open:\n%s", h.tm.plain())
	}
}

// Read-only rows say so, and why, so they are not mistaken for the editable ones beside
// them; the cursor passes over them, so enter never lands on one and does nothing.
func TestSettingsReadOnlyRowsAreMarked(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "j", "j")
		if l := rowWith(lines, "Remote roles"); !strings.Contains(l, "▌ Remote roles") {
			t.Errorf("%dx%d the cursor left Remote roles for a read-only row: %q", size[0], size[1], l)
		}
		if l := rowWith(lines, "Sources"); !strings.Contains(l, "read-only") {
			t.Errorf("%dx%d Sources: %q", size[0], size[1], l)
		}
		if l := rowWith(lines, "Blockers"); !strings.Contains(l, "read-only") {
			t.Errorf("%dx%d Blockers: %q", size[0], size[1], l)
		}
		lines = settingsAt(t, size[0], size[1], "]", "]", "]")
		if v := strings.Join(lines, "\n"); !strings.Contains(v, "fixed this session") {
			t.Errorf("%dx%d Workspace lacks its reason:\n%s", size[0], size[1], v)
		}
	}
}

// An unset editable value shows an example of what to type, not a bare "not set".
func TestSettingsUnsetRowsHintWhatToEnter(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		lines := settingsAt(t, size[0], size[1], "]", "]", "]")
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
		f := footer(settingsAt(t, size[0], size[1], "]", "]", "r"))
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
		h.tm.Type("]")
		h.tm.Type("enter")
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), "Saved Cover letter maximum: 300 words")
	})
	t.Run("gate names the stored number", func(t *testing.T) {
		h := newSettingsHarness(t, 120, `{"letterWordCap":300}`, nil)
		h.tm.Type("6")
		h.tm.Type("]")
		h.tm.Type("enter")
		for range 3 {
			h.tm.Type("backspace")
		}
		h.tm.Type("0300")
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), "Saved Cover letter maximum: 300 words")
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
		selectRow(d, 3, 0)
		d.Type("enter")
		for range 3 {
			d.Type("backspace")
		}
		d.Type("nano")
		d.Type("enter")
		d.Type("ctrl+s")
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
