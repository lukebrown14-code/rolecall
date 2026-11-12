package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
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
	m.overlay = &formMode{form: newForm(&addValues{
		title: a.Title, company: a.Company, track: a.Track, url: a.URL, closes: a.Closes, fit: a.Fit,
		resumeOnly: boolText(a.ResumeOnly),
	}, tracks, true), editOf: &a}
	return nil
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
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
	if len(changed) == 0 {
		m.toast = "Nothing changed · " + a.Title
		return nil
	}
	if err := m.store.Patch(a.Folder, changed); err != nil {
		m.err = err
		return nil
	}
	if m.reload(a.Folder) {
		m.toast = "Saved · " + now["title"]
		return m.startFlash(a.Folder)
	}
	return nil
}
