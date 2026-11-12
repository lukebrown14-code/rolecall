package ui

import (
	"errors"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// groupSearch is the Settings group that holds the Search area; its first two rows
// (Places, Remote roles) open the Search area form.
const groupSearch = 0

const (
	fieldPlaces = "Places"
	fieldRemote = "Remote roles"
)

// searchHelpKeys is the Search area key, shown while the Search rows have focus.
func (m model) searchHelpKeys() []helpKey {
	if m.settings == nil || m.settings.ws.Err != nil || !m.settingsRowsFocused() || m.settings.group != groupSearch || m.settings.row > 1 {
		return nil
	}
	return []helpKey{help(groupAction, "enter", "edit Search area").at(prioCore)}
}

// searchKey opens the Search area form on enter over Places or Remote roles.
func (m model) searchKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if k.String() != "enter" || !m.settingsRowsFocused() || st.group != groupSearch || st.row > 1 {
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
		choices = append(choices, choice{r, remoteLabels[r]})
	}
	f := newFormOf("Edit Search area",
		formField{label: fieldPlaces, required: true, value: strings.Join(places, ", "), placeholder: "Australia, APAC",
			note: "comma-separated"},
		formField{label: fieldRemote, value: remote, choices: choices})
	f.showDirty = true
	return f
}

// splitPlaces turns the Places answer into its list, dropping blanks.
func splitPlaces(s string) ([]string, error) {
	var places []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			places = append(places, p)
		}
	}
	if len(places) == 0 {
		return nil, errors.New(fieldPlaces + " is required")
	}
	return places, nil
}

// searchAreaPopup is the Search area form. Nothing is written until it is saved.
func (m model) searchAreaPopup() mode {
	wasPlaces, wasRemote := slices.Clone(m.settings.ws.Places), m.settings.ws.Remote
	return &formPopup{form: newSearchAreaForm(wasPlaces, wasRemote), width: popSmall, save: func(m *model, f *form) (mode, error) {
		places, err := splitPlaces(f.value(fieldPlaces))
		if err != nil {
			return nil, err
		}
		if err := m.store.SetSearchArea(places, f.value(fieldRemote)); err != nil {
			m.err = err
			return nil, nil
		}
		m.err = nil
		*m = m.loadSettings()
		*m = m.saved("Saved Search area", settingsFile, &settingsUndo{
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
