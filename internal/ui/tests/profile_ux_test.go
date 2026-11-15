package ui_test

import (
	"strings"
	"testing"
)

// e opens the whole file, so Profile offers it once, as "e edit file", not on every field.
func TestProfileOffersEditFileOnce(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "5"))
	if n := strings.Count(view, "e edit"); n != 2 { // the section row and the key line
		t.Fatalf("e edit shown %d times:\n%s", n, view)
	}
	if n := strings.Count(view, "e edit file"); n != 2 {
		t.Fatalf("e edit file shown %d times:\n%s", n, view)
	}
}

// The key line follows focus: once tab moves to the content, j k scroll it.
func TestProfileKeyLineFollowsFocus(t *testing.T) {
	if line := lastLine(t, 120, 36, "5"); !strings.Contains(line, "j k section") {
		t.Fatalf("list focus: %q", line)
	}
	if line := lastLine(t, 120, 36, "5", "tab"); !strings.Contains(line, "j k scroll") {
		t.Fatalf("content focus: %q", line)
	}
}

// g and G go to the first and last section, as in lists.
func TestProfileGAndShiftGGoToTheEnds(t *testing.T) {
	view := chipped(runKeys(t, fixtureCopy(t, nil), 120, "5", "G"))
	if !strings.Contains(view, "▌ projects.md") {
		t.Fatalf("G did not reach the last entry:\n%s", view)
	}
	view = chipped(runKeys(t, fixtureCopy(t, nil), 120, "5", "G", "g"))
	if !strings.Contains(view, "▌ Contact") {
		t.Fatalf("g did not reach the first section:\n%s", view)
	}
}
