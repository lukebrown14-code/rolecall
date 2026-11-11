package ui_test

import (
	"bytes"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFixtureTables(t *testing.T) {
	for _, tc := range []struct {
		name, screen string
		tabs         int
	}{{"Leads", "1", 0}, {"Dismissed", "1", 1}, {"Draft", "2", 0}, {"All", "3", 0}, {"Applied", "3", 1}, {"Interviewing", "3", 2}, {"Offer", "3", 3}, {"Closed", "3", 4}} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
				t.Fatal(err)
			}
			s, err := store.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			m, err := ui.New(s, fixtureClock)
			if err != nil {
				t.Fatal(err)
			}
			tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
			teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("Leads 4")) })
			tm.Type(tc.screen)
			for i := 0; i < tc.tabs; i++ {
				tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
			}
			tm.Type("q")
			final := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second))
			teatest.RequireEqualOutput(t, []byte(final.View().Content))
		})
	}
}

func TestMalformedMovePreservesBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	var path string
	for _, a := range apps {
		if a.Malformed != nil {
			path = filepath.Join(a.Folder, "meta.yml")
		}
	}
	if path == "" {
		t.Fatal("missing malformed fixture")
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, nil)
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("⚠")) })
	tm.Type("Gm")
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("Move")) })
	tm.Type("d")
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("/malformed/meta.y")) })
	tm.Type("q")
	io.ReadAll(tm.FinalOutput(t, teatest.WithFinalTimeout(3*time.Second)))
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Move changed malformed file")
	}
}
