package ui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// addValues holds the Add form's answers. It is a pointer in the model so the Huh fields keep
// writing to the same place as the model is copied between updates.
type addValues struct{ title, company, track, url, closes, fit, resumeOnly string }

func required(name string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
}

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
	m.addValues = v
	fields := []huh.Field{
		huh.NewInput().Title("Title*").Value(&v.title).Validate(required("Title")),
		huh.NewInput().Title("Company*").Value(&v.company).Validate(required("Company")),
		huh.NewSelect[string]().Title("Track*").Options(huh.NewOptions(tracks...)...).Value(&v.track),
		huh.NewInput().Title("URL").Value(&v.url),
		huh.NewInput().Title("Closes (YYYY-MM-DD)").Value(&v.closes).Validate(validCloses),
		huh.NewSelect[string]().Title("Fit").Options(
			huh.NewOption("—", ""), huh.NewOption("Strong", "Strong"), huh.NewOption("Good", "Good"),
			huh.NewOption("Marginal", "Marginal"), huh.NewOption("Poor", "Poor"),
		).Value(&v.fit),
	}
	if withResumeOnly {
		fields = append(fields, huh.NewSelect[string]().Title("Resume only").Options(
			huh.NewOption("No", "false"), huh.NewOption("Yes", "true"),
		).Value(&v.resumeOnly))
	}
	m.addForm = huh.NewForm(huh.NewGroup(fields...)).WithShowHelp(false)
	return m.addForm.Init()
}

// updateAdd routes a message to the open Add form. Nothing is written until the form completes.
func (m model) updateAdd(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "ctrl+c":
			return m, m.quit()
		case "esc":
			m.addForm, m.editOf = nil, nil
			return m, nil
		}
	}
	updated, cmd := m.addForm.Update(msg)
	m.addForm = updated.(*huh.Form)
	switch m.addForm.State {
	case huh.StateAborted:
		m.addForm, m.editOf = nil, nil
	case huh.StateCompleted:
		m.addForm = nil
		v := m.addValues
		if m.editOf != nil {
			return m.saveEdit(*m.editOf, *v)
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
	return m, cmd
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

func (m model) addView() string {
	switch {
	case m.addForm != nil:
		return m.addForm.View() + "\n"
	case m.duplicate != nil:
		d := m.duplicate
		return fmt.Sprintf("Duplicate URL: %s · %s is already saved (%s). o open that one · s save anyway · Esc cancel\n", d.Company, d.Title, d.Status)
	}
	return ""
}
