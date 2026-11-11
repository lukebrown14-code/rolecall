package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// openEdit starts the Edit form prefilled from an Application. A malformed Application is
// refused with the store's own parse error.
func (m *model) openEdit(a store.Application) tea.Cmd {
	if a.Malformed != nil {
		m.err = a.Malformed
		return nil
	}
	m.err = nil
	tracks := m.tracks
	if !slices.Contains(tracks, a.Track) {
		// A Track missing from workspace.json stays selectable, so saving never rewrites it.
		tracks = append(slices.Clone(tracks), a.Track)
	}
	m.editOf = &a
	return m.openForm(&addValues{
		title: a.Title, company: a.Company, track: a.Track, url: a.URL, closes: a.Closes, fit: a.Fit,
		resumeOnly: boolText(a.ResumeOnly),
	}, tracks, true)
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// saveEdit patches only the keys the Owner changed.
func (m model) saveEdit(a store.Application, v addValues) (tea.Model, tea.Cmd) {
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
	m.editOf = nil
	if len(changed) == 0 {
		return m, nil
	}
	if err := m.store.Patch(a.Folder, changed); err != nil {
		m.err = err
		return m, nil
	}
	apps, err := m.store.Scan()
	if err != nil {
		m.err = err
		return m, nil
	}
	m.apps, m.err = apps, nil
	for i, r := range m.rows() {
		if r.Folder == a.Folder {
			m.cursor = i
		}
	}
	return m, nil
}
