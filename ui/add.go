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
	m.overlay = &formMode{form: newForm(&addValues{track: m.tracks[0]}, m.tracks, false)}
	return nil
}

// newForm builds the fields shared by Add and Edit. Add puts URL first (v0.2 URL
// fetch); Edit keeps the v0.1 order and adds Resume only.
func newForm(v *addValues, tracks []string, withResumeOnly bool) *form {
	trackChoices := make([]choice, len(tracks))
	for i, t := range tracks {
		trackChoices[i] = choice{t, t}
	}
	url := formField{label: fieldURL, value: v.url, placeholder: "https://…"}
	titleField := formField{label: "Title", required: true, value: v.title}
	company := formField{label: "Company", required: true, value: v.company}
	track := formField{label: "Track", value: v.track, choices: trackChoices}
	closes := formField{label: "Closes", value: v.closes, placeholder: "YYYY-MM-DD", validate: validCloses}
	fit := formField{label: "Fit", value: v.fit, choices: []choice{{"", "—"}, {store.FitStrong, store.FitStrong}, {store.FitGood, store.FitGood}, {store.FitMarginal, store.FitMarginal}, {store.FitPoor, store.FitPoor}}}
	fields := []formField{url, titleField, company, track, closes, fit}
	title := "Add a Lead"
	if withResumeOnly {
		fields = []formField{titleField, company, track, url, closes, fit,
			{label: "Resume only", value: v.resumeOnly, choices: []choice{{"false", "No"}, {"true", "Yes"}}}}
		title = "Edit · " + v.title
	}
	return &form{title: title, fields: fields}
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

// createLead saves the Lead through the store. Without fetched text it opens jd.md for
// pasting the description; with it, the fetched ad is written straight to jd.md and no
// editor is opened.
func (m *model) createLead(fields store.CreateFields, jd string) tea.Cmd {
	folder, err := m.store.Create(fields, m.now())
	if err != nil {
		m.err = err
		return nil
	}
	if !m.reload("") {
		return nil
	}
	if jd != "" {
		if err := m.store.WriteDocument(folder, "jd.md", []byte(jd)); err != nil {
			m.err = err
			return nil
		}
		m.selectFolder(folder)
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
