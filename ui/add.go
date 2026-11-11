package ui

import (
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// addValues holds the Add/Edit form's answers.
type addValues struct{ title, company, track, url, closes, fit, resumeOnly string }

func validCloses(s string) error {
	if s == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return errors.New("Closes must be YYYY-MM-DD")
	}
	return nil
}

// openAdd starts the Add form, or reports why it cannot.
func (m *model) openAdd() tea.Cmd {
	if len(m.tracks) == 0 {
		m.err = errors.New("Add needs at least one track in workspace.json")
		return nil
	}
	m.overlay = &formMode{form: newForm(&addValues{track: m.tracks[0]}, m.tracks, false)}
	return nil
}

// newForm builds the fields shared by Add and Edit. Edit adds Resume only.
func newForm(v *addValues, tracks []string, withResumeOnly bool) *form {
	trackChoices := make([]choice, len(tracks))
	for i, t := range tracks {
		trackChoices[i] = choice{t, t}
	}
	fields := []formField{
		{label: "Title", required: true, value: v.title},
		{label: "Company", required: true, value: v.company},
		{label: "Track", value: v.track, choices: trackChoices},
		{label: "URL", value: v.url, placeholder: "https://…"},
		{label: "Closes", value: v.closes, placeholder: "YYYY-MM-DD", validate: validCloses},
		{label: "Fit", value: v.fit, choices: []choice{{"", "—"}, {store.FitStrong, store.FitStrong}, {store.FitGood, store.FitGood}, {store.FitMarginal, store.FitMarginal}, {store.FitPoor, store.FitPoor}}},
	}
	title := "Add a Lead"
	if withResumeOnly {
		fields = append(fields, formField{label: "Resume only", value: v.resumeOnly, choices: []choice{{"false", "No"}, {"true", "Yes"}}})
		title = "Edit · " + v.title
	}
	return &form{title: title, fields: fields}
}

// createLead saves the Lead through the store, then opens its jd.md for pasting the description.
func (m *model) createLead(fields store.CreateFields) tea.Cmd {
	folder, err := m.store.Create(fields, m.now())
	if err != nil {
		m.err = err
		return nil
	}
	if !m.reload("") {
		return nil
	}
	m.selectAfter = folder
	return m.editFile(folder, "jd.md")
}

// selectFolder moves the cursor to an Application's row, widening filters that would hide it.
func (m *model) selectFolder(folder string) {
	for _, a := range m.apps {
		if a.Folder != folder {
			continue
		}
		screen, table := StatusLocation(a.Status)
		for i, s := range screens {
			if s == screen {
				m.screen = i
			}
		}
		for i, t := range tables[m.screen] {
			if t == table {
				m.table = i
			}
		}
		if !m.matches(a) {
			m.track, m.text, m.overlay = 0, "", nil
		}
		for i, r := range m.rows() {
			if r.Folder == folder {
				m.cursor = i
			}
		}
		return
	}
}
