package ui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// refresh rescans the changed folders and keeps the Owner's place: the screen, table
// and cursor Application stay put, and an open detail view re-renders in place.
func (m model) refresh(folders []string) model {
	cursorFolder := m.cursorFolder()
	m.apps = rescanPaths(m.apps, folders, func(folder string, old *store.Application) *store.Application {
		now, err := m.store.Rescan(folder)
		if err != nil {
			if old != nil {
				m.err = err // the row stays as it was
			}
			// A loose file or half-made folder is not an Application.
			return old
		}
		return now
	})
	m.legacy = m.store.LegacyFiles()
	m.dropBodies()
	m.keepCursor(cursorFolder)
	// If the cursor Application left this table the index stays, clamped, so the
	// cursor lands on the nearest row.
	m.cursor = max(0, min(m.cursor, len(m.rows())-1))
	m.syncMove()
	return m.syncDetail()
}

// syncMove keeps an open Move pop-up true to its Application: closed if it is gone,
// its options rebuilt if its Status changed on disk.
func (m *model) syncMove() {
	mv, ok := m.overlay.(*moveMode)
	if !ok {
		return
	}
	a, ok := m.application(mv.app.Folder)
	if !ok {
		m.overlay = nil
		return
	}
	mv.app = a
	mv.options = MoveOptions(a.Status, statusBeforeDismissed(a.StatusLog))
	mv.sel = min(mv.sel, len(mv.options)-1)
}

// rescanPaths is apps brought in line with the changed paths, in one pass however many
// there are: a path naming an Application re-reads it, a Track folder re-reads every
// Application below it (one that went away drops them all), and any other path is read as
// a new Application. Each folder is read once, however many paths name it. read reads one
// folder, old being the row it replaces (nil for a new path), and returns the row to keep
// (nil drops it). apps is not changed; the result keeps its order, with new rows put in
// place as a full scan orders them.
func rescanPaths(apps []store.Application, paths []string, read func(folder string, old *store.Application) *store.Application) []store.Application {
	at := make(map[string]int, len(apps))
	dirs := map[string]bool{} // the Track folders holding Applications
	for i, a := range apps {
		at[a.Folder] = i
		dirs[filepath.Dir(a.Folder)] = true
	}
	stale := make([]bool, len(apps))
	staleDirs := map[string]bool{}
	var fresh []string
	isNew := map[string]bool{}
	for _, path := range paths {
		if i, ok := at[path]; ok {
			stale[i] = true
			continue
		}
		below, found := path+string(filepath.Separator), false
		for dir := range dirs {
			if dir == path || strings.HasPrefix(dir, below) {
				staleDirs[dir], found = true, true
			}
		}
		if !found && !isNew[path] {
			isNew[path] = true
			fresh = append(fresh, path)
		}
	}
	out := make([]store.Application, 0, len(apps)+len(fresh))
	for i, a := range apps {
		if !stale[i] && !staleDirs[filepath.Dir(a.Folder)] {
			out = append(out, a)
			continue
		}
		if now := read(a.Folder, &a); now != nil {
			out = append(out, *now)
		}
	}
	added := false
	for _, path := range fresh {
		if a := read(path, nil); a != nil {
			out, added = append(out, *a), true
		}
	}
	if added {
		sort.SliceStable(out, func(i, j int) bool { return folderLess(out[i].Folder, out[j].Folder) })
	}
	return out
}

// folderLess orders folders as a full scan does: by track, then by name.
func folderLess(a, b string) bool {
	if da, db := filepath.Dir(a), filepath.Dir(b); da != db {
		return da < db
	}
	return filepath.Base(a) < filepath.Base(b)
}
