package ui

import (
	"errors"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
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
	f := newForm(&addValues{track: m.tracks[0]}, m.tracks, false)
	if m.plugin != "" {
		// With the Plugin, enter on a URL fetches the ad and fills the fields below it.
		for i := range f.Fields {
			if f.Fields[i].Label == fieldURL {
				f.Fields[i].Note = "optional · enter fetches the ad"
			}
		}
	}
	m.overlay = &formMode{form: f}
	return nil
}

// newForm builds the fields shared by Add and Edit. Add puts URL first (v0.2 URL
// fetch); Edit keeps the v0.1 order and adds Resume only.
func newForm(v *addValues, tracks []string, withResumeOnly bool) *form {
	trackChoices := make([]choice, len(tracks))
	for i, t := range tracks {
		trackChoices[i] = choice{Value: t, Label: t}
	}
	url := formField{Label: fieldURL, Value: v.url, Placeholder: "https://…", Note: "optional"}
	titleField := formField{Label: "Title", Required: true, Value: v.title}
	company := formField{Label: "Company", Required: true, Value: v.company}
	track := formField{Label: "Track", Value: v.track, Choices: trackChoices}
	closes := formField{Label: "Closes", Value: v.closes, Placeholder: "YYYY-MM-DD", Validate: validCloses}
	fit := formField{Label: "Fit", Value: v.fit, Choices: []choice{{Value: "", Label: "—"}, {Value: store.FitStrong, Label: store.FitStrong}, {Value: store.FitGood, Label: store.FitGood}, {Value: store.FitMarginal, Label: store.FitMarginal}, {Value: store.FitPoor, Label: store.FitPoor}}}
	fields := []formField{url, titleField, company, track, closes, fit}
	if !withResumeOnly {
		return newFormOf("Add a Lead", fields...)
	}
	f := newFormOf("Edit "+v.title, titleField, company, track, url, closes, fit,
		formField{Label: "Resume only", Value: v.resumeOnly, Choices: []choice{{Value: "false", Label: "No"}, {Value: "true", Label: "Yes"}}})
	f.ShowDirty = true
	return f
}

// addValues reads the form's answers.
func readAddValues(f *form) addValues {
	return addValues{title: f.Value("Title"), company: f.Value("Company"), track: f.Value("Track"), url: f.Value(fieldURL),
		closes: f.Value("Closes"), fit: f.Value("Fit"), resumeOnly: f.Value("Resume only")}
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
	m.toast = "Added Lead · " + fields.Title
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
		for i, d := range screenDefs {
			if d.name == screen {
				m.screen = i
			}
		}
		for i, t := range m.views() {
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
