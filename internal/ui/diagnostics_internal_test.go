package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/diag"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// diagLog opens a diagnostics log for one test and returns a func reading it back.
func diagLog(t *testing.T) func() string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "diagnostics.jsonl")
	if err := diag.Start(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(diag.Stop)
	return func() string {
		diag.Stop()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
}

// TestObserveRecordsWhatChangedNotWhatIsShown covers each entry the UI writes, and that
// an unchanged error and a quick update write nothing.
func TestObserveRecordsWhatChangedNotWhatIsShown(t *testing.T) {
	s, err := store.Open(filepath.Join("..", "..", "fixtures", "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	read := diagLog(t)
	before := model{store: s}
	after := model{store: s, screen: screenIndex(screenStats), width: 80, height: 24, err: errors.New("typst failed")}
	before.observe(after, tea.WindowSizeMsg{Width: 80, Height: 24}, 150*time.Millisecond)
	after.observe(after, tea.KeyPressMsg{}, time.Millisecond) // nothing changed
	log := read()
	for _, want := range []string{
		`"msg":"terminal","width":80,"height":24`,
		`"msg":"screen","name":"Stats"`,
		`"msg":"feedback_error","err":"typst failed"`,
		`"msg":"slow_update","msg":"tea.WindowSizeMsg"`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %s:\n%s", want, log)
		}
	}
	if n := strings.Count(log, "\n"); n != 4 {
		t.Errorf("%d entries, want 4 (the unchanged update adds none):\n%s", n, log)
	}
}

// TestGuardCmdRecordsAPanicInACommand checks the panic still reaches Bubble Tea's own
// recovery, which restores the terminal.
func TestGuardCmdRecordsAPanicInACommand(t *testing.T) {
	read := diagLog(t)
	if guardCmd(nil) != nil {
		t.Error("guardCmd(nil) is not nil")
	}
	cmd := guardCmd(func() tea.Msg { panic("in cmd") })
	func() {
		defer func() {
			if r := recover(); r != "in cmd" {
				t.Errorf("recovered %v, want the command's panic", r)
			}
		}()
		cmd()
	}()
	if log := read(); !strings.Contains(log, `"where":"command","value":"in cmd"`) {
		t.Errorf("panic not recorded:\n%s", log)
	}
}
