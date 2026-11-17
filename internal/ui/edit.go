package ui

import (
	"errors"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// openEdit starts the Edit form prefilled from an Application. A malformed Application has
// no fields to fill, so its meta.yml opens in the Editor at the failing line instead.
func (m *model) openEdit(a store.Application) tea.Cmd {
	if pe, ok := parseError(a.Malformed); ok {
		m.err = nil
		return m.editAt(pe)
	}
	m.err = nil
	tracks := m.tracks
	if !slices.Contains(tracks, a.Track) {
		// A Track missing from workspace.json stays selectable, so saving never rewrites it.
		tracks = append(slices.Clone(tracks), a.Track)
	}
	v := addValues{
		title: a.Title, company: a.Company, track: a.Track, url: a.URL, closes: a.Closes, fit: a.Fit,
		resumeOnly: boolText(a.ResumeOnly),
	}
	if a.NextStep != nil {
		v.nextAt, v.nextWhat = a.NextStep.At, a.NextStep.What
	}
	if len(a.Contacts) > 0 {
		c := a.Contacts[0]
		v.contact, v.contactRole, v.contactReach = c.Name, c.Role, c.Reach()
	}
	m.overlay = &formMode{form: newForm(&v, tracks, true, m.now), editOf: &a}
	return nil
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// The Edit form's next step and first Contact fields.
const (
	fieldNextStep     = "Next step"
	fieldNextWhat     = "What"
	fieldContact      = "Contact"
	fieldContactRole  = "Role"
	fieldContactReach = "Email / URL"
)

// validReach accepts an email address or a web link; blank is left to editProblem.
func validReach(s string) error {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") ||
		(strings.Contains(s, "@") && !strings.ContainsAny(s, " \t")) {
		return nil
	}
	return errors.New("Use an email address or a link starting https://")
}

// editProblem is a rule that spans fields, as the field and why: a next step needs its
// date, and a Contact its name or a way to reach it.
func editProblem(v addValues) (label, problem string) {
	switch {
	case v.nextWhat != "" && v.nextAt == "":
		return fieldNextStep, "Add a date, or clear What"
	case v.contactRole != "" && v.contact == "" && v.contactReach == "":
		return fieldContact, "Add a name or an email, or clear Role"
	}
	return "", ""
}

// refuse keeps the form open with problem under the field named label.
func (fm *formMode) refuse(label, problem string) mode {
	for i := range fm.form.Fields {
		if fm.form.Fields[i].Label == label {
			fm.form.Focus = i
		}
	}
	fm.form.Focused().Err = problem
	return fm
}

// firstContact is the Contact the Edit form's fields describe: the saved first Contact
// with the typed name and role, and the address replaced only when it was changed (so a
// Contact with both an email and a link keeps both). ok is false when the fields are blank.
func firstContact(a store.Application, v addValues) (store.Contact, bool) {
	var c store.Contact
	if len(a.Contacts) > 0 {
		c = a.Contacts[0]
	}
	c.Name, c.Role = v.contact, v.contactRole
	if v.contactReach != c.Reach() {
		c.Email, c.URL = "", ""
		if strings.HasPrefix(v.contactReach, "http") {
			c.URL = v.contactReach
		} else {
			c.Email = v.contactReach
		}
	}
	return c, c != (store.Contact{})
}

// saveEdit patches only the keys the Owner changed.
func (m *model) saveEdit(a store.Application, v addValues) tea.Cmd {
	was := map[string]string{
		"title": a.Title, "company": a.Company, "track": a.Track, "url": a.URL, "closes": a.Closes, "fit": a.Fit,
		"resume_only": boolText(a.ResumeOnly),
	}
	now := map[string]string{
		"title": strings.TrimSpace(v.title), "company": strings.TrimSpace(v.company), "track": v.track,
		"url": strings.TrimSpace(v.url), "closes": v.closes, "fit": v.fit, "resume_only": v.resumeOnly,
	}
	changed := map[string]string{}
	for k, value := range now {
		if value != was[k] {
			changed[k] = value
		}
	}
	oldStep := store.NextStep{}
	if a.NextStep != nil {
		oldStep = *a.NextStep
	}
	newStep := store.NextStep{At: v.nextAt, What: v.nextWhat}
	stepChanged := newStep != oldStep
	contact, hasContact := firstContact(a, v)
	var wasContact store.Contact
	if len(a.Contacts) > 0 {
		wasContact = a.Contacts[0]
	}
	contactChanged := contact != wasContact
	if len(changed) == 0 && !stepChanged && !contactChanged {
		m.toast = "Nothing changed · " + appName(a.Title)
		return nil
	}
	if len(changed) > 0 {
		if err := m.store.Patch(a.Folder, changed); err != nil {
			m.err = err
			return nil
		}
	}
	if stepChanged {
		var ns *store.NextStep
		if newStep.At != "" {
			ns = &newStep
		}
		if err := m.store.SetNextStep(a.Folder, ns); err != nil {
			m.err = err
			return nil
		}
	}
	if contactChanged {
		var list []store.Contact
		if hasContact {
			list = append(list, contact)
		}
		if len(a.Contacts) > 1 {
			list = append(list, a.Contacts[1:]...)
		}
		if err := m.store.SetContacts(a.Folder, list); err != nil {
			m.err = err
			return nil
		}
	}
	if m.reload(a.Folder) {
		m.toast = "Saved · " + titleLabel(now["title"])
		return m.startFlash(a.Folder)
	}
	return nil
}
