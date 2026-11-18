package ui_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// workspaceFiles reads every file under root, so a test can tell whether anything was written.
func workspaceFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func quits(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, quit := cmd().(tea.QuitMsg)
	return quit
}

// While the too-small notice hides the list, keys must not act on it: nothing is written,
// the cursor stays put, and back at full size the screen is as it was.
func TestTooSmallIgnoresKeysThatWouldActOnTheHiddenList(t *testing.T) {
	for _, start := range [][]string{{"1"}, {"3"}, {"3", "enter"}} {
		root := fixtureCopy(t, nil)
		m := driveModel(t, driveStart(t, root, fixtureClock, 120, 36), start...)
		before, files := plain(m), workspaceFiles(t, root)

		m = runMsg(t, m, tea.WindowSizeMsg{Width: 48, Height: 12})
		m = driveModel(t, m, "j", "x", "m", "a", "enter", "/", "e", "down", "paste:pasted", "esc")
		m = runMsg(t, m, tea.WindowSizeMsg{Width: 120, Height: 36})

		if after := plain(m); after != before {
			t.Errorf("%v: screen changed while too small:\nbefore:\n%s\nafter:\n%s", start, before, after)
		}
		got := workspaceFiles(t, root)
		if len(got) != len(files) {
			t.Errorf("%v: %d files before, %d after", start, len(files), len(got))
		}
		for p, b := range files {
			if got[p] != b {
				t.Errorf("%v: %s was written while too small", start, p)
			}
		}
	}
}

// q and ctrl+c, the way out the notice names, still quit.
func TestTooSmallStillQuits(t *testing.T) {
	for _, k := range []tea.KeyPressMsg{{Code: 'q', Text: "q"}, {Code: 'c', Mod: tea.ModCtrl}} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 48, 12)
		if _, cmd := m.Update(k); !quits(cmd) {
			t.Errorf("%s at 48×12 did not quit", k.String())
		}
	}
}

// A q while too small never drops typed form text: it asks first, as ctrl+c does.
func TestTooSmallQuitKeepsTypedFormText(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	m = driveModel(t, m, "a", "Typed")
	m = runMsg(t, m, tea.WindowSizeMsg{Width: 48, Height: 12})
	next, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if quits(cmd) {
		t.Fatal("q at 48×12 dropped the typed form")
	}
	next = runMsg(t, next, tea.WindowSizeMsg{Width: 120, Height: 36})
	if v := plain(next); !strings.Contains(v, "Discard") || !strings.Contains(v, "Typed") {
		t.Fatalf("form text lost or no Discard? question:\n%s", v)
	}
}
