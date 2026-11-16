package ui_test

import (
	"strings"
	"testing"
)

// The form conventions every form pop-up shares: honest keys, calm required marks that
// turn only on the failing field, errors under the field, and esc that asks before it
// throws typed text away.

func addForm(t *testing.T, keys ...string) string {
	t.Helper()
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	for _, k := range append([]string{"a"}, keys...) {
		d.Type(k)
	}
	return d.plain()
}

func TestFormKeysSayWhatEnterDoes(t *testing.T) {
	v := addForm(t)
	requireAll(t, v, "* required", "enter next · ctrl+s save · ←→ change · esc cancel", "optional")
	if strings.Contains(v, "enter save") {
		t.Fatalf("the first field's enter moves on, but the hint says save:\n%s", v)
	}
	// On the last field enter saves, and says so.
	v = addForm(t, "tab", "tab", "tab", "tab", "tab")
	requireAll(t, v, "enter save · ctrl+s save")
}

func TestFormCtrlSShowsEveryMissingFieldUnderIt(t *testing.T) {
	v := addForm(t, "ctrl+s")
	requireAll(t, v, "✘ Title is required", "✘ Company is required")
	lines := strings.Split(v, "\n")
	for i, l := range lines {
		if strings.Contains(l, "Title*") && !strings.Contains(lines[i+1], "✘ Title is required") {
			t.Fatalf("the error is not under its field:\n%s", v)
		}
	}
}

func TestFormChecksAFieldsFormatOnLeavingIt(t *testing.T) {
	// From Title (where the form opens without a Plugin), Company, Track, then Closes: a bad
	// date is flagged as soon as tab leaves it.
	v := addForm(t, "tab", "tab", "tab", "31/12/2026", "tab")
	requireAll(t, v, "✘ Closes must be YYYY-MM-DD")
	// Passing an empty required field is fine: only saving asks for it.
	if strings.Contains(v, "Title is required") {
		t.Fatalf("leaving an empty required field flagged it:\n%s", v)
	}
}

func TestFormEscAsksBeforeDiscardingTypedText(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	d.Type("a")
	d.Type("esc")
	if strings.Contains(d.plain(), "Add a Lead") {
		t.Fatalf("esc on an untouched form did not close it:\n%s", d.plain())
	}
	d.Type("a")
	d.Type("tab")
	d.Type("Engineer")
	d.Type("esc")
	requireAll(t, d.plain(), "Discard changes?", "y discard")
	d.Type("n")
	requireAll(t, d.plain(), "Add a Lead", "Engineer", "esc cancel")
	d.Type("esc")
	d.Type("y")
	if strings.Contains(d.plain(), "Add a Lead") {
		t.Fatalf("y did not discard:\n%s", d.plain())
	}
}

func TestFormCtrlSSavesFromAnyField(t *testing.T) {
	h := searchHarness(t, `{"search_area":{"places":["Old"],"remote":"no"}}`)
	h.tm.Type("enter")
	h.tm.Type("right") // keep the selected value and type after it
	h.tm.Type("paste:, New")
	h.tm.Type("ctrl+s") // from the first of two fields
	requireAll(t, h.file(), `"Old"`, `"New"`)
	requireAll(t, h.tm.plain(), "Saved Search area · workspace.json")
}

func TestEditFormMarksChangedFields(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	d.Type("e")
	if strings.Contains(d.plain(), "Δ") {
		t.Fatalf("an untouched Edit form marks a field:\n%s", d.plain())
	}
	d.Type("\t") // past URL, the first field
	d.Type("x")
	if l := rowWith(strings.Split(d.plain(), "\n"), "Title"); !strings.Contains(l, "Δ") {
		t.Fatalf("the edited Title is not marked: %q", l)
	}
}

func TestSettingsUndoPutsTheOldValueBack(t *testing.T) {
	h := gatesHarness(t, `{"letterWordCap":350}`)
	h.tm.Type("enter")
	for range 3 {
		h.tm.Type("backspace")
	}
	h.tm.Type("300")
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "Saved Cover letter maximum: 300 words · workspace.json", "u undo")
	requireAll(t, h.file(), `"letterWordCap": 300`)
	h.tm.Type("u")
	requireAll(t, h.file(), `"letterWordCap": 350`)
	requireAll(t, h.tm.plain(), "Undone · Cover letter maximum back to 350 words")
	if strings.Contains(footer(strings.Split(h.tm.plain(), "\n")), "u undo") {
		t.Fatalf("u is still offered after the undo:\n%s", h.tm.plain())
	}
}

func TestSettingsUndoRestoresARemovedTrack(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["Spare","Other"]}`)
	h.tm.Type("x")
	if strings.Contains(h.file(), "Spare") {
		t.Fatalf("not removed: %s", h.file())
	}
	h.tm.Type("u")
	requireAll(t, h.file(), `"Spare"`, `"Other"`)
}

// Esc over typed text asks in the warning style, as strong as Docs not Ready: ⚠ in the
// title, what is lost said first, and no "* required" note.
func TestDiscardQuestionIsAWarning(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	d.Type("a")
	d.Type("Typed")
	d.Type("esc")
	v := d.plain()
	requireAll(t, v, "⚠ Discard changes?", "Your typed values will be lost.", "y discard")
	if strings.Contains(v, "* required") {
		t.Fatalf("the discard question still shows the required note:\n%s", v)
	}
}

// Editing a Settings value starts with it selected: typing replaces it instead of
// appending ("Australianano").
func TestSettingsPopupTypingReplacesTheValue(t *testing.T) {
	h := searchHarness(t, `{"search_area":{"places":["Australia"],"remote":"no"}}`)
	h.tm.Type("enter")
	h.tm.Type("nano")
	h.tm.Type("ctrl+s")
	if f := h.file(); !strings.Contains(f, `"nano"`) || strings.Contains(f, "Australia") {
		t.Fatalf("typing did not replace the value:\n%s", f)
	}
}
