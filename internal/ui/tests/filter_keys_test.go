package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// s is the status filter on every list screen: it cycles through the screen's views.
func TestSStatusFilterCyclesViews(t *testing.T) {
	root := fixtureCopy(t, nil)
	if v := chipped(runKeys(t, root, 120, "1", "s")); !strings.Contains(v, "Leads › Dismissed") {
		t.Fatalf("s on Leads did not switch to Dismissed:\n%s", v)
	}
	if v := chipped(runKeys(t, root, 120, "1", "s", "s")); !strings.Contains(v, "Leads › All tracks") {
		t.Fatalf("s did not wrap back to Leads:\n%s", v)
	}
	if v := chipped(runKeys(t, root, 120, "3", "s")); !strings.Contains(v, "Applied › All tracks · ") {
		t.Fatalf("s on the Applied screen did not step from All:\n%s", v)
	}
}

// o toggles the Leads sort; elsewhere it does nothing.
func TestOTogglesLeadsSort(t *testing.T) {
	root := fixtureCopy(t, fernCloses("2026-09-01"))
	v := chipped(runKeys(t, root, 120, "1", "o"))
	if !strings.Contains(v, "Leads newest first") {
		t.Fatalf("o did not sort newest:\n%s", v)
	}
	base := chipped(runKeys(t, root, 120, "3"))
	if got := chipped(runKeys(t, root, 120, "3", "o")); got != base {
		t.Fatalf("o changed the Applied screen:\n%s", got)
	}
}

// M is not bound.
func TestCapitalMOpensNothing(t *testing.T) {
	root := fixtureCopy(t, nil)
	base := chipped(runKeys(t, root, 120, "1"))
	if got := chipped(runKeys(t, root, 120, "1", "M")); got != base {
		t.Fatalf("M changed the screen:\n%s", got)
	}
}

// The palette finds the filter and the sort by name.
func TestPaletteFindsStatusFilterAndSort(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, q := range []string{"status filter", "sort"} {
		v := chipped(runKeys(t, root, 120, "1", ":", q))
		if !strings.Contains(v, q) || strings.Contains(v, "No matching") {
			t.Fatalf("palette lacks %q:\n%s", q, v)
		}
	}
}

// Help has three sections, keys as chips, and no internal wording.
func TestHelpSectionsChipsAndWording(t *testing.T) {
	v := chipped(runKeys(t, fixtureCopy(t, nil), 140, "1", "?"))
	for _, want := range []string{"NAVIGATE", "ACT", "FILTER & FIND", "[s]", "[o]", "[t]", "[/]", "[: ctrl+k]", "[m]"} {
		if !strings.Contains(v, want) {
			t.Errorf("help lacks %q:\n%s", want, v)
		}
	}
	for _, bad := range []string{"Supporting screen", "prototype", "placeholder", "lorem", "TODO"} {
		if strings.Contains(strings.ToLower(v), strings.ToLower(bad)) {
			t.Errorf("help contains %q:\n%s", bad, v)
		}
	}
	if !helpLists(v, "s status filter") || !helpLists(v, "o sort newest") {
		t.Errorf("help lacks the new keys:\n%s", v)
	}
}

// At 60x16 Help stays inside its box and the screen.
func TestHelpFitsAt60x16(t *testing.T) {
	v := chipped(runKeysSized(t, fixtureCopy(t, nil), 60, 16, "1", "?"))
	lines := strings.Split(v, "\n")
	if len(lines) > 16 {
		t.Fatalf("help is %d lines:\n%s", len(lines), v)
	}
	for _, l := range lines {
		if ansi.StringWidth(l) > 60 {
			t.Fatalf("line %q is %d wide:\n%s", l, ansi.StringWidth(l), v)
		}
	}
}
