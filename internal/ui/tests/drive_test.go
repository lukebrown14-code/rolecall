package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui"
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

// heldTick is a ui.Ticker that never fires, so nothing waits on a timer.
func heldTick(time.Duration, func(time.Time) tea.Msg) tea.Cmd { return nil }

// driveStart opens the Workspace, builds a model with the fake external runner and the
// given clock, and sends the initial size.
func driveStart(t *testing.T, root string, now func() time.Time, w, h int, opts ...ui.Option) tea.Model {
	t.Helper()
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// The row flash never ends on its own here: a test fires it with its own WithTicker.
	opts = append([]ui.Option{ui.WithExternalRunner(fakeExternal(t)), ui.WithTicker(heldTick)}, opts...)
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
	if k == "ctrl+s" {
		return []tea.Msg{tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}}
	}
	if k == "ctrl+z" {
		return []tea.Msg{tea.KeyPressMsg{Code: 'z', Mod: tea.ModCtrl}}
	}
	if k == "ctrl+u" {
		return []tea.Msg{tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}}
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
func (d *driver) plain() string { return chipped(d.View()) }

// FinalModel returns the current model, ignoring teatest's timeout options.
func (d *driver) FinalModel(...any) tea.Model { return d.m }

func TestDocsEmptyLabel(t *testing.T) {
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 40)
	m = driveModel(t, m, "2", "/docs-none", "enter", "enter", "right", "right", "right")
	v := chipped(m.View().Content)
	// An empty file is missing, like an absent one: no third state.
	if !strings.Contains(v, "○ Resume") || !strings.Contains(v, "missing") || strings.Contains(v, "(empty)") {
		t.Fatalf("empty Resume not shown as missing:\n%s", v)
	}
}

func TestDocsEnterOpensReader(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	m = driveModel(t, m, "3", "/docs-written", "enter", "enter", "left", "down", "down", "enter") // JD, Gaps, then the Resume
	if v := chipped(m.View().Content); !strings.Contains(v, "[Resume]") {
		t.Fatalf("reader did not open:\n%s", v)
	}
}

func TestReaderFilesFollowRefresh(t *testing.T) {
	for _, text := range []string{"", "   "} {
		t.Run(text, func(t *testing.T) {
			root := fixtureCopy(t, nil)
			m := driveStart(t, root, fixtureClock, 120, 40)
			m = driveModel(t, m, "3", "/docs-written", "enter", "enter", "right", "right")
			if err := os.WriteFile(filepath.Join(root, "jobs", "SWE", "docs-written", "resume.md"), []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			m = driveModel(t, m, "e")
			if v := chipped(m.View().Content); !strings.Contains(v, "[Docs]") || !strings.Contains(v, "Resume is gone") {
				t.Fatalf("missing fallback:\n%s", v)
			}
			m = driveModel(t, m, "down", "enter") // the missing Resume opens in the Editor
			if v := chipped(m.View().Content); strings.Contains(v, "is missing") {
				t.Fatalf("an error for opening a missing document:\n%s", v)
			}
		})
	}
}

func TestNewReaderTabKeepsCurrentTab(t *testing.T) {
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 40)
	m = driveModel(t, m, "2", "/docs-none", "enter", "enter", "right", "right")
	if err := os.WriteFile(filepath.Join(root, "jobs", "SWE", "docs-none", "resume.md"), []byte("# New resume"), 0600); err != nil {
		t.Fatal(err)
	}
	m = driveModel(t, m, "e")
	if v := chipped(m.View().Content); !strings.Contains(v, "[Notes]") || !strings.Contains(v, "Resume") {
		t.Fatalf("tab changed or missing:\n%s", v)
	}
}

func TestLetterAndGapsReaders(t *testing.T) {
	root := fixtureCopy(t, nil)
	folder := filepath.Join(root, "jobs", "SWE", "docs-written")
	for name, text := range map[string]string{"cover-letter.md": "Fictional letter text", "gaps.md": "Fictional gaps text"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := driveStart(t, root, fixtureClock, 80, 40)
	m = driveModel(t, m, "3", "/docs-written", "enter", "enter")
	v := chipped(m.View().Content)
	if !strings.Contains(v, "[Overview]  JD   Fit   Resume   Cover letter   Gaps   Notes   Docs") {
		t.Fatalf("tabs: %s", v)
	}
	m = driveModel(t, m, "right", "right", "right", "right")
	if !strings.Contains(chipped(m.View().Content), "Fictional letter text") {
		t.Fatal("letter reader missing")
	}
	m = driveModel(t, m, "right")
	if !strings.Contains(chipped(m.View().Content), "Fictional gaps text") {
		t.Fatal("gaps reader missing")
	}
}

// tabRow is the detail view's tab line.
func tabRow(v string) string {
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "[JD]") {
			return l
		}
	}
	return ""
}

const docCoverLetter = "Cover letter"

func TestResumeOnlyLetterChoice(t *testing.T) {
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 40)
	m = driveModel(t, m, "3", "/docs-resume-only", "enter", "enter")
	v := chipped(m.View().Content)
	// the card is narrow: the mark says the letter is not needed
	if !strings.Contains(v, "— Cover letter") || strings.Contains(tabRow(v), docCoverLetter) {
		t.Fatalf("Resume only card: %s", v)
	}
	m = driveModel(t, m, "left")
	v = chipped(m.View().Content)
	if !strings.Contains(v, "not needed · Resume only") {
		t.Fatalf("Resume only Docs: %s", v)
	}
}

func TestResumeOnlyWrittenLetterOpensFromDocs(t *testing.T) {
	root := fixtureCopy(t, nil)
	if err := os.WriteFile(filepath.Join(root, "jobs", "SWE", "docs-resume-only", "cover-letter.md"), []byte("Fictional retained letter"), 0600); err != nil {
		t.Fatal(err)
	}
	m := driveStart(t, root, fixtureClock, 120, 40)
	m = driveModel(t, m, "3", "/docs-resume-only", "enter", "enter", "left")
	if v := chipped(m.View().Content); strings.Contains(v, "not needed") {
		t.Fatalf("written letter labelled unnecessary: %s", v)
	}
	m = driveModel(t, m, "down", "down", "down", "enter") // JD, Gaps, Resume, then the letter
	if v := chipped(m.View().Content); !strings.Contains(v, "[Cover letter]") || !strings.Contains(v, "Fictional retained letter") || strings.Contains(v, "is gone") {
		t.Fatalf("letter did not open: %s", v)
	}
}
