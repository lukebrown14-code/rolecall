package ui_test

import (
	"strings"
	"testing"
)

// While a text filter is on, its chip shows on every screen, the detail view included,
// and esc clears it.
func TestSearchChipShowsOnEveryScreen(t *testing.T) {
	for _, keys := range [][]string{{"1"}, {"3", "enter"}, {"4"}, {"5"}, {"6"}} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
		m = driveModel(t, m, "3", "/en", "enter")
		m = driveModel(t, m, keys...)
		if view := chipped(m.View().Content); !strings.Contains(view, "/en ×") {
			t.Fatalf("%v: search chip missing:\n%s", keys, view)
		}
	}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "/en", "enter", "4", "esc")
	if view := chipped(m.View().Content); strings.Contains(view, "/en ×") {
		t.Fatalf("esc on Stats left the search on:\n%s", view)
	}
}

// A malformed row matches a search only by its folder, not by the partial fields its
// broken meta.yml happened to yield.
func TestSearchSkipsMalformedRowsThatDoNotMatch(t *testing.T) {
	// The malformed fixture's company is Example, which contains "pl".
	if view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "/pl", "enter")); strings.Contains(view, "Can't read") {
		t.Fatalf("malformed row shown for a query it does not match:\n%s", view)
	}
	if view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1", "/malformed", "enter")); !strings.Contains(view, "Can't read SWE/malformed") {
		t.Fatalf("malformed row hidden from a search for its folder:\n%s", view)
	}
}
