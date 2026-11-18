package settings

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/forms"
)

// Pop-up hints, worded as every other pop-up words them.
const (
	hintClose  = "close"
	hintReview = "review"
)

type popupKind int

const (
	popupForm popupKind = iota
	popupRenameConfirm
	popupRenameReport
	popupBlocked
)

// Popup is a Settings pop-up: a form (the Search area, a Rule, a Gate, a Source, a Track's
// name), a Track rename's review and report, or the refusal to remove a Track in use.
type Popup struct {
	// Form is the pop-up's form, drawn by the application as every form is; nil for the
	// rename review and the refusal, which View draws.
	Form *forms.Form
	// submit runs once the form is complete. An error goes under the focused field and
	// keeps the pop-up open; otherwise next replaces the pop-up (nil closes it).
	submit func(m Model, env Env, f *forms.Form) (next *Popup, in Intent, err error)
	kind   popupKind

	// A Track rename after the name is chosen.
	old, name string
	impact    store.RenameImpact
	result    store.RenameResult

	// The refusal to remove a Track Applications still use.
	track string
	n     int
	incl  string   // "1 Dismissed, 2 archived": the part of n the lists do not show
	names []string // the readable Applications on the Track, as "Company · Title"
}

// Keys are the pop-up's keys, for its bottom border and the key line.
func (p *Popup) Keys() [][2]string {
	switch p.kind {
	case popupRenameConfirm:
		return [][2]string{{"y enter", "rename"}, {keyNoEsc, "cancel"}}
	case popupRenameReport:
		return [][2]string{{"esc", hintClose}}
	case popupBlocked:
		if len(p.names) == 0 {
			return [][2]string{{"esc", hintClose}}
		}
		return [][2]string{{keyShowBlocking, "show them"}, {"esc", hintClose}}
	}
	return p.Form.Keys()
}

// Update handles one message. next is the pop-up that follows (nil closes it); a non-nil
// Intent is for the application to apply. A refused Save with OnForm goes back on Form.
func (p *Popup) Update(msg tea.Msg, m Model, env Env) (next *Popup, in Intent) {
	if p.kind == popupForm {
		switch p.Form.Update(msg) {
		case forms.Open:
			return p, nil
		case forms.Cancel:
			return nil, nil
		}
		next, in, err := p.submit(m, env, p.Form)
		if err != nil {
			p.Form.Focused().Err = err.Error()
			return p, nil
		}
		return next, in
	}
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return p, nil
	}
	switch p.kind {
	case popupRenameConfirm:
		switch k.String() {
		case "y", "enter":
			return p, RenameTrack{Old: p.old, New: p.name}
		case "n", "esc":
			return nil, nil
		}
	case popupRenameReport:
		if k.String() == "esc" || k.String() == "enter" {
			return nil, nil
		}
	case popupBlocked:
		switch k.String() {
		case "esc", "enter":
			return nil, nil
		case keyShowBlocking:
			if len(p.names) > 0 {
				return nil, ShowTrack{Track: p.track}
			}
		}
	}
	return p, nil
}

// Renamed takes a rename's result: the report when files were skipped, otherwise no
// pop-up and the save message.
func (p *Popup) Renamed(res store.RenameResult) (next *Popup, msg string) {
	if len(res.Skipped) > 0 {
		p.result, p.kind = res, popupRenameReport
		return p, ""
	}
	return nil, fmt.Sprintf("Renamed %s to %s (%s)", p.old, p.name, plural(res.Applications, "Application", "Applications"))
}

// formPopup is a pop-up that is only a form.
func formPopup(f *forms.Form, submit func(Model, Env, *forms.Form) (*Popup, Intent, error)) *Popup {
	return &Popup{Form: f, submit: submit}
}

// saved closes the pop-up with a save.
func saved(s Save) (*Popup, Intent, error) { return nil, s, nil }

// NewSearchAreaForm is the Search area form, opening with places and remote.
func NewSearchAreaForm(places []string, remote string) *forms.Form {
	if remote != store.RemoteAnywhere && remote != store.RemoteNo {
		remote = store.RemoteInPlaces
	}
	var choices []forms.Choice
	for _, r := range []string{store.RemoteInPlaces, store.RemoteAnywhere, store.RemoteNo} {
		choices = append(choices, forms.Choice{Value: r, Label: RemoteLabels[r]})
	}
	f := forms.New("Edit Search area",
		forms.Field{Label: fieldPlaces, Value: strings.Join(places, ", "), Placeholder: "Australia, APAC",
			Note: "comma-separated · empty keeps every location"},
		forms.Field{Label: fieldRemote, Value: remote, Choices: choices})
	f.ShowDirty = true
	f.SelectValue() // typing replaces the current Places, as on the inline rows
	return f
}

