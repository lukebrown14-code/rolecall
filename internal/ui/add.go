package ui

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/forms"
)

// addValues holds the Add/Edit form's answers.
type addValues struct {
	title, company, track, url, closes, fit, resumeOnly string
	// The Edit form's next step and first Contact.
	nextAt, nextWhat, contact, contactRole, contactReach string
}

// fieldURL, fieldTitle and fieldTrack are the Add/Edit form's URL, Title and Track field labels.
const (
	fieldURL   = "URL"
	fieldTitle = "Title"
	fieldTrack = "Track"
)

// validURL accepts an empty URL or a web address, so a title typed into the URL field is
// caught when the field is left rather than saved as a link w and c would open. orig, the
// value the form opened with, is always accepted: Edit never refuses a saved URL it did
// not change.
func validURL(orig string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" || s == strings.TrimSpace(orig) {
			return nil
		}
		if u, err := url.Parse(s); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return errors.New("URL must start with https://")
		}
		return nil
	}
}

// validFirstTrack applies the Track name rules to a typed first Track; blank is left to
// the required check.
func validFirstTrack(s string) error {
	if name := strings.TrimSpace(s); name != "" {
		return store.ValidTrackList([]string{name})
	}
	return nil
}

func validCloses(s string) error {
	if s == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		if datePattern.MatchString(s) {
			// The shape is right, so saying "must be YYYY-MM-DD" would read as wrong.
			return errors.New("Closes isn't a real date (YYYY-MM-DD)")
		}
		return errors.New("Closes must be YYYY-MM-DD")
	}
	return nil
}

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// openAdd starts the Add form, or reports why it cannot. With no Tracks yet, as in a
// fresh Workspace, the Track is typed in and saving adds it to workspace.json, so the
// first Lead needs no trip to Settings. A tracks list the app refused is not written
// over: the Owner fixes it in Settings first.
func (m *model) openAdd() tea.Cmd {
	first := ""
	if len(m.tracks) > 0 {
		first = m.tracks[0]
	} else if _, err := m.store.Tracks(); err != nil {
		m.err = errors.New("Add needs a Track, and workspace.json's tracks can't be read · 6 Settings › Tracking shows why")
		return nil
	}
	f := newForm(&addValues{track: first}, m.tracks, false)
	for i := range f.Fields {
		switch {
		case m.plugin != "" && f.Fields[i].Label == fieldURL:
			// With the Plugin, enter on a URL fetches the ad and fills the fields below
			// it, so the form opens there.
			f.Fields[i].Note = "optional · enter fetches the ad"
		case m.plugin == "" && f.Fields[i].Label == fieldTitle:
			// Without it the URL is only an optional link, so the form opens on the
			// first field it needs: a title typed straight away lands in Title.
			f.Focus = i
		}
	}
	m.overlay = &formMode{form: f}
	return nil
}

// newForm builds the fields shared by Add and Edit, in one order: URL first (v0.2 URL
// fetch), so the fields sit in the same place in both. Edit adds Resume only.
func newForm(v *addValues, tracks []string, withResumeOnly bool) *form {
	trackChoices := make([]choice, len(tracks))
	for i, t := range tracks {
		trackChoices[i] = choice{Value: t, Label: t}
	}
	link := formField{Label: fieldURL, Role: forms.RoleURL, Value: v.url, Placeholder: "https://…", Note: "optional",
		Validate: validURL(v.url)}
	titleField := formField{Label: fieldTitle, Required: true, Value: v.title}
	company := formField{Label: "Company", Required: true, Value: v.company}
	track := formField{Label: fieldTrack, Value: v.track, Choices: trackChoices}
	if len(tracks) == 0 {
		// No Track to choose yet: the first one is typed here and saved with the Lead.
		track = formField{Label: fieldTrack, Required: true, Value: v.track, Placeholder: "SWE", Note: "your first Track",
			Validate: validFirstTrack}
	}
	closes := formField{Label: "Closes", Value: v.closes, Placeholder: "YYYY-MM-DD", Validate: validCloses}
	fit := formField{Label: "Fit", Value: v.fit, Choices: []choice{{Value: "", Label: fitUnassessed}, {Value: store.FitStrong, Label: store.FitStrong}, {Value: store.FitGood, Label: store.FitGood}, {Value: store.FitMarginal, Label: store.FitMarginal}, {Value: store.FitPoor, Label: store.FitPoor}}}
	fields := []formField{link, titleField, company, track, closes, fit}
	if !withResumeOnly {
		return newFormOf("Add a Lead", fields...)
	}
	f := newFormOf("Edit "+titleLabel(v.title), link, titleField, company, track, closes, fit,
		formField{Label: "Resume only", Value: v.resumeOnly, Choices: []choice{{Value: "false", Label: "No"}, {Value: "true", Label: "Yes"}}},
		formField{Label: fieldNextStep, Value: v.nextAt, Placeholder: "YYYY-MM-DD or YYYY-MM-DDTHH:MM", Validate: validNextStepAt},
		formField{Label: fieldNextWhat, Value: v.nextWhat, Placeholder: "Phone screen"},
		formField{Label: fieldContact, Value: v.contact, Placeholder: "name", Note: "optional"},
		formField{Label: fieldContactRole, Value: v.contactRole, Placeholder: "Recruiter"},
		formField{Label: fieldContactReach, Value: v.contactReach, Placeholder: "email or https:// link", Validate: validReach})
	f.ShowDirty = true
	return f
}

