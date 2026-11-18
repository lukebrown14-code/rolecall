package settings

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Model is the Settings sub-model: the screen's State and the keys that change it.
// Update and the pop-ups return an Intent for whatever must be written, run or shown
// beyond the screen; package ui applies it, so the sub-model never touches a file. It is
// replaced, never mutated in place, so copies of the application model stay independent.
type Model struct{ State[Undo] }

// Reader is the Workspace reads Settings makes beyond its own reload, through store.
type Reader interface {
	TrackUse(track string) (int, error)
	TrackRenameImpact(old string) (store.RenameImpact, error)
}

// Env is what Settings reads from the running application on each call.
type Env struct {
	Machine Machine
	// Apps are the Applications the lists hold, for each Track's breakdown and the
	// refusal to remove a Track in use.
	Apps  []store.Application
	Store Reader
	// ConfigBroken is a config.toml that does not parse: its rows show the last good
	// values and Enter on them is refused with ConfigFeedback.
	ConfigBroken   bool
	ConfigFeedback error
	// ConfigErr is config.toml's load error, for the notice above This machine's rows.
	ConfigErr error
	// Narrow is below the medium width: no group list beside the rows, so the section
	// key is pinned and the Search key's words are short.
	Narrow bool
}

// GroupRows are a group's rows, each Track's count with its breakdown.
func (m Model) GroupRows(group int, env Env) []Row {
	st := m.State
	st.Breakdown = Breakdown(env.Apps, st)
	return st.Rows(group, env.Machine)
}

// Clamp puts the cursor on a row it can rest on, after a reload.
func (m Model) Clamp(env Env) Model {
	m.Row = SelectableRow(m.GroupRows(m.Group, env), m.Row, 1)
	return m
}

// groupHandler is one Settings group's keys: how the help describes them and how the
// screen dispatches them. The screen reads one table, so a group cannot be dispatched
// without also being described, and the group order is data rather than a chain of calls.
type groupHandler struct {
	group int
	help  func(Model, Env) []Key
	// key handles the group's row keys; nil when the group has none of its own (This
	// machine's rows activate through Enter, like every other row).
	key func(Model, tea.KeyPressMsg, Env) (Model, Intent, bool)
}

// handlers is every group's handler, in display order.
var handlers = []groupHandler{
	{group: GroupSearch, help: Model.searchKeys, key: Model.searchKey},
	{group: GroupRules, help: Model.rulesKeys, key: Model.rulesKey},
	{group: GroupSources, help: Model.sourcesKeys, key: Model.sourcesKey},
	{group: GroupDocuments, help: Model.gateKeys, key: Model.gatesKey},
	{group: GroupTracking, help: Model.trackKeys, key: Model.tracksKey},
	{group: GroupMachine, help: Model.machineKeys},
}

// HandlerGroups are the groups the handler table names, in its order.
func HandlerGroups() []int {
	out := make([]int, len(handlers))
	for i, h := range handlers {
		out[i] = h.group
	}
	return out
}

// Key is one key Settings offers, for the key line, the help and the command palette.
type Key struct {
	Key, Desc string
	Nav       bool // moves around (the key line's right) rather than acts
	Prio      Prio
}

// Prio is a Key's key-line priority, as package ui ranks it.
type Prio int

// Key priorities.
const (
	PrioDefault  Prio = iota // the default of the key's kind
	PrioCore                 // how a newcomer finds their way in
	PrioPinned               // how to get unstuck, never dropped
	PrioHelpOnly             // the help and palette only, never the key line
)

// KeyClearField empties the open inline field, as it empties a form's text field.
const KeyClearField = "ctrl+u"

// keyNoEsc is the way back from a question, as every pop-up words it.
const keyNoEsc = "n / esc"

// EditKeys are the open inline field's keys, or nil while none is open.
func (m Model) EditKeys() []Key {
	if m.Edit == nil {
		return nil
	}
	if m.Edit.Discarding {
		return []Key{{Key: "y", Desc: "discard", Prio: PrioCore}, {Key: keyNoEsc, Desc: "keep editing", Nav: true, Prio: PrioPinned}}
	}
	return []Key{{Key: "enter", Desc: "save", Prio: PrioCore}, {Key: KeyClearField, Desc: "clear", Prio: PrioCore},
		{Key: "esc", Desc: "cancel", Nav: true, Prio: PrioPinned}}
}

// RowKeys are every group's row keys, then u while there is a save to undo.
func (m Model) RowKeys(env Env) []Key {
	var keys []Key
	for _, h := range handlers {
		keys = append(keys, h.help(m, env)...)
	}
	if m.Undo != nil {
		keys = append(keys, Key{Key: "u", Desc: "undo", Prio: PrioPinned}) // the toast's offer stays reachable
	}
	return keys
}

// NavKeys are the section and row keys, while workspace.json is valid.
func (m Model) NavKeys(env Env) []Key {
	if m.Ws.Err != nil {
		return nil
	}
	// ←→ switch sections as they switch views elsewhere; tab does too, and [ ] unlisted,
	// as the help would wrap [ ] in brackets. Below the medium width there is no group
	// list beside the rows, so the section key is pinned: it is the only sign the other
	// sections are there.
	section := Key{Key: "←→", Desc: "section", Nav: true, Prio: PrioCore}
	if env.Narrow {
		section.Prio = PrioPinned
	}
	return []Key{section, {Key: "tab", Desc: "section", Nav: true, Prio: PrioHelpOnly},
		{Key: "j k", Desc: "select", Nav: true}, {Key: "g G", Desc: "first, last", Nav: true, Prio: PrioHelpOnly}}
}

