package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// rulesHelpKeys are the Rules keys, shown while those rows have focus.
func (m model) rulesHelpKeys() []helpKey { return m.enterEditHelp(settings.GroupRules) }

// rulesKey opens the focused Search rule's pop-up on enter. handled is false for any
// other key or group.
func (m model) rulesKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if !m.settingsRowsFocused() || st.Group != settings.GroupRules || k.String() != "enter" || st.Row >= len(settings.Rules) {
		return m, false
	}
	m.overlay = m.rulePopup(settings.Rules[st.Row])
	return m, true
}

// rulePopup edits one Search rule. Empty text clears it; a refused value shows its
// reason and nothing is saved.
func (m model) rulePopup(r settings.Rule) mode {
	was := settings.RuleText(m.settings.Ws.Rules, r.Key)
	f := newFormOf("Edit "+r.Label, formField{Label: r.Label, Value: was, Unit: r.Unit, Placeholder: r.Example,
		Note: "empty clears it"})
	f.ShowDirty = true
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		text := f.Value(r.Label)
		if err := m.store.SetSearchRule(r.Key, text); err != nil {
			return nil, err
		}
		*m = m.loadSettings()
		m.loadRules()
		msg := "Cleared " + r.Label
		if now := settings.RuleText(m.settings.Ws.Rules, r.Key); now != "" {
			msg = "Saved " + r.Label + ": " + now
			if r.Unit != "" {
				msg += " " + r.Unit
			}
		}
		back := "unset"
		if was != "" {
			back = was
		}
		*m = m.saved(msg, settingsFile, &settingsUndo{
			what: fmt.Sprintf("%s back to %s", r.Label, back),
			restore: func(m model) model {
				if err := m.store.SetSearchRule(r.Key, was); err != nil {
					m.err = err
					return m
				}
				m = m.loadSettings()
				m.loadRules()
				return m
			},
		})
		return nil, nil
	}}
}
