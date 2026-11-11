package ui_test

import (
	"bytes"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"os"
	"testing"
	"time"
)

func TestNavigation(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	wait := func(s string) {
		t.Helper()
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(s)) }, teatest.WithDuration(3*time.Second))
	}
	wait("Leads 4")
	tm.Type("2")
	wait("Draft 2")
	tm.Type("3")
	wait("Closed 4")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Type("?")
	wait("Keys")
	tm.Type("4fpb")
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}

func TestDismissRestoreAndUndo(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := s.Scan()
	m, err := ui.New(s, func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	wait := func(text string) {
		t.Helper()
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(text)) }, teatest.WithDuration(3*time.Second))
	}
	wait("Leads 4")
	tm.Type("x")
	wait("→ Dismissed")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	wait("dismissed today") // only the Dismissed view shows the row just dismissed
	tm.Type("m")            // the row just dismissed sorts first: newest first
	wait("Move")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	wait("→ Lead")
	// The renderer redraws only changed cells, so undo is awaited on disk rather than on screen.
	logLen := func() int {
		total := 0
		apps, _ := s.Scan()
		for _, a := range apps {
			total += len(a.StatusLog)
		}
		return total
	}
	waitLog := func(n int) {
		t.Helper()
		for deadline := time.Now().Add(3 * time.Second); logLen() < n; time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("status_log never reached %d entries", n)
			}
		}
	}
	start := logLen()
	tm.Type("u")
	waitLog(start + 1)
	tm.Type("u")
	waitLog(start + 2)
	tm.Type("u")
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	after, _ := s.Scan()
	for i, a := range after {
		if len(a.StatusLog) != len(before[i].StatusLog) {
			if a.Status != "Lead" || len(a.StatusLog) != len(before[i].StatusLog)+4 {
				t.Fatalf("unexpected history: %+v", a)
			}
			return
		}
	}
	t.Fatal("no status history appended")
}

func TestFilters(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	wait := func(want string) {
		t.Helper()
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(want)) }, teatest.WithDuration(3*time.Second))
	}
	wait("Leads 4")
	tm.Type("/zzznomatch")
	wait("No matches")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Type("2")
	wait("Draft")
	tm.Type("r")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	wait("Draft 2")
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
