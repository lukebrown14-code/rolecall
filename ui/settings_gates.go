package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// groupDocuments is the Settings group that holds the document gates.
const groupDocuments = 1

// gate is one Documents row: its workspace.json key, its label and the unit it counts.
type gate struct{ key, label, unit string }

// gates are the Documents rows in display order, worded alike: what is limited, then
// minimum or maximum.
var gates = []gate{
	{store.GateLetterCap, "Cover letter maximum", "words"},
	{store.GateLetterMin, "Cover letter minimum", "words"},
	{store.GateSkillsMin, "Key Skills minimum", "skills"},
	{store.GateSkillsMax, "Key Skills maximum", "skills"},
}

// gateValue is a gate's current value.
func gateValue(ws store.WorkspaceSettings, key string) int {
	switch key {
	case store.GateLetterCap:
		return ws.LetterWordCap
	case store.GateLetterMin:
		return ws.LetterWordMin
	case store.GateSkillsMin:
		return ws.KeySkillsMin
	}
	return ws.KeySkillsMax
}

// gateRange is the bound the other gate of the pair sets on this one: "min 150", "max 12".
func gateRange(ws store.WorkspaceSettings, key string) string {
	switch key {
	case store.GateLetterCap:
		return fmt.Sprintf("above the minimum, %d", ws.LetterWordMin)
	case store.GateLetterMin:
		return fmt.Sprintf("below the maximum, %d", ws.LetterWordCap)
	case store.GateSkillsMin:
		return fmt.Sprintf("at most the maximum, %d", ws.KeySkillsMax)
	}
	return fmt.Sprintf("at least the minimum, %d", ws.KeySkillsMin)
}

// gateRows lists the document gates with their current values. The template choice row is
// not listed: there is one template, and the row appears once there is more than one.
func (m model) gateRows() []settingRow {
	ws := m.settings.ws
	rows := make([]settingRow, len(gates))
	for i, g := range gates {
		rows[i] = settingRow{label: g.label, value: strconv.Itoa(gateValue(ws, g.key)), unit: g.unit, source: fileSource(ws.Set[g.key])}
	}
	return rows
}

// gateHelpKeys are the Documents keys, shown while those rows have focus.
func (m model) gateHelpKeys() []helpKey {
	if m.settings == nil || m.settings.ws.Err != nil || !m.settingsRowsFocused() || m.settings.group != groupDocuments {
		return nil
	}
	return []helpKey{help(groupAction, "enter", "edit").at(prioCore)}
}

// gatesKey handles enter on a Documents row by opening the value pop-up. handled is false
// for any other key or group.
func (m model) gatesKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if !m.settingsRowsFocused() || st.group != groupDocuments || k.String() != "enter" || st.row >= len(gates) {
		return m, false
	}
	m.overlay = m.gatePopup(gates[st.row])
	return m, true
}

// gatePopup edits one gate. The bound its pair sets is shown beside the value, so the
// range is never a surprise; a refused value shows its reason and nothing is saved.
func (m model) gatePopup(g gate) mode {
	ws := m.settings.ws
	was := strconv.Itoa(gateValue(ws, g.key))
	words := strings.Fields(g.label)
	label := strings.ToUpper(words[len(words)-1][:1]) + words[len(words)-1][1:] // "Maximum"
	f := newFormOf("Edit "+g.label, formField{label: label, required: true, value: was, unit: g.unit,
		note: gateRange(ws, g.key)})
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		text := f.value(label)
		if err := m.store.SetGate(g.key, text); err != nil {
			return nil, err
		}
		*m = m.loadSettings()
		// SetGate stores a canonical integer, so name that rather than the typed text.
		n, _ := strconv.Atoi(strings.TrimSpace(text))
		*m = m.saved(fmt.Sprintf("Saved %s: %d %s", g.label, n, g.unit), settingsFile, &settingsUndo{
			what: fmt.Sprintf("%s back to %s %s", g.label, was, g.unit),
			restore: func(m model) model {
				if err := m.store.SetGate(g.key, was); err != nil {
					m.err = err
					return m
				}
				return m.loadSettings()
			},
		})
		return nil, nil
	}}
}
