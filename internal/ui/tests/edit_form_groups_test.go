package ui_test

import (
	"strings"
	"testing"
)

// fieldText is what a form row shows after its label: the line holding label, from the
// first cell after it, without the pop-up's right border.
func fieldText(view, label string) string {
	l := lineWith(view, label)
	i := strings.Index(l, label)
	if i < 0 {
		return ""
	}
	rest := l[i+len(label):]
	if j := strings.Index(rest, "│"); j >= 0 {
		rest = rest[:j]
	}
	return strings.TrimSpace(strings.TrimLeft(rest, "*"))
}

// e on an Application whose card shows a NEXT STEP row opens the Edit form on that
// field, from the list or the detail, with the saved step as it would be typed and its
// echo; the fields sit under headings named as the card's sections.
func TestEditOpensOnTheNextStepAsTyped(t *testing.T) {
	for _, viaDetail := range []bool{false, true} {
		root := stepWorkspace(t, "next_step: {at: 2026-10-04T10:30, what: \"Phone screen\"}\n")
		d := newDriver(t, root, fixtureClock, 120, 36)
		d.Type("3")
		d.Type("/stepper")
		d.Type("enter")
		if viaDetail {
			d.Type("enter")
		}
		d.Type("e")
		v := d.plain()
		requireAll(t, v, "ROLE", "NEXT STEP", "CONTACT")
		when := lineWith(v, "When")
		if !strings.Contains(when, "▌") || !strings.Contains(when, "4 Oct 10:30") || !strings.Contains(when, "→ Sun 4 Oct 2026 10:30") {
			t.Fatalf("detail %v: the form is not on the next step, shown as typed:\n%s", viaDetail, v)
		}
		if strings.Contains(v, "2026-10-04T10:30") {
			t.Errorf("detail %v: the next step reopens as ISO:\n%s", viaDetail, v)
		}
		// The saved value is selected: typing replaces it.
		d.Type("fri 9:00")
		if !strings.Contains(d.plain(), "→ Fri 2 Oct 2026 09:00") {
			t.Fatalf("detail %v: fri 9:00 not echoed:\n%s", viaDetail, d.plain())
		}
		d.Type("ctrl+s")
		if got := metaText(t, root, "stepper"); !strings.Contains(got, "next_step: {at: 2026-10-02T09:00, what: \"Phone screen\"}\n") {
			t.Fatalf("detail %v: fri 9:00 not saved as Fri 2 Oct 09:00:\n%s", viaDetail, got)
		}
	}
}

// An untouched next step shown as typed saves nothing new, even one in the past, whose
// day and month alone would read as next year.
func TestEditKeepsAnUntouchedNextStep(t *testing.T) {
	root := stepWorkspace(t, "next_step: {at: 2026-09-20, what: \"Call\"}\n")
	d := newDriver(t, root, fixtureClock, 120, 36)
	editStepper(d)
	if when := fieldText(d.plain(), "When"); !strings.HasPrefix(when, "20 Sep 2026") {
		t.Fatalf("a past next step lost its year: %q", when)
	}
	d.Type("ctrl+s")
	if v := d.plain(); !strings.Contains(v, "Nothing changed") {
		t.Fatalf("an untouched Edit changed something:\n%s", v)
	}
}

// Closes and the next step offer the same date forms, and an empty optional field is
// drawn the same in Add and Edit: its placeholder, never an underline.
func TestAddAndEditDrawEmptyFieldsAlike(t *testing.T) {
	add := addForm(t)
	d := newDriver(t, stepWorkspace(t, ""), fixtureClock, 120, 40)
	editStepper(d)
	edit := d.plain()
	const hint = "tomorrow · fri · +7d · 30 Nov · 2026-12-01"
	for name, got := range map[string]string{
		"Add Closes": fieldText(add, "Closes"), "Edit Closes": fieldText(edit, "Closes"), "Edit When": fieldText(edit, "When"),
	} {
		if got != hint {
			t.Errorf("%s shows %q, want %q", name, got, hint)
		}
	}
	if a, e := fieldText(add, "URL"), fieldText(edit, "URL"); a != e {
		t.Errorf("an empty URL differs: Add %q, Edit %q", a, e)
	}
	for name, v := range map[string]string{"Add": add, "Edit": edit} {
		if strings.Contains(v, "___") {
			t.Errorf("%s draws an empty field as an underline:\n%s", name, v)
		}
	}
}

// An empty Title gets exactly one message, under it.
func TestEmptyTitleShowsOneError(t *testing.T) {
	v := addForm(t, "ctrl+s")
	if n := strings.Count(v, "Title is required"); n != 1 {
		t.Fatalf("an empty Title shows %d messages:\n%s", n, v)
	}
	d := newDriver(t, stepWorkspace(t, ""), fixtureClock, 120, 40)
	editStepper(d)
	for range 6 { // When … Title
		d.Type("up")
	}
	d.Type("ctrl+u")
	d.Type("ctrl+s")
	v = d.plain()
	if n := strings.Count(v, "Title is required"); n != 1 || strings.Count(v, "✘") != 1 {
		t.Fatalf("an emptied Title shows %d messages:\n%s", n, v)
	}
}
