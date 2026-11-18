package ui_test

import (
	"testing"
)

// The fixture Profile's Experience entries keep their Dates and Canonical title on two
// lines, as the file has them, rather than one run-on line.
func TestProfileExperienceFactsReadAsTwoLines(t *testing.T) {
	v := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "5", "j", "j"))
	if hasLineWith(v, "Ongoing", "Canonical title") {
		t.Fatalf("Dates and Canonical title run together:\n%s", v)
	}
	if !hasLineWith(v, "Dates: 2023 – Ongoing") || !hasLineWith(v, "Canonical title: Developer") {
		t.Fatalf("Dates or Canonical title missing:\n%s", v)
	}
}