// searchAreaPopup is the Search area form. Nothing is written until it is saved.
func (m Model) searchAreaPopup() *Popup {
	wasPlaces, wasRemote := slices.Clone(m.Ws.Places), m.Ws.Remote
	return formPopup(NewSearchAreaForm(wasPlaces, wasRemote), func(_ Model, _ Env, f *forms.Form) (*Popup, Intent, error) {
		places := SplitPlaces(f.Value(fieldPlaces))
		msg := "Saved Search area"
		if len(places) == 0 {
			msg = "Cleared Search area: every location is kept"
		}
		remote := wasRemote
		if remote == "" {
			remote = store.RemoteInPlaces
		}
		return saved(Save{Write: SetSearchArea{Places: places, Remote: f.Value(fieldRemote)}, Msg: msg,
			Undo: &Undo{What: "Search area as it was", Restore: SetSearchArea{Places: wasPlaces, Remote: remote}}})
	})
}

// rulePopup edits one Rules row. Empty text clears it; a refused value shows its reason
// and nothing is saved.
func (m Model) rulePopup(r Rule) *Popup {
	f := forms.New("Edit "+r.Label, forms.Field{Label: r.Label, Value: m.ruleText(r.Key), Unit: r.Unit, Placeholder: r.Example,
		Note: "empty clears it"})
	f.ShowDirty = true
	f.SelectValue() // typing replaces the current value, as on the inline rows
	return formPopup(f, func(m Model, env Env, f *forms.Form) (*Popup, Intent, error) {
		s := m.saveRule(r, f.Value(r.Label), env)
		s.OnForm = true
		return saved(s)
	})
}

// gatePopup edits one gate. The bound its pair sets is shown beside the value, so the
// range is never a surprise; a refused value shows its reason and nothing is saved.
func (m Model) gatePopup(g Gate) *Popup {
	ws := m.Ws
	was := strconv.Itoa(GateValue(ws, g.Key))
	words := strings.Fields(g.Label)
	label := strings.ToUpper(words[len(words)-1][:1]) + words[len(words)-1][1:] // "Maximum"
	f := forms.New("Edit "+g.Label, forms.Field{Label: label, Required: true, Value: was, Unit: g.Unit,
		Note: GateRange(ws, g.Key)})
	f.SelectValue() // typing replaces the current value, as on the inline rows
	return formPopup(f, func(_ Model, _ Env, f *forms.Form) (*Popup, Intent, error) {
		text := f.Value(label)
		// SetGate stores a canonical integer, so name that rather than the typed text.
		n, _ := strconv.Atoi(strings.TrimSpace(text))
		return saved(Save{Write: SetGate{Key: g.Key, Text: text}, OnForm: true,
			Msg:  fmt.Sprintf("Saved %s: %d %s", g.Label, n, g.Unit),
			Undo: &Undo{What: fmt.Sprintf("%s back to %s %s", g.Label, was, g.Unit), Restore: SetGate{Key: g.Key, Text: was}}})
	})
}

// Source form field labels.
const (
	fieldSourceName     = "Name"
	fieldSourceType     = "Type"
	fieldSourcePlatform = "Platform"
	fieldSourceTargets  = "Targets"
	fieldSourceURL      = "URL"
	fieldSourceAccess   = "Access"
)

// sourceForm is the add and edit form of a Source, opening with src's values.
func sourceForm(title string, src store.Source) *forms.Form {
	choices := func(values ...string) []forms.Choice {
		out := make([]forms.Choice, len(values))
		for i, v := range values {
			out[i] = forms.Choice{Value: v, Label: v}
		}
		return out
	}
	return forms.New(title,
		forms.Field{Label: fieldSourceName, Required: true, Value: src.Name, Placeholder: "linkedin"},
		forms.Field{Label: fieldSourceType, Value: src.Type, Choices: choices(store.SourceTypes...)},
		forms.Field{Label: fieldSourcePlatform, Value: src.Platform, Placeholder: "greenhouse", Note: "api only: " + strings.Join(store.SourcePlatforms, ", ")},
		forms.Field{Label: fieldSourceTargets, Value: strings.Join(src.Targets, ", "), Placeholder: "cultureamp, buildkite", Note: "api only · comma-separated"},
		forms.Field{Label: fieldSourceURL, Value: src.URL, Placeholder: "https://…", Note: "the search page, for browser or manual"},
		forms.Field{Label: fieldSourceAccess, Value: src.AccessText(), Choices: choices(store.SourceAccess...)})
}

