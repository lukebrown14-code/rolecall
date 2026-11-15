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
)

// setupContactFields are the fields the form asks for, in order.
var setupContactFields = []string{fieldName, fieldEmail, fieldPhone, fieldLocation, fieldLinkedIn}

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

func newContactForm(contact map[string]string) *form {
	placeholders := map[string]string{
		fieldName: "Alex Quinn", fieldEmail: "alex@example.com", fieldPhone: "+61 400 000 000",
		fieldLocation: "Brisbane, QLD", fieldLinkedIn: "linkedin.com/in/alexquinn",
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
		default:
			f.Note = "optional"
		}
		fields = append(fields, f)
	}
	return newFormOf("Welcome · your Contact", fields...)
}

// contactPopup is the first-run Contact form. Saving writes only the Contact table of
// profile/master-profile.md.
func contactPopup(contact map[string]string) mode {
	return &formPopup{form: newContactForm(contact), width: popSmall, save: func(m *model, f *form) (mode, error) {
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