// addValues reads the form's answers.
func readAddValues(f *form) addValues {
	return addValues{title: f.Value(fieldTitle), company: f.Value("Company"), track: f.Value(fieldTrack), url: f.Value(fieldURL),
		closes: f.Value("Closes"), fit: f.Value("Fit"), resumeOnly: f.Value("Resume only"),
		nextAt: f.Value(fieldNextStep), nextWhat: strings.TrimSpace(f.Value(fieldNextWhat)), contact: strings.TrimSpace(f.Value(fieldContact)),
		contactRole: strings.TrimSpace(f.Value(fieldContactRole)), contactReach: strings.TrimSpace(f.Value(fieldContactReach))}
}

// createFields turns the form's answers into the store's create fields.
func createFields(v addValues) store.CreateFields {
	return store.CreateFields{Title: strings.TrimSpace(v.title), Company: strings.TrimSpace(v.company), Track: strings.TrimSpace(v.track), URL: strings.TrimSpace(v.url), Closes: v.closes, Fit: v.fit}
}

// afterCreate shows the new Lead. Without fetched text it opens jd.md for pasting the
// description; with it, the fetched ad is written straight to jd.md and no editor is
// opened. The folder exists by now, so a failure here goes to the feedback line.
func (m *model) afterCreate(folder string, fields store.CreateFields, jd string) tea.Cmd {
	if !m.reload("") {
		return nil
	}
	m.toast = "Added Lead · " + titleLabel(fields.Title)
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

// submitAdd creates the Lead from the Add form, or keeps the form open with the error
// under its focused field: nothing typed is lost to a failed save.
func (fm *formMode) submitAdd(m *model, fields store.CreateFields) (mode, tea.Cmd) {
	if len(m.tracks) == 0 {
		// The first Track goes into workspace.json before the folder is made, so the new
		// Lead is never on a Track the list does not know.
		if err := m.store.AddTrack(fields.Track); err != nil {
			for i := range fm.form.Fields {
				if fm.form.Fields[i].Label == fieldTrack {
					fm.form.Focus = i
				}
			}
			fm.form.Focused().Err = m.errorText(err)
			return fm, nil
		}
		if tracks, err := m.store.Tracks(); err == nil {
			m.tracks = tracks
		}
	}
	folder, err := m.store.Create(fields, m.now())
	if err != nil {
		fm.form.Focused().Err = m.errorText(err)
		return fm, nil
	}
	jd := ""
	if fm.form.Fetched {
		jd = fm.form.Jd
	}
	return nil, m.afterCreate(folder, fields, jd)
}

// duplicateOf picks the match the Duplicate URL pop-up names: the first one still open,
// since a closed one is the least likely reason the Owner is adding the ad again.
func duplicateOf(matches []store.Application) store.Application {
	for _, a := range matches {
		if !closedStatus(a.Status) {
			return a
		}
	}
	return matches[0]
}

// selectFolder moves the cursor to an Application's row, widening filters that would hide it.
func (m *model) selectFolder(folder string) {
	for _, a := range m.apps {
		if a.Folder != folder {
			continue
		}
		screen, table := appLocation(a)
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
