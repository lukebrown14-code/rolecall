package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// groupDocuments is the Settings group that holds the document gates.
const groupDocuments = 1

// gateHelpKeys are the Documents keys, shown while those rows have focus.
func (m model) gateHelpKeys() []helpKey {
	if m.settings == nil || m.settings.Ws.Err != nil || !m.settingsRowsFocused() || m.settings.Group != groupDocuments {
		return nil
	}
	return []helpKey{help(groupAction, "enter", "edit").at(prioCore)}
}

// gatesKey handles enter on a Documents row by opening the value pop-up. handled is false
// for any other key or group.
func (m model) gatesKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if !m.settingsRowsFocused() || st.Group != groupDocuments || k.String() != "enter" || st.Row >= len(gates) {
		return m, false
	}
	m.overlay = m.gatePopup(gates[st.Row])
	return m, true
}

// gatePopup edits one gate. The bound its pair sets is shown beside the value, so the
// range is never a surprise; a refused value shows its reason and nothing is saved.
func (m model) gatePopup(g gate) mode {
	ws := m.settings.Ws
	was := strconv.Itoa(gateValue(ws, g.Key))
	words := strings.Fields(g.Label)
	label := strings.ToUpper(words[len(words)-1][:1]) + words[len(words)-1][1:] // "Maximum"
	f := newFormOf("Edit "+g.Label, formField{Label: label, Required: true, Value: was, Unit: g.Unit,
		Note: gateRange(ws, g.Key)})
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		text := f.Value(label)
		if err := m.store.SetGate(g.Key, text); err != nil {
			return nil, err
		}
		*m = m.loadSettings()
		// SetGate stores a canonical integer, so name that rather than the typed text.
		n, _ := strconv.Atoi(strings.TrimSpace(text))
		*m = m.saved(fmt.Sprintf("Saved %s: %d %s", g.Label, n, g.Unit), settingsFile, &settingsUndo{
			what: fmt.Sprintf("%s back to %s %s", g.Label, was, g.Unit),
			restore: func(m model) model {
				if err := m.store.SetGate(g.Key, was); err != nil {
					m.err = err
					return m
				}
				return m.loadSettings()
			},
		})
		return nil, nil
	}}
}

var gateValue = settings.GateValue

var gateRange = settings.GateRange

type gate = settings.Gate

var gates = settings.Gates
