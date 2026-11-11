package ui

import (
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// groupSearch is the Settings group that holds the Search area; its first two rows
// (Places, Remote) open the Search area form.
const groupSearch = 0

const (
	fieldPlaces = "Places"
	fieldRemote = "Remote"
)

// searchHelpKeys is the Search area key, shown while the Search rows have focus.
func (m model) searchHelpKeys() []helpKey {
	if m.settings == nil || m.settings.ws.Err != nil || m.settings.focus != settingsRows || m.settings.group != groupSearch {
		return nil
	}
	return []helpKey{help(groupAction, "enter", "edit Search area")}
}

// searchKey opens the Search area form on enter over Places or Remote.
func (m model) searchKey(k tea.KeyPressMsg) (next model, handled bool) {
	st := m.settings
	if k.String() != "enter" || st.focus != settingsRows || st.group != groupSearch || st.row > 1 {
		return m, false
	}
	m.overlay = &searchAreaMode{form: newSearchAreaForm(st.ws.Places, st.ws.Remote)}
	return m, true
}

func newSearchAreaForm(places []string, remote string) *form {
	if remote != store.RemoteAnywhere && remote != store.RemoteNo {
		remote = store.RemoteInPlaces
	}
	return &form{title: "Search area", fields: []formField{
		{label: fieldPlaces, required: true, value: strings.Join(places, ", "), placeholder: "Australia, APAC"},
		{label: fieldRemote, value: remote, choices: []choice{
			{store.RemoteInPlaces, "in my places"}, {store.RemoteAnywhere, "anywhere"}, {store.RemoteNo, "no"}}},
	}}
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

// searchAreaMode is the Search area pop-up. It reuses the Add form; nothing is written
// until the last field is confirmed.
type searchAreaMode struct{ form *form }

func (sm *searchAreaMode) Keys() [][2]string {
	return [][2]string{{"tab", hintNext}, {"←→", "change"}, {"enter", hintSave}, {"esc", "cancel"}}
}

func (sm *searchAreaMode) boxed() bool { return true }

func (sm *searchAreaMode) View(m *model, p pal) string {
	return m.formBox(p, &formMode{form: sm.form})
}

func (sm *searchAreaMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	if paste, ok := msg.(tea.PasteMsg); ok {
		sm.form.paste(paste.Content)
		return sm, nil
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return sm, nil
	}
	if k.String() == "esc" {
		return nil, nil
	}
	if !sm.form.key(k) {
		return sm, nil
	}
	places, err := splitPlaces(sm.form.value(fieldPlaces))
	if err != nil { // unreachable: Places is required
		sm.form.focused().err = err.Error()
		return sm, nil
	}
	if err := m.store.SetSearchArea(places, sm.form.value(fieldRemote)); err != nil {
		m.err = err
		return nil, nil
	}
	m.err = nil
	*m = m.loadSettings()
	return nil, nil
}