// formSource is the Source a saved sourceForm describes.
func formSource(f *forms.Form) store.Source {
	return store.Source{Name: strings.TrimSpace(f.Value(fieldSourceName)), Type: f.Value(fieldSourceType),
		Platform: strings.TrimSpace(f.Value(fieldSourcePlatform)), Targets: SplitPlaces(f.Value(fieldSourceTargets)),
		URL: strings.TrimSpace(f.Value(fieldSourceURL)), Access: f.Value(fieldSourceAccess)}
}

// addSourcePopup is the form that adds a Source. Nothing is written until it is saved.
func addSourcePopup() *Popup {
	f := sourceForm("Add a Source", store.Source{Type: store.SourceBrowser})
	return formPopup(f, func(_ Model, _ Env, f *forms.Form) (*Popup, Intent, error) {
		src := formSource(f)
		return saved(Save{Write: AddSource{Source: src}, OnForm: true, Msg: "Added Source " + src.Name,
			Undo: &Undo{What: "Source " + src.Name + " removed", Restore: RemoveSource{Name: src.Name}}})
	})
}

// editSourcePopup is enter on a Source: its form, opening with its values. Saving
// rewrites those fields in place and offers u to put them back; on or off is o's.
func editSourcePopup(old store.Source) *Popup {
	f := sourceForm("Edit Source "+old.Name, old)
	f.ShowDirty = true
	f.SelectValue() // typing replaces the name, as on every Settings edit
	return formPopup(f, func(_ Model, _ Env, f *forms.Form) (*Popup, Intent, error) {
		if !f.Dirty() {
			return nil, nil, nil
		}
		src := formSource(f)
		return saved(Save{Write: UpdateSource{Name: old.Name, Source: src}, OnForm: true, Msg: "Saved Source " + src.Name,
			Undo: &Undo{What: "Source " + old.Name + " as it was", Restore: UpdateSource{Name: src.Name, Source: old}}})
	})
}

// trackAddPopup asks for a new Track's name.
func trackAddPopup() *Popup {
	f := forms.New("Add a Track", forms.Field{Label: "Name", Required: true, Placeholder: "e.g. Data, Platform"})
	f.Submit = descAdd
	return formPopup(f, func(m Model, _ Env, f *forms.Form) (*Popup, Intent, error) {
		name := strings.TrimSpace(f.Value("Name"))
		if err := store.ValidTrackName(name, m.Ws.Tracks); err != nil {
			return nil, nil, err
		}
		was := m.Ws.Tracks
		tracks := append(slices.Clone(was), name)
		return saved(Save{Write: SetTracks{Tracks: tracks}, Msg: "Added Track " + name, place: placeAt, row: len(tracks) - 1,
			Undo: tracksUndo("Track "+name+" removed", was)})
	})
}

// renamePopup asks for a Track's new name; enter goes on to review what will change.
func renamePopup(old string) *Popup {
	f := forms.New("Rename Track "+old, forms.Field{Label: "New name", Required: true, Value: old})
	f.Submit = hintReview
	f.SelectValue() // typing replaces the old name
	return formPopup(f, func(m Model, env Env, f *forms.Form) (*Popup, Intent, error) {
		name := strings.TrimSpace(f.Value("New name"))
		if name == old {
			return nil, nil, nil
		}
		others := slices.DeleteFunc(slices.Clone(m.Ws.Tracks), func(t string) bool { return t == old })
		if err := store.ValidTrackName(name, others); err != nil {
			return nil, nil, err
		}
		impact, err := env.Store.TrackRenameImpact(old)
		if err != nil {
			return nil, Fail{Err: err}, nil
		}
		return &Popup{kind: popupRenameConfirm, old: old, name: name, impact: impact}, nil, nil
	})
}

// renameImpactLine is how many Applications a rename rewrites, with the verb agreeing:
// "1 Application changes its Track", "3 Applications change their Track".
func renameImpactLine(n int) string {
	if n == 1 {
		return "1 Application changes its Track in meta.yml, archived ones included."
	}
	return fmt.Sprintf("%d Applications change their Track in meta.yml, archived ones included.", n)
}

// blockedShown is how many blocking Applications the refusal names before "and n more".
const blockedShown = 3

// keyShowBlocking is the refusal's key that shows the lists filtered to the Track.
const keyShowBlocking = "f"

