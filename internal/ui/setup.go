package ui

import (
	"errors"
	"strings"
)

// First-run setup: rolecall init only creates the Workspace, so the questions it used to
// ask live here. When the Profile is missing a required Contact field, the app opens on
// this form. Esc skips it; the Profile screen keeps its warning and the form returns on
// the next launch until Name and Email are saved.

const (
	fieldName     = "Name"
	fieldEmail    = "Email"
	fieldPhone    = "Phone"
	fieldLocation = "Location"
	fieldLinkedIn = "LinkedIn"
	fieldGitHub   = "GitHub"
	fieldWork     = "Work rights"
)

// setupContactFields are the fields the form asks for, in order.
var setupContactFields = []string{fieldName, fieldEmail, fieldPhone, fieldLocation, fieldLinkedIn, fieldGitHub, fieldWork}

// WithFirstRunSetup opens the Contact form at startup when the Profile has no Name or
// Email. The app passes it; tests that do not exercise setup leave it out.
func WithFirstRunSetup() Option {
	return func(m *model) {
		p, err := m.store.Profile()
		if err != nil || len(p.Missing) == 0 {
			return
		}
		m.overlay = contactPopup(p.Contact)
	}
}

// contactIntro says, above the fields, why the Contact is asked for. The first-run form
// also says how to come back to it; from the Profile screen the Owner is already there.
// Only Name and Email are required, so the rest needs no "optional" beside each field,
// which at 80 columns wrapped onto a line of its own.
const (
	contactIntro      = "rolecall puts these on every Resume and Cover letter. Name and Email are required; the rest is optional."
	contactIntroFirst = contactIntro + " You can change them any time: 5 Profile, then c."
)

// newContactForm is the Contact form. Its placeholders say what goes in a field, in
// words no one would mistake for a value, never a plausible person's details.
func newContactForm(title string, contact map[string]string, intro string) *form {
	placeholders := map[string]string{
		fieldName: "your full name", fieldEmail: "your email address", fieldPhone: "your phone number",
		fieldLocation: "your city", fieldLinkedIn: "your LinkedIn link", fieldGitHub: "your GitHub link",
		fieldWork: "where you can work, e.g. Citizen",
	}
	var fields []formField
	for _, name := range setupContactFields {
		f := formField{Label: name, Value: contact[name], Placeholder: placeholders[name]}
		switch name {
		case fieldName:
			f.Required = true
		case fieldEmail:
			f.Required = true
			f.Validate = func(v string) error {
				if !strings.Contains(v, "@") {
					return errors.New("enter an email address")
				}
				return nil
			}
		}
		fields = append(fields, f)
	}
	f := newFormOf(title, fields...)
	f.Intro = intro
	return f
}

// contactPopup is the first-run Contact form: esc skips it until the next launch.
func contactPopup(contact map[string]string) mode {
	p := contactPopupTitled("Welcome to rolecall · first, your Contact", contact, contactIntroFirst).(*formPopup)
	p.form.Cancel = "skip"
	return p
}

// contactPopupTitled is the Contact form under title. Saving writes only the Contact table
// of profile/master-profile.md.
func contactPopupTitled(title string, contact map[string]string, intro string) mode {
	return &formPopup{form: newContactForm(title, contact, intro), width: popSmall, save: func(m *model, f *form) (mode, error) {
		values := map[string]string{}
		for _, name := range setupContactFields {
			values[name] = f.Value(name)
		}
		if err := m.store.SetContact(values); err != nil {
			return nil, err
		}
		if m.profile != nil {
			*m = m.loadProfile()
		}
		m.toast = "Saved Contact · " + profilePath
		return nil, nil
	}}
}
