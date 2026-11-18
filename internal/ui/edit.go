package ui

import (
	"errors"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/schedule"
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
		resumeOnly: boolText(a.ResumeOnly), ended: endedStatus(a.Status), closeReason: a.CloseReason,
	}
	if a.NextStep != nil {
		v.nextAt, v.nextWhat = a.NextStep.At, a.NextStep.What
	}
	if len(a.Contacts) > 0 {
		c := a.Contacts[0]
		v.contact, v.contactRole, v.contactReach = c.Name, c.Role, c.Reach()
	}
	f := newForm(&v, tracks, true, m.now)
	if label := editFocus(a); label != "" {
		for i := range f.Fields {
			if f.Fields[i].Label == label {
				f.Focus = i
			}
		}
		// A saved value is selected, so typing a new date replaces it.
		f.SelectValue()
	}
	m.overlay = &formMode{form: f, editOf: &a}
	return nil
}

// editFocus is the field the Edit form opens on: the one whose card row offers e. A
// Rejected or Withdrawn Application's is why it closed (CLOSED); one with a next step, or
// in the Pipeline, where setting the next step is the usual edit, opens on its date (NEXT
// STEP). Anything else opens at the top, "" here.
func editFocus(a store.Application) string {
	switch {
	case endedStatus(a.Status):
		return fieldCloseReason
	case a.Status == store.StatusApplied, a.Status == store.StatusInterviewing, a.Status == store.StatusOffer,
		a.NextStep != nil && schedule.OpenStatus(a.Status):
		return fieldNextStep
	}
	return ""
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// The Edit form's next step and first Contact fields, each under its group's heading, so
// their labels need not repeat it: "Their role" is never read as the job's.
const (
	fieldNextStep     = "When"
	fieldNextWhat     = "What"
	fieldContact      = "Name"
	fieldContactRole  = "Their role"
	fieldContactReach = "Email/link"
)

// The Edit form's groups, named as the card's sections they edit. An Application's contacts
// are People (Person here), never Contact, which is the Owner's own (GLOSSARY.md).
const (
	groupRole     = "Role"
	groupNextStep = "Next step"
	groupContact  = "Person"
	groupClosed   = "Closed"
)

// validReach accepts an email address or a web link; blank is left to editProblem. A
// value with an @ is meant as an email, so it is checked as one, in the same words as
// the Contact form's Email.
func validReach(s string) error {
	s = strings.TrimSpace(s)
	switch {
	case s == "" || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://"):
		return nil
	case strings.Contains(s, "@"):
		return validEmail(s)
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
		return fieldContact, "Add a name or an email, or clear Their role"
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
	reasonChanged := endedStatus(a.Status) && v.closeReason != a.CloseReason
	if len(changed) == 0 && !stepChanged && !contactChanged && !reasonChanged {
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
	if reasonChanged {
		// none removes the key, as skipping the question after the move does.
		if err := m.store.SetCloseReason(a.Folder, v.closeReason); err != nil {
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
