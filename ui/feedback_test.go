package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/ui"
)

// recordEditor is an external runner that records each command and changes nothing.
func recordEditor(got *[]string) ui.ExternalRunner {
	return func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			*got = append(*got, strings.Join(c.Args, " "))
			return done(nil)
		}
	}
}

// A malformed row is titled by what is wrong, and e on it — or on the error it causes —
// opens its meta.yml in the Editor at the failing line, writing nothing itself.
func TestMalformedRowOpensTheFailingLineInTheEditor(t *testing.T) {
	root := fixtureCopy(t, nil)
	path := filepath.Join(root, "jobs", "SWE", "malformed", "meta.yml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, keys := range [][]string{{"G", "e"}, {"G", "m", "d", "e"}, {"G", "enter", "e"}} {
		var got []string
		m := driveStart(t, root, fixtureClock, 120, 36, ui.WithEditor("vim"), ui.WithExternalRunner(recordEditor(&got)))
		if view := ansi.Strip(m.View().Content); !strings.Contains(view, "⚠ Can't read SWE/malformed") {
			t.Fatalf("malformed row is not titled by its problem:\n%s", view)
		}
		m = driveModel(t, m, keys...)
		if len(got) != 1 || !strings.Contains(got[0], "exec vim +4 \"$1\"") || !strings.HasSuffix(got[0], path) {
			t.Fatalf("%v: editor commands %q, want vim +4 on %s", keys, got, path)
		}
		if view := ansi.Strip(m.View().Content); strings.Contains(view, "Can't read SWE/malformed ·") {
			t.Fatalf("%v: the error stayed after e opened the file:\n%s", keys, view)
		}
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatal("e wrote to the malformed meta.yml")
	}
}

// The error line keeps its keys whole and cuts the message instead.
func TestErrorLineCutsTheMessageNotTheKeys(t *testing.T) {
	for _, w := range []int{120, 80, 60} {
		view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), w, 24, "G", "m", "d"))
		last := view[strings.LastIndex(view, "\n")+1:]
		if !strings.HasSuffix(strings.TrimRight(last, " "), "e fix in editor  esc hide") {
			t.Fatalf("%d: error keys cut or missing: %q", w, last)
		}
		if !strings.HasPrefix(strings.TrimSpace(last), "✕ Can't read SWE/malformed") {
			t.Fatalf("%d: error message missing: %q", w, last)
		}
	}
}
