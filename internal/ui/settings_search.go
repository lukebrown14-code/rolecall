package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

const (
	fieldPlaces = "Places"
	fieldRemote = "Remote roles"
)

// searchHelpKeys is the Search area key, shown while the Search rows have focus.
func (m model) searchHelpKeys() []helpKey {
	if m.settings == nil || m.settings.Ws.Err != nil || !m.settingsRowsFocused() || m.settings.Group != settings.GroupSearch || m.settings.Row > 1 {
		return nil
	}
	return []helpKey{help(groupAction, "enter", "edit Search area").at(prioCore)}
}

// searchKey opens the Search area form on enter over Places or Remote roles.
func (m model) searchKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if k.String() != "enter" || !m.settingsRowsFocused() || st.Group != settings.GroupSearch || st.Row > 1 {
		return m, false
	}
	m.overlay = m.searchAreaPopup()
	return m, true
}

func newSearchAreaForm(places []string, remote string) *form {
	if remote != store.RemoteAnywhere && remote != store.RemoteNo {
		remote = store.RemoteInPlaces
	}
	var choices []choice
	for _, r := range []string{store.RemoteInPlaces, store.RemoteAnywhere, store.RemoteNo} {
		choices = append(choices, choice{Value: r, Label: remoteLabels[r]})
	}
	f := newFormOf("Edit Search area",
		formField{Label: fieldPlaces, Value: strings.Join(places, ", "), Placeholder: "Australia, APAC",
			Note: "comma-separated · empty keeps every location"},
		formField{Label: fieldRemote, Value: remote, Choices: choices})
	f.ShowDirty = true
	return f
}

// searchAreaPopup is the Search area form. Nothing is written until it is saved.
func (m model) searchAreaPopup() mode {
	wasPlaces, wasRemote := slices.Clone(m.settings.Ws.Places), m.settings.Ws.Remote
	return &formPopup{form: newSearchAreaForm(wasPlaces, wasRemote), width: popSmall, save: func(m *model, f *form) (mode, error) {
		places := splitPlaces(f.Value(fieldPlaces))
		if err := m.store.SetSearchArea(places, f.Value(fieldRemote)); err != nil {
			m.err = err
			return nil, nil
		}
		m.err = nil
		*m = m.loadSettings()
		msg := "Saved Search area"
		if len(places) == 0 {
			msg = "Cleared Search area: every location is kept"
		}
		*m = m.saved(msg, settingsFile, &settingsUndo{
			what: "Search area as it was",
			restore: func(m model) model {
				remote := wasRemote
				if remote == "" {
					remote = store.RemoteInPlaces
				}
				if err := m.store.SetSearchArea(wasPlaces, remote); err != nil {
					m.err = err
					return m
				}
				return m.loadSettings()
			},
		})
		return nil, nil
	}}
}

var splitPlaces = settings.SplitPlaces

var remoteLabels = settings.RemoteLabels
