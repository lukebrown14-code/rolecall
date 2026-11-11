package ui_test

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveEditor(t *testing.T) {
	for _, tc := range []struct{ config, visual, editor, want string }{{"code -w", "vim", "nano", "code -w"}, {"", "vim", "nano", "vim"}, {"", "", "nano", "nano"}, {"", "", "", "vi"}} {
		t.Setenv("VISUAL", tc.visual)
		t.Setenv("EDITOR", tc.editor)
		if got := ui.ResolveEditor(tc.config); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
	}
}

func TestNotesEditor(t *testing.T) {
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
	var folder string
	for _, a := range apps {
		if a.Status == "Lead" {
			folder = a.Folder
			break
		}
	}
	os.Remove(filepath.Join(folder, "notes.md"))
	m, err := ui.New(s, nil, ui.WithExternalRunner(func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			path := c.Args[len(c.Args)-1]
			if _, err := os.Stat(path); err != nil {
				return done(err)
			}
			err := os.WriteFile(path, []byte("Owner's notes"), 0600)
			return done(err)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(120, 40))
	tm.Type("n")
	teatest.WaitFor(t, tm.Output(), func(_ []byte) bool {
		b, _ := os.ReadFile(filepath.Join(folder, "notes.md"))
		return string(b) == "Owner's notes"
	}, teatest.WithDuration(3*time.Second))
	tm.Type("q")
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
	a, err := s.Rescan(folder)
	if err != nil || !a.HasNotes {
		t.Fatalf("notes: %v %v", a, err)
	}
}
