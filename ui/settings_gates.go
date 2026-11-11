package ui

import (
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// groupDocuments is the Settings group that holds the document gates.
const groupDocuments = 1

// gateKeys are the Documents rows, in display order, as workspace.json keys.
var gateKeys = []string{store.GateLetterCap, store.GateLetterMin, store.GateSkillsMin, store.GateSkillsMax}

// gateRows lists the document gates with their current values. The template choice row is
// not listed: there is one template, and the row appears once there is more than one.
func (m model) gateRows() []settingRow {
	ws := m.settings.ws
	return []settingRow{
		{"Cover letter word cap", strconv.Itoa(ws.LetterWordCap)},
		{"Cover letter minimum", strconv.Itoa(ws.LetterWordMin)},
		{"Key Skills minimum", strconv.Itoa(ws.KeySkillsMin)},
		{"Key Skills maximum", strconv.Itoa(ws.KeySkillsMax)},
	}
}

// gateHelpKeys are the Documents keys, shown while those rows have focus.
func (m model) gateHelpKeys() []helpKey {
	if m.settings == nil || m.settings.ws.Err != nil || m.settings.focus != settingsRows || m.settings.group != groupDocuments {
		return nil
	}
	return []helpKey{help(groupAction, "enter", "edit")}
}

// gatesKey handles enter on a Documents row by opening the value pop-up. handled is false
// for any other key or group.
func (m model) gatesKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if st.focus != settingsRows || st.group != groupDocuments || k.String() != "enter" || st.row >= len(gateKeys) {
		return m, false
	}
	row := m.gateRows()[st.row]
	m.overlay = &gateEditMode{key: gateKeys[st.row], label: row.label, value: row.value}
	return m, true
}

// gateEditMode is the pop-up that edits one gate. The value is checked on enter; a refused
// value shows its reason and nothing is saved.
type gateEditMode struct {
	key, label, value string
	err               string
}

func (gm *gateEditMode) Keys() [][2]string {
	return [][2]string{{"enter", hintSave}, {"esc", "cancel"}}
}

func (gm *gateEditMode) boxed() bool { return true }

func (gm *gateEditMode) View(m *model, p pal) string {
	body := p.text(p.dim).Render(gm.label+"  ") + p.text(p.fg).Render(gm.value) + p.text(p.accent).Render("▏")
	if gm.err != "" {
		body += "\n" + p.text(p.bad).Render("✗ "+gm.err)
	}
	return popup(p, "Edit setting", false, popSmall, m.viewWidth(), body, p.keys(gm.Keys()))
}

func (gm *gateEditMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.PasteMsg:
		gm.value += pasted(msg.Content)
		gm.err = ""
	case tea.KeyPressMsg:
		switch {
		case msg.String() == "esc":
			return nil, nil
		case msg.Code == tea.KeyBackspace:
			gm.value = dropLastGrapheme(gm.value)
			gm.err = ""
		case msg.String() == "enter":
			if err := m.store.SetGate(gm.key, gm.value); err != nil {
				gm.err = err.Error()
				return gm, nil
			}
			*m = m.loadSettings()
			return nil, nil
		default:
			gm.value += msg.Text
			gm.err = ""
		}
	}
	return gm, nil
}