// removeBlocked is the refusal for a Track still in use, counting what the lists do not
// show so the number adds up, and naming the Applications the lists hold on it. It is a
// pop-up by the rows rather than a footer error, with the way out: f shows them all,
// filtered to the Track, so each can be moved.
func removeBlocked(track string, n int, apps []store.Application) *Popup {
	listed, dismissed, unreadable := trackCount(track, apps)
	var incl []string
	if dismissed > 0 {
		incl = append(incl, strconv.Itoa(dismissed)+" Dismissed")
	}
	if unreadable > 0 {
		incl = append(incl, strconv.Itoa(unreadable)+" unreadable")
	}
	if other := n - listed; other > 0 {
		incl = append(incl, strconv.Itoa(other)+" archived")
	}
	var names []string
	for _, a := range apps {
		if a.Track == track && a.Malformed == nil {
			names = append(names, appLabel(a))
		}
	}
	return &Popup{kind: popupBlocked, track: track, n: n, incl: strings.Join(incl, ", "), names: names}
}

// trackCount is how many of a Track's Applications the lists hold, and how many of those
// are Dismissed or unreadable, the ones easy to miss when counting the lists. The rest
// of TrackUse's count is archived.
func trackCount(track string, apps []store.Application) (listed, dismissed, unreadable int) {
	for _, a := range apps {
		if a.Track != track {
			continue
		}
		listed++
		switch {
		case a.Malformed != nil:
			unreadable++
		case a.Status == store.StatusDismissed:
			dismissed++
		}
	}
	return listed, dismissed, unreadable
}

// appLabel names an Application in a list of them: its company and title.
func appLabel(a store.Application) string {
	switch {
	case a.Company == "":
		return a.Title
	case a.Title == "":
		return a.Company
	}
	return a.Company + " · " + a.Title
}

// View draws the rename review and report and the refusal; a form pop-up is drawn by
// the application.
func (p *Popup) View(look Look) string {
	pl := look.Palette
	inner := look.PopWidth - look.PopInset
	para := func(s string) string {
		return lipgloss.NewStyle().Foreground(pl.Fg).Width(inner).Render(s)
	}
	switch p.kind {
	case popupRenameConfirm:
		roles := "roles.json has no role on " + p.old + "."
		if p.impact.Roles {
			roles = "roles.json: its roles on " + p.old + " move to " + p.name + "."
		}
		body := strings.Join([]string{
			pl.Text(pl.Fg).Bold(true).Render(p.old + " → " + p.name),
			"",
			para(renameImpactLine(p.impact.Applications)),
			para(roles),
			pl.Text(pl.Dim).Width(inner).Render("Folders stay where they are, under jobs/" + p.old + "/."),
			"",
			pl.Text(pl.Warn).Render("This can't be undone with u."),
		}, "\n")
		return look.Popup("Rename Track "+p.old, true, body, pl.Keys(p.Keys()))
	case popupRenameReport:
		lines := []string{fmt.Sprintf("Renamed %s to %s.", p.old, p.name),
			plural(len(p.result.Skipped), "malformed meta.yml", "malformed meta.yml files") + " skipped, fix by hand:"}
		for _, s := range p.result.Skipped {
			lines = append(lines, "  "+s)
		}
		return look.Popup("Rename Track "+p.old, true, para(strings.Join(lines, "\n")), pl.Keys(p.Keys()))
	case popupBlocked:
		return p.blockedView(look)
	}
	return ""
}

func (p *Popup) blockedView(look Look) string {
	pl := look.Palette
	what := fmt.Sprintf("%s is used by %s", p.track, plural(p.n, "Application", "Applications"))
	if p.incl != "" {
		what += ",\nincl. " + p.incl
	}
	inner := look.PopWidth - look.PopInset
	body := lipgloss.NewStyle().Foreground(pl.Fg).Width(inner).Render(what + ".")
	title := "Can't remove Track " + p.track
	if len(p.names) == 0 {
		next := "Move its Applications to another Track first: e on each, then change its Track."
		body += "\n\n" + pl.Text(pl.Dim).Width(inner).Render(next)
		return look.Popup(title, true, body, pl.Keys(p.Keys()))
	}
	// The way out comes before the names, so a short terminal clips the names first.
	next := "Move its Applications to another Track first: f shows them, then e on each to change its Track."
	body += "\n\n" + pl.Text(pl.Dim).Width(inner).Render(next) + "\n"
	for _, name := range p.names[:min(blockedShown, len(p.names))] {
		body += "\n" + pl.Text(pl.Sub).Render(xansi.Truncate("· "+name, inner, "…"))
	}
	if more := len(p.names) - blockedShown; more > 0 {
		body += "\n" + pl.Text(pl.Dim).Render(fmt.Sprintf("  and %d more", more))
	}
	return look.Popup(title, true, body, pl.Keys(p.Keys()))
}
