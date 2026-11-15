package store

import (
	"os"
	"path/filepath"
)

// legacyFileNames are the dashboard-era files that held Status in shared tables before
// each Application got its own folder (ADR 0002). Nothing reads them now: a Harness
// still writing one is following old instructions, and its rows never reach the TUI.
// sweep-candidates.md is not here: the current Sweep still writes it.
var legacyFileNames = []string{"TRACKER.md", "SHORTLIST.md", "DISMISSED.md", "INBOX.md"}

// LegacyFiles lists the dashboard-era files present in jobs/ or the Workspace root, as
// Workspace-relative paths, jobs/ first.
func (s *Store) LegacyFiles() []string {
	var found []string
	for _, dir := range []string{"jobs", ""} {
		for _, name := range legacyFileNames {
			rel := filepath.Join(dir, name)
			if info, err := os.Stat(filepath.Join(s.root, rel)); err == nil && info.Mode().IsRegular() {
				found = append(found, rel)
			}
		}
	}
	return found
}
