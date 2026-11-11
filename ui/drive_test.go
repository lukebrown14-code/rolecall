package ui_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

// This file drives a model without teatest: each Update is called directly and every
// command it returns is run and fed back, so tests never wait on streamed output and
// never sleep. Only the handful of smoke tests keep a whole teatest program.

// fakeExternal substitutes the suspend/run/resume boundary, completing at once.
func fakeExternal(t *testing.T) ui.ExternalRunner {
	t.Helper()
	return func(_ *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg { return cb(nil) }
	}
}

// driveStart opens the Workspace, builds a model with the fake external runner and the
// given clock, and sends the initial size.
func driveStart(t *testing.T, root string, now func() time.Time, w, h int, opts ...ui.Option) tea.Model {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	opts = append([]ui.Option{ui.WithExternalRunner(fakeExternal(t))}, opts...)
	m, err := ui.New(s, now, opts...)
	if err != nil {
		t.Fatal(err)
	}
	next, cmd := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return runCmd(t, next, cmd)
}

// driveModel runs keys against a model, expanding named keys and multi-character text.
func driveModel(t *testing.T, m tea.Model, keys ...string) tea.Model {
	t.Helper()
	for _, k := range keys {
		for _, msg := range keyMessages(k) {
			m = runMsg(t, m, msg)
		}
	}
	return m
}

// keyMessages turns a drive key name into the messages the program expects.
func keyMessages(k string) []tea.Msg {
	named := map[string]rune{"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "backspace": tea.KeyBackspace,
		"right": tea.KeyRight, "left": tea.KeyLeft, "up": tea.KeyUp, "down": tea.KeyDown, "\t": tea.KeyTab}
	if code, ok := named[k]; ok {
		return []tea.Msg{tea.KeyPressMsg{Code: code}}
	}
	if text, ok := strings.CutPrefix(k, "paste:"); ok {
		return []tea.Msg{tea.PasteMsg{Content: text}}
	}
	out := make([]tea.Msg, 0, len(k))
	for _, r := range k {
		out = append(out, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return out
}

// runMsg feeds one message to the model, running and feeding back every command.
func runMsg(t *testing.T, m tea.Model, msg tea.Msg) tea.Model {
	t.Helper()
	if msg == nil {
		return m
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = runCmd(t, m, c)
		}
		return m
	}
	next, cmd := m.Update(msg)
	return runCmd(t, next, cmd)
}

func runCmd(t *testing.T, m tea.Model, cmd tea.Cmd) tea.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	return runMsg(t, m, cmd())
}

// driver replaces teatest.TestModel in the harnesses: it drives the model directly and
// runs every command, so nothing is waited on and no test sleeps. It keeps the teatest
// method names the harnesses already use.
type driver struct {
	t *testing.T
	m tea.Model
}

func newDriver(t *testing.T, root string, now func() time.Time, w, h int, opts ...ui.Option) *driver {
	return &driver{t: t, m: driveStart(t, root, now, w, h, opts...)}
}

func (d *driver) Type(s string)    { d.m = driveModel(d.t, d.m, s) }
func (d *driver) Send(msg tea.Msg) { d.m = runMsg(d.t, d.m, msg) }
func (d *driver) key(code rune)    { d.Send(tea.KeyPressMsg{Code: code}) }

// View is the styled screen; plain is it with ANSI stripped.
func (d *driver) View() string  { return d.m.View().Content }
func (d *driver) plain() string { return ansi.Strip(d.View()) }

// FinalModel returns the current model, ignoring teatest's timeout options.
func (d *driver) FinalModel(...any) tea.Model { return d.m }

func TestDocsEmptyLabel(t *testing.T) {
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 40)
	m = driveModel(t, m, "2", "/docs-none", "enter", "enter", "right", "right", "right")
	v := ansi.Strip(m.View().Content)
	if !strings.Contains(v, "resume.md (empty)") {
		t.Fatalf("missing empty label:\n%s", v)
	}
}
