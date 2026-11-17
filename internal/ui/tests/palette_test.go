package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// The command palette lists the context's actions, filters them as the Owner types and
// runs the chosen one as if its key were pressed.
func TestCommandPaletteRunsTheChosenAction(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", ":")
	view := chipped(m.View().Content)
	for _, want := range []string{"move", "dismiss", "notes", "help", "enter run"} {
		if !strings.Contains(view, want) {
			t.Fatalf("palette lacks %q:\n%s", want, view)
		}
	}
	m = driveModel(t, m, "dism")
	view = chipped(m.View().Content)
	if !strings.Contains(view, "x      dismiss") || strings.Contains(view, "n      notes") {
		t.Fatalf("palette did not filter to dismiss:\n%s", view)
	}
	m = driveModel(t, m, "enter")
	if view := chipped(m.View().Content); !strings.Contains(view, "✓ Dismissed ") || strings.Contains(view, "enter run") {
		t.Fatalf("palette did not run dismiss and close:\n%s", view)
	}
}

func TestCommandPaletteOpensOnCtrlKAndClosesOnEsc(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "4")
	m = runMsg(t, m, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	view := chipped(m.View().Content)
	if !strings.Contains(view, "enter run") || !strings.Contains(view, "add") || strings.Contains(view, "dismiss") {
		t.Fatalf("ctrl+k palette on Stats should list Stats' actions only:\n%s", view)
	}
	m = driveModel(t, m, "zzz")
	if view := chipped(m.View().Content); !strings.Contains(view, `No command matches "zzz"`) {
		t.Fatalf("palette lacks its empty state:\n%s", view)
	}
	m = driveModel(t, m, "esc")
	if view := chipped(m.View().Content); strings.Contains(view, "enter run") {
		t.Fatalf("esc left the palette open:\n%s", view)
	}
}

// One command finder: space, : and ctrl+k open the same palette, which lists the screens and
// the row's own actions, all with lower-case descriptions (screen names keep their capital).
func TestSpaceOpensTheSamePaletteAsColon(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, size[0], size[1])
		m = driveModel(t, m, "1")
		viaSpace := chipped(driveModel(t, m, " ").View().Content)
		viaColon := chipped(driveModel(t, m, ":").View().Content)
		if viaSpace != viaColon || !strings.Contains(viaSpace, "╭─ : ") {
			t.Fatalf("%dx%d: space and : differ:\n%s\n%s", size[0], size[1], viaSpace, viaColon)
		}
		if strings.Contains(viaSpace, "Menu") {
			t.Fatalf("a second Menu is still drawn:\n%s", viaSpace)
		}
	}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 60)
	view := chipped(driveModel(t, m, "1", " ").View().Content)
	for _, want := range []string{"Drafts screen", "Settings screen", "move", "dismiss", "notes", "search"} {
		if !strings.Contains(view, want) {
			t.Errorf("finder lacks %q:\n%s", want, view)
		}
	}
	for _, bad := range []string{"m      Move", "x      Dismiss", "n      Notes", "/      Search"} {
		if strings.Contains(view, bad) {
			t.Errorf("finder has Title Case entry %q:\n%s", bad, view)
		}
	}
}

// The finder is searchable by screen name too, and runs the screen's key.
func TestPaletteSearchFindsAScreen(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	got := chipped(driveModel(t, m, " ", "stat", "enter").View().Content)
	if !strings.Contains(got, "Stats") || strings.Contains(got, "╭─ : ") {
		t.Fatalf("finder did not open Stats:\n%s", got)
	}
}

// breadcrumbLine is the line of the screen that carries the "›" breadcrumb.
func breadcrumbLine(view string) string {
	for _, l := range strings.Split(chipped(view), "\n") {
		if strings.Contains(l, "›") {
			return l
		}
	}
	return ""
}

// The palette never covers the breadcrumb, and its pop-up draws the title in the top border
// and esc in the bottom one.
func TestPaletteLeavesTheBreadcrumbAlone(t *testing.T) {
	for _, size := range [][2]int{{120, 36}, {60, 16}} {
		m := driveStart(t, fixtureCopy(t, nil), heldNow, size[0], size[1])
		before := m.View().Content
		open := driveModel(t, m, " ").View().Content
		if breadcrumbLine(before) == "" || breadcrumbLine(open) != breadcrumbLine(before) {
			t.Fatalf("%dx%d: palette covers the breadcrumb:\nbefore %q\nafter  %q", size[0], size[1], breadcrumbLine(before), breadcrumbLine(open))
		}
		if driveModel(t, m, " ", "esc").View().Content != before {
			t.Fatal("esc did not restore the screen")
		}
	}
}
