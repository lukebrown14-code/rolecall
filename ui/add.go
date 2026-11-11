package ui

import (
	"errors"
	"strings"
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
	m.editOf = nil
	return m.openForm(&addValues{track: m.tracks[0]}, m.tracks, false)
}

// openForm shows the fields shared by Add and Edit. Edit adds Resume only.
func (m *model) openForm(v *addValues, tracks []string, withResumeOnly bool) tea.Cmd {
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
		{label: "Fit", value: v.fit, choices: []choice{{"", "—"}, {"Strong", "Strong"}, {"Good", "Good"}, {"Marginal", "Marginal"}, {"Poor", "Poor"}}},
	}
	title := "Add a Lead"
	if withResumeOnly {
		fields = append(fields, formField{label: "Resume only", value: v.resumeOnly, choices: []choice{{"false", "No"}, {"true", "Yes"}}})
		title = "Edit · " + v.title
	}
	m.form = &form{title: title, fields: fields}
	return nil
}

// updateAdd routes a message to the open Add/Edit form. Nothing is written until the form completes.
func (m model) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	if paste, ok := msg.(tea.PasteMsg); ok {
		m.form.paste(paste.Content)
		return m, nil
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "ctrl+c":
		return m, m.quit()
	case "esc":
		m.form, m.editOf = nil, nil
		return m, nil
	}
	if !m.form.key(k) {
		return m, nil
	}
	f := m.form
	m.form = nil
	v := addValues{title: f.value("Title"), company: f.value("Company"), track: f.value("Track"), url: f.value("URL"),
		closes: f.value("Closes"), fit: f.value("Fit"), resumeOnly: f.value("Resume only")}
	if m.editOf != nil {
		return m.saveEdit(*m.editOf, v)
	}
	fields := store.CreateFields{Title: strings.TrimSpace(v.title), Company: strings.TrimSpace(v.company), Track: v.track, URL: strings.TrimSpace(v.url), Closes: v.closes, Fit: v.fit}
	if fields.URL != "" {
		if matches, err := m.store.FindByURL(fields.URL); err != nil {
			m.err = err
			return m, nil
		} else if len(matches) > 0 {
			m.duplicate, m.duplicateFields = &matches[0], fields
			return m, nil
		}
	}
	return m.createLead(fields)
}

// updateDuplicate answers the duplicate-URL warning.
func (m model) updateDuplicate(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	existing, fields := *m.duplicate, m.duplicateFields
	switch k.String() {
	case "ctrl+c":
		return m, m.quit()
	case "o", "enter":
		m.duplicate = nil
		m.selectFolder(existing.Folder)
	case "s":
		m.duplicate = nil
		return m.createLead(fields)
	case "esc":
		m.duplicate = nil
	}
	return m, nil
}

// createLead saves the Lead through the store, then opens its jd.md for pasting the description.
func (m model) createLead(fields store.CreateFields) (tea.Model, tea.Cmd) {
	folder, err := m.store.Create(fields, m.now())
	if err != nil {
		m.err = err
		return m, nil
	}
	if !m.reload("") {
		return m, nil
	}
	m.selectAfter = folder
	return m, m.editFile(folder, "jd.md")
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
			m.track, m.text, m.filtering = 0, "", false
		}
		for i, r := range m.rows() {
			if r.Folder == folder {
				m.cursor = i
			}
		}
		return
	}
}
