package ui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/lukebrown14-code/rolecall/store"
)

// refresh rescans the changed folders and keeps the Owner's place: the screen, table
// and cursor Application stay put, and an open detail view re-renders in place.
func (m model) refresh(folders []string) model {
	var cursorFolder string
	if rows := m.rows(); m.cursor >= 0 && m.cursor < len(rows) {
		cursorFolder = rows[m.cursor].Folder
	}
	apps := append([]store.Application(nil), m.apps...)
	for _, folder := range folders {
		apps = m.rescanInto(apps, folder)
	}
	m.apps = apps
	for i, r := range m.rows() {
		if r.Folder == cursorFolder {
			m.cursor = i
			break
		}
	}
	// If the cursor Application left this table the index stays, clamped, so the
	// cursor lands on the nearest row.
	m.cursor = max(0, min(m.cursor, len(m.rows())-1))
	return m.syncDetail()
}

// rescanInto brings apps in line with one changed path, rescanning each folder once.
// A path that no longer holds an Application removes it; a track folder that went
// away removes everything below it.
func (m *model) rescanInto(apps []store.Application, path string) []store.Application {
	below := path + string(filepath.Separator)
	kept := make([]store.Application, 0, len(apps)+1)
	seen := false
	for _, a := range apps {
		if a.Folder != path && !strings.HasPrefix(a.Folder, below) {
			kept = append(kept, a)
			continue
		}
		seen = true
		switch now, err := m.store.Rescan(a.Folder); {
		case err != nil:
			m.err = err
			kept = append(kept, a)
		case now != nil:
			kept = append(kept, *now)
		}
	}
	if !seen {
		fresh, err := m.store.Rescan(path)
		if err != nil {
			// A loose file or half-made folder is not an Application.
			return kept
		}
		if fresh != nil {
			kept = append(kept, *fresh)
			sort.SliceStable(kept, func(i, j int) bool { return folderLess(kept[i].Folder, kept[j].Folder) })
		}
	}
	return kept
}

// folderLess orders folders as a full scan does: by track, then by name.
func folderLess(a, b string) bool {
	if da, db := filepath.Dir(a), filepath.Dir(b); da != db {
		return da < db
	}
	return filepath.Base(a) < filepath.Base(b)
}
