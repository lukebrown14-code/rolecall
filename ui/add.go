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

// fieldURL is the Add/Edit form's URL field label.
const fieldURL = "URL"

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

// openForm shows the fields shared by Add and Edit. Add puts URL first (v0.2 URL
// fetch); Edit keeps the v0.1 order and adds Resume only.
func (m *model) openForm(v *addValues, tracks []string, withResumeOnly bool) tea.Cmd {
	trackChoices := make([]choice, len(tracks))
	for i, t := range tracks {
		trackChoices[i] = choice{t, t}
	}
	url := formField{label: fieldURL, value: v.url, placeholder: "https://…"}
	fields := []formField{
		url,
		{label: "Title", required: true, value: v.title},
		{label: "Company", required: true, value: v.company},
		{label: "Track", value: v.track, choices: trackChoices},
		{label: "Closes", value: v.closes, placeholder: "YYYY-MM-DD", validate: validCloses},
		{label: "Fit", value: v.fit, choices: []choice{{"", "—"}, {store.FitStrong, store.FitStrong}, {store.FitGood, store.FitGood}, {store.FitMarginal, store.FitMarginal}, {store.FitPoor, store.FitPoor}}},
	}
	title := "Add a Lead"
	if withResumeOnly {
		fields = []formField{fields[1], fields[2], fields[3], fields[0], fields[4], fields[5],
			{label: "Resume only", value: v.resumeOnly, choices: []choice{{"false", "No"}, {"true", "Yes"}}}}
		title = "Edit · " + v.title
	}
	m.form = &form{title: title, fields: fields}
	return nil
}

// updateAdd routes a message to the open Add/Edit form. Nothing is written until the form completes.
func (m model) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	if done, ok := msg.(fetchFinished); ok {
		return m.finishFetch(done)
	}
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
	if m.canFetch() && k.String() == "enter" {
		return m.startFetch()
	}
	if !m.form.key(k) {
		return m, nil
	}
	f := m.form
	m.form = nil
	v := f.addValues()
	if m.editOf != nil {
		return m.saveEdit(*m.editOf, v)
	}
	fields := createFields(v)
	if fields.URL != "" {
		if matches, err := m.store.FindByURL(fields.URL); err != nil {
			m.err = err
			return m, nil
		} else if len(matches) > 0 {
			m.duplicate, m.duplicateFields = &matches[0], fields
			return m, nil
		}
	}
	jd := ""
	if f.fetched {
		jd = f.jd
	}
	return m.createLead(fields, jd)
}

// addValues reads the form's answers.
func (f *form) addValues() addValues {
	return addValues{title: f.value("Title"), company: f.value("Company"), track: f.value("Track"), url: f.value(fieldURL),
		closes: f.value("Closes"), fit: f.value("Fit"), resumeOnly: f.value("Resume only")}
}

// createFields turns the form's answers into the store's create fields.
func createFields(v addValues) store.CreateFields {
	return store.CreateFields{Title: strings.TrimSpace(v.title), Company: strings.TrimSpace(v.company), Track: v.track, URL: strings.TrimSpace(v.url), Closes: v.closes, Fit: v.fit}
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
		return m.createLead(fields, "")
	case "esc":
		m.duplicate = nil
	}
	return m, nil
}

// createLead saves the Lead through the store. Without fetched text it opens jd.md for
// pasting the description; with it, the fetched ad is written straight to jd.md and no
// editor is opened.
func (m model) createLead(fields store.CreateFields, jd string) (tea.Model, tea.Cmd) {
	folder, err := m.store.Create(fields, m.now())
	if err != nil {
		m.err = err
		return m, nil
	}
	if !m.reload("") {
		return m, nil
	}
	if jd != "" {
		if err := m.store.WriteDocument(folder, "jd.md", []byte(jd)); err != nil {
			m.err = err
			return m, nil
		}
		m.selectFolder(folder)
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
