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
	m, err := ui.New(s, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	wait := func(s string) {
		t.Helper()
		teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte(s)) }, teatest.WithDuration(3*time.Second))
	}
	wait("Leads (4)")
	tm.Type("2")
	wait("Draft (2)")
	tm.Type("3")
	wait("Closed (4)")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	tm.Type("?")
	wait("Full help")
	tm.Type("4fpb")
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
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
	wait("Leads (4)")
	tm.Type("/zzznomatch")
	wait("Leads (0)")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEnter})
	tm.Type("2")
	wait("Draft")
	tm.Type("r")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEscape})
	wait("Draft (2)")
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
