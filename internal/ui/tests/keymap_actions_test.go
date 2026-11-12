package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// Every row action the list help advertises is one the screen runs: pressing each key from
// a fresh Leads screen changes the frame or opens a pop-up, so the help never lists a dead
// key. The help and the dispatcher read one action table, so they cannot drift apart.
func TestListHelpListsOnlyKeysThatRun(t *testing.T) {
	for _, key := range []string{"enter", "m", "x", "a", "/", "s"} {
		base := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1"))
		got := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1", key))
		if got == base {
			t.Fatalf("the help lists %q but it does nothing:\n%s", key, got)
		}
	}
}

// The key line and the ? help draw the same keys: every action the key line shows is one
// the help documents, because both come from the one action table.
func TestKeyLineKeysAreAllDocumentedInTheHelp(t *testing.T) {
	line := keyLineOf(runKeys(t, fixtureCopy(t, nil), 120, "1"))
	help := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1", "?"))
	for _, shown := range []string{"enter open", "m move", "a add", "/ search"} {
		if !strings.Contains(line, shown) {
			t.Fatalf("the key line does not show %q:\n%s", shown, line)
		}
		if !strings.Contains(help, shown) {
			t.Fatalf("the key line shows %q but the help omits it:\n%s", shown, help)
		}
	}
}
