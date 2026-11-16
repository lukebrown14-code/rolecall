package ui

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// rulesHelpKeys are the Rules keys, shown while those rows have focus.
func (m model) rulesHelpKeys() []helpKey { return m.enterEditHelp(settings.GroupRules) }

// rulesKey opens the focused Search rule's pop-up on enter, or flips Needs sponsorship.
// handled is false for any other key or group.
func (m model) rulesKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if !m.settingsRowsFocused() || st.Group != settings.GroupRules || k.String() != "enter" || st.Row >= len(settings.Rules)+len(settings.VisaRules) {
		return m, false
	}
	if st.Row < len(settings.Rules) {
		m.overlay = m.rulePopup(settings.Rules[st.Row])
		return m, true
	}
	r := settings.VisaRules[st.Row-len(settings.Rules)]
	if r.Key == store.VisaNeedsSponsorship {
		next, err := m.saveRule(r, strconv.FormatBool(!st.Ws.Visa.NeedsSponsorship))
		if err != nil {
			m.err = err
			return m, true
		}
		return next, true
	}
	m.overlay = m.rulePopup(r)
	return m, true
}

// isVisa reports a Rules row that writes workspace.json visa rather than search_rules.
func isVisa(key string) bool {
	return key == store.VisaNeedsSponsorship || key == store.VisaMaxContract
}

// ruleText is a Rules row's value as the Owner types it.
func (m model) ruleText(key string) string {
	if isVisa(key) {
		return settings.VisaText(m.settings.Ws.Visa, key)
	}
	return settings.RuleText(m.settings.Ws.Rules, key)
}

// setRule writes a Rules row's value; empty text clears it.
func (m model) setRule(key, text string) error {
	if isVisa(key) {
		return m.store.SetVisa(key, text)
	}
	return m.store.SetSearchRule(key, text)
}

// rulePopup edits one Rules row. Empty text clears it; a refused value shows its reason
// and nothing is saved.
func (m model) rulePopup(r settings.Rule) mode {
	f := newFormOf("Edit "+r.Label, formField{Label: r.Label, Value: m.ruleText(r.Key), Unit: r.Unit, Placeholder: r.Example,
		Note: "empty clears it"})
	f.ShowDirty = true
	return &formPopup{form: f, width: popSmall, save: func(m *model, f *form) (mode, error) {
		next, err := m.saveRule(r, f.Value(r.Label))
		if err != nil {
			return nil, err
		}
		*m = next
		return nil, nil
	}}
}

// saveRule writes one Rules row, names what it saved and offers its undo.
func (m model) saveRule(r settings.Rule, text string) (model, error) {
	was := m.ruleText(r.Key)
	if err := m.setRule(r.Key, text); err != nil {
		return m, err
	}
	m = m.loadSettings()
	m.loadRules()
	msg := "Cleared " + r.Label
	if now := m.ruleText(r.Key); now != "" {
		if r.Key == store.VisaNeedsSponsorship {
			now = settings.YesNo(now == "true")
		}
		msg = "Saved " + r.Label + ": " + now
		if r.Unit != "" {
			msg += " " + r.Unit
		}
	}
	back := "unset"
	if was != "" {
		back = was
	}
	return m.saved(msg, settingsFile, &settingsUndo{
		what: fmt.Sprintf("%s back to %s", r.Label, back),
		restore: func(m model) model {
			if err := m.setRule(r.Key, was); err != nil {
				m.err = err
				return m
			}
			m = m.loadSettings()
			m.loadRules()
			return m
		},
	}), nil
}
