package store

import (
	"os"
	"path/filepath"
)

// EnsureNotes creates an empty notes file without changing existing notes.
func (s *Store) EnsureNotes(folder string) error {
	if _, err := s.Rescan(folder); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(folder, "notes.md"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return f.Close()
}
