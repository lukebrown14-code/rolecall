package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The command palette lists the context's actions, filters them as the Owner types and
// runs the chosen one as if its key were pressed.
func TestCommandPaletteRunsTheChosenAction(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "1", ":")
	view := ansi.Strip(m.View().Content)
	for _, want := range []string{"move", "dismiss", "notes", "help", "enter run"} {
		if !strings.Contains(view, want) {
			t.Fatalf("palette lacks %q:\n%s", want, view)
		}
	}
	m = driveModel(t, m, "dism")
	view = ansi.Strip(m.View().Content)
	if !strings.Contains(view, "x      dismiss") || strings.Contains(view, "notes") {
		t.Fatalf("palette did not filter to dismiss:\n%s", view)
	}
	m = driveModel(t, m, "enter")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "✓ Dismissed ") || strings.Contains(view, "enter run") {
		t.Fatalf("palette did not run dismiss and close:\n%s", view)
	}
}

func TestCommandPaletteOpensOnCtrlKAndClosesOnEsc(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "4")
	m = runMsg(t, m, tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	view := ansi.Strip(m.View().Content)
	if !strings.Contains(view, "enter run") || !strings.Contains(view, "add") || strings.Contains(view, "dismiss") {
		t.Fatalf("ctrl+k palette on Stats should list Stats' actions only:\n%s", view)
	}
	m = driveModel(t, m, "zzz")
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, `No command matches "zzz"`) {
		t.Fatalf("palette lacks its empty state:\n%s", view)
	}
	m = driveModel(t, m, "esc")
	if view := ansi.Strip(m.View().Content); strings.Contains(view, "enter run") {
		t.Fatalf("esc left the palette open:\n%s", view)
	}
}
