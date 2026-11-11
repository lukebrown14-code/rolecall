package ui

import "testing"

// TestScreenDefsAreTheWholeDeclaration pins the one place a screen declares itself:
// its name, the views in it, and whether it holds Application tables. A screen without
// them owns its view, keys and help, so the shared layout needs no check for Stats.
func TestScreenDefsAreTheWholeDeclaration(t *testing.T) {
	for _, d := range screenDefs {
		if !d.hasTables {
			if d.render == nil || d.key == nil || d.help == nil {
				t.Errorf("%s has no Application tables but leaves its view, keys or help undeclared", d.name)
			}
			continue
		}
		if len(d.tables) == 0 {
			t.Errorf("%s has Application tables but declares no views", d.name)
		}
	}
}

// TestScreenWithoutTablesHasNoRowsOrViewRow checks the layout question is answered from
// the declaration, not from the screen's number.
func TestScreenWithoutTablesHasNoRowsOrViewRow(t *testing.T) {
	for i, d := range screenDefs {
		if d.hasTables {
			continue
		}
		m := model{screen: i}
		if rows := m.rows(); rows != nil {
			t.Errorf("%s: %d rows, want none", d.name, len(rows))
		}
		if row := m.viewRow(basePal); row != "" {
			t.Errorf("%s: view row %q, want empty", d.name, row)
		}
	}
}
