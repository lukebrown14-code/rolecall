package settings_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// The Settings sub-model is driven as the application drives it: a key in, the next
// model and an intent out. Nothing here writes a file; the intents say what would be.

type reader struct {
	use    map[string]int
	impact store.RenameImpact
	err    error
}

func (r reader) TrackUse(track string) (int, error) { return r.use[track], r.err }
func (r reader) TrackRenameImpact(string) (store.RenameImpact, error) {
	return r.impact, r.err
}

func key(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func typeText(t *testing.T, p *settings.Popup, m settings.Model, env settings.Env, text string) {
	t.Helper()
	for _, r := range text {
		if next, in := p.Update(key(string(r)), m, env); next != p || in != nil {
			t.Fatalf("typing %q closed the pop-up or asked for %#v", r, in)
		}
	}
}

func baseEnv() settings.Env {
	return settings.Env{
		Machine: settings.Machine{Workspace: "/ws", Editor: "vi", EditorSource: config.SourceDefault, EditorAvailable: true,
			FollowUpDays: 14, Sources: settings.Sources{FollowUpDays: config.SourceDefault}},
		Store: reader{},
	}
}

func modelOn(group, row int) settings.Model {
	m := settings.Model{}
	m.Ws = store.WorkspaceSettings{Tracks: []string{"SWE", "Data"}, LetterWordCap: 400, LetterWordMin: 150, KeySkillsMin: 6, KeySkillsMax: 12,
		SourceList: []store.Source{{Name: "seek", Type: store.SourceBrowser}}}
	m.TrackUse = map[string]int{}
	m.Group, m.Row = group, row
	return m
}

func save(t *testing.T, in settings.Intent) settings.Save {
	t.Helper()
	s, ok := in.(settings.Save)
	if !ok {
		t.Fatalf("intent = %#v, want a Save", in)
	}
	return s
}

func open(t *testing.T, in settings.Intent) *settings.Popup {
	t.Helper()
	o, ok := in.(settings.Open)
	if !ok {
		t.Fatalf("intent = %#v, want a pop-up", in)
	}
	return o.Popup
}

// The handler table names every group once, in display order, so reordering the groups
// cannot silently move one group's keys onto another.
func TestEveryGroupHasOneHandlerInDisplayOrder(t *testing.T) {
	got := settings.HandlerGroups()
	want := make([]int, len(settings.Groups))
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(got, want) {
		t.Fatalf("handler groups = %v, want %v", got, want)
	}
}

// A Gate's pop-up saves through an intent and offers the old value back.
func TestGatePopupSavesThroughAnIntent(t *testing.T) {
	env := baseEnv()
	m := modelOn(settings.GroupDocuments, 0)
	m, in := m.Update(key("enter"), env)
	p := open(t, in)
	if p.Form == nil || p.Form.Title != "Edit Cover letter maximum" {
		t.Fatalf("pop-up = %+v", p)
	}
	typeText(t, p, m, env, "500")
	next, in := p.Update(key("enter"), m, env)
	if next != nil {
		t.Fatal("a saved pop-up stays open")
	}
	s := save(t, in)
	if s.Write != (settings.SetGate{Key: store.GateLetterCap, Text: "500"}) || !s.OnForm {
		t.Fatalf("save = %+v", s)
	}
	if s.Undo == nil || s.Undo.Restore != (settings.SetGate{Key: store.GateLetterCap, Text: "400"}) ||
		s.Undo.What != "Cover letter maximum back to 400 words" {
		t.Fatalf("undo = %+v", s.Undo)
	}
	m, msg := m.Saved(s, env)
	if msg != "Saved Cover letter maximum: 500 words" || m.Undo == nil {
		t.Fatalf("Saved = %q, undo %v", msg, m.Undo)
	}
	if _, in := m.Update(key("u"), env); in != (settings.Restore{Undo: *s.Undo}) {
		t.Fatalf("u = %#v", in)
	}
}

func TestUWithNothingToUndoAnswers(t *testing.T) {
	if _, in := modelOn(settings.GroupSearch, 0).Update(key("u"), baseEnv()); in != (settings.Note{Text: "Nothing to undo in Settings"}) {
		t.Fatalf("u = %#v", in)
	}
}

// Enter on the Editor opens its inline field; typing replaces the selected value, and
// enter asks for the config.toml write, its undo naming what was in effect.
func TestInlineFieldSavesAMachineSetting(t *testing.T) {
	env := baseEnv()
	env.Machine.EditorSet = "vi"
	m := modelOn(settings.GroupMachine, 1)
	m, in := m.Update(key("enter"), env)
	if in != nil || m.Edit == nil || !m.Edit.Selected || m.Edit.Buf != "vi" {
		t.Fatalf("edit = %+v, intent %#v", m.Edit, in)
	}
	m, _ = m.EditKey(key("h"), env)
	m = m.EditPaste("x\n --wait") // pasted text lands on one line
	m, in = m.EditKey(key("enter"), env)
	s := save(t, in)
	if m.Edit != nil || s.Write != (settings.SetMachine{Key: "editor", Value: "hx --wait"}) || s.Write.File() != "config.toml" {
		t.Fatalf("save = %+v, edit %+v", s, m.Edit)
	}
	if s.Msg != "Saved Editor: hx --wait" || s.Undo.What != "Editor back to vi" || s.Undo.Restore != (settings.SetMachine{Key: "editor", Value: "vi"}) {
		t.Fatalf("save = %+v, undo %+v", s, s.Undo)
	}
}

// Follow-up days is checked as the field is left: a refused value keeps the field open
// with its reason, and nothing is written.
func TestFollowUpDaysAreValidatedBeforeAnyWrite(t *testing.T) {
	env := baseEnv()
	m := modelOn(settings.GroupTracking, 0)
	m = m.Clamp(env)
	m, _ = m.Update(key("G"), env)
	m, in := m.Update(key("enter"), env)
	if m.Edit == nil || m.Edit.Key != settings.KeyFollowUpDays || m.Edit.Placeholder != "14" || in != nil {
		t.Fatalf("edit = %+v", m.Edit)
	}
	m, _ = m.EditKey(key("0"), env)
	m, in = m.EditKey(key("enter"), env)
	if in != nil || m.Edit == nil || m.Edit.Err == "" {
		t.Fatalf("a refused value was saved: %#v, edit %+v", in, m.Edit)
	}
}

// ctrl+c over typed text asks first; y drops the edit.
func TestQuitOverTypedTextAsksFirst(t *testing.T) {
	env := baseEnv()
	m := modelOn(settings.GroupMachine, 2)
	m, _ = m.Update(key("enter"), env)
	if _, held := m.HoldQuit(); held {
		t.Fatal("an untouched field held the quit")
	}
	m, _ = m.EditKey(key("x"), env)
	m, held := m.HoldQuit()
	if !held || !m.Edit.Discarding || m.EditKeys()[0].Key != "y" {
		t.Fatalf("held = %v, edit %+v", held, m.Edit)
	}
	if m, _ = m.EditKey(key("y"), env); m.Edit != nil {
		t.Fatal("y did not drop the edit")
	}
}

// Enter on a config.toml row while config.toml does not parse is refused with its problem.
func TestBrokenConfigRefusesMachineEdits(t *testing.T) {
	env := baseEnv()
	env.ConfigBroken, env.ConfigFeedback = true, errors.New("config.toml line 2: bad")
	m, in := modelOn(settings.GroupMachine, 1).Update(key("enter"), env)
	if in != (settings.Fail{Err: env.ConfigFeedback}) || m.Edit != nil {
		t.Fatalf("intent = %#v", in)
	}
	if keys := m.RowKeys(env); len(keys) != 0 {
		t.Fatalf("keys offered on a broken config.toml: %+v", keys)
	}
}

// J moves a Track down: one write of the new order, the cursor following it.
func TestReorderingTracksWritesTheListAndFollowsTheTrack(t *testing.T) {
	env := baseEnv()
	m := modelOn(settings.GroupTracking, 0)
	m, in := m.Update(key("J"), env)
	s := save(t, in)
	w, ok := s.Write.(settings.SetTracks)
	if !ok || !slices.Equal(w.Tracks, []string{"Data", "SWE"}) || s.Msg != "Moved Track SWE" {
		t.Fatalf("save = %+v", s)
	}
	if r := s.Undo.Restore.(settings.SetTracks); !slices.Equal(r.Tracks, []string{"SWE", "Data"}) {
		t.Fatalf("undo = %+v", s.Undo)
	}
	if m, _ = m.Saved(s, env); m.Row != 1 {
		t.Fatalf("cursor on row %d, want 1", m.Row)
	}
	if _, in := modelOn(settings.GroupTracking, 1).Update(key("J"), env); in != nil {
		t.Fatalf("J past the last Track = %#v", in)
	}
}

// x on a Track Applications use is refused in a pop-up naming them; f shows them.
func TestRemovingATrackInUseIsRefused(t *testing.T) {
	env := baseEnv()
	env.Store = reader{use: map[string]int{"SWE": 3}}
	env.Apps = []store.Application{{Track: "SWE", Company: "Acme", Title: "Engineer"}, {Track: "SWE", Status: store.StatusDismissed, Company: "Beta"}}
	m := modelOn(settings.GroupTracking, 0)
	_, in := m.Update(key("x"), env)
	p := open(t, in)
	if p.Form != nil || p.Keys()[0][0] != "f" {
		t.Fatalf("pop-up = %+v", p)
	}
	if next, in := p.Update(key("f"), m, env); next != nil || in != (settings.ShowTrack{Track: "SWE"}) {
		t.Fatalf("f = %v, %#v", next, in)
	}
	// A Track nothing uses is removed at once.
	_, in = modelOn(settings.GroupTracking, 1).Update(key("x"), env)
	if w := save(t, in).Write.(settings.SetTracks); !slices.Equal(w.Tracks, []string{"SWE"}) {
		t.Fatalf("remove = %+v", w)
	}
}

// A Track's rename is chosen, reviewed, then asked for; nothing is written before y.
func TestRenamingATrackIsReviewedBeforeItIsAskedFor(t *testing.T) {
	env := baseEnv()
	env.Store = reader{impact: store.RenameImpact{Applications: 2}}
	m := modelOn(settings.GroupTracking, 0)
	_, in := m.Update(key("enter"), env)
	p := open(t, in)
	typeText(t, p, m, env, "data")
	if next, in := p.Update(key("enter"), m, env); next != p || in != nil || !strings.Contains(p.Form.Focused().Err, "exists") {
		t.Fatalf("a name twin of a Track went on: %v, %#v, %q", next, in, p.Form.Focused().Err)
	}
	p.Form.Fields[0].Value = "Platform"
	review, in := p.Update(key("enter"), m, env)
	if review == nil || review.Form != nil || in != nil {
		t.Fatalf("review = %+v, %#v", review, in)
	}
	if _, in := review.Update(key("y"), m, env); in != (settings.RenameTrack{Old: "SWE", New: "Platform"}) {
		t.Fatalf("y = %#v", in)
	}
	if next, msg := review.Renamed(store.RenameResult{RenameImpact: store.RenameImpact{Applications: 2}}); next != nil || msg != "Renamed SWE to Platform (2 Applications)" {
		t.Fatalf("Renamed = %v, %q", next, msg)
	}
}

// A new Track's name is checked against the list before anything is written.
func TestAddingATrackChecksTheName(t *testing.T) {
	env := baseEnv()
	m := modelOn(settings.GroupTracking, 0)
	_, in := m.Update(key("a"), env)
	p := open(t, in)
	typeText(t, p, m, env, "a/b")
	if next, in := p.Update(key("enter"), m, env); next != p || in != nil || p.Form.Focused().Err == "" {
		t.Fatalf("a bad name was saved: %#v", in)
	}
}

// On a Source, o turns it off at once and c asks for a check; the keys say so.
func TestSourceKeys(t *testing.T) {
	env := baseEnv()
	m := modelOn(settings.GroupSources, 0)
	_, in := m.Update(key("o"), env)
	if s := save(t, in); s.Write != (settings.SetSourceEnabled{Name: "seek", On: false}) || s.Msg != "Turned off seek" {
		t.Fatalf("o = %+v", s)
	}
	if _, in := m.Update(key("c"), env); in != (settings.CheckSource{Name: "seek"}) {
		t.Fatalf("c = %#v", in)
	}
	var descs []string
	for _, k := range m.RowKeys(env) {
		descs = append(descs, k.Key+" "+k.Desc)
	}
	if !slices.Equal(descs, []string{"enter edit", "o turn off", "c check"}) {
		t.Fatalf("keys = %v", descs)
	}
}

// Below the medium width the section key is pinned: it is the only sign of the others.
func TestNarrowPinsTheSectionKey(t *testing.T) {
	m := modelOn(settings.GroupSearch, 0)
	if k := m.NavKeys(settings.Env{})[0]; k.Prio != settings.PrioCore {
		t.Fatalf("wide section key = %+v", k)
	}
	if k := m.NavKeys(settings.Env{Narrow: true})[0]; k.Prio != settings.PrioPinned {
		t.Fatalf("narrow section key = %+v", k)
	}
}