// Update handles a key on the Settings rows, once the application has taken its own
// (rescan, help, quit, e, esc) and only while workspace.json is valid.
func (m Model) Update(k tea.KeyPressMsg, env Env) (Model, Intent) {
	if k.String() == "u" {
		if m.Undo == nil {
			// u reverts Settings changes here, not moves; with none it still answers.
			return m, Note{Text: "Nothing to undo in Settings"}
		}
		return m, Restore{Undo: *m.Undo}
	}
	if k.String() == keySourceCheck {
		if src, ok := m.focusedSource(); ok {
			return m, CheckSource{Name: src.Name}
		}
	}
	for _, h := range handlers {
		if h.key == nil {
			continue
		}
		if next, in, ok := h.key(m, k, env); ok {
			return next, in
		}
	}
	switch k.String() {
	case "enter":
		if !m.Header {
			return m.activate(env)
		}
	default:
		m.State = m.Navigate(k.String(), env.Machine)
	}
	return m, nil
}

// Saved reports a save once its write is applied and reloaded: the cursor goes where the
// save puts it, the message is worded, and the undo is kept so u can put the old value back.
func (m Model) Saved(s Save, env Env) (Model, string) {
	switch s.place {
	case placeAt:
		m.Row = s.row
	case placeRemove:
		rows := m.GroupRows(GroupTracking, env)
		m.Row = SelectableRow(rows, min(m.Row, len(rows)-1), -1)
	}
	msg := s.Msg
	if s.rule != nil {
		msg = m.ruleSaved(*s.rule)
	}
	m.Undo = s.Undo
	return m, msg
}

// Undone drops the undo once it is used.
func (m Model) Undone() Model {
	m.Undo = nil
	return m
}

// rowsFocused reports whether keys act on the settings rows: the cursor is on a field,
// not the header.
func (m Model) rowsFocused() bool { return !m.Header }

// rowShown is the value a group's row named label shows, unit and all, or "not set": what
// an undo names, so it reads as the row did before the save.
func (m Model) rowShown(group int, label string, env Env) string {
	for _, r := range m.GroupRows(group, env) {
		if r.Label == label && r.Value != "" {
			return r.ValueText()
		}
	}
	return "not set"
}

// EditKey types into the open inline field: enter saves the value at once, esc drops it.
// While a quit's "Discard changes?" is asked, y drops the edit and n or esc goes back to it.
func (m Model) EditKey(k tea.KeyPressMsg, env Env) (Model, Intent) {
	e := *m.Edit
	if e.Discarding {
		switch k.String() {
		case "y":
			m.Edit = nil
		case "n", "esc":
			e.Discarding = false
			m.Edit = &e
		}
		return m, nil
	}
	s := settingByKey(e.Key)
	switch k.String() {
	case "esc":
		m.Edit = nil
	case "enter":
		text := strings.TrimSpace(e.Buf)
		if text == strings.TrimSpace(e.Orig) {
			m.Edit = nil // nothing changed, so nothing is written
			break
		}
		if s.key == KeyFollowUpDays {
			if err := ValidFollowUpDays(text); err != nil {
				e.Err, e.Selected = err.Error(), false
				m.Edit = &e
				break
			}
		}
		m.Edit = nil
		return m, saveMachine(s, text, env.Machine)
	case "backspace":
		if r := []rune(e.Buf); e.Selected {
			e.Buf = ""
		} else if len(r) > 0 {
			e.Buf = string(r[:len(r)-1])
		}
		e.Err, e.Selected, m.Edit = "", false, &e
	case KeyClearField:
		e.Buf, e.Err, e.Selected, m.Edit = "", "", false, &e
	case "left", "right", "end":
		e.Selected, m.Edit = false, &e // the selection drops and the cursor stays at the end
	default:
		if k.Text != "" {
			if e.Selected {
				e.Buf = ""
			}
			e.Buf, e.Err, e.Selected, m.Edit = e.Buf+k.Text, "", false, &e
		}
	}
	return m, nil
}

// EditPaste types pasted text into the open inline field, on one line, replacing a
// selected opening value.
func (m Model) EditPaste(text string) Model {
	e := *m.Edit
	if e.Discarding {
		return m
	}
	if e.Selected {
		e.Buf = ""
	}
	e.Buf, e.Err, e.Selected = e.Buf+strings.Join(strings.Fields(text), " "), "", false
	m.Edit = &e
	return m
}

// HoldQuit is ctrl+c while the Owner has typed into the open field: rather than drop the
// text, it asks "Discard changes?". held is false when there is nothing typed to lose, or
// the question is already up, and the quit goes ahead.
func (m Model) HoldQuit() (next Model, held bool) {
	if m.Edit == nil {
		return m, false
	}
	e := *m.Edit
	if e.Discarding || !e.Changed() {
		return m, false
	}
	e.Discarding = true
	m.Edit = &e
	return m, true
}
