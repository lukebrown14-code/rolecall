package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The bottom rule and the footer sit on the same rows on every screen: the breadcrumb
// line above the keys never comes and goes.
func TestFooterStaysPutAcrossScreens(t *testing.T) {
	for _, key := range []string{"1", "3", "4", "5", "6"} {
		lines := plainAt(t, 80, 24, key)
		if !strings.HasPrefix(lines[21], "────") || !strings.Contains(lines[22], " › ") && !strings.Contains(lines[22], "Profile") {
			t.Errorf("screen %s: bottom rule or breadcrumb moved:\n%s", key, strings.Join(lines, "\n"))
		}
	}
}

// Opening the detail at wide sizes keeps the divider where the list's was, the card on
// the right where the preview stood.
func TestWideDetailKeepsTheDividerAndCardInPlace(t *testing.T) {
	list := plainAt(t, 120, 36, "3", "j")
	detail := plainAt(t, 120, 36, "3", "j", "enter")
	col := func(lines []string) int { return strings.Index(lines[3], "┬") }
	if col(list) < 0 || col(list) != col(detail) {
		t.Fatalf("divider moved from %d to %d:\n%s", col(list), col(detail), strings.Join(detail, "\n"))
	}
	if !strings.Contains(lineWith(strings.Join(detail, "\n"), "Engineer #1  "), "│ Engineer #1") {
		t.Fatalf("card is not on the right:\n%s", strings.Join(detail, "\n"))
	}
}

// At narrow widths a toast takes the whole footer line rather than both being cut, and
// the Status it names comes first, so it survives.
func TestNarrowToastTakesTheFooterLine(t *testing.T) {
	lines := plainAt(t, 56, 14, "1", "j", "j", "m", "d")
	crumb := lines[len(lines)-2]
	if strings.Contains(crumb, "Leads ›") || !strings.Contains(crumb, "✓ Moved to Draft · Platform") {
		t.Fatalf("toast squeezed beside the breadcrumb: %q", crumb)
	}
}

// Pop-up hints that do not fit drop whole, before esc, rather than being cut.
func TestPopupHintsShedRatherThanCut(t *testing.T) {
	view := strings.Join(plainAt(t, 56, 14, "a"), "\n")
	border := lineWith(view, "╰─")
	if strings.Contains(border, "…") || !strings.Contains(border, "esc cancel") {
		t.Fatalf("hints cut: %q", border)
	}
}

// On a short terminal the Move pop-up keeps which Application it moves in view.
func TestMovePopupPinsItsSubject(t *testing.T) {
	view := strings.Join(plainAt(t, 56, 14, "1", "j", "j", "m", "j", "j", "j", "j"), "\n")
	if !strings.Contains(view, "Platform Engineer") || !strings.Contains(view, "Fern Systems · now") {
		t.Fatalf("Move pop-up scrolled its subject away:\n%s", view)
	}
}

// A long preview title fills its two lines and ends in "…", never on a lone "…" line.
func TestPreviewTitleCutsItsSecondLine(t *testing.T) {
	long := "Senior Staff Principal Distinguished Software Engineering Manager for Platform Infrastructure Reliability"
	root := fixtureCopy(t, func(root string) {
		meta := filepath.Join(root, "jobs", "SWE", "orbit", "meta.yml")
		b, _ := os.ReadFile(meta)
		os.WriteFile(meta, []byte(strings.Replace(string(b), "title: Software Engineer", "title: "+long, 1)), 0o600)
	})
	view := chipped(runKeysSized(t, root, 100, 30, "3"))
	for _, l := range strings.Split(view, "\n") {
		if i := strings.Index(l, "│ "); i >= 0 && strings.TrimSpace(l[i+len("│ "):]) == "…" {
			t.Fatalf("title ends on a lone …:\n%s", view)
		}
	}
	if !strings.Contains(view, "Distinguished") {
		t.Fatalf("title's second line missing:\n%s", view)
	}
}

// A Track list rolecall cannot use shows on Settings with its path, the reason and e,
// as a broken file does, rather than an empty screen.
func TestSettingsShowsABadTrackList(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		p := filepath.Join(root, "workspace.json")
		b, _ := os.ReadFile(p)
		os.WriteFile(p, []byte(strings.Replace(string(b), `"tracks":["SWE","Platform"]`, `"tracks":["SWE","SWE"]`, 1)), 0o600)
	})
	view := chipped(runKeysSized(t, root, 100, 24, "6"))
	for _, want := range []string{"workspace.json lists a Track rolecall can't use", "Track SWE is listed twice", "Press e to fix it"} {
		if !strings.Contains(view, want) {
			t.Fatalf("Settings lacks %q:\n%s", want, view)
		}
	}
}

// An empty Profile file says how to fill it, and a missing one how to create it.
func TestProfileEmptyStatesSayHowToFixThem(t *testing.T) {
	empty := fixtureCopy(t, func(root string) {
		os.WriteFile(filepath.Join(root, "profile", "master-profile.md"), nil, 0o600)
	})
	if v := chipped(runKeysSized(t, empty, 100, 24, "5")); !strings.Contains(v, "Nothing here yet · e to write it") {
		t.Fatalf("empty Profile says nothing:\n%s", v)
	}
	missing := fixtureCopy(t, func(root string) { os.RemoveAll(filepath.Join(root, "profile")) })
	if v := chipped(runKeysSized(t, missing, 100, 24, "5")); !strings.Contains(v, "then press r to read it") {
		t.Fatalf("missing Profile says nothing:\n%s", v)
	}
}

// The medium bottom sheet lines up with the list: its text at the list's text column, and
// its list part shows the selected row's section with one cue line, no blank above it.
func TestBottomSheetSharesTheListsColumns(t *testing.T) {
	lines := plainAt(t, 80, 24, "3", "j", "enter")
	view := strings.Join(lines, "\n")
	for _, want := range []string{"   Engineer #1 · Example", "  [Overview]", "   ◆ Interviewing  "} {
		if !strings.Contains(view, want) {
			t.Fatalf("sheet lacks %q at the text column:\n%s", want, view)
		}
	}
	if strings.Contains(view, "↑ 1 more\n") || strings.Count(view, "more") > 2 {
		t.Fatalf("sheet's list part spends lines on cues:\n%s", view)
	}
}
